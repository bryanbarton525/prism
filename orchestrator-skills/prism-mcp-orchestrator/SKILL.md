---
name: prism-mcp-orchestrator
description: Instruct the parent LLM when to delegate through Prism MCP and how to execute the correct call sequence using list_agents, list_prompts/get_prompt, list_resources/get_resource, and run_agent.
compatibility: Requires Prism MCP server availability, local repository context, and an MCP-capable host editor/agent.
metadata:
  prism-agents: orchestrator parent-model
---

# Prism MCP orchestrator skill

## Purpose

Use this skill in the parent/orchestrator model to decide **when** delegation is worth it and **how** to call Prism tools correctly.

This skill is for orchestration behavior, not domain diagnosis itself.

## When to use

Use when any of the following is true:

- The task includes heavy evidence (logs, runbooks, docs, chat exports, JSON blobs).
- The task naturally splits across specialists (GitHub, Kubernetes, Argo, docs, Go helper/scaffold, frontend subtasking).
- You need source-backed diagnostics before final synthesis.
- You want to reduce parent-model context bloat while preserving quality.

## Do not use when

- The task is tiny and single-step.
- Delegation overhead exceeds expected value.

## Required MCP tool sequence

1. `list_agents`
   - Discover candidate agents and valid `allowed_skills`.
2. `list_resources` then `get_resource`
   - Fetch `prism://resource/tooling/run_agent`
   - Fetch `prism://resource/tooling/orchestration-guide`
   - Fetch `prism://resource/agent/<agent_id>/constitution` for selected agent(s)
3. Optional templates
   - `list_prompts`, then `get_prompt` for a call starter.
4. Execute delegation
   - Call `run_agent` with bounded task and valid `skill_names`.
5. Parent synthesis
   - Aggregate specialist summaries and produce final judgment in parent model.

## run_agent shaping rules

- Keep the task bounded and evidence-oriented.
- Attach only the evidence needed for that specialist.
- Prefer multiple narrow calls over one mixed-domain call.
- Keep user-facing recommendations and tradeoffs in parent synthesis.

## Minimum call template

```json
{
  "agent_id": "github-cli",
  "skill_names": ["gh-pr-triage"],
  "task": "Repository owner/repo. Triage PR #42 for merge blockers with evidence.",
  "format": "json"
}
```

## Parent output contract

- Concise synthesis (no raw log dump).
- Ordered findings with evidence.
- Confidence and specific next actions.
