package goproject

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
	ToolMetadata = "goproject.metadata"
	outputLimit  = 12000
)

type Plugin struct {
	root fs.FS
}

func New(root fs.FS) *Plugin {
	return &Plugin{root: root}
}

func (p *Plugin) Name() string {
	return "goproject"
}

func (p *Plugin) Tools() []plugins.ToolSpec {
	return []plugins.ToolSpec{{
		Name:        ToolMetadata,
		Description: "Collect bounded Go project metadata from repo-local files.",
		ReadOnly:    true,
		Mode:        "read_only",
		MaxBytes:    outputLimit,
	}}
}

func (p *Plugin) Call(ctx context.Context, call plugins.ToolCall) (plugins.ToolResult, error) {
	if call.Tool != ToolMetadata {
		return plugins.ToolResult{}, fmt.Errorf("unsupported Go project tool %q", call.Tool)
	}
	var b strings.Builder
	module := ""
	if data, err := fs.ReadFile(p.root, "go.mod"); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "module ") {
				module = strings.TrimSpace(strings.TrimPrefix(line, "module "))
				break
			}
		}
		b.WriteString("go.mod\n")
		b.WriteString(trim(string(data), 3000))
		b.WriteString("\n\n")
	}
	var packages []string
	_ = fs.WalkDir(p.root, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || ctx.Err() != nil || len(packages) >= 80 {
			return err
		}
		if d.IsDir() {
			if path == ".git" || path == "vendor" {
				return fs.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			dir := "."
			if i := strings.LastIndex(path, "/"); i >= 0 {
				dir = path[:i]
			}
			if !contains(packages, dir) {
				packages = append(packages, dir)
			}
		}
		return nil
	})
	if len(packages) > 0 {
		b.WriteString("packages\n")
		for _, pkg := range packages {
			b.WriteString("- ")
			b.WriteString(pkg)
			b.WriteString("\n")
		}
	}
	content := trim(strings.TrimSpace(b.String()), outputLimit)
	pack := evidence.Pack{
		Kind:           "goproject.metadata",
		Plugin:         "goproject",
		CollectionTime: time.Now().UTC(),
		Limits:         evidence.Limits{MaxBytes: outputLimit, MaxArtifacts: 80},
		Summary:        map[string]any{"module": module, "packages": len(packages), "bounded": true},
		Artifacts:      []evidence.Artifact{{Type: "go_project_metadata", Name: "go-project", Content: content}},
	}
	return plugins.ToolResult{Label: "runtime-plugin:goproject", Content: content, EvidencePack: &pack}, nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func trim(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}
