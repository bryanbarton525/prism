// Package mcp provides the MCP server adapter for Prism.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	prismbundle "github.com/bryanbarton525/prism"
	"github.com/bryanbarton525/prism/internal/agent"
	"github.com/bryanbarton525/prism/internal/app"
	"github.com/bryanbarton525/prism/internal/buildinfo"
	"github.com/bryanbarton525/prism/internal/downstreammcp"
	"github.com/bryanbarton525/prism/internal/events"
	"github.com/bryanbarton525/prism/internal/extensions"
	internalgraph "github.com/bryanbarton525/prism/internal/graph"
	internalpolicy "github.com/bryanbarton525/prism/internal/policy"
	"github.com/bryanbarton525/prism/internal/result"
	"github.com/bryanbarton525/prism/internal/router"
	"github.com/bryanbarton525/prism/internal/skill"
	graphpkg "github.com/bryanbarton525/prism/pkg/graph"
	"github.com/bryanbarton525/prism/pkg/observe"
	policypkg "github.com/bryanbarton525/prism/pkg/policy"
)

const serverName = "prism"

// Serve starts the MCP server over stdio until the client disconnects.
func Serve(ctx context.Context, runner app.AgentRunner) error {
	return ServeWithConfig(ctx, runner, Config{})
}

type Config struct {
	Policy         *internalpolicy.Engine
	EventSink      observe.Sink
	DownstreamMCP  *downstreammcp.Client
	EventStorePath string
	RootDir        string
	SkillsDir      string
	SkillsFS       fs.FS
}

func ServeWithConfig(ctx context.Context, runner app.AgentRunner, cfg Config) error {
	srv := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    serverName,
		Version: buildinfo.Current().Version,
	}, nil)
	registerTools(srv, runner, cfg)
	return srv.Run(ctx, &mcpsdk.StdioTransport{})
}

func registerTools(srv *mcpsdk.Server, runner app.AgentRunner, cfg Config) {
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_agents",
		Description: "List active Prism agents and the complete active/inactive extension catalog with recovery diagnostics.",
	}, listAgentsHandler(runner))

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "run_agent",
		Description: "Invoke a specialist agent with a bounded task; skill_names may be empty for managed agents.",
	}, runAgentHandler(runner, cfg))

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
		Name:        "get_usage_summary",
		Description: "Summarize local Prism usage from the event store.",
	}, usageSummaryHandler(cfg))

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_skill_health",
		Description: "Return structural health for local Prism skills.",
	}, skillHealthHandler(cfg))

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_skill_resources",
		Description: "List bounded resources packaged with one active Prism skill.",
	}, listSkillResourcesHandler(cfg))

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "read_skill_resource",
		Description: "Read a bounded UTF-8 resource packaged with one active Prism skill.",
	}, readSkillResourceHandler(cfg))

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

type ListSkillResourcesInput struct {
	SkillName string `json:"skill_name"`
}

type ListSkillResourcesOutput struct {
	SkillName string                `json:"skill_name"`
	Resources []skill.ResourceEntry `json:"resources"`
}

func listSkillResourcesHandler(cfg Config) func(context.Context, *mcpsdk.CallToolRequest, ListSkillResourcesInput) (*mcpsdk.CallToolResult, ListSkillResourcesOutput, error) {
	return func(_ context.Context, _ *mcpsdk.CallToolRequest, input ListSkillResourcesInput) (*mcpsdk.CallToolResult, ListSkillResourcesOutput, error) {
		if cfg.SkillsFS == nil {
			return nil, ListSkillResourcesOutput{}, fmt.Errorf("skills filesystem is unavailable")
		}
		entries, err := skill.ListResources(cfg.SkillsFS, input.SkillName)
		return nil, ListSkillResourcesOutput{SkillName: input.SkillName, Resources: entries}, err
	}
}

type ReadSkillResourceInput struct {
	SkillName string `json:"skill_name"`
	Path      string `json:"path"`
	Offset    int64  `json:"offset,omitempty"`
	Limit     int64  `json:"limit,omitempty"`
}

func readSkillResourceHandler(cfg Config) func(context.Context, *mcpsdk.CallToolRequest, ReadSkillResourceInput) (*mcpsdk.CallToolResult, skill.ReadResourceResult, error) {
	return func(_ context.Context, _ *mcpsdk.CallToolRequest, input ReadSkillResourceInput) (*mcpsdk.CallToolResult, skill.ReadResourceResult, error) {
		if cfg.SkillsFS == nil {
			return nil, skill.ReadResourceResult{}, fmt.Errorf("skills filesystem is unavailable")
		}
		out, err := skill.ReadResource(cfg.SkillsFS, input.SkillName, input.Path, skill.ReadResourceOptions{Offset: input.Offset, Limit: input.Limit, MaxReadBytes: 32 * 1024})
		return nil, out, err
	}
}

type ListAgentsInput struct{}

type ListAgentsOutput struct {
	Agents  []agent.Summary          `json:"agents"`
	Catalog []extensions.CatalogItem `json:"extension_catalog,omitempty"`
	Count   int                      `json:"count"`
}

