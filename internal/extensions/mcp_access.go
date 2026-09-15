package extensions

import (
	"context"
	"errors"
	"fmt"
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
	manifest, err := NewStore(stateDir).LoadManifest()
	if err != nil {
		return MCPAccessState{}, false, err
	}
	if manifest.MCPAccess != nil {
		state, normalizeErr := normalizeMCPAccess(*manifest.MCPAccess)
		return state, true, normalizeErr
	}
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
	state, err = normalizeMCPAccess(state)
	if err != nil {
		return MCPAccessState{}, false, err
	}
	return state, true, nil
}

func SaveMCPAccess(stateDir string, state MCPAccessState) error {
	_, err := UpdateMCPAccess(context.Background(), stateDir, func(candidate *MCPAccessState) error { *candidate = state; return nil })
	return err
}

// UpdateMCPAccess serializes the full read-modify-write operation with the
// extension manifest lock and recovery journal so batch activation can include
// access policy without exposing partial state.
func UpdateMCPAccess(ctx context.Context, stateDir string, update func(*MCPAccessState) error) (MCPAccessState, error) {
	store := NewStore(stateDir)
	tx, err := store.BeginTransaction(ctx)
	if err != nil {
		return MCPAccessState{}, err
	}
	defer tx.Rollback()
	state, _, err := LoadMCPAccess(stateDir)
	if err != nil {
		return MCPAccessState{}, err
	}
	if err := update(&state); err != nil {
		return MCPAccessState{}, err
	}
	if err := tx.StageMCPAccess(state); err != nil {
		return MCPAccessState{}, err
	}
	if err := tx.Commit(); err != nil {
		return MCPAccessState{}, err
	}
	return state, nil
}

func normalizeMCPAccess(state MCPAccessState) (MCPAccessState, error) {
	if state.Agents == nil {
		state.Agents = map[string]MCPAccessRule{}
	}
	state.DefaultServers = dedupeServers(state.DefaultServers)
	normalized := make(map[string]MCPAccessRule, len(state.Agents))
	for key, rule := range state.Agents {
		mode, err := normalizeMode(rule.Mode)
		if err != nil {
			return MCPAccessState{}, fmt.Errorf("MCP access rule for agent %q: %w", key, err)
		}
		rule.Mode = mode
		rule.Servers = dedupeServers(rule.Servers)
		normalized[strings.ToLower(strings.TrimSpace(key))] = rule
	}
	state.Agents = normalized
	return state, nil
}

func normalizeMode(mode string) (string, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	switch mode {
	case MCPAccessModeDefault, MCPAccessModeCustom, MCPAccessModeNone:
		return mode, nil
	default:
		return "", fmt.Errorf("mode %q must be default, custom, or none", mode)
	}
}

// ValidateMCPAccessRule verifies a rule before a CLI preview or update.
func ValidateMCPAccessRule(rule MCPAccessRule) error {
	mode, err := normalizeMode(rule.Mode)
	if err != nil {
		return err
	}
	if mode != MCPAccessModeCustom && len(dedupeServers(rule.Servers)) > 0 {
		return fmt.Errorf("mode %s cannot include explicit servers", mode)
	}
	return nil
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
	mode, err := normalizeMode(rule.Mode)
	if err != nil {
		return []string{}
	}
	switch mode {
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
