package importconfig

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

const Version = 1

type Config struct {
	Version int               `yaml:"version" json:"version"`
	Map     map[string]string `yaml:"map,omitempty" json:"map,omitempty"`   // legacy v1 shorthand
	Omit    []string          `yaml:"omit,omitempty" json:"omit,omitempty"` // legacy v1 shorthand
	Agents  []AgentDecision   `yaml:"agents,omitempty" json:"agents,omitempty"`
}

type AgentDecision struct {
	ID            string          `yaml:"id" json:"id"`
	SourceDigest  string          `yaml:"source_digest" json:"source_digest"`
	Adapter       string          `yaml:"adapter" json:"adapter"`
	AdapterDigest string          `yaml:"adapter_digest,omitempty" json:"adapter_digest,omitempty"`
	Model         string          `yaml:"model" json:"model"`
	ContextBudget int             `yaml:"context_budget,omitempty" json:"context_budget,omitempty"`
	LatencyMS     int             `yaml:"latency_budget_ms,omitempty" json:"latency_budget_ms,omitempty"`
	Decisions     []FieldDecision `yaml:"decisions,omitempty" json:"decisions,omitempty"`
}

type FieldDecision struct {
	Field  string `yaml:"field" json:"field"`
	Action string `yaml:"action" json:"action"` // map | omit
	Target string `yaml:"target,omitempty" json:"target,omitempty"`
	Value  string `yaml:"value,omitempty" json:"value,omitempty"`
	Reason string `yaml:"reason,omitempty" json:"reason,omitempty"`
}

func Parse(data []byte) (Config, error) {
	var cfg Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, err
	}
	if cfg.Version != Version {
		return Config{}, fmt.Errorf("unsupported import config version %d (want %d)", cfg.Version, Version)
	}
	for i, agent := range cfg.Agents {
		if strings.TrimSpace(agent.ID) == "" || strings.TrimSpace(agent.SourceDigest) == "" || strings.TrimSpace(agent.Adapter) == "" {
			return Config{}, fmt.Errorf("agents[%d] requires id, source_digest, and adapter", i)
		}
		for j, decision := range agent.Decisions {
			switch decision.Action {
			case "map":
				if decision.Field == "" || decision.Target == "" || decision.Value == "" {
					return Config{}, fmt.Errorf("agents[%d].decisions[%d] map requires field, target, and value", i, j)
				}
			case "omit":
				if decision.Field == "" || decision.Reason == "" {
					return Config{}, fmt.Errorf("agents[%d].decisions[%d] omit requires field and reason", i, j)
				}
			default:
				return Config{}, fmt.Errorf("agents[%d].decisions[%d] action must be map or omit", i, j)
			}
		}
	}
	return cfg, nil
}

func (c Config) For(id, sourceDigest, adapter, adapterDigest string) (AgentDecision, error) {
	for _, candidate := range c.Agents {
		if !strings.EqualFold(candidate.ID, id) {
			continue
		}
		if !strings.EqualFold(candidate.SourceDigest, sourceDigest) {
			return AgentDecision{}, fmt.Errorf("import config for %q is stale: source digest changed", id)
		}
		if !strings.EqualFold(candidate.Adapter, adapter) {
			return AgentDecision{}, fmt.Errorf("import config for %q selects adapter %q, detected %q", id, candidate.Adapter, adapter)
		}
		if candidate.AdapterDigest != "" && !strings.EqualFold(candidate.AdapterDigest, adapterDigest) {
			return AgentDecision{}, fmt.Errorf("import config for %q is stale: adapter digest changed", id)
		}
		return candidate, nil
	}
	return AgentDecision{}, fmt.Errorf("import config has no decision for agent %q", id)
}

func Digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
