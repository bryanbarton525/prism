package fileatomic

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReplaceExistingDestination(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "staged")
	destination := filepath.Join(dir, "state")
	for path, content := range map[string]string{source: "new", destination: "old"} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := Replace(source, destination); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(destination)
	if err != nil || string(content) != "new" {
		t.Fatalf("destination content = %q, %v", content, err)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("staged source remains: %v", err)
	}
}
