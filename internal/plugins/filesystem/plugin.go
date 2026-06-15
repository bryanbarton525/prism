package filesystem

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
	ToolSearch  = "filesystem.search"
	outputLimit = 16000
	maxFiles    = 60
)

type Plugin struct {
	root fs.FS
}

func New(root fs.FS) *Plugin {
	return &Plugin{root: root}
}

func (p *Plugin) Name() string {
	return "filesystem"
}

func (p *Plugin) Tools() []plugins.ToolSpec {
	return []plugins.ToolSpec{{
		Name:        ToolSearch,
		Description: "Search bounded repo-local text files without write access.",
		ReadOnly:    true,
		Mode:        "read_only",
		MaxBytes:    outputLimit,
	}}
}

func (p *Plugin) Call(ctx context.Context, call plugins.ToolCall) (plugins.ToolResult, error) {
	if call.Tool != ToolSearch {
		return plugins.ToolResult{}, fmt.Errorf("unsupported filesystem tool %q", call.Tool)
	}
	query := strings.ToLower(strings.TrimSpace(call.Args["query"]))
	var b strings.Builder
	var count int
	err := fs.WalkDir(p.root, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || ctx.Err() != nil {
			return err
		}
		if d.IsDir() {
			if path == ".git" || path == "vendor" || path == "node_modules" || strings.HasPrefix(path, ".tmp") {
				return fs.SkipDir
			}
			return nil
		}
		if count >= maxFiles || !isTextPath(path) {
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
		b.WriteString(path)
		b.WriteString("\n")
		b.WriteString(trim(text, 800))
		b.WriteString("\n\n")
		return nil
	})
	if err != nil {
		return plugins.ToolResult{}, err
	}
	content := trim(strings.TrimSpace(b.String()), outputLimit)
	pack := evidence.Pack{
		Kind:           "filesystem.search",
		Plugin:         "filesystem",
		CollectionTime: time.Now().UTC(),
		Limits:         evidence.Limits{MaxBytes: outputLimit, MaxArtifacts: maxFiles},
		Summary:        map[string]any{"matches": count, "bounded": true},
		Artifacts:      []evidence.Artifact{{Type: "filesystem_search", Name: "matches", Content: content}},
	}
	return plugins.ToolResult{Label: "runtime-plugin:filesystem", Content: content, EvidencePack: &pack}, nil
}

func isTextPath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".md", ".yaml", ".yml", ".json", ".txt", ".toml", ".mod", ".sum":
		return true
	default:
		return false
	}
}

func trim(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}
