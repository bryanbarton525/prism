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
2. Pick agent(s) and valid skills.
3. Shape narrow tasks with scoped evidence.
4. Run specialist calls.
5. Synthesize in parent with confidence + next action.
