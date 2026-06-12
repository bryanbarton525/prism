// Package mcp provides the MCP server adapter for Prism.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bryanbarton525/prism/internal/agent"
	"github.com/bryanbarton525/prism/internal/app"
	"github.com/bryanbarton525/prism/internal/bundles"
	"github.com/bryanbarton525/prism/internal/downstreammcp"
	"github.com/bryanbarton525/prism/internal/events"
	internalgraph "github.com/bryanbarton525/prism/internal/graph"
	internalpolicy "github.com/bryanbarton525/prism/internal/policy"
	"github.com/bryanbarton525/prism/internal/result"
	"github.com/bryanbarton525/prism/internal/router"
	"github.com/bryanbarton525/prism/internal/skill"
	graphpkg "github.com/bryanbarton525/prism/pkg/graph"
	"github.com/bryanbarton525/prism/pkg/observe"
	policypkg "github.com/bryanbarton525/prism/pkg/policy"
)

const (
	serverName    = "prism"
	serverVersion = "v0.1.0"
)

// Serve starts the MCP server over stdio until the client disconnects.
func Serve(ctx context.Context, runner app.AgentRunner) error {
	return ServeWithConfig(ctx, runner, Config{})
}

type Config struct {
	Policy          *internalpolicy.Engine
	EventSink       observe.Sink
	DownstreamMCP   *downstreammcp.Client
	BundleStatePath string
	EventStorePath  string
	RootDir         string
	SkillsDir       string
}

func ServeWithConfig(ctx context.Context, runner app.AgentRunner, cfg Config) error {
	srv := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    serverName,
		Version: serverVersion,
	}, nil)
	registerTools(srv, runner, cfg)
	return srv.Run(ctx, &mcpsdk.StdioTransport{})
}

func registerTools(srv *mcpsdk.Server, runner app.AgentRunner, cfg Config) {
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_agents",
		Description: "List registered Prism agents with model hints and allowed skills.",
	}, listAgentsHandler(runner))

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "run_agent",
		Description: "Invoke a specialist agent with required skill_names and a bounded task.",
	}, runAgentHandler(runner))

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_constitution",
		Description: "Return the resolved constitution text for an agent.",
	}, getConstitutionHandler(runner))

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "doctor",
		Description: "Report Ollama connectivity, models, and agent/skill registry health.",
	}, doctorHandler(runner))

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "suggest_route",
		Description: "Suggest a deterministic Prism agent and skill route for a bounded task.",
	}, suggestRouteHandler(runner, cfg.Policy))

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "run_graph",
		Description: "Run a bounded Prism graph definition.",
	}, runGraphHandler(runner, cfg))

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "explain_policy",
		Description: "Explain the configured Prism policy decision for an agent request.",
	}, explainPolicyHandler(cfg.Policy))

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_policies",
		Description: "List configured Prism policy sources visible to this MCP server.",
	}, listPoliciesHandler(cfg.Policy))

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_bundles",
		Description: "List installed Prism bundles from local state.",
	}, listBundlesHandler(cfg))

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "install_bundle",
		Description: "Verify and install a signed Prism registry bundle manifest.",
	}, installBundleHandler(cfg))

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_usage_summary",
		Description: "Summarize local Prism usage from the event store.",
	}, usageSummaryHandler(cfg))

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_skill_health",
		Description: "Return structural health for local Prism skills.",
	}, skillHealthHandler(cfg))

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_mcp_servers",
		Description: "List downstream MCP servers configured for Prism to call.",
	}, listMCPServersHandler(cfg.DownstreamMCP))

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_mcp_server_tools",
		Description: "List compact tool inventory for a downstream MCP server.",
	}, listMCPServerToolsHandler(cfg.DownstreamMCP))

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "call_mcp_tool",
		Description: "Call one tool on a configured downstream MCP server and return a bounded result.",
	}, callMCPToolHandler(cfg.DownstreamMCP))

	// Compatibility tools for MCP hosts that do not yet support native prompts/resources.
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_prompts",
		Description: "List reusable Prism prompt templates for accurate tool calling.",
	}, listPromptsHandler(runner))

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_prompt",
		Description: "Return a concrete prompt template with optional variable substitution.",
	}, getPromptHandler(runner))

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_resources",
		Description: "List Prism resources (tooling docs, agents index, constitutions).",
	}, listResourcesHandler(runner))

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_resource",
		Description: "Fetch a Prism resource by URI.",
	}, getResourceHandler(runner))
}

