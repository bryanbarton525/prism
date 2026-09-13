// Package graphify defines the persisted, workspace-bound Graphify contract.
package graphify

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	ConfigVersion  = 1
	MaxResultBytes = 32 << 10
	MaxQueryRounds = 8
)

type EndpointKind string

const (
	// EndpointLocal is an operator-managed executable on the local machine.
	EndpointLocal EndpointKind = "local"
	// EndpointSelfHosted is an operator-managed service or MCP bridge.
	EndpointSelfHosted EndpointKind = "self-hosted"
	// EndpointManaged is a specifically named and version-pinned managed service.
	EndpointManaged EndpointKind = "managed"
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
	Server             string       `yaml:"server" json:"server"`
	Kind               EndpointKind `yaml:"kind" json:"kind"`
	Executable         string       `yaml:"executable,omitempty" json:"executable,omitempty"`
	Environment        string       `yaml:"environment,omitempty" json:"environment,omitempty"`
	EnvironmentVersion string       `yaml:"environment_version,omitempty" json:"environment_version,omitempty"`
}

func (e Endpoint) Validate() error {
	if strings.TrimSpace(e.Server) == "" {
		return fmt.Errorf("Graphify endpoint server is required")
	}
	switch e.Kind {
	case EndpointLocal:
		if strings.TrimSpace(e.Executable) == "" {
			return fmt.Errorf("local Graphify endpoint executable is required")
		}
		if e.Environment != "" || e.EnvironmentVersion != "" {
			return fmt.Errorf("local Graphify endpoint cannot set managed environment metadata")
		}
	case EndpointSelfHosted:
		if e.Executable != "" || e.Environment != "" || e.EnvironmentVersion != "" {
			return fmt.Errorf("self-hosted Graphify endpoint cannot set local executable or managed environment metadata")
		}
	case EndpointManaged:
		if e.Executable != "" {
			return fmt.Errorf("managed Graphify endpoint cannot set a local executable")
		}
		if strings.TrimSpace(e.Environment) == "" || strings.TrimSpace(e.EnvironmentVersion) == "" {
			return fmt.Errorf("managed Graphify endpoint requires environment and environment_version pins")
		}
	default:
		return fmt.Errorf("Graphify endpoint kind must be local, self-hosted, or managed")
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
	Version          int       `yaml:"version" json:"version"`
	OperatorApproved bool      `yaml:"operator_approved" json:"operator_approved"`
	Binding          *Binding  `yaml:"binding,omitempty" json:"binding,omitempty"`
	Endpoint         *Endpoint `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
}

type Check struct {
	Name    string `json:"name"`
	Ready   bool   `json:"ready"`
	Message string `json:"message"`
}

type Readiness struct {
	Ready    bool      `json:"ready"`
	Message  string    `json:"message"`
	Binding  *Binding  `json:"binding,omitempty"`
	Endpoint *Endpoint `json:"endpoint,omitempty"`
	Checks   []Check   `json:"checks"`
}

// CheckReadiness is read-only. It never downloads dependencies or builds an
// index; callers must use an explicit setup flow for either action.
func CheckReadiness(cfg Config, workspace, fingerprint string) Readiness {
	ready := Readiness{Binding: cfg.Binding, Endpoint: cfg.Endpoint}
	add := func(name string, ok bool, message string) {
		ready.Checks = append(ready.Checks, Check{Name: name, Ready: ok, Message: message})
	}
	add("operator_approval", cfg.OperatorApproved, approvalMessage(cfg.OperatorApproved))
	add("configuration_version", cfg.Version == ConfigVersion,
		fmt.Sprintf("Graphify configuration version is %d", cfg.Version))
	if cfg.Binding == nil {
		add("binding", false, "Graphify workspace binding is not configured")
	} else if err := cfg.Binding.Matches(workspace, fingerprint); err != nil {
		add("binding", false, err.Error())
	} else {
		add("binding", true, "Graphify binding matches this workspace generation")
		if info, err := os.Stat(cfg.Binding.IndexPath); err != nil {
			if os.IsNotExist(err) {
				add("index", false, fmt.Sprintf("Graphify index %q is missing", cfg.Binding.IndexPath))
			} else {
				add("index", false, fmt.Sprintf("inspect Graphify index: %v", err))
			}
		} else if !info.IsDir() {
			add("index", false, fmt.Sprintf("Graphify index %q is not a directory", cfg.Binding.IndexPath))
		} else {
			add("index", true, "Graphify index directory is present")
		}
	}
	if cfg.Binding != nil {
		schemaReady := cfg.Binding.SchemaVersion == "v1"
		add("schema", schemaReady, schemaMessage(cfg.Binding.SchemaVersion, schemaReady))
		add("upstream_version", strings.TrimSpace(cfg.Binding.UpstreamVersion) != "",
			fmt.Sprintf("Graphify upstream version is pinned to %q", cfg.Binding.UpstreamVersion))
	}
	if cfg.Endpoint == nil {
		add("endpoint", false, "Graphify endpoint is not configured")
	} else if err := cfg.Endpoint.Validate(); err != nil {
		add("endpoint", false, err.Error())
	} else {
		add("endpoint", true, endpointMessage(*cfg.Endpoint))
		if cfg.Endpoint.Kind == EndpointLocal {
			if path, err := exec.LookPath(cfg.Endpoint.Executable); err != nil {
				add("executable", false, fmt.Sprintf("Graphify executable %q is unavailable: %v", cfg.Endpoint.Executable, err))
			} else {
				add("executable", true, fmt.Sprintf("Graphify executable %q is available at %q", cfg.Endpoint.Executable, path))
			}
		}
	}
	ready.Ready = true
	for _, check := range ready.Checks {
		if !check.Ready {
			ready.Ready = false
			ready.Message = check.Message
			break
		}
	}
	if ready.Ready {
		ready.Message = "Graphify configuration, binding, and local prerequisites are ready"
	}
	return ready
}

func approvalMessage(approved bool) string {
	if approved {
		return "Graphify setup was explicitly operator-approved"
	}
	return "Graphify setup requires explicit operator approval; run graphify setup --approve"
}

func schemaMessage(version string, ready bool) string {
	if ready {
		return `Graphify tool schema "v1" is supported`
	}
	return fmt.Sprintf("Graphify tool schema %q is unsupported; Prism requires %q", version, "v1")
}

func endpointMessage(endpoint Endpoint) string {
	switch endpoint.Kind {
	case EndpointLocal:
		return fmt.Sprintf("local user-managed endpoint %q is configured", endpoint.Server)
	case EndpointSelfHosted:
		return fmt.Sprintf("self-hosted endpoint %q is configured (not contacted by doctor)", endpoint.Server)
	case EndpointManaged:
		return fmt.Sprintf("managed endpoint %q is pinned to %s@%s (not contacted by doctor)", endpoint.Server, endpoint.Environment, endpoint.EnvironmentVersion)
	default:
		return "Graphify endpoint kind is invalid"
	}
}

// ValidateServer verifies that the separately configured downstream MCP server
// is compatible with this explicit endpoint declaration. It does not connect
// to or start the endpoint.
func (e Endpoint) ValidateServer(name, transport, command string) error {
	if err := e.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(name) != strings.TrimSpace(e.Server) {
		return fmt.Errorf("configured Graphify endpoint %q is not available", e.Server)
	}
	switch e.Kind {
	case EndpointLocal:
		if transport != "command" {
			return fmt.Errorf("local Graphify endpoint %q must use command transport", e.Server)
		}
		configured, err := exec.LookPath(command)
		if err != nil {
			return fmt.Errorf("local Graphify endpoint command %q is unavailable: %v", command, err)
		}
		expected, err := exec.LookPath(e.Executable)
		if err != nil {
			return fmt.Errorf("Graphify executable %q is unavailable: %v", e.Executable, err)
		}
		if filepath.Clean(configured) != filepath.Clean(expected) {
			return fmt.Errorf("local Graphify endpoint command %q does not match approved executable %q", command, e.Executable)
		}
	case EndpointManaged:
		if transport != "sse" && transport != "streamable-http" {
			return fmt.Errorf("managed Graphify endpoint %q must use SSE or streamable HTTP transport", e.Server)
		}
	}
	return nil
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
