package mcp

import (
	"context"
	"fmt"
	"sort"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bryanbarton525/prism/internal/agent"
	"github.com/bryanbarton525/prism/internal/app"
)

// PromptTemplate describes an LLM-callable prompt helper exposed as an MCP tool.
type PromptTemplate struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Variables   []string `json:"variables,omitempty"`
}

type ListPromptsInput struct{}

type ListPromptsOutput struct {
	Prompts []PromptTemplate `json:"prompts"`
	Count   int              `json:"count"`
}

type GetPromptInput struct {
	PromptID  string            `json:"prompt_id"`
	Variables map[string]string `json:"variables,omitempty"`
}

type GetPromptOutput struct {
	PromptID string `json:"prompt_id"`
	Title    string `json:"title"`
	Prompt   string `json:"prompt"`
}

type MCPResource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MimeType    string `json:"mime_type"`
}

type ListResourcesInput struct{}

type ListResourcesOutput struct {
	Resources []MCPResource `json:"resources"`
	Count     int           `json:"count"`
}

type GetResourceInput struct {
	URI string `json:"uri"`
}

type GetResourceOutput struct {
	URI      string `json:"uri"`
	Name     string `json:"name"`
	MimeType string `json:"mime_type"`
	Content  string `json:"content"`
}

func promptCatalog() []PromptTemplate {
	return []PromptTemplate{
		{
			ID:          "run_agent_json_call",
			Title:       "Run agent tool call (JSON)",
			Description: "Build a correct `run_agent` tool call payload with required fields.",
			Variables:   []string{"agent_id", "skill_names_csv", "task"},
		},
		{
			ID:          "github_pr_triage",
			Title:       "GitHub PR triage delegation",
			Description: "Template to triage PR health and blockers via github-cli.",
			Variables:   []string{"pr_ref", "repo"},
		},
		{
			ID:          "k8s_incident_triage",
			Title:       "Kubernetes incident triage delegation",
			Description: "Template to diagnose CrashLoopBackOff/rollout incidents via kubectl.",
			Variables:   []string{"namespace", "symptoms"},
		},
		{
			ID:          "argo_failure_debug",
			Title:       "Argo sync/workflow failure delegation",
			Description: "Template to diagnose OutOfSync, degraded apps, or workflow failures.",
			Variables:   []string{"app_or_workflow", "namespace", "symptoms"},
		},
		{
			ID:          "go_codegen_helper",
			Title:       "Go helper codegen delegation",
			Description: "Template to offload small helper implementation or test scaffold tasks.",
			Variables:   []string{"package_path", "request"},
		},
	}
}

func listPromptsHandler(_ app.AgentRunner) func(context.Context, *mcpsdk.CallToolRequest, ListPromptsInput) (*mcpsdk.CallToolResult, ListPromptsOutput, error) {
	return func(_ context.Context, _ *mcpsdk.CallToolRequest, _ ListPromptsInput) (*mcpsdk.CallToolResult, ListPromptsOutput, error) {
		catalog := promptCatalog()
		out := ListPromptsOutput{Prompts: catalog, Count: len(catalog)}
		return textResult(marshalJSON(out)), out, nil
	}
}

func getPromptHandler(_ app.AgentRunner) func(context.Context, *mcpsdk.CallToolRequest, GetPromptInput) (*mcpsdk.CallToolResult, GetPromptOutput, error) {
	return func(_ context.Context, _ *mcpsdk.CallToolRequest, input GetPromptInput) (*mcpsdk.CallToolResult, GetPromptOutput, error) {
		if strings.TrimSpace(input.PromptID) == "" {
			return nil, GetPromptOutput{}, fmt.Errorf("get_prompt: prompt_id is required")
		}
		title, prompt, err := renderPrompt(input.PromptID, input.Variables)
		if err != nil {
			return nil, GetPromptOutput{}, err
		}
		out := GetPromptOutput{
			PromptID: input.PromptID,
			Title:    title,
			Prompt:   prompt,
		}
		return textResult(marshalJSON(out)), out, nil
	}
}

