# Prism

Just as a prism separates light into distinct, pure colors, this project separates work into small, specialized local agents.

Prism runs **tool-specific specialist agents** on local [Ollama](https://ollama.com/) models. A primary LLM (Cursor, Copilot, etc.) stays the **orchestrator**—it picks the agent, attaches skills, and judges results. Prism handles loading specs, assembling prompts, calling Ollama, and returning a normalized JSON envelope.

## Features (current)

- **Shared runtime** (`AgentRunner`) used by both CLI and MCP
- **Agent specs** — Markdown + YAML frontmatter under `agents/`
- **Agent Skills** — per-invocation skill attachment from `skills/` (not the full library)
- **CLI** — `agent`, `run`, `config doctor`, `mcp serve`
- **MCP server** — `list_agents`, `run_agent`, `get_constitution`, `doctor`
- **Contract tests** — agent/skill validation and prompt golden checks

## Requirements

- Go 1.22+
- [Ollama](https://ollama.com/) running locally (default `http://127.0.0.1:11434`)
- Models referenced in agent specs (default tag `llama3.1:8b` — pull or edit specs to match what you have)

## Install

From the repository root:

```bash
go install ./cmd/prism
```

Or build a binary:

```bash
go build -o prism ./cmd/prism
```

Run from the **repository root** (or pass `--root`) so `agents/` and `skills/` resolve correctly.

## Quick start

```bash
# Health check: Ollama, agents, skills
prism config doctor

# List specialists
prism agent list

# Inspect one agent (frontmatter + constitution body)
prism agent show github-cli

# Show resolved constitution only
prism agent constitution github-cli

# Run a task (requires at least one --skills value)
prism run github-cli --skills gh-pr-triage --input task.md

# Same via stdin, Markdown output
echo "Summarize PR #42 CI status" | prism run github-cli --skills gh-pr-triage --format markdown
```

`prism run` calls Ollama. Ensure the agent’s `model` is available (`ollama pull llama3.1:8b` or change the spec).

## CLI reference

Global flags (all subcommands):

| Flag | Env | Default | Purpose |
|------|-----|---------|---------|
| `--root` | — | current directory | Repo root for `agents/`, `skills/`, `constitutions/` |
| `--agent-dir` | `PRISM_AGENT_DIR` | `<root>/agents` | Agent spec directory |
| `--skills-dir` | — | `<root>/skills` | Skills root directory |
| `--ollama-host` | `PRISM_OLLAMA_HOST` | `http://127.0.0.1:11434` | Ollama base URL |
| `--verbose` | — | off | Log to stderr |
| `--json` | — | off | JSON output where supported |

### Commands

| Command | Description |
|---------|-------------|
| `prism agent list [--json]` | List registered agents |
| `prism agent show <id> [--json]` | Full spec + constitution body |
| `prism agent constitution <id> [--json]` | Resolved constitution text |
| `prism run <id> --skills <name>...` | Run one agent (see flags below) |
| `prism config doctor [--json]` | Connectivity and registry diagnostics |
| `prism mcp serve` | Start MCP server on stdio |
| `prism benchmark run [scenario]` | Compare no-MCP vs MCP token/cost/time |

**`prism run` flags**

| Flag | Required | Description |
|------|----------|-------------|
| `--skills` | yes | One or more skill names (must be in agent `allowed_skills`) |
| `--input <file>` | * | Task text from file |
| `--stdin` | * | Read task from stdin |
| `--format` | no | `json` (default) or `markdown` |

\* Provide task via `--input`, `--stdin`, or a pipe (stdin detected automatically).

### Run result (JSON)

Every run returns a normalized envelope (CLI and MCP):

```json
{
  "agent_id": "github-cli",
  "model": "llama3.1:8b",
  "status": "ok",
  "summary": "...",
  "findings": [],
  "artifacts": [],
  "confidence": "",
  "usage": {
    "prompt_tokens_estimate": 0,
    "completion_tokens_estimate": 0,
    "duration_ms": 0
  },
  "skills_used": ["gh-pr-triage"],
  "constitution_source": "legacy",
  "raw_output": "..."
}
```

`status` may be `ok`, `error`, `validation_fail`, or `timeout`.

## MCP server

Prism speaks MCP over **stdio**. Logs go to **stderr**; JSON-RPC uses stdin/stdout.

### Cursor

Add to `~/.cursor/mcp.json` (or `.cursor/mcp.json` in a project):

```json
{
  "mcpServers": {
    "prism": {
      "command": "prism",
      "args": [
        "mcp",
        "serve",
        "--root",
        "/absolute/path/to/prism"
      ],
      "env": {
        "PRISM_OLLAMA_HOST": "http://127.0.0.1:11434"
      }
    }
  }
}
```

Use the full path to `prism` in `command` if it is not on your `PATH`. Reload MCP in Cursor after editing.

### MCP tools

| Tool | Arguments | Description |
|------|-----------|-------------|
| `list_agents` | (none) | Agent summaries, models, allowed skills |
| `get_constitution` | `agent_id` | Constitution text and source |
| `doctor` | (none) | Ollama + registry health |
| `run_agent` | `agent_id`, `task`, `skill_names`, `format?` | Full agent run (same schema as CLI) |

Example `run_agent` payload:

```json
{
  "agent_id": "github-cli",
  "task": "What failed in the latest CI run?",
  "skill_names": ["gh-pr-triage"],
  "format": "json"
}
```

### Test with MCP Inspector

```bash
cd /path/to/prism
npx @modelcontextprotocol/inspector prism mcp serve --root "$(pwd)"
```

See [docs/usage.md](docs/usage.md) for more examples and troubleshooting.

## Built-in agents

| ID | Domain |
|----|--------|
| `github-cli` | `gh` — PRs, CI, repo metadata |
| `web-docs-search` | Docs and API reference lookup |
| `kubectl` | Cluster inspection |
| `argo` | Argo CD / Argo Workflows diagnostics |

Each agent declares `allowed_skills` in its spec. Skills live under `skills/<name>/` with `SKILL.md`, `references/`, and `scripts/`.

## Project layout

```text
cmd/prism/           CLI entrypoint
internal/
  agent/             Agent spec parsing and registry
  skill/             SKILL.md loader and validation
  app/               AgentRunner, prompt assembly
  ollama/            HTTP client for Ollama
  result/            RunResult and DoctorResult schemas
  cli/               Cobra commands
  mcp/               MCP stdio adapter
agents/              Agent specs (*.md)
skills/              Agent Skills directories
constitutions/       Legacy contracts (fallback resolution)
docs/                Architecture and usage docs
testdata/benchmarks/ CI threshold fixtures
```

## Testing

```bash
go test ./...
```

Contract tests under `internal/benchmark/` validate all `agents/*.md` and `skills/*` layouts. Integration tests against a live Ollama server are not required for default CI.

## Benchmark suite

Golden scenario **`homelab-release-incident`** uses all **8 skills** across 4 agents in a mock post-upgrade incident (GitHub CI, Kubernetes, Argo CD/Workflows, docs).

Compare **orchestrator-only** (no MCP — one huge prompt) vs **Prism-delegated** (eight local runs + short synthesis):

```bash
prism benchmark run homelab-release-incident              # real Ollama (default)
prism benchmark run homelab-release-incident --mock       # offline simulation
prism benchmark run homelab-release-incident --output /tmp/report.md
```

See [Benchmark: homelab release incident](docs/benchmark-homelab-incident.md) for the manual Cursor workflow (without MCP vs with MCP).

## Documentation

| Document | Contents |
|----------|----------|
| [docs/usage.md](docs/usage.md) | Detailed CLI/MCP usage, examples, troubleshooting |
| [docs/benchmark-homelab-incident.md](docs/benchmark-homelab-incident.md) | Full-skill benchmark scenario and Cursor A/B steps |
| [docs/implementation-plan.md](docs/implementation-plan.md) | Architecture, milestones, design decisions |
| [docs/success-metrics.md](docs/success-metrics.md) | Benchmark targets and report format |
| [docs/tooling-references.md](docs/tooling-references.md) | Dependencies and external specs |
| [agents/README.md](agents/README.md) | Agent spec frontmatter |
| [skills/README.md](skills/README.md) | Skill directory layout and rules |
| [constitutions/README.md](constitutions/README.md) | Legacy constitution layout |

## License

See [LICENSE](LICENSE).
