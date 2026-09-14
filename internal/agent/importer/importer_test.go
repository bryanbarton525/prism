package importer

import (
	"bytes"
	"testing"
)

func TestTranslateNativePrismPassThrough(t *testing.T) {
	source := []byte("---\nid: a\nname: \"A\"\ndescription: \"d\"\nmodel: \"m\"\ncontext_budget: 100\nallowed_skills: [x]\n---\nbody")
	out, report, err := Translate("agent.md", source, Config{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Adapter != "prism-native" {
		t.Fatalf("adapter = %s", report.Adapter)
	}
	if !bytes.Equal(out, source) {
		t.Fatal("expected passthrough output")
	}
}

func TestTranslateCodexTOMLDeterministic(t *testing.T) {
	source := []byte(`name = "Codex Agent"
description = "Imported"
model = "llama3.2"
allowed_skills = ["gh-pr-triage","go-helper-fn"]`)
	outA, reportA, err := Translate("codex.toml", source, Config{DefaultModel: "fallback"})
	if err != nil {
		t.Fatal(err)
	}
	outB, reportB, err := Translate("codex.toml", source, Config{DefaultModel: "fallback"})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(outA, outB) || reportA.OutputDigest != reportB.OutputDigest {
		t.Fatal("translation must be deterministic")
	}
	if reportA.Adapter != "codex-toml" {
		t.Fatalf("adapter = %s", reportA.Adapter)
	}
}

func TestTranslateCodexTOMLUsesDeclaredIdentityAndInstructions(t *testing.T) {
	source := []byte("name = \"Release Triage\"\nmodel = \"local-model\"\nallowed_skills = [\n  \"gh-pr-triage\",\n]\ndeveloper_instructions = \"Review release changes.\"\n")
	out, _, err := Translate("unrelated.toml", source, Config{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte("id: release-triage")) || !bytes.Contains(out, []byte("Review release changes.")) {
		t.Fatalf("unexpected translation: %s", out)
	}
}

func TestTranslateRejectsIncompleteAgentDefinitions(t *testing.T) {
	for _, tc := range []struct {
		name     string
		filename string
		source   []byte
		config   Config
	}{
		{"unrelated TOML", "settings.toml", []byte("title = \"model notes\""), Config{}},
		{"Codex missing skills", "agent.toml", []byte("name = \"Agent\"\nmodel = \"m\""), Config{}},
		{"Claude missing model", "agent.md", []byte("Claude\nAllowed skills: s"), Config{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := Translate(tc.filename, tc.source, tc.config); err == nil {
				t.Fatal("expected incomplete definition error")
			}
		})
	}
}

func TestTranslateClaudeMarkdown(t *testing.T) {
	source := []byte(`# Claude Prompt
Role: triage
Allowed skills: gh-pr-triage, localdocs`)
	out, report, err := Translate("claude-agent.md", source, Config{DefaultModel: "llama"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Adapter != "claude-markdown" {
		t.Fatalf("adapter = %s", report.Adapter)
	}
	if !bytes.Contains(out, []byte("allowed_skills")) {
		t.Fatalf("output missing allowed skills: %s", string(out))
	}
}