type ListAgentsInput struct{}

type ListAgentsOutput struct {
	Agents []agent.Summary `json:"agents"`
	Count  int             `json:"count"`
}

func listAgentsHandler(runner app.AgentRunner) func(context.Context, *mcpsdk.CallToolRequest, ListAgentsInput) (*mcpsdk.CallToolResult, ListAgentsOutput, error) {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ ListAgentsInput) (*mcpsdk.CallToolResult, ListAgentsOutput, error) {
		agents, err := runner.ListAgents(ctx)
		if err != nil {
			return nil, ListAgentsOutput{}, err
		}
		out := ListAgentsOutput{Agents: agents, Count: len(agents)}
		return textResult(marshalJSON(out)), out, nil
	}
}

type RunAgentInput struct {
	AgentID       string   `json:"agent_id"`
	Task          string   `json:"task"`
	SkillNames    []string `json:"skill_names"`
	Format        string   `json:"format,omitempty"`
	BundleID      string   `json:"bundle_id,omitempty"`
	BundleVersion string   `json:"bundle_version,omitempty"`
}

func runAgentHandler(runner app.AgentRunner) func(context.Context, *mcpsdk.CallToolRequest, RunAgentInput) (*mcpsdk.CallToolResult, result.RunResult, error) {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, input RunAgentInput) (*mcpsdk.CallToolResult, result.RunResult, error) {
		if input.AgentID == "" {
			return nil, result.RunResult{}, fmt.Errorf("run_agent: agent_id is required")
		}
		if input.Task == "" {
			return nil, result.RunResult{}, fmt.Errorf("run_agent: task is required")
		}
		if len(input.SkillNames) == 0 {
			return nil, result.RunResult{}, fmt.Errorf("run_agent: skill_names is required")
		}
		format := input.Format
		if format == "" {
			format = "json"
		}
		res, err := runner.Run(ctx, app.RunRequest{
			AgentID:       input.AgentID,
			Task:          input.Task,
			SkillNames:    input.SkillNames,
			Format:        format,
			Metadata:      observe.Metadata{Source: "mcp"},
			BundleID:      input.BundleID,
			BundleVersion: input.BundleVersion,
		})
		if err != nil {
			return nil, result.RunResult{}, err
		}
		return textResult(marshalJSON(res)), res, nil
	}
}

type GetConstitutionInput struct {
	AgentID string `json:"agent_id"`
}

func getConstitutionHandler(runner app.AgentRunner) func(context.Context, *mcpsdk.CallToolRequest, GetConstitutionInput) (*mcpsdk.CallToolResult, app.Constitution, error) {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, input GetConstitutionInput) (*mcpsdk.CallToolResult, app.Constitution, error) {
		if input.AgentID == "" {
			return nil, app.Constitution{}, fmt.Errorf("get_constitution: agent_id is required")
		}
		c, err := runner.GetConstitution(ctx, input.AgentID)
		if err != nil {
			return nil, app.Constitution{}, err
		}
		return textResult(marshalJSON(c)), c, nil
	}
}

type DoctorInput struct{}

func doctorHandler(runner app.AgentRunner) func(context.Context, *mcpsdk.CallToolRequest, DoctorInput) (*mcpsdk.CallToolResult, result.DoctorResult, error) {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ DoctorInput) (*mcpsdk.CallToolResult, result.DoctorResult, error) {
		dr, err := runner.Doctor(ctx)
		if err != nil {
			return nil, result.DoctorResult{}, err
		}
		return textResult(marshalJSON(dr)), dr, nil
	}
}

type SuggestRouteInput struct {
	Task   string `json:"task"`
	Source string `json:"source"`
}

func suggestRouteHandler(runner app.AgentRunner, policy *internalpolicy.Engine) func(context.Context, *mcpsdk.CallToolRequest, SuggestRouteInput) (*mcpsdk.CallToolResult, router.Result, error) {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, input SuggestRouteInput) (*mcpsdk.CallToolResult, router.Result, error) {
		if input.Task == "" {
			return nil, router.Result{}, fmt.Errorf("suggest_route: task is required")
		}
		if input.Source == "" {
			input.Source = "mcp"
		}
		res, err := router.New(runner, policy).Suggest(ctx, router.Request{Task: input.Task, Source: input.Source})
		if err != nil {
			return nil, router.Result{}, err
		}
		return textResult(marshalJSON(res)), res, nil
	}
}

