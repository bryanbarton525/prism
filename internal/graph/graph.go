package graph

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/bryanbarton525/prism/internal/agent"
	"github.com/bryanbarton525/prism/internal/app"
	internalpolicy "github.com/bryanbarton525/prism/internal/policy"
	"github.com/bryanbarton525/prism/internal/result"
	graphpkg "github.com/bryanbarton525/prism/pkg/graph"
	"github.com/bryanbarton525/prism/pkg/observe"
	policypkg "github.com/bryanbarton525/prism/pkg/policy"
	"gopkg.in/yaml.v3"
)

const (
	defaultTimeoutSeconds = 120
	maxPriorArtifacts     = 8
	maxArtifactBytes      = 400
)

func Load(path string) (graphpkg.Definition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return graphpkg.Definition{}, err
	}
	var def graphpkg.Definition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return graphpkg.Definition{}, err
	}
	return def, nil
}

func Validate(def graphpkg.Definition) graphpkg.ValidationResult {
	res := graphpkg.ValidationResult{GraphID: def.ID, Valid: true, Nodes: len(def.Nodes)}
	if def.ID == "" {
		res.Errors = append(res.Errors, "id is required")
	}
	if def.Version == 0 {
		res.Errors = append(res.Errors, "version is required")
	}
	if len(def.Nodes) == 0 {
		res.Errors = append(res.Errors, "at least one node is required")
	}
	if def.Limits.MaxNodes > 0 && len(def.Nodes) > def.Limits.MaxNodes {
		res.Errors = append(res.Errors, fmt.Sprintf("node count %d exceeds max_nodes %d", len(def.Nodes), def.Limits.MaxNodes))
	}
	for id, node := range def.Nodes {
		if node.Agent == "" {
			res.Errors = append(res.Errors, fmt.Sprintf("node %q missing agent", id))
		}
		if len(node.Skills) == 0 {
			res.Errors = append(res.Errors, fmt.Sprintf("node %q missing skills", id))
		}
		if strings.TrimSpace(node.Task) == "" {
			res.Errors = append(res.Errors, fmt.Sprintf("node %q missing task", id))
		}
		for _, dep := range node.DependsOn {
			if _, ok := def.Nodes[dep]; !ok {
				res.Errors = append(res.Errors, fmt.Sprintf("node %q depends on unknown node %q", id, dep))
			}
		}
	}
	depth, cycle := graphDepth(def)
	res.Depth = depth
	if cycle != "" {
		res.Errors = append(res.Errors, "cycle detected: "+cycle)
	}
	if def.Limits.MaxDepth > 0 && depth > def.Limits.MaxDepth {
		res.Errors = append(res.Errors, fmt.Sprintf("graph depth %d exceeds max_depth %d", depth, def.Limits.MaxDepth))
	}
	if def.Limits.MaxParallel < 0 {
		res.Errors = append(res.Errors, "max_parallel cannot be negative")
	}
	if def.Limits.MaxParallel > 1 {
		res.Errors = append(res.Errors, "parallel graph execution is not supported in v1; set max_parallel to 1 or omit it")
	}
	if def.Limits.TimeoutSeconds < 0 {
		res.Errors = append(res.Errors, "timeout_seconds cannot be negative")
	}
	if def.Limits.MaxRetries < 0 {
		res.Errors = append(res.Errors, "max_retries cannot be negative")
	}
	if def.Limits.MaxRetries > 0 {
		res.Errors = append(res.Errors, "node retries are not supported in v1; set max_retries to 0 or omit it")
	}
	res.Valid = len(res.Errors) == 0
	return res
}

func Run(ctx context.Context, runner app.AgentRunner, def graphpkg.Definition, source string) (graphpkg.RunResult, error) {
	return RunWithOptions(ctx, runner, def, RunOptions{Source: source})
}

type RunOptions struct {
	Source    string
	Policy    *internalpolicy.Engine
	EventSink observe.Sink
}

type specGetter interface {
	GetSpec(context.Context, string) (*agent.Spec, error)
}

