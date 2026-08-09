---
name: verify
description: Build, launch, and drive prism's real surfaces (CLI, MCP stdio server, downstream MCP bridge, benchmarks) to verify changes end-to-end.
---

# Verifying prism changes

## Build

```bash
go build -o ./prism ./cmd/prism
```

## Runtime environment gotchas

- Model runtime config comes from `~/.prism/config.env` (points at a homelab
  SGLang server, `sglang.barton.local`, serving Qwen2.5-Coder-14B). Explicit
  env vars override it:
  `PRISM_MODEL_RUNTIME_ENGINE=ollama PRISM_MODEL_RUNTIME_BASE_URL=http://127.0.0.1:11434 PRISM_MODEL_RUNTIME_MODEL=qwen3.5:9b`
- Local Ollama is usually up (`./prism config doctor`); qwen3.5:9b supports
  native tool calling. The SGLang endpoint returns tool calls as fenced JSON
  *text* (no `--tool-call-parser`), so use Ollama to exercise the tool loop.
- Always pass `--state-dir <tmpdir>` to avoid mutating `~/.prism`.
- Agent latency budgets (~45s) can trip on Ollama cold model loads
  (per-agent `num_ctx` differences force reloads). A `status: timeout` on the
  first run of a new agent is usually cold load — retry once warm.
- macOS: no `timeout` command; use the Bash tool timeout instead.

## Drive the surfaces

CLI basics: `./prism agent list`, `./prism agent show github-cli`,
`./prism config doctor`.

Benchmarks (fixtures, mock server, runner, parse):
`./prism benchmark run --mock --mock-delay-ms 1 --json` (determinism: hash 3 runs).

MCP stdio server — pace the input or responses never flush:

```bash
# session.jsonl: initialize, notifications/initialized, then tools/list / tools/call
(while IFS= read -r l; do printf '%s\n' "$l"; sleep 0.4; done < session.jsonl; sleep 3) \
  | ./prism --state-dir "$STATE" mcp serve 2>/dev/null > out.jsonl
```

Downstream MCP bridge — register prism itself as a real downstream server:

```bash
./prism --state-dir "$STATE" mcp server add-command prism-self "$PWD/prism" mcp serve
./prism --state-dir "$STATE" mcp server tools prism-self
```

Note: `add-command` stops flag parsing at the first positional arg — put
`--timeout-ms`/`--max-bytes` BEFORE `<name> <command>` or they become args to
the command. Edit `$STATE/mcp-servers.yaml` directly when in doubt.

Unresponsive-downstream simulation: `/usr/bin/tail -f /dev/null` (stays alive,
never answers) → should fail with `context deadline exceeded` at timeout_ms.

Live tool loop end-to-end (real model, real downstream):

```bash
echo "Use call_mcp_tool to call 'list_agents' on server 'prism-self'..." \
  | env PRISM_MODEL_RUNTIME_ENGINE=ollama PRISM_MODEL_RUNTIME_BASE_URL=http://127.0.0.1:11434 \
        PRISM_MODEL_RUNTIME_MODEL=qwen3.5:9b \
    ./prism --state-dir "$STATE" run linear --skills linear-issue-management --stdin
```

Check `artifacts[]` for `mcp_tool_call` entries to confirm the model actually
made the call rather than answering from the evidence block.