type RunGraphInput struct {
	Graph graphpkg.Definition `json:"graph"`
}

func runGraphHandler(runner app.AgentRunner, cfg Config) func(context.Context, *mcpsdk.CallToolRequest, RunGraphInput) (*mcpsdk.CallToolResult, graphpkg.RunResult, error) {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, input RunGraphInput) (*mcpsdk.CallToolResult, graphpkg.RunResult, error) {
		if input.Graph.ID == "" {
			return nil, graphpkg.RunResult{}, fmt.Errorf("run_graph: graph.id is required")
		}
		res, err := internalgraph.RunWithOptions(ctx, runner, input.Graph, internalgraph.RunOptions{Source: "mcp", Policy: cfg.Policy, EventSink: cfg.EventSink})
		if err != nil {
			return nil, graphpkg.RunResult{}, err
		}
		return textResult(marshalJSON(res)), res, nil
	}
}

type ExplainPolicyInput struct {
	AgentID string   `json:"agent_id"`
	Skills  []string `json:"skills"`
	Plugins []string `json:"plugins"`
	Source  string   `json:"source"`
}

type ListPoliciesInput struct{}

type ListPoliciesOutput struct {
	Configured bool   `json:"configured"`
	Reason     string `json:"reason"`
}

type ListBundlesInput struct{}

func listBundlesHandler(cfg Config) func(context.Context, *mcpsdk.CallToolRequest, ListBundlesInput) (*mcpsdk.CallToolResult, bundles.State, error) {
	return func(_ context.Context, _ *mcpsdk.CallToolRequest, _ ListBundlesInput) (*mcpsdk.CallToolResult, bundles.State, error) {
		if cfg.BundleStatePath == "" {
			return nil, bundles.State{}, fmt.Errorf("bundle state path is not configured")
		}
		state, err := bundles.Load(cfg.BundleStatePath)
		if err != nil {
			return nil, bundles.State{}, err
		}
		return textResult(marshalJSON(state)), state, nil
	}
}

type InstallBundleInput struct {
	ManifestPath string `json:"manifest_path"`
	SourceRoot   string `json:"source_root,omitempty"`
	DestRoot     string `json:"dest_root,omitempty"`
	PublicKey    string `json:"public_key"`
	PrismVersion string `json:"prism_version,omitempty"`
}

func installBundleHandler(cfg Config) func(context.Context, *mcpsdk.CallToolRequest, InstallBundleInput) (*mcpsdk.CallToolResult, map[string]any, error) {
	return func(_ context.Context, _ *mcpsdk.CallToolRequest, input InstallBundleInput) (*mcpsdk.CallToolResult, map[string]any, error) {
		if cfg.BundleStatePath == "" {
			return nil, nil, fmt.Errorf("bundle state path is not configured")
		}
		destRoot := input.DestRoot
		if destRoot == "" {
			destRoot = cfg.RootDir
		}
		manifest, err := bundles.InstallVerified(bundles.InstallOptions{
			ManifestPath: input.ManifestPath,
			SourceRoot:   input.SourceRoot,
			DestRoot:     destRoot,
			StatePath:    cfg.BundleStatePath,
			PublicKey:    input.PublicKey,
			PrismVersion: input.PrismVersion,
		})
		if err != nil {
			return nil, nil, err
		}
		out := map[string]any{"installed": true, "registry_id": manifest.RegistryID, "version": manifest.Version, "bundles": manifest.Bundles}
		return textResult(marshalJSON(out)), out, nil
	}
}

type UsageSummaryInput struct{}

func usageSummaryHandler(cfg Config) func(context.Context, *mcpsdk.CallToolRequest, UsageSummaryInput) (*mcpsdk.CallToolResult, events.Summary, error) {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ UsageSummaryInput) (*mcpsdk.CallToolResult, events.Summary, error) {
		if cfg.EventStorePath == "" {
			return nil, events.Summary{}, fmt.Errorf("event store path is not configured")
		}
		store, err := events.Open(cfg.EventStorePath)
		if err != nil {
			return nil, events.Summary{}, err
		}
		defer store.Close()
		sum, err := store.Summary(ctx)
		if err != nil {
			return nil, events.Summary{}, err
		}
		return textResult(marshalJSON(sum)), sum, nil
	}
}

