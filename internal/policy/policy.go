package policy

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	policypkg "github.com/bryanbarton525/prism/pkg/policy"
	"gopkg.in/yaml.v3"
)

type Engine struct {
	policy policypkg.Policy
}

type TestSuite struct {
	Cases []TestCase `json:"cases" yaml:"cases"`
}

type TestCase struct {
	Name         string            `json:"name" yaml:"name"`
	Request      policypkg.Request `json:"request" yaml:"request"`
	WantDecision string            `json:"want_decision" yaml:"want_decision"`
	WantReason   string            `json:"want_reason,omitempty" yaml:"want_reason,omitempty"`
}

type TestResult struct {
	Name         string             `json:"name"`
	Passed       bool               `json:"passed"`
	Request      policypkg.Request  `json:"request"`
	WantDecision string             `json:"want_decision"`
	Got          policypkg.Decision `json:"got"`
	Error        string             `json:"error,omitempty"`
}

func Load(path string) (*Engine, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p policypkg.Policy
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	if err := Validate(p); err != nil {
		return nil, err
	}
	return New(p), nil
}

func New(p policypkg.Policy) *Engine {
	return &Engine{policy: p}
}

func LoadTestSuite(path string) (TestSuite, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return TestSuite{}, err
	}
	var suite TestSuite
	if err := yaml.Unmarshal(data, &suite); err != nil {
		return TestSuite{}, err
	}
	if len(suite.Cases) == 0 {
		return TestSuite{}, fmt.Errorf("policy test suite must include at least one case")
	}
	for i, tc := range suite.Cases {
		if strings.TrimSpace(tc.Name) == "" {
			return TestSuite{}, fmt.Errorf("case %d missing name", i)
		}
		if strings.TrimSpace(tc.WantDecision) == "" {
			return TestSuite{}, fmt.Errorf("case %q missing want_decision", tc.Name)
		}
	}
	return suite, nil
}

func (e *Engine) Test(suite TestSuite) []TestResult {
	out := make([]TestResult, 0, len(suite.Cases))
	for _, tc := range suite.Cases {
		got := e.Explain(tc.Request)
		res := TestResult{
			Name:         tc.Name,
			Request:      tc.Request,
			WantDecision: tc.WantDecision,
			Got:          got,
		}
		if got.Decision != tc.WantDecision {
			res.Error = fmt.Sprintf("decision = %q, want %q", got.Decision, tc.WantDecision)
		} else if tc.WantReason != "" && !strings.Contains(got.Reason, tc.WantReason) {
			res.Error = fmt.Sprintf("reason = %q, want substring %q", got.Reason, tc.WantReason)
		}
		res.Passed = res.Error == ""
		out = append(out, res)
	}
	return out
}

func Validate(p policypkg.Policy) error {
	if p.Version == 0 {
		return fmt.Errorf("policy version is required")
	}
	if p.Defaults.MaxGraphNodes < 0 {
		return fmt.Errorf("defaults.max_graph_nodes cannot be negative")
	}
	if p.Defaults.MaxGraphDepth < 0 {
		return fmt.Errorf("defaults.max_graph_depth cannot be negative")
	}
	if p.Defaults.MaxParallelNodes < 0 {
		return fmt.Errorf("defaults.max_parallel_nodes cannot be negative")
	}
	if p.Defaults.MaxRuntimeSeconds < 0 {
		return fmt.Errorf("defaults.max_runtime_seconds cannot be negative")
	}
	if p.Defaults.MaxEvidenceBytes < 0 {
		return fmt.Errorf("defaults.max_evidence_bytes cannot be negative")
	}
	for id, agent := range p.Agents {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("agent id cannot be empty")
		}
		if !agent.Allowed && (len(agent.Skills) > 0 || len(agent.Plugins) > 0) {
			return fmt.Errorf("agent %q is not allowed but declares skills/plugins", id)
		}
	}
	for id := range p.Sources {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("source id cannot be empty")
		}
	}
	for id := range p.Workspaces {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("workspace id cannot be empty")
		}
	}
	for id := range p.Bundles {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("bundle id cannot be empty")
		}
	}
	return nil
}