func RunWithOptions(ctx context.Context, runner app.AgentRunner, def graphpkg.Definition, opts RunOptions) (graphpkg.RunResult, error) {
	source := opts.Source
	if source == "" {
		source = "cli"
	}
	start := time.Now()
	emitGraph := func(out graphpkg.RunResult, decision policypkg.Decision) {
		if opts.EventSink == nil {
			return
		}
		_ = opts.EventSink.ObserveRun(context.WithoutCancel(ctx), observe.RunEvent{
			Timestamp:      time.Now().UTC(),
			RunID:          newGraphRunID(),
			GraphID:        def.ID,
			EventKind:      "graph",
			Metadata:       observe.Metadata{Source: source, CorrelationID: def.ID},
			Status:         out.Status,
			DurationMS:     time.Since(start).Milliseconds(),
			PolicyDecision: decision.Decision,
			PolicyReason:   decision.Reason,
			Error:          graphError(out),
		})
	}
	validation := Validate(def)
	if !validation.Valid {
		decision := policypkg.Deny(strings.Join(validation.Errors, "; "))
		out := graphpkg.RunResult{GraphID: def.ID, Status: result.StatusValidationFail, PolicyDecision: decision.Decision, PolicyReason: decision.Reason, AggregateResult: decision.Reason}
		emitGraph(out, policypkg.Deny(out.AggregateResult))
		return out, nil
	}
	policyDecision := graphPolicyDecision(ctx, runner, def, validation, opts.Policy, source)
	if internalpolicy.IsBlocking(policyDecision) {
		out := graphpkg.RunResult{GraphID: def.ID, Status: result.StatusValidationFail, PolicyDecision: policyDecision.Decision, PolicyReason: policyDecision.Reason, AggregateResult: policyDecision.Reason}
		emitGraph(out, policyDecision)
		return out, nil
	}
	timeoutSeconds := def.Limits.TimeoutSeconds
	if timeoutSeconds == 0 {
		timeoutSeconds = defaultTimeoutSeconds
	}
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()
	order := topologicalOrder(def)
	out := graphpkg.RunResult{
		GraphID:        def.ID,
		Status:         result.StatusOK,
		PolicyDecision: policyDecision.Decision,
		PolicyReason:   policyDecision.Reason,
		NodeOrder:      order,
		NodeResults:    make(map[string]any),
	}
	var summaries []string
	var priorArtifacts []result.Artifact
	for _, id := range order {
		node := def.Nodes[id]
		task := node.Task
		if prior := priorContext(summaries, priorArtifacts); prior != "" {
			task += "\n\nPrior graph context:\n" + prior
		}
		res, err := runner.Run(ctx, app.RunRequest{
			AgentID:     node.Agent,
			Task:        task,
			SkillNames:  node.Skills,
			Format:      "json",
			Metadata:    observe.Metadata{Source: source, CorrelationID: def.ID},
			GraphID:     def.ID,
			GraphNodeID: id,
		})
		if err != nil {
			return out, err
		}
		out.NodeResults[id] = res
		summaries = append(summaries, fmt.Sprintf("- %s: %s", id, res.Summary))
		priorArtifacts = append(priorArtifacts, res.Artifacts...)
		out.Artifacts = append(out.Artifacts, graphArtifacts(id, res.Artifacts)...)
		if res.Status != result.StatusOK {
			out.Status = res.Status
			break
		}
	}
	out.AggregateResult = strings.Join(summaries, "\n")
	emitGraph(out, policyDecision)
	return out, nil
}