func renderPrompt(id string, vars map[string]string) (title, prompt string, err error) {
	get := func(key, def string) string {
		if v, ok := vars[key]; ok && strings.TrimSpace(v) != "" {
			return v
		}
		return def
	}
	switch id {
	case "run_agent_json_call":
		agentID := get("agent_id", "github-cli")
		skillsCSV := get("skill_names_csv", "gh-pr-triage")
		task := get("task", "Summarize PR #42 merge blockers and failing checks.")
		title = "Run agent tool call (JSON)"
		prompt = fmt.Sprintf(
			"Call Prism MCP tool `run_agent` with JSON:\n\n{\n  \"agent_id\": %q,\n  \"skill_names\": [%s],\n  \"task\": %q,\n  \"format\": \"json\"\n}\n\nRules:\n- `skill_names` must be allowed by the target agent.\n- Provide bounded, evidence-oriented tasks.\n- Keep orchestration and final judgment in the parent model.",
			agentID, quoteCSV(skillsCSV), task,
		)
		return title, prompt, nil
	case "github_pr_triage":
		pr := get("pr_ref", "#42")
		repo := get("repo", "owner/repo")
		title = "GitHub PR triage delegation"
		prompt = fmt.Sprintf(
			"Use Prism `run_agent` for GitHub triage.\n\nagent_id: \"github-cli\"\nskill_names: [\"gh-pr-triage\"]\ntask: \"Repository %s. Triage PR %s for merge readiness. Include check status, review blockers, risky changed files, and confidence.\"",
			repo, pr,
		)
		return title, prompt, nil
	case "k8s_incident_triage":
		ns := get("namespace", "payments")
		symptoms := get("symptoms", "CrashLoopBackOff and rollout stalled")
		title = "Kubernetes incident triage delegation"
		prompt = fmt.Sprintf(
			"Use Prism `run_agent` for Kubernetes triage.\n\nagent_id: \"kubectl\"\nskill_names: [\"kubectl-triage\",\"k8s-rollout-diagnostics\"]\ntask: \"Namespace %s. Diagnose incident: %s. Return evidence-backed findings, likely causes, and safe next checks.\"",
			ns, symptoms,
		)
		return title, prompt, nil
	case "argo_failure_debug":
		target := get("app_or_workflow", "payments-api / nightly-etl")
		ns := get("namespace", "payments")
		symptoms := get("symptoms", "OutOfSync, Degraded, workflow extract failure")
		title = "Argo sync/workflow failure delegation"
		prompt = fmt.Sprintf(
			"Use Prism `run_agent` for Argo diagnostics.\n\nagent_id: \"argo\"\nskill_names: [\"argo-sync-health\",\"argo-workflow-debug\"]\ntask: \"Namespace %s. Target %s. Symptoms: %s. Separate Argo sync drift from workflow runtime failure and include confidence.\"",
			ns, target, symptoms,
		)
		return title, prompt, nil
	case "go_codegen_helper":
		pkg := get("package_path", "internal/metrics")
		req := get("request", "Implement a ParseLabels helper and suggest focused tests.")
		title = "Go helper codegen delegation"
		prompt = fmt.Sprintf(
			"Use Prism `run_agent` for bounded codegen.\n\nagent_id: \"go-helper\"\nskill_names: [\"go-helper-fn\"]\ntask: \"Package %s. %s Return compact summary/findings and full code in artifacts.\"",
			pkg, req,
		)
		return title, prompt, nil
	default:
		return "", "", fmt.Errorf("get_prompt: unknown prompt_id %q", id)
	}
}

func quoteCSV(csv string) string {
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, fmt.Sprintf("%q", p))
		}
	}
	if len(out) == 0 {
		return `"gh-pr-triage"`
	}
	return strings.Join(out, ", ")
}

func listResourcesHandler(runner app.AgentRunner) func(context.Context, *mcpsdk.CallToolRequest, ListResourcesInput) (*mcpsdk.CallToolResult, ListResourcesOutput, error) {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ ListResourcesInput) (*mcpsdk.CallToolResult, ListResourcesOutput, error) {
		rs, err := buildResourceCatalog(ctx, runner)
		if err != nil {
			return nil, ListResourcesOutput{}, err
		}
		out := ListResourcesOutput{Resources: rs, Count: len(rs)}
		return textResult(marshalJSON(out)), out, nil
	}
}

