# Prism usage guide

This guide covers day-to-day use of the Prism CLI and MCP server as implemented today.

## Before you start

1. **Clone** the repository and `cd` into it (or pass `--root` to every command).
2. **Install** the binary: `go install ./cmd/prism`
3. **Start Ollama** and confirm it responds: `curl http://127.0.0.1:11434/api/version`
4. **Pull models** used by your agents (see `model:` in `agents/*.md`), e.g.:
  ```bash
   ollama pull llama3.1:8b
  ```
5. **Run doctor**:
  ```bash
   prism config doctor
  ```
   A `warn` on `agent_models` means Ollama is up but the tag in the agent spec is not installed yet.

## Configuration

Prism resolves paths relative to `**--root**` (default: current working directory).


| Setting      | Flag            | Environment variable |
| ------------ | --------------- | -------------------- |
| Project root | `--root`        | —                    |
| Agent specs  | `--agent-dir`   | `PRISM_AGENT_DIR`    |
| Skills       | `--skills-dir`  | —                    |
| Ollama URL   | `--ollama-host` | `PRISM_OLLAMA_HOST`  |


Example running from another directory:

```bash
prism --root ~/src/prism agent list
prism --root ~/src/prism config doctor
```

## CLI workflows

### Inspect agents

```bash
prism agent list
prism agent list --json

prism agent show github-cli
prism agent constitution kubectl --json
```

`agent show` prints frontmatter fields and the inline Markdown body. `agent constitution` prints only the **resolved** contract (from `constitution_path`, inline body, or `constitutions/<id>.md`).

### Run an agent

Skills are **required** and must be listed in the agent’s `allowed_skills`:

```bash
# From a file
prism run github-cli --skills gh-pr-triage --input ./task.md

# Multiple skills (order affects prompt assembly)
prism run kubectl --skills kubectl-triage,k8s-rollout-diagnostics --input task.md

# Stdin
prism run web-docs-search --skills docs-source-harvest --stdin <<'EOF'
Find the rate limit for the GitHub REST API.
EOF

# Piped stdin (no --stdin flag needed)
echo "Check sync health" | prism run argo --skills argo-sync-health
```

**Output formats**

- `--format json` (default) — full `RunResult` envelope on stdout
- `--format markdown` — human-readable report

**Validation**

If a skill is not in `allowed_skills`, the run returns `status: validation_fail` without calling Ollama.

If the assembled prompt exceeds `context_budget`, the runtime truncates the system prompt and sets `context_budget_exceeded: true` in the result.

If the run exceeds `latency_budget_ms`, the context deadline may return `status: timeout`.

### Diagnostics

```bash
prism config doctor
prism config doctor --json
```

Checks include:

- `ollama_connectivity` — ping `/api/version`
- `ollama_models` — list tags from `/api/tags`
- `agent_models` — warn when a spec’s `model` is not in the local list
- `agent_registry` — loaded agent count and IDs
- `skill_registry` — skills passing structure validation

## MCP server

### How it works

- Command: `prism mcp serve`
- Transport: **stdio** (JSON-RPC)
- Logging: **stderr** only (`[prism-mcp] ...`)
- Same `AgentRunner` as the CLI — tool outputs match CLI JSON shapes

Always pass `**--root`** to the absolute path of this repository when the MCP host’s working directory is not the repo root.

### Cursor configuration

`~/.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "prism": {
      "command": "/Users/you/go/bin/prism",
      "args": [
        "mcp",
        "serve",
        "--root",
        "/Users/you/src/prism"
      ]
    }
  }
}
```

After saving, reload MCP servers in Cursor settings. The **prism** server should list:
- Core tools: `list_agents`, `run_agent`, `get_constitution`, `doctor`
- Compatibility tools: `list_prompts`, `get_prompt`, `list_resources`, `get_resource`

### Tool reference

#### `list_agents`

No parameters. Returns agent id, name, description, model, `allowed_skills`, and `latency_budget_ms`.

#### `get_constitution`

```json
{ "agent_id": "github-cli" }
```

Returns `agent_id`, `source` (`path` | `body` | `legacy`), optional `path`, and `text`.

