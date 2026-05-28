package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---- helpers ----------------------------------------------------------------

// scaffold creates a temporary directory tree for testing.
// layout maps relative paths to file contents.
func scaffold(t *testing.T, layout map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range layout {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("scaffold mkdir %q: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("scaffold write %q: %v", full, err)
		}
	}
	return root
}

const minimalAgentFM = `---
id: test-agent
name: Test Agent
description: A test agent.
model: llama3.1:8b
context_budget: 4096
allowed_skills:
  - my-skill
latency_budget_ms: 20000
---
`

const minimalSkillFM = `---
name: my-skill
description: A test skill for unit testing.
---

# My Skill

Do things.
`

// ---- splitFrontmatter -------------------------------------------------------

func TestSplitFrontmatter_Valid(t *testing.T) {
	input := "---\nid: foo\n---\n# Body\n"
	fm, body, err := splitFrontmatter([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(fm), "id: foo") {
		t.Errorf("frontmatter missing expected content, got: %q", string(fm))
	}
	if !strings.Contains(body, "# Body") {
		t.Errorf("body missing expected content, got: %q", body)
	}
}

func TestSplitFrontmatter_NoOpenDelimiter(t *testing.T) {
	_, _, err := splitFrontmatter([]byte("# Just markdown\n"))
	if err == nil {
		t.Fatal("expected error for missing opening ---")
	}
}

func TestSplitFrontmatter_NoCloseDelimiter(t *testing.T) {
	_, _, err := splitFrontmatter([]byte("---\nid: foo\n"))
	if err == nil {
		t.Fatal("expected error for missing closing ---")
	}
}

func TestSplitFrontmatter_EmptyBody(t *testing.T) {
	input := "---\nid: foo\n---\n"
	_, body, err := splitFrontmatter([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body != "" {
		t.Errorf("expected empty body, got: %q", body)
	}
}

// ---- parseSpecFile ----------------------------------------------------------

func TestParseSpecFile_AllFields(t *testing.T) {
	raw := `---
id: github-cli
name: GitHub CLI
description: Inspect PRs with gh.
model: llama3.1:8b
context_budget: 6144
temperature: 0.1
allowed_skills:
  - gh-pr-triage
  - gh-actions-diagnostics
latency_budget_ms: 30000
tools:
  - gh
outputs: summary findings
constitution_path: constitutions/github-cli.md
---

# GitHub CLI constitution body.
`
	spec, err := parseSpecFile([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checks := []struct {
		field string
		got   interface{}
		want  interface{}
	}{
		{"ID", spec.ID, "github-cli"},
		{"Name", spec.Name, "GitHub CLI"},
		{"Model", spec.Model, "llama3.1:8b"},
		{"ContextBudget", spec.ContextBudget, 6144},
		{"LatencyBudgetMS", spec.LatencyBudgetMS, 30000},
		{"Temperature", spec.Temperature, 0.1},
		{"ConstitutionPath", spec.ConstitutionPath, "constitutions/github-cli.md"},
		{"Outputs", spec.Outputs, "summary findings"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %v, want %v", c.field, c.got, c.want)
		}
	}
	if len(spec.AllowedSkills) != 2 {
		t.Errorf("AllowedSkills: got %d, want 2", len(spec.AllowedSkills))
	}
	if len(spec.Tools) != 1 || spec.Tools[0] != "gh" {
		t.Errorf("Tools: got %v, want [gh]", spec.Tools)
	}
	if !strings.Contains(spec.Body, "GitHub CLI constitution body") {
		t.Errorf("Body not captured: %q", spec.Body)
	}
}

// ---- validateSpec -----------------------------------------------------------

func TestValidateSpec_MissingRequired(t *testing.T) {
	cases := []struct {
		name    string
		spec    Spec
		wantErr string
	}{
		{
			name:    "missing id",
			spec:    Spec{Name: "N", Description: "D", Model: "m", ContextBudget: 1, AllowedSkills: []string{"s"}, LatencyBudgetMS: 1},
			wantErr: "missing required field: id",
		},
		{
			name:    "missing name",
			spec:    Spec{ID: "x", Description: "D", Model: "m", ContextBudget: 1, AllowedSkills: []string{"s"}, LatencyBudgetMS: 1},
			wantErr: "missing required field: name",
		},
		{
			name:    "missing description",
			spec:    Spec{ID: "x", Name: "N", Model: "m", ContextBudget: 1, AllowedSkills: []string{"s"}, LatencyBudgetMS: 1},
			wantErr: "missing required field: description",
		},
		{
			name:    "missing model",
			spec:    Spec{ID: "x", Name: "N", Description: "D", ContextBudget: 1, AllowedSkills: []string{"s"}, LatencyBudgetMS: 1},
			wantErr: "missing required field: model",
		},
		{
			name:    "zero context_budget",
			spec:    Spec{ID: "x", Name: "N", Description: "D", Model: "m", AllowedSkills: []string{"s"}, LatencyBudgetMS: 1},
			wantErr: "context_budget",
		},
		{
			name:    "empty allowed_skills",
			spec:    Spec{ID: "x", Name: "N", Description: "D", Model: "m", ContextBudget: 1, LatencyBudgetMS: 1},
			wantErr: "allowed_skills",
		},
		{
			name:    "zero latency_budget_ms",
			spec:    Spec{ID: "x", Name: "N", Description: "D", Model: "m", ContextBudget: 1, AllowedSkills: []string{"s"}},
			wantErr: "latency_budget_ms",
		},
	}

	l := NewLoader(LoaderOptions{ValidateSkillRefs: false})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := l.validateSpec(&tc.spec, tc.spec.ID)
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestValidateSpec_IDMismatch(t *testing.T) {
	spec := Spec{
		ID: "foo", Name: "N", Description: "D", Model: "m",
		ContextBudget: 1, AllowedSkills: []string{"s"}, LatencyBudgetMS: 1,
	}
	l := NewLoader(LoaderOptions{ValidateSkillRefs: false})
	err := l.validateSpec(&spec, "bar")
	if err == nil {
		t.Fatal("expected id mismatch error")
	}
	if !strings.Contains(err.Error(), "does not match file stem") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---- skill reference validation ---------------------------------------------

func TestValidateSkillRefs_UnknownSkill(t *testing.T) {
	root := scaffold(t, map[string]string{
		"skills/known-skill/SKILL.md": minimalSkillFM,
	})
	// Override skill name to match the directory.
	root2 := scaffold(t, map[string]string{
		"skills/my-skill/SKILL.md": minimalSkillFM,
		"agents/test-agent.md": minimalAgentFM + "\nBody content here.",
	})
	_ = root

	l := NewLoader(LoaderOptions{
		RepoRoot:          root2,
		ValidateSkillRefs: true,
	})
	if err := l.LoadSkills(); err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}

	spec := Spec{
		ID: "x", Name: "N", Description: "D", Model: "m",
		ContextBudget: 1, LatencyBudgetMS: 1,
		AllowedSkills: []string{"my-skill", "does-not-exist"},
	}
	err := l.validateSkillRefs(&spec)
	if err == nil {
		t.Fatal("expected unknown skill error")
	}
	if !strings.Contains(err.Error(), "does-not-exist") {
		t.Errorf("error should mention the missing skill, got: %v", err)
	}
}

func TestValidateSkillRefs_AllKnown(t *testing.T) {
	root := scaffold(t, map[string]string{
		"skills/my-skill/SKILL.md": minimalSkillFM,
	})
	l := NewLoader(LoaderOptions{RepoRoot: root, ValidateSkillRefs: true})
	if err := l.LoadSkills(); err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}
	spec := Spec{AllowedSkills: []string{"my-skill"}}
	if err := l.validateSkillRefs(&spec); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---- constitution resolution ------------------------------------------------

func TestResolveConstitution_FromBody(t *testing.T) {
	spec := &Spec{ID: "a", Body: "# My constitution\nDo things.\n"}
	l := NewLoader(LoaderOptions{RepoRoot: t.TempDir()})
	if err := l.resolveConstitution(spec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Constitution != spec.Body {
		t.Errorf("expected constitution == body, got: %q", spec.Constitution)
	}
}

func TestResolveConstitution_FromConstitutionPath(t *testing.T) {
	root := scaffold(t, map[string]string{
		"constitutions/my-agent.md": "# From constitution path\n",
	})
	spec := &Spec{
		ID:               "my-agent",
		ConstitutionPath: "constitutions/my-agent.md",
		Body:             "# Body that should be ignored",
	}
	l := NewLoader(LoaderOptions{RepoRoot: root})
	if err := l.resolveConstitution(spec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(spec.Constitution, "From constitution path") {
		t.Errorf("expected constitution from path, got: %q", spec.Constitution)
	}
}

func TestResolveConstitution_FallbackToLegacy(t *testing.T) {
	root := scaffold(t, map[string]string{
		"constitutions/legacy-agent.md": "# Legacy constitution\n",
	})
	spec := &Spec{ID: "legacy-agent", Body: "   \n"}
	l := NewLoader(LoaderOptions{RepoRoot: root})
	if err := l.resolveConstitution(spec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(spec.Constitution, "Legacy constitution") {
		t.Errorf("expected legacy constitution, got: %q", spec.Constitution)
	}
}

func TestResolveConstitution_NoSourceOK(t *testing.T) {
	// No body, no constitution_path, no legacy file — should not error.
	spec := &Spec{ID: "orphan", Body: ""}
	l := NewLoader(LoaderOptions{RepoRoot: t.TempDir()})
	if err := l.resolveConstitution(spec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Constitution != "" {
		t.Errorf("expected empty constitution, got: %q", spec.Constitution)
	}
}

func TestResolveConstitution_MissingConstitutionPath(t *testing.T) {
	spec := &Spec{ID: "a", ConstitutionPath: "constitutions/missing.md"}
	l := NewLoader(LoaderOptions{RepoRoot: t.TempDir()})
	err := l.resolveConstitution(spec)
	if err == nil {
		t.Fatal("expected error for missing constitution_path target")
	}
}

// ---- LoadFile ---------------------------------------------------------------

func TestLoadFile_FullSpec(t *testing.T) {
	root := scaffold(t, map[string]string{
		"agents/my-agent.md": `---
id: my-agent
name: My Agent
description: Does things.
model: llama3.1:8b
context_budget: 4096
allowed_skills:
  - my-skill
latency_budget_ms: 20000
---

# Constitution
Do stuff.
`,
		"skills/my-skill/SKILL.md": minimalSkillFM,
	})

	l := NewLoader(LoaderOptions{RepoRoot: root, ValidateSkillRefs: true})
	if err := l.LoadSkills(); err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}

	spec, err := l.LoadFile(filepath.Join(root, "agents", "my-agent.md"))
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	if spec.ID != "my-agent" {
		t.Errorf("ID: got %q", spec.ID)
	}
	if !strings.Contains(spec.Constitution, "Do stuff") {
		t.Errorf("Constitution: got %q", spec.Constitution)
	}
}

func TestLoadFile_WithConstitutionPath(t *testing.T) {
	root := scaffold(t, map[string]string{
		"agents/my-agent.md": `---
id: my-agent
name: My Agent
description: Does things.
model: llama3.1:8b
context_budget: 4096
allowed_skills:
  - my-skill
latency_budget_ms: 20000
constitution_path: constitutions/my-agent.md
---
`,
		"constitutions/my-agent.md": "# Constitution from path\n",
		"skills/my-skill/SKILL.md":  minimalSkillFM,
	})

	l := NewLoader(LoaderOptions{RepoRoot: root, ValidateSkillRefs: true})
	_ = l.LoadSkills()

	spec, err := l.LoadFile(filepath.Join(root, "agents", "my-agent.md"))
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if !strings.Contains(spec.Constitution, "Constitution from path") {
		t.Errorf("unexpected constitution: %q", spec.Constitution)
	}
}

func TestLoadFile_InvalidSkillRef(t *testing.T) {
	root := scaffold(t, map[string]string{
		"agents/my-agent.md": `---
id: my-agent
name: My Agent
description: Does things.
model: llama3.1:8b
context_budget: 4096
allowed_skills:
  - nonexistent-skill
latency_budget_ms: 20000
---
Body.
`,
		"skills/my-skill/SKILL.md": minimalSkillFM,
	})

	l := NewLoader(LoaderOptions{RepoRoot: root, ValidateSkillRefs: true})
	if err := l.LoadSkills(); err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}

	_, err := l.LoadFile(filepath.Join(root, "agents", "my-agent.md"))
	if err == nil {
		t.Fatal("expected error for invalid skill ref")
	}
	if !strings.Contains(err.Error(), "nonexistent-skill") {
		t.Errorf("error should name the bad skill, got: %v", err)
	}
}

// ---- LoadAll / Registry -----------------------------------------------------

func TestLoadAll_SkipsREADME(t *testing.T) {
	root := scaffold(t, map[string]string{
		"agents/README.md": "# readme",
		"agents/my-agent.md": `---
id: my-agent
name: My Agent
description: D.
model: m
context_budget: 1
allowed_skills:
  - my-skill
latency_budget_ms: 1
---
Body.
`,
		"skills/my-skill/SKILL.md": minimalSkillFM,
	})

	l := NewLoader(LoaderOptions{RepoRoot: root, ValidateSkillRefs: true})
	_ = l.LoadSkills()

	specs, err := l.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(specs) != 1 {
		t.Errorf("expected 1 spec (README excluded), got %d", len(specs))
	}
	if specs[0].ID != "my-agent" {
		t.Errorf("unexpected spec ID: %q", specs[0].ID)
	}
}

func TestRegistry_GetAndList(t *testing.T) {
	specs := []*Spec{
		{ID: "zzz", Name: "Z", Description: "z", Model: "m"},
		{ID: "aaa", Name: "A", Description: "a", Model: "m"},
	}
	r := NewRegistry(specs)

	if _, ok := r.Get("zzz"); !ok {
		t.Error("Get(zzz) not found")
	}
	if _, ok := r.Get("missing"); ok {
		t.Error("Get(missing) should return false")
	}

	list := r.List()
	if len(list) != 2 {
		t.Fatalf("List len: got %d, want 2", len(list))
	}
	if list[0].ID != "aaa" || list[1].ID != "zzz" {
		t.Errorf("List not sorted by ID: %v", list)
	}
}

// ---- LoadRegistry (integration) ---------------------------------------------

func TestLoadRegistry_RealAgentFiles(t *testing.T) {
	// Verify that the actual agents/ and skills/ directories in the repo can
	// be loaded without errors.  Find the repo root relative to this test file.
	repoRoot := findRepoRoot(t)

	reg, err := LoadRegistry(LoaderOptions{
		RepoRoot:          repoRoot,
		ValidateSkillRefs: true,
	})
	if err != nil {
		t.Fatalf("LoadRegistry on real repo: %v", err)
	}

	expectedIDs := []string{"argo", "github-cli", "kubectl", "web-docs-search"}
	for _, id := range expectedIDs {
		spec, ok := reg.Get(id)
		if !ok {
			t.Errorf("expected agent %q not found in registry", id)
			continue
		}
		if spec.Constitution == "" {
			t.Errorf("agent %q has empty constitution after resolution", id)
		}
	}
}

// findRepoRoot walks up from the test file location to find the repo root
// (the directory containing go.mod).
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (no go.mod found)")
		}
		dir = parent
	}
}

// ---- skill loader -----------------------------------------------------------

func TestLoadSkillFile_Valid(t *testing.T) {
	root := scaffold(t, map[string]string{
		"skills/my-skill/SKILL.md": minimalSkillFM,
	})
	sk, err := LoadSkillFile(filepath.Join(root, "skills", "my-skill", "SKILL.md"))
	if err != nil {
		t.Fatalf("LoadSkillFile: %v", err)
	}
	if sk.Name != "my-skill" {
		t.Errorf("Name: got %q", sk.Name)
	}
	if sk.DirPath == "" {
		t.Error("DirPath not populated")
	}
}

func TestLoadSkillFile_MissingName(t *testing.T) {
	root := scaffold(t, map[string]string{
		"skills/my-skill/SKILL.md": "---\ndescription: Desc.\n---\nBody.\n",
	})
	_, err := LoadSkillFile(filepath.Join(root, "skills", "my-skill", "SKILL.md"))
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestLoadSkillFile_NameDirMismatch(t *testing.T) {
	root := scaffold(t, map[string]string{
		"skills/my-skill/SKILL.md": "---\nname: different-name\ndescription: Desc.\n---\nBody.\n",
	})
	_, err := LoadSkillFile(filepath.Join(root, "skills", "my-skill", "SKILL.md"))
	if err == nil {
		t.Fatal("expected error for name/dir mismatch")
	}
	if !strings.Contains(err.Error(), "does not match directory name") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadSkillFile_DescriptionTooLong(t *testing.T) {
	longDesc := strings.Repeat("x", 1025)
	content := "---\nname: my-skill\ndescription: " + longDesc + "\n---\nBody.\n"
	root := scaffold(t, map[string]string{
		"skills/my-skill/SKILL.md": content,
	})
	_, err := LoadSkillFile(filepath.Join(root, "skills", "my-skill", "SKILL.md"))
	if err == nil {
		t.Fatal("expected error for over-length description")
	}
}

func TestLoadSkillsFrom_RealSkillDir(t *testing.T) {
	repoRoot := findRepoRoot(t)
	skillDir := filepath.Join(repoRoot, "skills")

	reg, err := LoadSkillsFrom(skillDir)
	if err != nil {
		t.Fatalf("LoadSkillsFrom: %v", err)
	}

	names := reg.Names()
	if len(names) == 0 {
		t.Error("expected at least one skill loaded")
	}

	// Spot-check a known skill.
	if _, ok := reg.Get("gh-pr-triage"); !ok {
		t.Error("expected skill gh-pr-triage not found")
	}
}

// ---- fileStem ---------------------------------------------------------------

func TestFileStem(t *testing.T) {
	cases := []struct{ in, want string }{
		{"agents/github-cli.md", "github-cli"},
		{"/abs/path/kubectl.md", "kubectl"},
		{"argo.md", "argo"},
	}
	for _, c := range cases {
		if got := fileStem(c.in); got != c.want {
			t.Errorf("fileStem(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