func buildResourceCatalog(ctx context.Context, runner app.AgentRunner) ([]MCPResource, error) {
	agents, err := runner.ListAgents(ctx)
	if err != nil {
		return nil, err
	}
	sort.Slice(agents, func(i, j int) bool { return agents[i].ID < agents[j].ID })

	out := []MCPResource{
		{
			URI:         "prism://resource/tooling/run_agent",
			Name:        "run_agent tool contract",
			Description: "Required fields, call rules, and examples for run_agent.",
			MimeType:    "text/markdown",
		},
		{
			URI:         "prism://resource/tooling/orchestration-guide",
			Name:        "orchestration guide",
			Description: "How to choose agents/skills and synthesize specialist outputs.",
			MimeType:    "text/markdown",
		},
		{
			URI:         "prism://resource/agents/index",
			Name:        "agents index",
			Description: "Machine-readable list of registered agents.",
			MimeType:    "application/json",
		},
	}
	for _, a := range agents {
		out = append(out, MCPResource{
			URI:         fmt.Sprintf("prism://resource/agent/%s/constitution", a.ID),
			Name:        fmt.Sprintf("%s constitution", a.ID),
			Description: "Resolved constitution text for this agent.",
			MimeType:    "text/markdown",
		})
	}
	return out, nil
}

func getResourceHandler(runner app.AgentRunner) func(context.Context, *mcpsdk.CallToolRequest, GetResourceInput) (*mcpsdk.CallToolResult, GetResourceOutput, error) {
	return func(ctx context.Context, _ *mcpsdk.CallToolRequest, input GetResourceInput) (*mcpsdk.CallToolResult, GetResourceOutput, error) {
		uri := strings.TrimSpace(input.URI)
		if uri == "" {
			return nil, GetResourceOutput{}, fmt.Errorf("get_resource: uri is required")
		}

		out, err := renderResource(ctx, runner, uri)
		if err != nil {
			return nil, GetResourceOutput{}, err
		}
		return textResult(marshalJSON(out)), out, nil
	}
}

func renderResource(ctx context.Context, runner app.AgentRunner, uri string) (GetResourceOutput, error) {
	switch uri {
	case "prism://resource/tooling/run_agent":
		return GetResourceOutput{
			URI:      uri,
			Name:     "run_agent tool contract",
			MimeType: "text/markdown",
			Content: `# run_agent contract

Required fields:
- agent_id (string)
- skill_names (array of strings)
- task (string)

Optional:
- format: "json" (default) or "markdown"

Rules:
- skill_names must be allowed by the selected agent.
- Keep task bounded and evidence-oriented.
- Delegate narrow subtasks; synthesize in the parent orchestrator.`,
		}, nil
	case "prism://resource/tooling/orchestration-guide":
		return GetResourceOutput{
			URI:      uri,
			Name:     "orchestration guide",
			MimeType: "text/markdown",
			Content: `# Prism orchestration guide

1. Start with a brief and explicit success criteria.
2. Choose specialist agent(s) by domain:
   - github-cli, kubectl, argo, web-docs-search, go-helper, go-scaffold
3. Attach only relevant skills.
4. Pass evidence in task text (logs, JSON, snippets).
5. Aggregate specialist summaries and synthesize final output in parent model.

Best practice:
- Use multiple narrow run_agent calls rather than one giant prompt.`,
		}, nil
	case "prism://resource/agents/index":
		agents, err := runner.ListAgents(ctx)
		if err != nil {
			return GetResourceOutput{}, err
		}
		return GetResourceOutput{
			URI:      uri,
			Name:     "agents index",
			MimeType: "application/json",
			Content:  marshalJSON(struct{ Agents []agent.Summary `json:"agents"` }{Agents: agents}),
		}, nil
	default:
		const prefix = "prism://resource/agent/"
		if strings.HasPrefix(uri, prefix) && strings.HasSuffix(uri, "/constitution") {
			agentID := strings.TrimSuffix(strings.TrimPrefix(uri, prefix), "/constitution")
			if agentID == "" {
				return GetResourceOutput{}, fmt.Errorf("get_resource: invalid constitution uri %q", uri)
			}
			c, err := runner.GetConstitution(ctx, agentID)
			if err != nil {
				return GetResourceOutput{}, err
			}
			return GetResourceOutput{
				URI:      uri,
				Name:     fmt.Sprintf("%s constitution", agentID),
				MimeType: "text/markdown",
				Content:  c.Text,
			}, nil
		}
	}
	return GetResourceOutput{}, fmt.Errorf("get_resource: unknown uri %q", uri)
}
