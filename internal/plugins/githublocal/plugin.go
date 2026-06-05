package githublocal

import (
	"context"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/bryanbarton525/prism/internal/plugins"
	"github.com/bryanbarton525/prism/pkg/evidence"
)

const (
	ToolRepoMetadata = "github.repo_metadata"
	outputLimit      = 14000
	maxFiles         = 30
)

type Plugin struct {
	root fs.FS
}

func New(root fs.FS) *Plugin {
	return &Plugin{root: root}
}

func (p *Plugin) Name() string {
	return "github"
}

func (p *Plugin) Tools() []plugins.ToolSpec {
	return []plugins.ToolSpec{{
		Name:        ToolRepoMetadata,
		Description: "Collect bounded repo-local GitHub workflow and template metadata.",
		ReadOnly:    true,
		Mode:        "read_only",
		MaxBytes:    outputLimit,
	}}
}

func (p *Plugin) Call(ctx context.Context, call plugins.ToolCall) (plugins.ToolResult, error) {
	if call.Tool != ToolRepoMetadata {
		return plugins.ToolResult{}, fmt.Errorf("unsupported GitHub tool %q", call.Tool)
	}
	var b strings.Builder
	var count int
	err := fs.WalkDir(p.root, ".github", func(path string, d fs.DirEntry, err error) error {
		if err != nil || ctx.Err() != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if count >= maxFiles || !isInteresting(path) {
			return nil
		}
		data, err := fs.ReadFile(p.root, path)
		if err != nil {
			return nil
		}
		count++
		b.WriteString("## ")
		b.WriteString(path)
		b.WriteString("\n")
		b.WriteString(trim(string(data), 1000))
		b.WriteString("\n\n")
		return nil
	})
	if err != nil {
		return plugins.ToolResult{}, err
	}
	content := trim(strings.TrimSpace(b.String()), outputLimit)
	pack := evidence.Pack{
		Kind:           "github.repo_metadata",
		Plugin:         "github",
		CollectionTime: time.Now().UTC(),
		Limits:         evidence.Limits{MaxBytes: outputLimit, MaxArtifacts: maxFiles},
		Summary:        map[string]any{"files": count, "bounded": true, "live_api": false},
		Artifacts:      []evidence.Artifact{{Type: "github_repo_metadata", Name: ".github", Content: content}},
	}
	return plugins.ToolResult{Label: "runtime-plugin:github", Content: content, EvidencePack: &pack}, nil
}

func isInteresting(path string) bool {
	return strings.HasSuffix(path, ".yml") ||
		strings.HasSuffix(path, ".yaml") ||
		strings.HasSuffix(path, ".md") ||
		strings.HasSuffix(path, ".json")
}

func trim(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}