type SkillHealthInput struct {
	SkillName string `json:"skill_name,omitempty"`
}

type SkillHealthOutput struct {
	Skills []SkillHealth `json:"skills"`
	Count  int           `json:"count"`
}

type SkillHealth struct {
	Name     string   `json:"name"`
	OK       bool     `json:"ok"`
	Chars    int      `json:"chars"`
	Evals    int      `json:"evals,omitempty"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

func skillHealthHandler(cfg Config) func(context.Context, *mcpsdk.CallToolRequest, SkillHealthInput) (*mcpsdk.CallToolResult, SkillHealthOutput, error) {
	return func(_ context.Context, _ *mcpsdk.CallToolRequest, input SkillHealthInput) (*mcpsdk.CallToolResult, SkillHealthOutput, error) {
		root := cfg.SkillsDir
		if root == "" && cfg.RootDir != "" {
			root = filepath.Join(cfg.RootDir, "skills")
		}
		if root == "" {
			return nil, SkillHealthOutput{}, fmt.Errorf("skills directory is not configured")
		}
		items, err := collectSkillHealth(root, input.SkillName)
		if err != nil {
			return nil, SkillHealthOutput{}, err
		}
		out := SkillHealthOutput{Skills: items, Count: len(items)}
		return textResult(marshalJSON(out)), out, nil
	}
}

func collectSkillHealth(root, only string) ([]SkillHealth, error) {
	var names []string
	if only != "" {
		names = []string{only}
	} else {
		entries, err := os.ReadDir(root)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				names = append(names, entry.Name())
			}
		}
	}
	fsys := os.DirFS(root)
	out := make([]SkillHealth, 0, len(names))
	for _, name := range names {
		item := SkillHealth{Name: name, OK: true}
		data, err := fs.ReadFile(fsys, filepath.ToSlash(filepath.Join(name, "SKILL.md")))
		if err != nil {
			item.OK = false
			item.Errors = append(item.Errors, err.Error())
			out = append(out, item)
			continue
		}
		item.Chars = len(data)
		if _, err := skill.LoadDir(fsys, name); err != nil {
			item.OK = false
			item.Errors = append(item.Errors, err.Error())
		}
		if err := skill.ValidateStructure(fsys, name); err != nil {
			item.OK = false
			item.Errors = append(item.Errors, err.Error())
		}
		count, err := skill.ValidateEvals(fsys, name)
		if err != nil {
			item.OK = false
			item.Errors = append(item.Errors, err.Error())
		} else {
			item.Evals = count
		}
		if !strings.Contains(string(data), "##") {
			item.Warnings = append(item.Warnings, "no markdown section headings")
		}
		out = append(out, item)
	}
	return out, nil
}

func explainPolicyHandler(policy *internalpolicy.Engine) func(context.Context, *mcpsdk.CallToolRequest, ExplainPolicyInput) (*mcpsdk.CallToolResult, policypkg.Decision, error) {
	return func(_ context.Context, _ *mcpsdk.CallToolRequest, input ExplainPolicyInput) (*mcpsdk.CallToolResult, policypkg.Decision, error) {
		if input.Source == "" {
			input.Source = "mcp"
		}
		decision := policypkg.Allow("no policy configured")
		if policy != nil {
			decision = policy.Explain(policypkg.Request{
				AgentID: input.AgentID,
				Skills:  input.Skills,
				Plugins: input.Plugins,
				Source:  input.Source,
			})
		}
		return textResult(marshalJSON(decision)), decision, nil
	}
}

func listPoliciesHandler(policy *internalpolicy.Engine) func(context.Context, *mcpsdk.CallToolRequest, ListPoliciesInput) (*mcpsdk.CallToolResult, ListPoliciesOutput, error) {
	return func(_ context.Context, _ *mcpsdk.CallToolRequest, _ ListPoliciesInput) (*mcpsdk.CallToolResult, ListPoliciesOutput, error) {
		out := ListPoliciesOutput{Configured: policy != nil}
		if policy == nil {
			out.Reason = "no policy configured"
		} else {
			out.Reason = "policy configured for this MCP server"
		}
		return textResult(marshalJSON(out)), out, nil
	}
}

type ListMCPServersInput struct{}

type ListMCPServersOutput struct {
	Configured bool                   `json:"configured"`
	Servers    []downstreammcp.Server `json:"servers"`
}

func listMCPServersHandler(client *downstreammcp.Client) func(context.Context, *mcpsdk.CallToolRequest, ListMCPServersInput) (*mcpsdk.CallToolResult, ListMCPServersOutput, error) {
	return func(_ context.Context, _ *mcpsdk.CallToolRequest, _ ListMCPServersInput) (*mcpsdk.CallToolResult, ListMCPServersOutput, error) {
		out := ListMCPServersOutput{}
		if client != nil {
			out.Servers = client.Servers()
			out.Configured = len(out.Servers) > 0
		}
		return textResult(marshalJSON(out)), out, nil
	}
}

type ListMCPServerToolsInput struct {
	Server        string `json:"server"`
	IncludeSchema bool   `json:"include_schema,omitempty"`
	MaxTools      int    `json:"max_tools,omitempty"`
}

type ListMCPServerToolsOutput struct {
	Server string                      `json:"server"`
	Tools  []downstreammcp.ToolSummary `json:"tools"`
	Count  int                         `json:"count"`
}

func listMCPServerToolsHandler(client *downstreammcp.Client) func(context.Context, *mcpsdk.CallToolRequest, ListMCPServerToolsInput) (*mcpsdk.CallToolResult, ListMCPServerToolsOutput, error) {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, input ListMCPServerToolsInput) (*mcpsdk.CallToolResult, ListMCPServerToolsOutput, error) {
		if client == nil {
			return nil, ListMCPServerToolsOutput{}, fmt.Errorf("downstream MCP client is not configured")
		}
		if input.Server == "" {
			return nil, ListMCPServerToolsOutput{}, fmt.Errorf("list_mcp_server_tools: server is required")
		}
		tools, err := client.ListTools(ctx, input.Server, downstreammcp.ListToolsOptions{IncludeSchema: input.IncludeSchema, MaxTools: input.MaxTools})
		if err != nil {
			return nil, ListMCPServerToolsOutput{}, err
		}
		out := ListMCPServerToolsOutput{Server: input.Server, Tools: tools, Count: len(tools)}
		return textResult(marshalJSON(out)), out, nil
	}
}

type CallMCPToolInput struct {
	Server    string         `json:"server"`
	Tool      string         `json:"tool"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

func callMCPToolHandler(client *downstreammcp.Client) func(context.Context, *mcpsdk.CallToolRequest, CallMCPToolInput) (*mcpsdk.CallToolResult, downstreammcp.CallResult, error) {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, input CallMCPToolInput) (*mcpsdk.CallToolResult, downstreammcp.CallResult, error) {
		if client == nil {
			return nil, downstreammcp.CallResult{}, fmt.Errorf("downstream MCP client is not configured")
		}
		if input.Server == "" {
			return nil, downstreammcp.CallResult{}, fmt.Errorf("call_mcp_tool: server is required")
		}
		if input.Tool == "" {
			return nil, downstreammcp.CallResult{}, fmt.Errorf("call_mcp_tool: tool is required")
		}
		if input.Arguments == nil {
			input.Arguments = map[string]any{}
		}
		res, err := client.CallTool(ctx, input.Server, input.Tool, input.Arguments)
		if err != nil {
			return nil, downstreammcp.CallResult{}, err
		}
		return textResult(marshalJSON(res)), res, nil
	}
}

func marshalJSON(v any) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": %q}`, err.Error())
	}
	return string(data)
}

func textResult(text string) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: text}},
	}
}

// StatusSummary returns a one-line startup message for logging.
func StatusSummary(runner app.AgentRunner) string {
	agents, err := runner.ListAgents(context.Background())
	if err != nil {
		return fmt.Sprintf("prism MCP server starting (registry error: %v)", err)
	}
	ids := make([]string, len(agents))
	for i, a := range agents {
		ids[i] = a.ID
	}
	return fmt.Sprintf("prism MCP server ready: %d agent(s) [%s]", len(ids), strings.Join(ids, ", "))
}
