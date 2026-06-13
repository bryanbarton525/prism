// Package evidence defines bounded runtime evidence contracts.
package evidence

import "time"

// Pack is a structured, bounded context artifact collected by a runtime plugin.
type Pack struct {
	Kind           string            `json:"kind"`
	Source         string            `json:"source,omitempty"`
	Plugin         string            `json:"plugin"`
	CollectionTime time.Time         `json:"collection_time"`
	Limits         Limits            `json:"limits,omitempty"`
	Summary        map[string]any    `json:"summary,omitempty"`
	Artifacts      []Artifact        `json:"artifacts,omitempty"`
	Redactions     []string          `json:"redactions,omitempty"`
	Errors         []string          `json:"errors,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

type Limits struct {
	MaxBytes     int `json:"max_bytes,omitempty"`
	MaxPods      int `json:"max_pods,omitempty"`
	MaxEvents    int `json:"max_events,omitempty"`
	MaxArtifacts int `json:"max_artifacts,omitempty"`
}

type Artifact struct {
	Type    string         `json:"type"`
	Name    string         `json:"name,omitempty"`
	Status  string         `json:"status,omitempty"`
	Summary map[string]any `json:"summary,omitempty"`
	Content string         `json:"content,omitempty"`
}