func graphPolicyDecision(ctx context.Context, runner app.AgentRunner, def graphpkg.Definition, validation graphpkg.ValidationResult, policy *internalpolicy.Engine, source string) policypkg.Decision {
	if policy == nil {
		return policypkg.Allow("no policy configured")
	}
	decision := policy.Explain(policypkg.Request{
		Source:     source,
		GraphNodes: validation.Nodes,
		GraphDepth: validation.Depth,
	})
	if internalpolicy.IsBlocking(decision) {
		return decision
	}
	getter, ok := runner.(specGetter)
	for id, node := range def.Nodes {
		plugins := []string{}
		if ok {
			spec, err := getter.GetSpec(ctx, node.Agent)
			if err != nil {
				return policypkg.Deny(fmt.Sprintf("node %q agent %q could not be resolved for policy: %v", id, node.Agent, err))
			}
			plugins = append(plugins, spec.Tools...)
		}
		decision = policy.Explain(policypkg.Request{
			AgentID: node.Agent,
			Skills:  append([]string{}, node.Skills...),
			Plugins: plugins,
			Source:  source,
		})
		if internalpolicy.IsBlocking(decision) {
			return policypkg.Deny(fmt.Sprintf("node %q blocked by policy: %s", id, decision.Reason))
		}
	}
	return policypkg.Allow("policy allowed graph")
}

func priorContext(summaries []string, artifacts []result.Artifact) string {
	var b strings.Builder
	if len(summaries) > 0 {
		b.WriteString("Summaries:\n")
		b.WriteString(strings.Join(summaries, "\n"))
	}
	if len(artifacts) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("Artifacts:\n")
		limit := len(artifacts)
		if limit > maxPriorArtifacts {
			limit = maxPriorArtifacts
		}
		for i := 0; i < limit; i++ {
			content := strings.TrimSpace(artifacts[i].Content)
			if len(content) > maxArtifactBytes {
				content = content[:maxArtifactBytes] + "..."
			}
			b.WriteString(fmt.Sprintf("- %s (%s): %s\n", artifacts[i].Label, artifacts[i].Type, content))
		}
	}
	return strings.TrimSpace(b.String())
}

func graphError(out graphpkg.RunResult) string {
	if out.Status == result.StatusOK {
		return ""
	}
	return out.AggregateResult
}

func newGraphRunID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("graph-%d", time.Now().UnixNano())
	}
	return "graph-" + hex.EncodeToString(b[:])
}

func graphDepth(def graphpkg.Definition) (int, string) {
	memo := map[string]int{}
	visiting := map[string]bool{}
	var visit func(string) (int, string)
	visit = func(id string) (int, string) {
		if visiting[id] {
			return 0, id
		}
		if d, ok := memo[id]; ok {
			return d, ""
		}
		visiting[id] = true
		maxDep := 0
		for _, dep := range def.Nodes[id].DependsOn {
			d, cycle := visit(dep)
			if cycle != "" {
				return 0, cycle + " -> " + id
			}
			if d > maxDep {
				maxDep = d
			}
		}
		visiting[id] = false
		memo[id] = maxDep + 1
		return memo[id], ""
	}
	depth := 0
	for id := range def.Nodes {
		d, cycle := visit(id)
		if cycle != "" {
			return 0, cycle
		}
		if d > depth {
			depth = d
		}
	}
	return depth, ""
}

func topologicalOrder(def graphpkg.Definition) []string {
	var order []string
	seen := map[string]bool{}
	var visit func(string)
	visit = func(id string) {
		if seen[id] {
			return
		}
		seen[id] = true
		deps := append([]string{}, def.Nodes[id].DependsOn...)
		sort.Strings(deps)
		for _, dep := range deps {
			visit(dep)
		}
		order = append(order, id)
	}
	ids := make([]string, 0, len(def.Nodes))
	for id := range def.Nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		visit(id)
	}
	return order
}

func graphArtifacts(nodeID string, artifacts []result.Artifact) []graphpkg.Artifact {
	out := make([]graphpkg.Artifact, 0, len(artifacts))
	for _, artifact := range artifacts {
		content := strings.TrimSpace(artifact.Content)
		if len(content) > maxArtifactBytes {
			content = content[:maxArtifactBytes] + "..."
		}
		out = append(out, graphpkg.Artifact{
			NodeID:  nodeID,
			Type:    artifact.Type,
			Label:   artifact.Label,
			Content: content,
		})
	}
	return out
}
