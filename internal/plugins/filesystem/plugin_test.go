package filesystem

import (
	"context"
	"fmt"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/bryanbarton525/prism/internal/plugins"
)

type readDirGuardFS struct {
	fs.ReadDirFS
	forbidden string
}

func (g readDirGuardFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if name == g.forbidden {
		return nil, fmt.Errorf("walk continued into %s after reaching the file limit", name)
	}
	return g.ReadDirFS.ReadDir(name)
}

func TestSearchStopsWalkingAtFileLimit(t *testing.T) {
	files := fstest.MapFS{}
	for i := range maxFiles {
		files[fmt.Sprintf("a/%02d.go", i)] = &fstest.MapFile{Data: []byte("match")}
	}
	files["z/later.go"] = &fstest.MapFile{Data: []byte("match")}

	plugin := New(readDirGuardFS{ReadDirFS: files, forbidden: "z"})
	result, err := plugin.Call(context.Background(), plugins.ToolCall{
		Tool: ToolSearch,
		Args: map[string]string{"query": "match"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := result.EvidencePack.Summary["matches"]; got != maxFiles {
		t.Fatalf("matches = %v, want %d", got, maxFiles)
	}
}
