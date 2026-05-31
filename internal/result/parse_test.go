package result

import (
	"strings"
	"testing"
)

func TestParseAgentOutput_JSON(t *testing.T) {
	raw := `{"summary":"PR blocked","findings":["CI failed","review pending"],"confidence":"high"}`
	r := ParseAgentOutput(raw, 400)
	if r.Summary == "" {
		t.Fatal("expected compact summary")
	}
	if !strings.Contains(r.Summary, "PR blocked") {
		t.Errorf("summary missing headline: %q", r.Summary)
	}
	if len(r.Findings) != 2 {
		t.Errorf("findings: want 2, got %d", len(r.Findings))
	}
	if r.Confidence != ConfidenceHigh {
		t.Errorf("confidence: want high, got %q", r.Confidence)
	}
	if r.RawOutput != raw {
		t.Error("raw output not preserved")
	}
}

func TestParseAgentOutput_JSONFence(t *testing.T) {
	raw := "Here is the result:\n```json\n{\"summary\":\"done\",\"findings\":[\"a\"],\"confidence\":\"medium\"}\n```"
	r := ParseAgentOutput(raw, 200)
	if !strings.Contains(r.Summary, "done") {
		t.Errorf("expected parsed summary, got %q", r.Summary)
	}
	if r.Confidence != ConfidenceMedium {
		t.Errorf("confidence: got %q", r.Confidence)
	}
}

func TestParseAgentOutput_CodeArtifact(t *testing.T) {
	raw := `{"summary":"helper added","code":"func Foo() int { return 1 }","confidence":"high"}`
	r := ParseAgentOutput(raw, 400)
	if len(r.Artifacts) != 1 || r.Artifacts[0].Content == "" {
		t.Fatalf("expected code artifact, got %+v", r.Artifacts)
	}
	if strings.Contains(r.Summary, "func Foo") {
		t.Error("compact summary should not embed full code")
	}
}

func TestParseAgentOutput_FallbackBullets(t *testing.T) {
	raw := "Headline result\n\n- first finding\n- second finding\n\nMore prose."
	r := ParseAgentOutput(raw, 400)
	if !strings.Contains(r.Summary, "Headline") {
		t.Errorf("expected headline in compact summary: %q", r.Summary)
	}
	if len(r.Findings) < 2 {
		t.Errorf("expected bullet findings, got %d", len(r.Findings))
	}
}

func TestParseAgentOutput_TruncatesLongOutput(t *testing.T) {
	raw := strings.Repeat("word ", 200)
	r := ParseAgentOutput(raw, 80)
	if len(r.Summary) > 80 {
		t.Errorf("compact summary too long: %d", len(r.Summary))
	}
}
