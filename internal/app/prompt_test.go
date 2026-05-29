package app

import (
	"strings"
	"testing"

	"github.com/bryanbarton525/prism/internal/skill"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func makeFakeSkill(name, description, body string) *skill.Skill {
	return &skill.Skill{
		Name:        name,
		Description: description,
		Body:        body,
	}
}

// ---------------------------------------------------------------------------
// assemblePrompt tests
// ---------------------------------------------------------------------------

func TestAssemblePrompt_ConstitutionFirst(t *testing.T) {
	skills := map[string]*skill.Skill{
		"gh-pr-triage": makeFakeSkill(
			"gh-pr-triage",
			"Triage PRs with gh.",
			"1. Gather PR metadata.",
		),
	}
	system, user := assemblePrompt("Constitution text.", skills, []string{"gh-pr-triage"}, "My task")

	// Constitution must appear before skills.
	constIdx := strings.Index(system, "Constitution text.")
	skillsIdx := strings.Index(system, "# Attached Agent Skills")
	if constIdx < 0 {
		t.Error("system prompt missing constitution")
	}
	if skillsIdx < 0 {
		t.Error("system prompt missing skills section")
	}
	if constIdx > skillsIdx {
		t.Error("constitution must appear before skills section")
	}
	if user != "My task" {
		t.Errorf("user prompt: want 'My task', got %q", user)
	}
}

func TestAssemblePrompt_SkillIndexBeforeBody(t *testing.T) {
	skills := map[string]*skill.Skill{
		"gh-pr-triage": makeFakeSkill(
			"gh-pr-triage",
			"Triage pull requests.",
			"Step 1: collect output.",
		),
	}
	system, _ := assemblePrompt("Constitution.", skills, []string{"gh-pr-triage"}, "task")

	// Metadata summary (- **name**: description) must appear before full body.
	metaIdx := strings.Index(system, "- **gh-pr-triage**")
	bodyIdx := strings.Index(system, "Step 1: collect output.")
	if metaIdx < 0 {
		t.Error("system prompt missing metadata summary line")
	}
	if bodyIdx < 0 {
		t.Error("system prompt missing skill body")
	}
	if metaIdx > bodyIdx {
		t.Error("metadata summary must appear before full skill body (progressive disclosure)")
	}
}

func TestAssemblePrompt_MultipleSkills_Order(t *testing.T) {
	skills := map[string]*skill.Skill{
		"skill-a": makeFakeSkill("skill-a", "First skill.", "Body A."),
		"skill-b": makeFakeSkill("skill-b", "Second skill.", "Body B."),
	}
	// skillNames determines order, not map iteration.
	system, _ := assemblePrompt("C.", skills, []string{"skill-a", "skill-b"}, "t")

	idxA := strings.Index(system, "Body A.")
	idxB := strings.Index(system, "Body B.")
	if idxA < 0 || idxB < 0 {
		t.Fatal("both skill bodies must appear in system prompt")
	}
	if idxA > idxB {
		t.Error("skill-a body should appear before skill-b body (respects skillNames order)")
	}
}

func TestAssemblePrompt_NoSkills(t *testing.T) {
	system, user := assemblePrompt("Constitution only.", map[string]*skill.Skill{}, []string{}, "do this")
	if !strings.Contains(system, "Constitution only.") {
		t.Error("constitution must appear even with no skills")
	}
	if strings.Contains(system, "# Attached Agent Skills") {
		t.Error("skills section should not appear when no skills are attached")
	}
	if user != "do this" {
		t.Errorf("user prompt: got %q", user)
	}
}

func TestAssemblePrompt_ScopeReminder(t *testing.T) {
	skills := map[string]*skill.Skill{
		"s": makeFakeSkill("s", "desc", "body"),
	}
	system, _ := assemblePrompt("C.", skills, []string{"s"}, "t")
	if !strings.Contains(system, "Refuse any request") {
		t.Error("system prompt should include scope enforcement reminder")
	}
}

func TestAssemblePrompt_EmptyConstitution(t *testing.T) {
	skills := map[string]*skill.Skill{
		"s": makeFakeSkill("s", "d", "body"),
	}
	system, _ := assemblePrompt("", skills, []string{"s"}, "t")
	if !strings.Contains(system, "# Attached Agent Skills") {
		t.Error("skills section should still be present without constitution")
	}
}

// ---------------------------------------------------------------------------
// truncateToTokenBudget tests
// ---------------------------------------------------------------------------

func TestTruncateToTokenBudget_WithinBudget(t *testing.T) {
	text := strings.Repeat("a", 100)
	result := truncateToTokenBudget(text, 100)
	if result != text {
		t.Error("should not truncate text within budget")
	}
}

func TestTruncateToTokenBudget_ExceedsBudget(t *testing.T) {
	text := strings.Repeat("a", 2000)
	result := truncateToTokenBudget(text, 100) // budget=100 tokens = 400 chars
	if len(result) >= len(text) {
		t.Error("result should be shorter than input when budget exceeded")
	}
	if !strings.Contains(result, "truncated") {
		t.Error("truncated result should contain notice")
	}
}

func TestTruncateToTokenBudget_ZeroBudget(t *testing.T) {
	text := "some text"
	if truncateToTokenBudget(text, 0) != text {
		t.Error("zero budget should return text unchanged")
	}
}
