// Package mcp provides the MCP server adapter for prism. It exposes the shared
// AgentRunner as MCP tools, keeping request/response schemas aligned with the
// CLI wherever possible.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bryanbarton525/prism/internal/app"
	"github.com/bryanbarton525/prism/internal/result"
)

const (
	serverName    = "prism"
	serverVersion = "v0.1.0"
)

// Serve starts the MCP server and blocks until the client disconnects or the
// context is cancelled. It communicates with the MCP host over stdio.
func Serve(ctx context.Context, runner app.AgentRunner) error {
	srv := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    serverName,
		Version: serverVersion,
	}, nil)

	registerTools(srv, runner)

	return srv.Run(ctx, &mcpsdk.StdioTransport{})
}

// registerTools wires all four MCP tools to the server.
func registerTools(srv *mcpsdk.Server, runner app.AgentRunner) {
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_agents",
		Description: "List all registered Prism agents with their IDs, descriptions, model hints, allowed skills, and latency budgets.",
	}, listAgentsHandler(runner))

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "run_agent",
		Description: "Invoke a specialist agent with a bounded task and required skill names. Returns a normalized result envelope with summary, findings, confidence, and usage metrics.",
	}, runAgentHandler(runner))

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_constitution",
		Description: "Return the full constitution text for an agent for auditability and prompt inspection.",
	}, getConstitutionHandler(runner))

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "doctor",
		Description: "Report Ollama connectivity, model availability, agent count, and configuration state.",
	}, doctorHandler(runner))
}

// --- list_agents ---

// ListAgentsInput is the (empty) parameter struct for list_agents.
type ListAgentsInput struct{}

// ListAgentsOutput is the result of the list_agents tool.
type ListAgentsOutput struct {
	Agents []result.AgentSummary `json:"agents"`
	Count  int                   `json:"count"`
}

func listAgentsHandler(runner app.AgentRunner) func(context.Context, *mcpsdk.CallToolRequest, ListAgentsInput) (*mcpsdk.CallToolResult, ListAgentsOutput, error) {
	return func(ctx context.Context, req *mcpsdk.CallToolRequest, _ ListAgentsInput) (*mcpsdk.CallToolResult, ListAgentsOutput, error) {
		agents, err := runner.ListAgents(ctx)
		if err != nil {
			return nil, ListAgentsOutput{}, fmt.Errorf("list_agents: %w", err)
		}
		out := ListAgentsOutput{Agents: agents, Count: len(agents)}
		return textResult(marshalJSON(out)), out, nil
	}
}

// --- run_agent ---

// RunAgentInput maps to the run_agent tool parameters.
type RunAgentInput struct {
	AgentID    string   `json:"agent_id"    jsonschema:"the ID of the agent to invoke"`
	Task       string   `json:"task"        jsonschema:"the task or question to delegate to the agent"`
	SkillNames []string `json:"skill_names" jsonschema:"skill names to attach; must be a subset of the agent allowed_skills"`
	Format     string   `json:"format"      jsonschema:"output format: json or markdown (default: json)"`
}

func runAgentHandler(runner app.AgentRunner) func(context.Context, *mcpsdk.CallToolRequest, RunAgentInput) (*mcpsdk.CallToolResult, *result.RunResult, error) {
	return func(ctx context.Context, req *mcpsdk.CallToolRequest, input RunAgentInput) (*mcpsdk.CallToolResult, *result.RunResult, error) {
		if input.AgentID == "" {
			return nil, nil, fmt.Errorf("run_agent: agent_id is required")
		}
		if input.Task == "" {
			return nil, nil, fmt.Errorf("run_agent: task is required")
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
		})
		if err != nil {
			return nil, nil, fmt.Errorf("run_agent: %w", err)
		}
		return textResult(marshalJSON(res)), res, nil
	}
}

// --- get_constitution ---

// GetConstitutionInput maps to the get_constitution tool parameters.
type GetConstitutionInput struct {
	AgentID string `json:"agent_id" jsonschema:"the agent ID whose constitution to retrieve"`
}

func getConstitutionHandler(runner app.AgentRunner) func(context.Context, *mcpsdk.CallToolRequest, GetConstitutionInput) (*mcpsdk.CallToolResult, *result.Constitution, error) {
	return func(ctx context.Context, req *mcpsdk.CallToolRequest, input GetConstitutionInput) (*mcpsdk.CallToolResult, *result.Constitution, error) {
		if input.AgentID == "" {
			return nil, nil, fmt.Errorf("get_constitution: agent_id is required")
		}
		constitution, err := runner.GetConstitution(ctx, input.AgentID)
		if err != nil {
			return nil, nil, fmt.Errorf("get_constitution: %w", err)
		}
		return textResult(marshalJSON(constitution)), constitution, nil
	}
}

// --- doctor ---

// DoctorInput is the (empty) parameter struct for the doctor tool.
type DoctorInput struct{}

func doctorHandler(runner app.AgentRunner) func(context.Context, *mcpsdk.CallToolRequest, DoctorInput) (*mcpsdk.CallToolResult, *result.DoctorResult, error) {
	return func(ctx context.Context, req *mcpsdk.CallToolRequest, _ DoctorInput) (*mcpsdk.CallToolResult, *result.DoctorResult, error) {
		dr, err := runner.Doctor(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("doctor: %w", err)
		}
		return textResult(marshalJSON(dr)), dr, nil
	}
}

// --- helpers ---

func marshalJSON(v any) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": %q}`, err.Error())
	}
	return string(data)
}

func textResult(text string) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: text},
		},
	}
}

// StatusSummary returns a single-line status string suitable for log output on startup.
func StatusSummary(runner app.AgentRunner) string {
	agents, err := runner.ListAgents(context.Background())
	if err != nil {
		return fmt.Sprintf("prism MCP server starting (registry error: %v)", err)
	}
	ids := make([]string, 0, len(agents))
	for _, a := range agents {
		ids = append(ids, a.ID)
	}
	return fmt.Sprintf("prism MCP server ready: %d agent(s) [%s]", len(ids), strings.Join(ids, ", "))
}
