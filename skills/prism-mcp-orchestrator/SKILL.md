---
name: prism-mcp-orchestrator
description: >-
  Auto-use for context-heavy prompts (logs, CI failures, Kubernetes issues,
  runbooks, docs lookup, PR triage, codegen subtasks). Decide when to delegate
  via Prism MCP and execute the required call order — list_agents, list_resources,
  get_resource, optional list_prompts/get_prompt, run_agent, then parent synthesis.
compatibility: Requires Prism MCP server availability, local repository context, and an MCP-capable host editor/agent.
metadata:
  prism-agents: orchestrator parent-model
---

# Prism MCP orchestrator skill

Use this skill for orchestration only (not domain diagnosis). If the task is evidence-heavy or specialist-shaped, delegate via Prism MCP.

## Activation rule

Activate when prompts mention things like: CI failures, PR blockers, Kubernetes incidents, Argo drift/workflow failures, release notes/docs lookup, large logs/JSON, or "delegate/run_agent/MCP".

Do **not** activate for tiny single-step tasks where delegation overhead is not worth it.

## Required sequence (must follow)

1. Call `list_agents` first.
2. Call `list_resources`, then `get_resource` for:
   - `prism://resource/tooling/run_agent`
   - `prism://resource/tooling/orchestration-guide`
   - `prism://resource/agent/<agent_id>/constitution` (for selected agents)
3. Optional: `list_prompts` + `get_prompt` for payload templates.
4. Call `run_agent` with bounded task + valid `skill_names`.
5. Synthesize specialist outputs in parent model.

## run_agent shaping

- Keep tasks narrow and evidence-oriented.
- Attach only relevant evidence per specialist.
- Prefer multiple focused calls over one mixed-domain call.
- Keep final judgment/tradeoffs/recommendation in parent synthesis.

## Minimal call template

```json
{
  "agent_id": "github-cli",
  "skill_names": ["gh-pr-triage"],
  "task": "Repository owner/repo. Triage PR #42 for merge blockers with evidence.",
  "format": "json"
}
```

## Parent output contract

- Concise synthesis (no raw dumps)
- Ordered findings with evidence
- Confidence + concrete next action