func listAgentsHandler(runner app.AgentRunner) func(context.Context, *mcpsdk.CallToolRequest, ListAgentsInput) (*mcpsdk.CallToolResult, ListAgentsOutput, error) {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ ListAgentsInput) (*mcpsdk.CallToolResult, ListAgentsOutput, error) {
		agents, err := runner.ListAgents(ctx)
		if err != nil {
			return nil, ListAgentsOutput{}, err
		}
		out := ListAgentsOutput{Agents: agents, Count: len(agents)}
		if concrete, ok := runner.(interface {
			CatalogSnapshot() extensions.CatalogSnapshot
		}); ok {
			out.Catalog = append([]extensions.CatalogItem{}, concrete.CatalogSnapshot().Agents...)
		}
		return textResult(marshalJSON(out)), out, nil
	}
}

type RunAgentInput struct {
	AgentID        string          `json:"agent_id"`
	Task           string          `json:"task"`
	SkillNames     []string        `json:"skill_names,omitempty"`
	SkillResources []string        `json:"skill_resources,omitempty"`
	Format         string          `json:"format,omitempty"`
	Workspace      *WorkspaceInput `json:"workspace,omitempty"`
}

type WorkspaceInput struct {
	Root                  string `json:"root"`
	GenerationFingerprint string `json:"generation_fingerprint,omitempty"`
}

func runAgentHandler(runner app.AgentRunner, cfg Config) func(context.Context, *mcpsdk.CallToolRequest, RunAgentInput) (*mcpsdk.CallToolResult, result.RunResult, error) {
	return func(ctx context.Context, request *mcpsdk.CallToolRequest, input RunAgentInput) (*mcpsdk.CallToolResult, result.RunResult, error) {
		if input.AgentID == "" {
			return nil, result.RunResult{}, fmt.Errorf("run_agent: agent_id is required")
		}
		if input.Task == "" {
			return nil, result.RunResult{}, fmt.Errorf("run_agent: task is required")
		}
		format := input.Format
		if format == "" {
			format = "json"
		}
		workspaceRoot := ""
		if input.Workspace != nil || runnerRequiresWorkspace(runner, input.AgentID) {
			var err error
			workspaceRoot, err = resolveWorkspace(ctx, request, input.Workspace, cfg.RootDir)
			if err != nil {
				return nil, result.RunResult{}, err
			}
		}
		generationFingerprint := ""
		if input.Workspace != nil {
			generationFingerprint = input.Workspace.GenerationFingerprint
		}
		res, err := runner.Run(ctx, app.RunRequest{
			AgentID:        input.AgentID,
			Task:           input.Task,
			SkillNames:     input.SkillNames,
			SkillResources: input.SkillResources,
			Format:         format,
			Metadata:       observe.Metadata{Source: "mcp"},
			Workspace: app.Workspace{
				Root:                  workspaceRoot,
				GenerationFingerprint: generationFingerprint,
			},
		})
		if err != nil {
			return nil, result.RunResult{}, err
		}
		out := textResult(marshalJSON(res))
		// Failed runs come back as envelopes (status error/timeout/
		// validation_fail) rather than Go errors so observability still sees
		// them; mark the MCP result so the host model doesn't read the
		// envelope as successful specialist evidence.
		out.IsError = res.Status != result.StatusOK
		return out, res, nil
	}
}

func runnerRequiresWorkspace(runner app.AgentRunner, agentID string) bool {
	type requirement interface {
		RequiresWorkspace(string) (bool, error)
	}
	if aware, ok := runner.(requirement); ok {
		required, err := aware.RequiresWorkspace(agentID)
		return err != nil || required
	}
	return true
}

func resolveWorkspace(ctx context.Context, request *mcpsdk.CallToolRequest, explicit *WorkspaceInput, fallback string) (string, error) {
	if explicit != nil {
		if strings.TrimSpace(explicit.Root) == "" {
			return "", fmt.Errorf("run_agent: workspace.root must not be empty")
		}
		return canonicalWorkspace(explicit.Root)
	}
	if request != nil && request.Session != nil {
		listed, err := request.Session.ListRoots(ctx, nil)
		if err == nil && listed != nil {
			switch len(listed.Roots) {
			case 1:
				parsed, parseErr := url.Parse(listed.Roots[0].URI)
				if parseErr != nil || parsed.Scheme != "file" {
					return "", fmt.Errorf("run_agent: MCP root must be a local file URI")
				}
				path, err := fileURIPath(parsed)
				if err != nil {
					return "", err
				}
				return canonicalWorkspace(path)
			case 0:
			default:
				return "", fmt.Errorf("run_agent: MCP host advertised multiple roots; select one with workspace.root")
			}
		}
	}

	return fallback, nil
}

func fileURIPath(parsed *url.URL) (string, error) {
	if parsed == nil || parsed.Scheme != "file" {
		return "", fmt.Errorf("run_agent: MCP root must be a local file URI")
	}
	if parsed.Host != "" && !strings.EqualFold(parsed.Host, "localhost") {
		return "", fmt.Errorf("run_agent: MCP root file URI must not name a remote host")
	}
	path := filepath.FromSlash(parsed.Path)
	if path == "" || !filepath.IsAbs(path) {
		return "", fmt.Errorf("run_agent: MCP root file URI must contain an absolute path")
	}
	return path, nil
}

