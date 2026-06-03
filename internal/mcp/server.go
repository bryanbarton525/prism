// Package mcp provides the MCP server adapter for Prism.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bryanbarton525/prism/internal/agent"
	"github.com/bryanbarton525/prism/internal/app"
	"github.com/bryanbarton525/prism/internal/result"
	"github.com/bryanbarton525/prism/pkg/observe"
)

const (
	serverName    = "prism"
	serverVersion = "v0.1.0"
)

// Serve starts the MCP server over stdio until the client disconnects.
func Serve(ctx context.Context, runner app.AgentRunner) error {
	srv := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    serverName,
		Version: serverVersion,
	}, nil)
	registerTools(srv, runner)
	return srv.Run(ctx, &mcpsdk.StdioTransport{})
}

func registerTools(srv *mcpsdk.Server, runner app.AgentRunner) {
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
	AgentID    string   `json:"agent_id"`
	Task       string   `json:"task"`
	SkillNames []string `json:"skill_names"`
	Format     string   `json:"format"`
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
			AgentID:    input.AgentID,
			Task:       input.Task,
			SkillNames: input.SkillNames,
			Format:     format,
			Metadata:   observe.Metadata{Source: "mcp"},
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
