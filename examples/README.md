# Prism example configs

Sample configuration files for running Prism. Copy the ones you need into
`~/.prism/` (or point the matching env var at them) and edit the values.

| File | Purpose | How Prism finds it |
| --- | --- | --- |
| [`mcp-servers.yaml`](mcp-servers.yaml) | Downstream MCP servers Prism can call on behalf of agents | `~/.prism/mcp-servers.yaml` (managed via `prism mcp server ...`) |
| [`config.env`](config.env) | Process settings: root, model runtime engine, tokens, policy path | `PRISM_CONFIG_FILE` (or `~/.prism/config.env`) |
| [`prism-policy.json`](prism-policy.json) | Bounded allow/deny policy for agents, sources, workspaces, bundles | `PRISM_POLICY_FILE` |

## Quick start

```bash
mkdir -p ~/.prism
cp examples/mcp-servers.yaml ~/.prism/mcp-servers.yaml
cp examples/config.env       ~/.prism/config.env
cp examples/prism-policy.json ~/.prism/prism-policy.json

export PRISM_CONFIG_FILE="$HOME/.prism/config.env"
```

Notes:

- The model runtime **engine** (`ollama` vs `sglang`) is a global setting, not
  per-agent. Every agent's declared model is served by whichever engine is
  configured here.
- Downstream MCP servers are shared across all agents; an agent only reaches
  them if its spec declares `tools: [mcp]`.
- Per-server `timeout_ms` (default 30000) and `max_bytes` (default 20000) keep
  downstream surfaces bounded.
