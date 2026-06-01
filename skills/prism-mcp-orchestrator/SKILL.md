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

## Fail-closed evidence gate

Before delegation, check for minimum identifiers:

- GitHub: `repo` + (`PR`, `run URL`, or `run ID`)
- Kubernetes: `context` + `namespace` + workload (`deployment`/`statefulset`/`pod`)
- Release notes/docs: product/library + version range or target release

If required identifiers are missing, **do not run specialists yet**. Ask the parent LLM to request only missing fields.

## First-party skill precedence

When a subtask targets a specific platform/tool/vendor, prefer that vendor's
first-party skills/docs before community or generic sources.

Examples:

- LaunchDarkly tasks -> prefer LaunchDarkly first-party skills (for example from
  the LaunchDarkly `ai-tooling/skills` source) and official docs.
- Cloud/provider SDK tasks -> prefer official provider skills/docs first.

If a first-party source exists but is not available in the current environment,
callback to parent with `insufficient_evidence` and request permission/inputs to
access the first-party source.

## Required sequence (must follow)

1. Call `list_agents` first.
2. Call `list_resources`, then `get_resource` for:
   - `prism://resource/tooling/run_agent`
   - `prism://resource/tooling/orchestration-guide`
   - `prism://resource/agent/<agent_id>/constitution` (for selected agents)
3. Optional: `list_prompts` + `get_prompt` for payload templates.
4. Call `run_agent` with bounded task + valid `skill_names`.
5. Synthesize specialist outputs in parent model.

If evidence is insufficient at any step, callback to parent with:

```json
{
  "status": "insufficient_evidence",
  "needs_from_parent": [
    "repo owner/name",
    "PR URL or workflow run URL",
    "namespace/deployment"
  ],
  "reason": "Identifiers required for evidence-backed delegation are missing."
}
```

## run_agent shaping

- Keep tasks narrow and evidence-oriented.
- Attach only relevant evidence per specialist.
- Prefer multiple focused calls over one mixed-domain call.
- Keep final judgment/tradeoffs/recommendation in parent synthesis.
- For tool-specific tasks, include first-party source preference in the task
  text so specialists ground findings in vendor-authored guidance.

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
- If evidence is missing: return `insufficient_evidence` with required follow-up fields, not speculative findings
