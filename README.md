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
| `prism benchmark project [--json]` | Monthly/annual savings projection from committed results |

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
| `go-helper` | Small Go helpers and pure utilities |
| `go-scaffold` | Package boilerplate and test scaffolds |

Each agent declares `allowed_skills` in its spec. Skills live under `skills/<name>/` with `SKILL.md`, `references/`, and `scripts/`.

## How Prism compares

There is no widely adopted product that matches **Prism’s exact combination** today. Several tools overlap in pieces; the full pattern — **editor orchestrator → MCP → local Ollama specialists with per-run skills, constitutions, and measured token savings** — is still fairly niche.

### What makes Prism different

Prism is a **delegation layer**, not a replacement orchestrator or autonomous swarm:

- The **primary LLM stays in the editor** (Cursor, Copilot, etc.) — it picks the agent, attaches skills, and judges results.
- **Narrow specialists** run on **local Ollama**, invoked via **MCP** (`run_agent`, not pasted prompt bloat).
- **Progressive disclosure** — only attached skills and constitution per call; the orchestrator does not load the full skill library.
- **Structured, compact results** — JSON envelope with summary, findings, artifacts, and confidence.
- **Executable proof** — benchmark suite and monthly projections in `results.yaml` / `scale-profiles.yaml`.

### Closest alternatives

**[Claude Code subagents](https://code.claude.com/docs/en/sub-agents)** — closest *architecture*: isolated specialist context, parent gets a summary, MCP can be scoped to a subagent only. Difference: subagents run **Anthropic models** (Haiku/Sonnet/Opus), not local Ollama — you save context and can use cheaper cloud tiers, but not offline/local compute at $0.

**[Cursor Skills + MCP](https://cursor.com/docs/skills)** — same *host*, different economics: Skills (same [Agent Skills](https://agentskills.io/) standard Prism uses) and MCP tools live in the orchestrator session. Cursor also offers cloud agents for long autonomous work — the opposite of offloading to local specialists. No built-in “run this skill on Ollama and return a compact envelope.”

**CrewAI, LangGraph, AutoGen, OpenAI Agents SDK** — same *delegation idea*, different *orchestrator*: you build the orchestrator app; the editor is not the brain. MCP and local models are optional. Dify, n8n, and Flowise are workflow-first variants of the same pattern.

**Ollama + MCP orchestrators** (community projects) — inverted model: Ollama **is** the orchestrator and MCP servers are tools, not editor-side specialists.

**Continue, Aider, OpenHands, SWE-agent** — coding assistants or autonomous agents with local/cloud models, but not repo-native agent specs, skill allowlists, MCP `run_agent`, or benchmarked orchestrator token reduction.

### Honest landscape summary

| Capability | Prism | Claude subagents | Cursor Skills+MCP | CrewAI/LangGraph |
|------------|-------|------------------|-------------------|------------------|
| Orchestrator stays in editor | ✓ | ✓ | ✓ | ✗ (you build it) |
| Specialist isolation | ✓ | ✓ | partial | ✓ |
| Local Ollama specialists | ✓ | ✗ | ✗ | optional |
| MCP integration | ✓ | ✓ | ✓ | optional |
| Agent Skills standard | ✓ | ✓ | ✓ | varies |
| Token/cost benchmark suite | ✓ | ✗ | ✗ | ✗ |
| Ops specialists (gh/kubectl/argo) | ✓ | DIY | DIY | DIY |

**Bottom line:** nothing mainstream is “Prism under another name.” The closest mental model is **Claude Code subagents + MCP scoping**, but with **Ollama instead of Haiku** and **repo-defined ops/codegen specialists** plus **measured orchestrator savings**. Cursor and Claude are converging on Skills + MCP + subagents; Prism’s bet is combining those primitives with **local execution** and **provable token reduction**.

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

Prism includes reproducible benchmarks that compare **orchestrator-only** (no MCP — one huge prompt) vs **Prism-delegated** (narrow local agent runs + compact synthesis). Results are committed in `testdata/benchmarks/results.yaml` (live Ollama `llama3.1:8b`, orchestrator priced as GPT-4.1 in `rates.yaml`).

### Per-run results (live, 2025-05-31)

| Scenario | Baseline input | Delegated input | Input reduction | Savings/run |
|----------|----------------|-----------------|-----------------|-------------|
| `homelab-release-incident` | 3,547 | 816 | **77.0%** | $0.0052 |
| `homelab-release-incident-at-scale` | 5,734 | 986 | **82.8%** | $0.0106 |
| `codegen-helper-task` | 2,518 | 173 | **93.1%** | $0.0061 |

At-scale adds enterprise session padding (Cursor rules, runbooks, chat history) plus two codegen delegations during the incident. Codegen scenario models offloading a single `go-helper` task while building a package.

### Monthly projection

```bash
prism benchmark project
```

| Profile | Incidents/mo | Codegen/mo | Monthly savings | Annual savings |
|---------|--------------|------------|-----------------|----------------|
| Solo developer | 4 | 25 | **$0.29** | $3.48 |
| Platform team (5 eng) | 12 | 120 | **$1.56** | $18.72 |
| Enterprise SRE (20 eng) | 40 | 400 | **$14.40** | $172.80 |

Local Ollama compute is **$0** in these estimates; savings are orchestrator token cost avoided. Tune volumes in `testdata/benchmarks/scale-profiles.yaml` and re-run benchmarks to refresh `results.yaml`.

### Run benchmarks

```bash
prism benchmark run homelab-release-incident              # real Ollama (default)
prism benchmark run homelab-release-incident-at-scale     # larger context + 10 delegations
prism benchmark run codegen-helper-task                   # single go-helper offload
prism benchmark run homelab-release-incident --mock         # offline CI simulation
prism benchmark run homelab-release-incident --output /tmp/report.md
prism benchmark project --json
```

See [Benchmark: homelab release incident](docs/benchmark-homelab-incident.md) and [Benchmark at scale](docs/benchmark-scale.md).

## Documentation

| Document | Contents |
|----------|----------|
| [docs/usage.md](docs/usage.md) | Detailed CLI/MCP usage, examples, troubleshooting |
| [docs/benchmark-homelab-incident.md](docs/benchmark-homelab-incident.md) | Full-skill benchmark scenario and Cursor A/B steps |
| [docs/benchmark-scale.md](docs/benchmark-scale.md) | At-scale scenarios and monthly savings projection |
| [docs/implementation-plan.md](docs/implementation-plan.md) | Architecture, milestones, design decisions |
| [docs/success-metrics.md](docs/success-metrics.md) | Benchmark targets and report format |
| [docs/tooling-references.md](docs/tooling-references.md) | Dependencies and external specs |
| [agents/README.md](agents/README.md) | Agent spec frontmatter |
| [skills/README.md](skills/README.md) | Skill directory layout and rules |
| [constitutions/README.md](constitutions/README.md) | Legacy constitution layout |

## License

See [LICENSE](LICENSE).
