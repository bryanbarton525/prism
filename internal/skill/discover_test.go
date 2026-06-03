package skill_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bryanbarton525/prism/internal/skill"
)

func TestDiscoverAll_realSkillsDir(t *testing.T) {
	root := filepath.Join("..", "..", "skills")
	if _, err := os.Stat(root); err != nil {
		t.Skip("skills directory not found")
	}
	skills, err := skill.DiscoverAll(os.DirFS(root))
	if err != nil {
		t.Fatalf("DiscoverAll: %v", err)
	}
	if len(skills) < 4 {
		t.Fatalf("expected at least 4 skills, got %d", len(skills))
	}
}
