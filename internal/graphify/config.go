// Package graphify defines the persisted, workspace-bound Graphify contract.
package graphify

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	ConfigVersion  = 1
	MaxResultBytes = 32 << 10
	MaxQueryRounds = 8
)

var approvedTools = map[string]struct{}{
	"query_graph":   {},
	"get_node":      {},
	"get_neighbors": {},
	"shortest_path": {},
}

// Binding ties a Graphify index to one exact workspace and generation.
// Prism never discovers an index implicitly: setup must record this binding.
type Binding struct {
	Workspace             string `yaml:"workspace" json:"workspace"`
	IndexPath             string `yaml:"index_path" json:"index_path"`
	UpstreamVersion       string `yaml:"upstream_version" json:"upstream_version"`
	SchemaVersion         string `yaml:"schema_version" json:"schema_version"`
	GenerationFingerprint string `yaml:"generation_fingerprint" json:"generation_fingerprint"`
}

// Endpoint identifies the explicitly configured downstream MCP server that
// serves the bound Graphify index. Its server configuration remains in Prism's
// downstream-MCP state; this reference keeps Graphify access separate from
// generic per-agent MCP access policy.
type Endpoint struct {
	Server string `yaml:"server" json:"server"`
}

func (e Endpoint) Validate() error {
	if strings.TrimSpace(e.Server) == "" {
		return fmt.Errorf("Graphify endpoint server is required")
	}
	return nil
}

func (b Binding) Validate() error {
	for field, value := range map[string]string{
		"workspace":              b.Workspace,
		"index_path":             b.IndexPath,
		"upstream_version":       b.UpstreamVersion,
		"schema_version":         b.SchemaVersion,
		"generation_fingerprint": b.GenerationFingerprint,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("Graphify binding %s is required", field)
		}
	}
	if !filepath.IsAbs(b.Workspace) {
		return fmt.Errorf("Graphify binding workspace must be an absolute path")
	}
	if !filepath.IsAbs(b.IndexPath) {
		return fmt.Errorf("Graphify binding index_path must be an absolute path")
	}
	return nil
}

func (b Binding) Matches(workspace, fingerprint string) error {
	if err := b.Validate(); err != nil {
		return err
	}
	workspace, err := filepath.Abs(workspace)
	if err != nil {
		return err
	}
	if filepath.Clean(workspace) != filepath.Clean(b.Workspace) {
		return fmt.Errorf("Graphify index belongs to workspace %q, not %q", b.Workspace, workspace)
	}
	if strings.TrimSpace(fingerprint) == "" {
		return fmt.Errorf("Graphify workspace generation fingerprint is required")
	}
	if fingerprint != b.GenerationFingerprint {
		return fmt.Errorf("Graphify index is stale for workspace %q", workspace)
	}
	return nil
}

func IsApprovedTool(name string) bool {
	_, ok := approvedTools[name]
	return ok
}

func ValidateTool(name string, round int, payload []byte) error {
	if !IsApprovedTool(name) {
		return fmt.Errorf("Graphify tool %q is not approved", name)
	}
	if round < 1 || round > MaxQueryRounds {
		return fmt.Errorf("Graphify query round must be between 1 and %d", MaxQueryRounds)
	}
	if len(payload) > MaxResultBytes {
		return fmt.Errorf("Graphify tool payload exceeds %d byte limit", MaxResultBytes)
	}
	return nil
}

type Config struct {
	Version  int       `yaml:"version" json:"version"`
	Binding  *Binding  `yaml:"binding,omitempty" json:"binding,omitempty"`
	Endpoint *Endpoint `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
}

type Readiness struct {
	Ready   bool     `json:"ready"`
	Message string   `json:"message"`
	Binding *Binding `json:"binding,omitempty"`
}

// CheckReadiness is read-only. It never downloads dependencies or builds an
// index; callers must use an explicit setup flow for either action.
func CheckReadiness(cfg Config, workspace, fingerprint string) Readiness {
	if cfg.Binding == nil {
		return Readiness{Message: "Graphify is not configured"}
	}
	if err := cfg.Binding.Matches(workspace, fingerprint); err != nil {
		return Readiness{Message: err.Error(), Binding: cfg.Binding}
	}
	info, err := os.Stat(cfg.Binding.IndexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Readiness{Message: fmt.Sprintf("Graphify index %q is missing", cfg.Binding.IndexPath), Binding: cfg.Binding}
		}
		return Readiness{Message: fmt.Sprintf("inspect Graphify index: %v", err), Binding: cfg.Binding}
	}
	if !info.IsDir() {
		return Readiness{Message: fmt.Sprintf("Graphify index %q is not a directory", cfg.Binding.IndexPath), Binding: cfg.Binding}
	}
	return Readiness{Ready: true, Message: "Graphify binding is ready", Binding: cfg.Binding}
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{Version: ConfigVersion}, nil
		}
		return Config{}, err
	}
	var cfg Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode Graphify configuration: %w", err)
	}
	if cfg.Version != ConfigVersion {
		return Config{}, fmt.Errorf("unsupported Graphify configuration version %d", cfg.Version)
	}
	if cfg.Binding != nil {
		if err := cfg.Binding.Validate(); err != nil {
			return Config{}, err
		}
	}
	if cfg.Endpoint != nil {
		if err := cfg.Endpoint.Validate(); err != nil {
			return Config{}, err
		}
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	if cfg.Version == 0 {
		cfg.Version = ConfigVersion
	}
	if cfg.Version != ConfigVersion {
		return fmt.Errorf("unsupported Graphify configuration version %d", cfg.Version)
	}
	if cfg.Binding != nil {
		if err := cfg.Binding.Validate(); err != nil {
			return err
		}
	}
	if cfg.Endpoint != nil {
		if err := cfg.Endpoint.Validate(); err != nil {
			return err
		}
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
