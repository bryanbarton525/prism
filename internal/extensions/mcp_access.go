package extensions

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	MCPAccessModeDefault = "default"
	MCPAccessModeCustom  = "custom"
	MCPAccessModeNone    = "none"
)

type MCPAccessRule struct {
	Mode    string   `yaml:"mode" json:"mode"`
	Servers []string `yaml:"servers,omitempty" json:"servers,omitempty"`
}

type MCPAccessState struct {
	DefaultServers []string                 `yaml:"default_servers,omitempty" json:"default_servers,omitempty"`
	Agents         map[string]MCPAccessRule `yaml:"agents,omitempty" json:"agents,omitempty"`
}

func mcpAccessPath(stateDir string) string {
	return filepath.Join(stateDir, "mcp-access.yaml")
}

func LoadMCPAccess(stateDir string) (MCPAccessState, bool, error) {
	path := mcpAccessPath(stateDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return MCPAccessState{}, false, nil
		}
		return MCPAccessState{}, false, err
	}
	var state MCPAccessState
	if err := yaml.Unmarshal(data, &state); err != nil {
		return MCPAccessState{}, false, err
	}
	if state.Agents == nil {
		state.Agents = map[string]MCPAccessRule{}
	}
	state.DefaultServers = dedupeServers(state.DefaultServers)
	for key, rule := range state.Agents {
		rule.Mode = normalizeMode(rule.Mode)
		rule.Servers = dedupeServers(rule.Servers)
		state.Agents[strings.ToLower(strings.TrimSpace(key))] = rule
	}
	return state, true, nil
}

func SaveMCPAccess(stateDir string, state MCPAccessState) error {
	if state.Agents == nil {
		state.Agents = map[string]MCPAccessRule{}
	}
	state.DefaultServers = dedupeServers(state.DefaultServers)
	for key, rule := range state.Agents {
		rule.Mode = normalizeMode(rule.Mode)
		rule.Servers = dedupeServers(rule.Servers)
		state.Agents[strings.ToLower(strings.TrimSpace(key))] = rule
	}
	data, err := yaml.Marshal(state)
	if err != nil {
		return err
	}
	return writeFileAtomically(mcpAccessPath(stateDir), data)
}

func normalizeMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	switch mode {
	case MCPAccessModeDefault, MCPAccessModeCustom, MCPAccessModeNone:
		return mode
	default:
		return MCPAccessModeDefault
	}
}

func (s MCPAccessState) AllowedServers(agentID string, all []string, hasConfig bool) []string {
	if !hasConfig {
		return dedupeServers(all)
	}
	agentID = strings.ToLower(strings.TrimSpace(agentID))
	rule, ok := s.Agents[agentID]
	if !ok {
		rule = MCPAccessRule{Mode: MCPAccessModeDefault}
	}
	switch normalizeMode(rule.Mode) {
	case MCPAccessModeNone:
		return []string{}
	case MCPAccessModeCustom:
		return intersectServers(rule.Servers, all)
	default:
		return intersectServers(s.DefaultServers, all)
	}
}

func intersectServers(selected, all []string) []string {
	set := map[string]struct{}{}
	for _, name := range all {
		set[strings.ToLower(strings.TrimSpace(name))] = struct{}{}
	}
	out := []string{}
	for _, name := range selected {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		if _, ok := set[strings.ToLower(trimmed)]; ok {
			out = append(out, trimmed)
		}
	}
	return dedupeServers(out)
}

func dedupeServers(items []string) []string {
	set := map[string]string{}
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := set[key]; !ok {
			set[key] = trimmed
		}
	}
	out := make([]string, 0, len(set))
	for _, value := range set {
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i]) < strings.ToLower(out[j]) })
	return out
}
