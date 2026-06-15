// Package graph defines bounded Prism workflow graph contracts.
package graph

type Definition struct {
	ID      string          `json:"id" yaml:"id"`
	Version int             `json:"version" yaml:"version"`
	Limits  Limits          `json:"limits,omitempty" yaml:"limits,omitempty"`
	Nodes   map[string]Node `json:"nodes" yaml:"nodes"`
}

type Limits struct {
	MaxNodes       int `json:"max_nodes,omitempty" yaml:"max_nodes,omitempty"`
	MaxDepth       int `json:"max_depth,omitempty" yaml:"max_depth,omitempty"`
	MaxParallel    int `json:"max_parallel,omitempty" yaml:"max_parallel,omitempty"`
	TimeoutSeconds int `json:"timeout_seconds,omitempty" yaml:"timeout_seconds,omitempty"`
	MaxRetries     int `json:"max_retries,omitempty" yaml:"max_retries,omitempty"`
}

type Node struct {
	DependsOn []string `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
	Agent     string   `json:"agent" yaml:"agent"`
	Skills    []string `json:"skills" yaml:"skills"`
	Task      string   `json:"task" yaml:"task"`
}

type ValidationResult struct {
	GraphID string   `json:"graph_id"`
	Valid   bool     `json:"valid"`
	Errors  []string `json:"errors,omitempty"`
	Nodes   int      `json:"nodes"`
	Depth   int      `json:"depth"`
}

type RunResult struct {
	GraphID         string         `json:"graph_id"`
	Status          string         `json:"status"`
	PolicyDecision  string         `json:"policy_decision,omitempty"`
	PolicyReason    string         `json:"policy_reason,omitempty"`
	NodeOrder       []string       `json:"node_order"`
	NodeResults     map[string]any `json:"node_results"`
	Artifacts       []Artifact     `json:"artifacts,omitempty"`
	AggregateResult string         `json:"aggregate_result,omitempty"`
}

type Artifact struct {
	NodeID  string `json:"node_id,omitempty"`
	Type    string `json:"type,omitempty"`
	Label   string `json:"label,omitempty"`
	Content string `json:"content"`
}
