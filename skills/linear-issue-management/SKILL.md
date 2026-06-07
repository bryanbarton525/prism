---
name: linear-issue-management
description: Plan, create, edit, comment on, and triage Linear issues through the Linear MCP server. Trigger on Linear tickets, issues, teams, projects, cycles, labels, roadmap, or backlog work.
compatibility: Requires an authenticated downstream Linear MCP server for live reads or writes. When the agent declares the mcp runtime tool, Prism can let the local model list and call configured downstream MCP tools during the run.
metadata:
  prism-agents: linear
---

# Linear issue management

## Inputs expected

- Operation: search, create issue, update issue, add comment, assign, label, move to project/cycle, close/archive, or triage.
- Workspace context: team key/name, project, cycle, issue key, assignee, labels, priority, and status when relevant.
- For create operations: title, team, and description or enough source context to draft one.
- For edit/comment/archive operations: issue key or issue UUID plus the exact fields to change.
- Approval evidence for requested writes, if the parent has already collected it.

## Workflow

1. Identify whether the request is read-only or write-oriented.
2. Use attached runtime evidence to report Linear MCP setup context, configured downstream MCP servers, and issue keys inferred from the task.
3. For read-only work, use Prism MCP bridge tools against the configured `linear` downstream server: list tools, then search teams, issues, projects, labels, cycles, or users.
4. For create work, build a minimal issue payload with title, team, description, and optional assignee, priority, labels, project, cycle, and parent issue.
5. For edit work, require a target issue key or UUID and list only the fields being changed.
6. For triage work, search before mutating: dedupe likely duplicates, summarize current state, then propose comments/status/label changes.
7. If Linear MCP response evidence is supplied from `call_mcp_tool`, summarize the actual result and include the response artifact. Otherwise, mark write actions as proposed.

## Linear MCP call artifact shape

When a write still needs approval or exact schema resolution, return proposed MCP operations as artifacts with this JSON shape:

```json
{
  "type": "linear_mcp_call",
  "server": "prism",
  "tool": "call_mcp_tool",
  "arguments": {
    "server": "linear",
    "tool": "create_issue",
    "arguments": {
      "team": "TEAM",
      "title": "Short actionable title",
      "description": "Evidence-backed context and acceptance notes"
    }
  },
  "approval_required": true,
  "status": "proposed"
}
```

Use the exact tool names exposed by `list_mcp_server_tools` when available. If the exact tool schema is not present in evidence, ask the parent to call Prism MCP `list_mcp_server_tools` for `server: "linear"` before executing a write.

## Output requirements

- `summary`: one sentence verdict or proposed action.
- `findings`: concise evidence-backed observations.
- `linear_actions`: split into `proposed`, `executed`, and `blocked`.
- `artifacts`: include proposed Linear MCP calls and any attached Linear MCP response evidence.
- `confidence`: high only when identifiers and Linear MCP evidence are complete.
- `next_action`: exact missing approval, identifier, or Linear MCP call the parent should perform.

## Guardrails

- Never invent Linear IDs, team keys, user handles, labels, or project names.
- Do not claim writes were performed unless a Linear MCP response artifact proves it.
- For destructive or workflow-changing operations, require explicit approval from the parent/user.
- Keep issue descriptions actionable and source-backed; avoid private secrets or unrelated logs.