#### `doctor`

No parameters. Same information as `prism config doctor` (JSON).

#### `run_agent`

```json
{
  "agent_id": "github-cli",
  "task": "Describe what to investigate.",
  "skill_names": ["gh-pr-triage"],
  "format": "json"
}
```

- `skill_names` is required (at least one entry).
- `format` is optional (`json` or `markdown`).

Response is a `RunResult` object (see README).

#### `list_prompts` / `get_prompt`

Compatibility tools that expose reusable prompt templates for accurate `run_agent` calls
in MCP hosts that do not support native MCP prompts yet.

```json
{ "prompt_id": "k8s_incident_triage", "variables": { "namespace": "payments" } }
```

#### `list_resources` / `get_resource`

Compatibility tools that expose Prism resources (tool contracts, orchestration guide,
agents index, constitutions) in hosts that do not support native MCP resources.

Example URIs:
- `prism://resource/tooling/run_agent`
- `prism://resource/tooling/orchestration-guide`
- `prism://resource/agents/index`
- `prism://resource/agent/github-cli/constitution`

### Test without Cursor

**MCP Inspector** (browser UI):

```bash
cd /path/to/prism
npx @modelcontextprotocol/inspector prism mcp serve --root "$(pwd)"
```

Suggested order:

1. Connect / initialize
2. `doctor`
3. `list_agents`
4. `get_constitution` with `agent_id: github-cli`
5. `run_agent` with a short task (requires Ollama + model)

## Adding agents and skills

### New agent

1. Create `agents/<id>.md` with required frontmatter (see [agents/README.md](../agents/README.md)).
2. Put the constitution in the file body, or set `constitution_path`, or add `constitutions/<id>.md`.
3. List skill names in `allowed_skills`.
4. Verify: `prism agent show <id>` and `prism config doctor`.

### New skill

1. Create `skills/<name>/` with:
  - `SKILL.md` (frontmatter `name` must match directory name)
  - `references/REFERENCE.md`
  - `scripts/collect.sh`
2. Add `<name>` to an agent’s `allowed_skills`.
3. Verify: `go test ./internal/benchmark/...`

## Troubleshooting


| Problem                      | Likely cause                           | Fix                                                                        |
| ---------------------------- | -------------------------------------- | -------------------------------------------------------------------------- |
| `no agents found`            | Wrong `--root` or cwd                  | `cd` to repo or set `--root`                                               |
| MCP tools missing in Cursor  | Bad `command` path or MCP not reloaded | Use absolute path to `prism`; reload MCP                                   |
| `validation_fail` for skills | Skill not in `allowed_skills`          | Check `prism agent show <id>`                                              |
| `error` / timeout on run     | Ollama down or slow                    | `prism config doctor`; increase `latency_budget_ms`                        |
| Empty or poor model output   | Wrong/missing model                    | `ollama list`; pull or edit `model:` in spec                               |
| MCP hangs                    | Inspecting stdout                      | Use Inspector or Cursor; don’t run `mcp serve` interactively in a terminal |


## Benchmark comparison (no MCP vs MCP)

Run the all-skill mock incident and print a comparison report:

```bash
prism benchmark run homelab-release-incident
prism benchmark run homelab-release-incident-at-scale   # enterprise context padding
prism benchmark project                               # monthly/annual savings
```

`prism benchmark project` also includes a model showcase matrix for GPT 5.4/5.5
and Claude Opus/Sonnet variants configured in
`testdata/benchmarks/orchestrator-models.yaml`.

See [benchmark-homelab-incident.md](benchmark-homelab-incident.md) for the full scenario and [benchmark-scale.md](benchmark-scale.md) for monthly projections.

## Related docs

- [Comparison / landscape](comparison.md) — vs Claude subagents, Cursor, frameworks
- [Implementation plan](implementation-plan.md) — architecture and future milestones
- [Success metrics](success-metrics.md) — benchmark goals
- [Benchmark: homelab incident](benchmark-homelab-incident.md) — eight-delegation A/B scenario
- [Benchmark at scale](benchmark-scale.md) — monthly savings projection
- [Tooling references](tooling-references.md) — SDKs and specs

