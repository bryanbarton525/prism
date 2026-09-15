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
