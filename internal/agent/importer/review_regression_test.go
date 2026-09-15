package importer

import (
	"bytes"
	"testing"
)

func TestReviewImporterUsesSelectedModelAndAvoidsSkillMisclassification(t *testing.T) {
	codex := []byte("name = \"Codex\"\nmodel = \"cloud-model\"\nallowed_skills = [\"skill\"]")
	out, report, err := Translate("codex.toml", codex, Config{DefaultModel: "local-model"})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte(`model: "local-model"`)) || len(report.Findings) == 0 {
		t.Fatalf("selected model was not applied: %s %#v", out, report.Findings)
	}
	if _, _, err := Translate("SKILL.md", []byte("---\nname: skill\ndescription: not an agent\n---\nbody"), Config{DefaultModel: "local-model"}); err == nil {
		t.Fatal("Agent Skill frontmatter was treated as a Prism agent")
	}
}

func TestReviewImporterRequiresExplicitTargetForEveryFormat(t *testing.T) {
	cases := []struct {
		name, format, filename string
		source                 []byte
	}{
		{"Prism", "prism", "agent.md", []byte("---\nid: agent\nname: Agent\ndescription: d\nmodel: source-model\ncontext_budget: 100\nlatency_budget_ms: 100\nallowed_skills: []\n---\nbody")},
		{"Codex", "codex", "agent.toml", []byte("name = \"Agent\"\nmodel = \"source-model\"")},
		{"Claude", "claude", "agent.md", []byte("---\nname: Agent\nmodel: source-model\ntools: Bash\n---\nbody")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := TranslateWithFormat(tc.format, tc.filename, tc.source, Config{}); err == nil {
				t.Fatal("source model was accepted without an explicit Prism target")
			}
			out, report, err := TranslateWithFormat(tc.format, tc.filename, tc.source, Config{DefaultModel: "selected-model"})
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(out, []byte("model: selected-model")) && !bytes.Contains(out, []byte(`model: "selected-model"`)) {
				t.Fatalf("selected target missing: %s", out)
			}
			if report.SourceModel != "source-model" {
				t.Fatalf("source provenance lost: %#v", report)
			}
		})
	}
}
