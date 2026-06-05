// Package policy defines stable Prism governance contracts.
package policy

import "time"

const (
	DecisionAllow           = "allow"
	DecisionDeny            = "deny"
	DecisionWarn            = "warn"
	DecisionRequireApproval = "require_approval"
)

// Policy is the YAML-backed contract used to govern Prism execution.
type Policy struct {
	Version int `json:"version" yaml:"version"`

	Defaults   Defaults             `json:"defaults" yaml:"defaults"`
	Agents     map[string]Agent     `json:"agents" yaml:"agents"`
	Sources    map[string]Source    `json:"sources" yaml:"sources"`
	Workspaces map[string]Workspace `json:"workspaces,omitempty" yaml:"workspaces,omitempty"`
	Bundles    map[string]Bundle    `json:"bundles,omitempty" yaml:"bundles,omitempty"`

	WriteActions WriteActions `json:"write_actions" yaml:"write_actions"`
}

type Defaults struct {
	Mode                string `json:"mode,omitempty" yaml:"mode,omitempty"`
	RawPromptCapture    bool   `json:"raw_prompt_capture" yaml:"raw_prompt_capture"`
	MaxGraphNodes       int    `json:"max_graph_nodes,omitempty" yaml:"max_graph_nodes,omitempty"`
	MaxGraphDepth       int    `json:"max_graph_depth,omitempty" yaml:"max_graph_depth,omitempty"`
	MaxParallelNodes    int    `json:"max_parallel_nodes,omitempty" yaml:"max_parallel_nodes,omitempty"`
	MaxRuntimeSeconds   int    `json:"max_runtime_seconds,omitempty" yaml:"max_runtime_seconds,omitempty"`
	MaxEvidenceBytes    int    `json:"max_evidence_bytes,omitempty" yaml:"max_evidence_bytes,omitempty"`
	RemoteModelsAllowed bool   `json:"remote_models_allowed" yaml:"remote_models_allowed"`
}

type Agent struct {
	Allowed bool              `json:"allowed" yaml:"allowed"`
	Skills  []string          `json:"skills,omitempty" yaml:"skills,omitempty"`
	Plugins map[string]Plugin `json:"plugins,omitempty" yaml:"plugins,omitempty"`
}

type Plugin struct {
	Mode string `json:"mode,omitempty" yaml:"mode,omitempty"`
}

type Source struct {
	Allowed bool `json:"allowed" yaml:"allowed"`
}

type Workspace struct {
	Allowed bool `json:"allowed" yaml:"allowed"`
}

type Bundle struct {
	Allowed bool `json:"allowed" yaml:"allowed"`
}

type WriteActions struct {
	Allowed bool `json:"allowed" yaml:"allowed"`
}

// Request is the normalized policy evaluation input for one Prism action.
type Request struct {
	AgentID                   string        `json:"agent_id,omitempty" yaml:"agent_id,omitempty"`
	Skills                    []string      `json:"skills,omitempty" yaml:"skills,omitempty"`
	Plugins                   []string      `json:"plugins,omitempty" yaml:"plugins,omitempty"`
	Source                    string        `json:"source,omitempty" yaml:"source,omitempty"`
	WorkspaceID               string        `json:"workspace_id,omitempty" yaml:"workspace_id,omitempty"`
	BundleID                  string        `json:"bundle_id,omitempty" yaml:"bundle_id,omitempty"`
	Runtime                   time.Duration `json:"-" yaml:"-"`
	GraphNodes                int           `json:"graph_nodes,omitempty" yaml:"graph_nodes,omitempty"`
	GraphDepth                int           `json:"graph_depth,omitempty" yaml:"graph_depth,omitempty"`
	EvidenceBytes             int           `json:"evidence_bytes,omitempty" yaml:"evidence_bytes,omitempty"`
	RawPromptCaptureRequested bool          `json:"raw_prompt_capture_requested,omitempty" yaml:"raw_prompt_capture_requested,omitempty"`
	RemoteModelRequested      bool          `json:"remote_model_requested,omitempty" yaml:"remote_model_requested,omitempty"`
	WriteRequested            bool          `json:"write_requested,omitempty" yaml:"write_requested,omitempty"`
}

// Decision is an explainable policy result.
type Decision struct {
	Decision string   `json:"decision"`
	Reason   string   `json:"reason"`
	Warnings []string `json:"warnings,omitempty"`
}

func Allow(reason string, warnings ...string) Decision {
	return Decision{Decision: DecisionAllow, Reason: reason, Warnings: warnings}
}

func Deny(reason string) Decision {
	return Decision{Decision: DecisionDeny, Reason: reason}
}

func Warn(reason string, warnings ...string) Decision {
	return Decision{Decision: DecisionWarn, Reason: reason, Warnings: warnings}
}

func RequireApproval(reason string) Decision {
	return Decision{Decision: DecisionRequireApproval, Reason: reason}
}