func canonicalWorkspace(root string) (string, error) {
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("run_agent: workspace.root must be an absolute local path")
	}
	clean, err := filepath.EvalSymlinks(filepath.Clean(root))
	if err != nil {
		return "", fmt.Errorf("run_agent: resolving workspace.root: %w", err)
	}
	info, err := os.Stat(clean)
	if err != nil {
		return "", fmt.Errorf("run_agent: reading workspace.root: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("run_agent: workspace.root must name a directory")
	}
	return clean, nil
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
	Task string `json:"task"`
	// omitempty keeps source optional in the inferred tool schema; the
	// handler defaults it to "mcp". Without it the SDK rejects calls that
	// omit source before the handler can apply the default.
	Source string `json:"source,omitempty"`
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
	AgentID string `json:"agent_id"`
	// omitempty keeps these optional in the inferred tool schema — a policy
	// question may involve no skills or plugins, and source defaults to
	// "mcp" in the handler.
	Skills  []string `json:"skills,omitempty"`
	Plugins []string `json:"plugins,omitempty"`
	Source  string   `json:"source,omitempty"`
}

type ListPoliciesInput struct{}

type ListPoliciesOutput struct {
	Configured bool   `json:"configured"`
	Reason     string `json:"reason"`
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
		fsys := cfg.SkillsFS
		if fsys == nil && cfg.SkillsDir != "" {
			fsys = os.DirFS(cfg.SkillsDir)
		}
		if fsys == nil {
			fsys, _ = fs.Sub(prismbundle.BundleFS(), "skills")
		}
		items, err := collectSkillHealthFS(fsys, input.SkillName)
		if err != nil {
			return nil, SkillHealthOutput{}, err
		}
		out := SkillHealthOutput{Skills: items, Count: len(items)}
		return textResult(marshalJSON(out)), out, nil
	}
}

func collectSkillHealth(root, only string) ([]SkillHealth, error) {
	return collectSkillHealthFS(os.DirFS(root), only)
}

func collectSkillHealthFS(fsys fs.FS, only string) ([]SkillHealth, error) {
	var names []string
	if only != "" {
		names = []string{only}
	} else {
		entries, err := fs.ReadDir(fsys, ".")
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				names = append(names, entry.Name())
			}
		}
	}
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
		if err := skill.ValidatePortableStructure(fsys, name); err != nil {
			item.OK = false
			item.Errors = append(item.Errors, err.Error())
		}
		item.Warnings = append(item.Warnings, skill.ExecutionLimitations(fsys, name)...)
		count, err := skill.ValidateEvals(fsys, name)
		if err != nil {
			item.Warnings = append(item.Warnings, "eval validation unavailable: "+err.Error())
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
	// Total is the number of tools the downstream server actually exposes;
	// when Truncated is true the list was cut at max_tools.
	Total     int  `json:"total"`
	Truncated bool `json:"truncated,omitempty"`
}

func listMCPServerToolsHandler(client *downstreammcp.Client) func(context.Context, *mcpsdk.CallToolRequest, ListMCPServerToolsInput) (*mcpsdk.CallToolResult, ListMCPServerToolsOutput, error) {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, input ListMCPServerToolsInput) (*mcpsdk.CallToolResult, ListMCPServerToolsOutput, error) {
		if client == nil {
			return nil, ListMCPServerToolsOutput{}, fmt.Errorf("downstream MCP client is not configured")
		}
		if input.Server == "" {
			return nil, ListMCPServerToolsOutput{}, fmt.Errorf("list_mcp_server_tools: server is required")
		}
		res, err := client.ListTools(ctx, input.Server, downstreammcp.ListToolsOptions{IncludeSchema: input.IncludeSchema, MaxTools: input.MaxTools})
		if err != nil {
			return nil, ListMCPServerToolsOutput{}, err
		}
		out := ListMCPServerToolsOutput{Server: input.Server, Tools: res.Tools, Count: len(res.Tools), Total: res.Total, Truncated: res.Truncated}
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
		out := textResult(marshalJSON(res))
		// Surface downstream tool failures on the MCP result itself so hosts
		// (and their models) that key off isError do not treat the payload as
		// successful evidence.
		out.IsError = res.IsError
		return out, res, nil
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
	managed := 0
	if concrete, ok := runner.(interface {
		CatalogSnapshot() extensions.CatalogSnapshot
	}); ok {
		snapshot := concrete.CatalogSnapshot()
		for _, item := range snapshot.Agents {
			if item.Origin == "managed" && item.Active {
				managed++
			}
		}
	}
	if managed == 0 {
		return fmt.Sprintf("prism MCP server ready: %d agent(s) [%s]", len(ids), strings.Join(ids, ", "))
	}
	return fmt.Sprintf("prism MCP server ready: %d agent(s) [%s] + %d managed extension agent(s)", len(ids), strings.Join(ids, ", "), managed)
}
