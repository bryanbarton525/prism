package plugins

import (
	"context"
	"fmt"
	"sort"

	"github.com/bryanbarton525/prism/pkg/evidence"
)

// Plugin is a bounded runtime capability provider exposed to Prism agents.
type Plugin interface {
	Name() string
	Tools() []ToolSpec
	Call(ctx context.Context, call ToolCall) (ToolResult, error)
}

// ToolSpec describes one callable operation within a Plugin.
type ToolSpec struct {
	Name        string
	Description string
	ReadOnly    bool
	Mode        string
	MaxBytes    int
}

// ToolCall is a structured request from Prism's runner into a Plugin.
type ToolCall struct {
	Tool string
	Args map[string]string
}

// ToolResult is the evidence returned by a Plugin call.
type ToolResult struct {
	Label        string
	Content      string
	EvidencePack *evidence.Pack
}

// Registry maps agent-declared tool names to concrete runtime plugins.
type Registry struct {
	plugins map[string]Plugin
	aliases map[string]string
}

func NewRegistry(plugins ...Plugin) *Registry {
	r := &Registry{
		plugins: make(map[string]Plugin),
		aliases: make(map[string]string),
	}
	for _, plugin := range plugins {
		r.Register(plugin)
	}
	return r
}

func (r *Registry) Register(plugin Plugin) {
	if plugin == nil {
		return
	}
	r.plugins[plugin.Name()] = plugin
}

func (r *Registry) Alias(alias, target string) {
	r.aliases[alias] = target
}

func (r *Registry) Get(name string) (Plugin, bool) {
	if target, ok := r.aliases[name]; ok {
		name = target
	}
	plugin, ok := r.plugins[name]
	return plugin, ok
}

func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.plugins))
	for name := range r.plugins {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (r *Registry) MustGet(name string) (Plugin, error) {
	plugin, ok := r.Get(name)
	if !ok {
		return nil, fmt.Errorf("runtime plugin %q is not registered", name)
	}
	return plugin, nil
}
