package localdocs

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/bryanbarton525/prism/internal/plugins"
	"github.com/bryanbarton525/prism/pkg/evidence"
)

const (
	ToolSearchDocs = "localdocs.search"
	outputLimit    = 16000
	maxFiles       = 40
)

type Plugin struct {
	root fs.FS
}

func New(root fs.FS) *Plugin {
	return &Plugin{root: root}
}

func (p *Plugin) Name() string {
	return "localdocs"
}

func (p *Plugin) Tools() []plugins.ToolSpec {
	return []plugins.ToolSpec{{
		Name:        ToolSearchDocs,
		Description: "Search bounded repo-local documentation files.",
		ReadOnly:    true,
		Mode:        "read_only",
		MaxBytes:    outputLimit,
	}}
}

func (p *Plugin) Call(ctx context.Context, call plugins.ToolCall) (plugins.ToolResult, error) {
	if call.Tool != ToolSearchDocs {
		return plugins.ToolResult{}, fmt.Errorf("unsupported localdocs tool %q", call.Tool)
	}
	query := strings.ToLower(strings.TrimSpace(call.Args["query"]))
	if query == "" {
		query = strings.ToLower(strings.TrimSpace(call.Args["task"]))
	}
	var b strings.Builder
	var count int
	err := fs.WalkDir(p.root, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || ctx.Err() != nil {
			return err
		}
		if d.IsDir() {
			if path == ".git" || path == "vendor" || path == "node_modules" {
				return fs.SkipDir
			}
			return nil
		}
		if count >= maxFiles || !isDocPath(path) {
			return nil
		}
		data, err := fs.ReadFile(p.root, path)
		if err != nil {
			return nil
		}
		text := string(data)
		if query != "" && !strings.Contains(strings.ToLower(text+" "+path), query) {
			return nil
		}
		count++
		b.WriteString("## ")
		b.WriteString(path)
		b.WriteString("\n")
		b.WriteString(trim(text, 1200))
		b.WriteString("\n\n")
		return nil
	})
	if err != nil {
		return plugins.ToolResult{}, err
	}
	content := trim(strings.TrimSpace(b.String()), outputLimit)
	pack := evidence.Pack{
		Kind:           "localdocs.search",
		Plugin:         "localdocs",
		CollectionTime: time.Now().UTC(),
		Limits:         evidence.Limits{MaxBytes: outputLimit, MaxArtifacts: maxFiles},
		Summary:        map[string]any{"matches": count, "bounded": true},
		Artifacts:      []evidence.Artifact{{Type: "docs_search", Name: "matches", Content: content}},
	}
	return plugins.ToolResult{Label: "runtime-plugin:localdocs", Content: content, EvidencePack: &pack}, nil
}

func isDocPath(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	ext := strings.ToLower(filepath.Ext(path))
	return strings.HasPrefix(path, "docs/") || base == "readme.md" || ext == ".md"
}

func trim(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}
