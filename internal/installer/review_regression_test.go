package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReviewManifestPathsStayWithinScope(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	opts := Options{Scope: Project, Root: root, Targets: []string{"copilot"}, Skills: []string{"gh-pr-triage"}, Binary: "/opt/prism"}
	plan, err := Install(opts)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := LoadManifest(plan.ManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Entries = append(manifest.Entries, Entry{Path: outside, Kind: "skill"})
	data, _ := json.Marshal(manifest)
	if err := os.WriteFile(plan.ManifestPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(opts); err == nil {
		t.Fatal("reinstall accepted outside manifest path")
	}
	if _, err := Uninstall(Project, root, false); err == nil {
		t.Fatal("uninstall accepted outside manifest path")
	}
	data, err = os.ReadFile(outside)
	if err != nil || string(data) != "keep" {
		t.Fatalf("outside path was modified: %q, %v", data, err)
	}
}

func TestReviewInstallerPreservesSafeJSONAndStageLookalikes(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, ".vscode", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(config), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte("null"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(Options{Scope: Project, Root: root, Targets: []string{"copilot"}, Skills: []string{"gh-pr-triage"}, Binary: "/opt/prism"}); err != nil {
		t.Fatalf("null JSON should be initialized safely: %v", err)
	}

	root = t.TempDir()
	config = filepath.Join(root, ".vscode", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(config), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte(`["unmanaged"]`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(Options{Scope: Project, Root: root, Targets: []string{"copilot"}, Skills: []string{"gh-pr-triage"}, Binary: "/opt/prism", Force: true}); err == nil {
		t.Fatal("non-object JSON was overwritten")
	}

	root = t.TempDir()
	lookalike := filepath.Join(root, ".claude", "skills", ".prism-stage-link-gh-pr-triage")
	if err := os.MkdirAll(filepath.Dir(lookalike), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lookalike, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(Options{Scope: Project, Root: root, Targets: []string{"claude"}, Skills: []string{"gh-pr-triage"}, Binary: "/opt/prism"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(lookalike)
	if err != nil || string(data) != "keep" {
		t.Fatalf("installer deleted deterministic stage lookalike: %q, %v", data, err)
	}
}

func TestReviewInstallerRejectsSymlinkedDestinationParents(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, ".agents")); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(Options{Scope: Project, Root: root, Targets: []string{"copilot"}, Skills: []string{"gh-pr-triage"}, Binary: "/opt/prism"}); err == nil {
		t.Fatal("installer accepted a destination parent that escapes through symlink")
	}
}
