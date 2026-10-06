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

## Laya tool ranking

The optional commented section in `config.env` separates MiniLM tool retrieval,
Laya reranking, and the primary offload model runtime. For a managed CPU service:

```bash
prism install --runtime-only --runtime-scope user \
  --tool-model onnx --decision-service install-laya
```

This writes the selected state's configuration after installing pinned weights
and checking real inference. For an existing Laya endpoint use
`--decision-service laya --decision-url http://127.0.0.1:8010` instead. Use
`--dry-run` to preview without downloads, writes, service changes, or probes.
If `PRISM_CONFIG_FILE` is set, it must point to the selected state's `config.env`.

After registering and authorizing a downstream MCP server, call `recommend_tools`
from your host with `agent_id`, `task`, and optional `top_k`. Laya returns
`score_kind=laya_choice`; recommendations never execute tools or change access.
Automatic injection remains off unless you select agent identities, and its
500 ms budget is shorter than the evaluated 20-candidate CPU inference time.
Reconnect Prism MCP after changing its configuration or executable.

See [usage](../docs/usage.md#laya-tool-recommendations) for the complete workflow
and [evaluation](../docs/laya-evaluation.md) for measurements and limitations.