func (e *Engine) Explain(req policypkg.Request) policypkg.Decision {
	if e == nil {
		return policypkg.Allow("no policy configured")
	}
	var warnings []string
	if req.Source != "" {
		src, ok := e.policy.Sources[req.Source]
		if ok && !src.Allowed {
			return policypkg.Deny(fmt.Sprintf("source %q is not allowed", req.Source))
		}
		if len(e.policy.Sources) > 0 && !ok {
			return policypkg.Deny(fmt.Sprintf("source %q is not allowed", req.Source))
		}
	}
	if req.WorkspaceID != "" {
		workspace, ok := e.policy.Workspaces[req.WorkspaceID]
		if ok && !workspace.Allowed {
			return policypkg.Deny(fmt.Sprintf("workspace %q is not allowed", req.WorkspaceID))
		}
		if len(e.policy.Workspaces) > 0 && !ok {
			return policypkg.Deny(fmt.Sprintf("workspace %q is not allowed", req.WorkspaceID))
		}
	}
	if req.BundleID != "" {
		bundle, ok := e.policy.Bundles[req.BundleID]
		if ok && !bundle.Allowed {
			return policypkg.Deny(fmt.Sprintf("bundle %q is not allowed", req.BundleID))
		}
		if len(e.policy.Bundles) > 0 && !ok {
			return policypkg.Deny(fmt.Sprintf("bundle %q is not allowed", req.BundleID))
		}
	}
	if req.WriteRequested && !e.policy.WriteActions.Allowed {
		return policypkg.Deny("write actions are disabled")
	}
	if req.RawPromptCaptureRequested && !e.policy.Defaults.RawPromptCapture {
		return policypkg.Deny("raw prompt capture is disabled")
	}
	if req.RemoteModelRequested && !e.policy.Defaults.RemoteModelsAllowed {
		return policypkg.Deny("remote model runtimes are disabled")
	}
	if req.AgentID != "" {
		agent, ok := e.policy.Agents[req.AgentID]
		if !ok || !agent.Allowed {
			return policypkg.Deny(fmt.Sprintf("agent %q is not allowed", req.AgentID))
		}
		for _, skill := range req.Skills {
			if len(agent.Skills) > 0 && !slices.Contains(agent.Skills, skill) {
				return policypkg.Deny(fmt.Sprintf("agent %q requested skill %q, which is not allowed by policy", req.AgentID, skill))
			}
		}
		for _, plugin := range req.Plugins {
			spec, ok := agent.Plugins[plugin]
			if !ok {
				return policypkg.Deny(fmt.Sprintf("agent %q requested plugin %q, which is not allowed by policy", req.AgentID, plugin))
			}
			if spec.Mode != "" && spec.Mode != "read_only" {
				return policypkg.RequireApproval(fmt.Sprintf("plugin %q requires %q mode", plugin, spec.Mode))
			}
		}
	}
	if e.policy.Defaults.MaxRuntimeSeconds > 0 && req.Runtime > time.Duration(e.policy.Defaults.MaxRuntimeSeconds)*time.Second {
		warnings = append(warnings, fmt.Sprintf("runtime %s exceeds policy max_runtime_seconds %d", req.Runtime, e.policy.Defaults.MaxRuntimeSeconds))
	}
	if e.policy.Defaults.MaxGraphNodes > 0 && req.GraphNodes > e.policy.Defaults.MaxGraphNodes {
		return policypkg.Deny(fmt.Sprintf("graph has %d nodes, above max_graph_nodes %d", req.GraphNodes, e.policy.Defaults.MaxGraphNodes))
	}
	if e.policy.Defaults.MaxGraphDepth > 0 && req.GraphDepth > e.policy.Defaults.MaxGraphDepth {
		return policypkg.Deny(fmt.Sprintf("graph depth %d is above max_graph_depth %d", req.GraphDepth, e.policy.Defaults.MaxGraphDepth))
	}
	if e.policy.Defaults.MaxEvidenceBytes > 0 && req.EvidenceBytes > e.policy.Defaults.MaxEvidenceBytes {
		return policypkg.Deny(fmt.Sprintf("evidence size %d bytes is above max_evidence_bytes %d", req.EvidenceBytes, e.policy.Defaults.MaxEvidenceBytes))
	}
	if len(warnings) > 0 {
		return policypkg.Warn("policy allowed with warnings", warnings...)
	}
	return policypkg.Allow("policy allowed request")
}

func IsBlocking(decision policypkg.Decision) bool {
	return decision.Decision == policypkg.DecisionDeny || decision.Decision == policypkg.DecisionRequireApproval
}
