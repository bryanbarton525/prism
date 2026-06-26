## Prism

Prism is a local-first specialist agent runner. It delegates focused, evidence-heavy subtasks to bounded local agents (served by Ollama or an OpenAI-compatible runtime such as SGLang) and returns compact, auditable summaries.

Use Prism when a task benefits from a dedicated specialist that owns a bulky tool or context surface (Kubernetes, GitHub, Go scaffolding, Linear, docs search, downstream MCP servers) instead of loading everything into the parent model.
<!-- END_INTRO -->

### CLI

- `prism agent list` — list available specialist agents.
- `prism run --agent <id> --skill <skill> --input "<task>"` — run one specialist with a required skill and a short task brief.
- `prism doctor` — check runtime, registry, and skill health.

### MCP

Prism also runs as an MCP server (`prism mcp serve`, stdio). Core tools:

- `list_agents`, `run_agent`, `get_constitution`, `doctor`
- `suggest_route`, `run_graph`, `explain_policy`, `list_policies`
- `list_mcp_servers`, `list_mcp_server_tools`, `call_mcp_tool`

### Workflow

1. Paste a short brief — do not paste every skill and all evidence into chat.
2. Delegate evidence-heavy subtasks to Prism specialists via `run_agent`.
3. Synthesize their compact summaries; rely on returned evidence artifacts for proof of any downstream action.

See README.md and docs/usage.md for setup, configuration, and flags.
