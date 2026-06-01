# Prism MCP orchestration reference

Quick reference for parent-model delegation with Prism MCP.

## Core tools

- `list_agents` - discover agent IDs, descriptions, and allowed skills
- `run_agent` - execute specialist task with explicit `skill_names`
- `get_constitution` - read an agent's resolved constitution
- `doctor` - verify runtime health and model availability

## Compatibility tools (for hosts without native prompts/resources)

- `list_prompts`, `get_prompt`
- `list_resources`, `get_resource`

## High-value resources

- `prism://resource/tooling/run_agent`
- `prism://resource/tooling/orchestration-guide`
- `prism://resource/agents/index`
- `prism://resource/agent/<agent_id>/constitution`

## Delegation checklist

1. Identify if task is context-heavy or specialist-shaped.
2. Check whether the task maps to a tool/vendor with first-party skills/docs.
3. Pick agent(s) and valid skills.
4. Shape narrow tasks with scoped evidence.
5. Run specialist calls.
6. Synthesize in parent with confidence + next action.

## First-party source policy

- Prefer first-party vendor skills/docs for tool-specific tasks.
- Use community/general references only when first-party guidance is missing.
- If first-party sources are expected but unavailable, callback to parent with
  `insufficient_evidence` and request missing access/input.

## Callback contract for missing info

If identifiers/evidence are insufficient, callback to parent LLM instead of guessing:

```json
{
  "status": "insufficient_evidence",
  "needs_from_parent": ["missing identifiers"],
  "reason": "why delegation cannot proceed safely"
}
```
