# Prism

![Prism logo](docs/img/prism-logo.png)

**Keep your AI editor or MCP host as the orchestrator. Offload narrow work to local Ollama specialists.**

Prism is an MCP server + CLI that runs tool-specific agents on [Ollama](https://ollama.com/) — GitHub CI, Kubernetes, Argo, docs lookup, Go codegen — and returns compact JSON summaries. Your paid model sees a short brief, not every skill, constitution, and evidence dump.

## Why use it

- **Lower orchestrator cost** — **94% orchestrator input reduction** on a live todo-app coding task ([benchmarks](#proof-it-saves-tokens))
- **Local specialists at $0** — Ollama runs absorb skill bodies and domain context; the orchestrator synthesizes from summaries
- **Editor stays in charge** — no autonomous swarm; you pick agent + skills per subtask
- **Progressive disclosure** — only attached [Agent Skills](https://agentskills.io/) per call, enforced by allowlists
- **Native runtime plugins** — read-only evidence collectors, such as Kubernetes diagnostics, run through Prism plugins instead of ad hoc shell calls
- **Repo-native specs** — agents, skills, and constitutions are versioned Markdown in this tree

## Quick start

**Requires:** Go 1.25+, [Ollama](https://ollama.com/) at `http://127.0.0.1:11434`, model from agent specs (default `llama3.1:8b`).

```bash
go install ./cmd/prism
ollama pull llama3.1:8b

cd /path/to/prism          # or pass --root everywhere
prism config doctor        # Ollama + agent registry check
prism agent list

# CLI: one specialist run
echo "Summarize PR #42 CI status" | \
  prism run github-cli --skills gh-pr-triage
```

## Use with an MCP host

Register Prism as an MCP server in your AI editor. Example for Cursor (`~/.cursor/mcp.json`; other MCP hosts use equivalent config — see [docs/usage.md](docs/usage.md)):

Use the full path to your `prism` binary:

```json
{
  "mcpServers": {
    "prism": {
      "command": "/absolute/path/to/prism",
      "args": ["mcp", "serve", "--root", "/absolute/path/to/prism"],
      "env": { "PRISM_OLLAMA_HOST": "http://127.0.0.1:11434" }
    }
  }
}
```

`--root` also accepts a github.com URL — Prism reads files directly via the GitHub Contents API (set `GITHUB_TOKEN` to avoid rate limits for public repos; required for private repos). It falls back to `git clone` if the API is inaccessible:

```json
{
  "mcpServers": {
    "prism": {
      "command": "/absolute/path/to/prism",
      "args": ["mcp", "serve", "--root", "https://github.com/bryanbarton525/prism"],
      "env": {
        "PRISM_OLLAMA_HOST": "http://127.0.0.1:11434",
        "GITHUB_TOKEN": "ghp_yourtokenhere"
      }
    }
  }
}
```

Reload MCP servers in your editor, then call `**run_agent**` with `agent_id`, `task`, and `skill_names`.

For local Gemini MCP setup, review and run `scripts/install_mcp.py` from the repo root.

Available tools include:

- core: `list_agents`, `run_agent`, `get_constitution`, `doctor`
- prompt/resource compatibility: `list_prompts`, `get_prompt`, `list_resources`, `get_resource`

Typical flow: paste a **short brief** → delegate evidence-heavy subtasks to specialists → synthesize their compact summaries. Do not paste all skills and evidence into chat.

When an agent declares `tools:` in its spec, Prism resolves those names through its runtime plugin registry before the local model runs. The Kubernetes agent declares `kubernetes`, so Prism collects bounded read-only cluster evidence with the native Kubernetes client and returns it as a `runtime-plugin:kubernetes` artifact for the specialist to analyze.

Full setup, flags, troubleshooting: **[docs/usage.md](docs/usage.md)**

## Built-in agents


| Agent              | Use for                                          |
| ------------------ | ------------------------------------------------ |
| `github-cli`       | PR triage, GitHub Actions failures               |
| `kubectl`          | Kubernetes pod/rollout diagnostics via native plugin |
| `argo`             | Argo CD sync, workflow debug                     |
| `web-docs-search`  | Docs harvest, release notes                      |
| `go-helper`        | Small Go helpers and utilities                   |
| `go-scaffold`      | Package boilerplate, test scaffolds              |
| `frontend-builder` | Vanilla HTML/CSS/JS UI subtasks                  |


Add your own under `agents/` and `skills/`. See [agents/README.md](agents/README.md) and [skills/README.md](skills/README.md).

## Install skills via skills CLI

You can install Prism skills with the [skills.sh](https://www.skills.sh) CLI:

```bash
# List discoverable skills from this repo
npx skills add github.com/bryanbarton525/prism -l --full-depth

# Install a specific skill (example: MCP orchestration playbook)
npx skills add github.com/bryanbarton525/prism --skill prism-mcp-orchestrator --full-depth
```

Useful skill for parent-model delegation behavior:

- `prism-mcp-orchestrator` — tells the orchestrator **when** to delegate and **how** to call Prism MCP tools/resources/prompts in the correct sequence.

Note: `prism-mcp-orchestrator` is for your editor/host orchestrator only — it is not in any agent `allowed_skills`, so Prism runtime specialists will not attach it during `run_agent`.

## How it compares


| Capability                    | Prism | Claude subagents | Cursor Skills+MCP | CrewAI/LangGraph |
| ----------------------------- | ----- | ---------------- | ----------------- | ---------------- |
| Orchestrator stays in editor  | ✓     | ✓                | ✓                 | ✗                |
| Specialist isolation          | ✓     | ✓                | partial           | ✓                |
| Local Ollama specialists      | ✓     | ✗                | ✗                 | optional         |
| Token/cost benchmarks         | ✓     | ✗                | ✗                 | ✗                |
| Ops specialists (gh/k8s/argo) | ✓     | DIY              | DIY               | DIY              |


Full breakdown: **[docs/comparison.md](docs/comparison.md)**

## Proof it saves tokens



### Benchmark view

<!-- benchmark-showcase:start -->
### Executive benchmark view

**Workload assumption (per engineer):** 20 coding prompts/day, 400 prompts/month, 4800 prompts/year.

**Task definition:** one completed coding request equal to `todo-spa-build` (live run on 2026-05-31), including implementation output + README.

Orchestrator token footprint per task: **without Prism** `6,191 in / 811 out` → **with Prism** `363 in / 1,072 out` (**94.1% input reduction**).

| Model | Monthly cost without Prism | Monthly cost with Prism | Monthly savings | Annual savings |
|---|---:|---:|---:|---:|
| `gpt-5.4` | $11.04 | $6.80 | $4.24 | $50.88 |
| `gpt-5.5` | $22.12 | $13.60 | $8.52 | $102.24 |
| `claude-opus-4.7` | $20.48 | $11.44 | $9.04 | $108.48 |
| `claude-opus-4.6` | $20.48 | $11.44 | $9.04 | $108.48 |
| `claude-sonnet-4.6` | $12.28 | $6.88 | $5.40 | $64.80 |

| Model | Without ($/task) | With ($/task) | Savings/task | Daily savings |
|---|---:|---:|---:|---:|
| `gpt-5.4` | $0.0276 | $0.0170 | $0.0107 | $0.2120 |
| `gpt-5.5` | $0.0553 | $0.0340 | $0.0213 | $0.4260 |
| `claude-opus-4.7` | $0.0512 | $0.0286 | $0.0226 | $0.4520 |
| `claude-opus-4.6` | $0.0512 | $0.0286 | $0.0226 | $0.4520 |
| `claude-sonnet-4.6` | $0.0307 | $0.0172 | $0.0136 | $0.2700 |

Pricing sources: [OpenAI](https://openai.com/api/pricing/) and [Anthropic](https://www.anthropic.com/pricing/) list rates configured in `testdata/benchmarks/orchestrator-models.yaml`. Token counts come from `testdata/benchmarks/results.yaml`. Regenerate with `prism benchmark project --write`.

<!-- benchmark-showcase:end -->



```bash
prism benchmark run todo-spa-build
prism benchmark project --write
```

More scenarios: **[docs/benchmark-scale.md](docs/benchmark-scale.md)**

## Documentation


| Doc                                                        | What's inside                                |
| ---------------------------------------------------------- | -------------------------------------------- |
| [docs/usage.md](docs/usage.md)                             | CLI/MCP reference, examples, troubleshooting |
| [docs/comparison.md](docs/comparison.md)                   | Landscape vs Claude, Cursor, frameworks      |
| [docs/benchmark-scale.md](docs/benchmark-scale.md)         | At-scale scenarios, monthly projections      |
| [docs/workflow-token-diagrams.md](docs/workflow-token-diagrams.md) | Why token savings happen (visual diagrams) |
| [docs/blog-prism-launch.md](docs/blog-prism-launch.md)     | Launch post / product narrative              |
| [docs/implementation-plan.md](docs/implementation-plan.md) | Architecture and design                      |
| [docs/success-metrics.md](docs/success-metrics.md)         | Benchmark targets and report format          |


## Develop

```bash
go test ./...
go build -o prism ./cmd/prism
```

### Prevent direct pushes to main

Install `pre-commit` and enable both commit checks and the push guard:

```bash
pre-commit install
pre-commit install --hook-type pre-push
```

Run all commit-stage hooks manually:

```bash
pre-commit run --all-files
```

Commit hooks include `go test ./...`, `go build ./cmd/prism`, and basic file checks. The pre-push hook in `.pre-commit-config.yaml` runs `scripts/hooks/pre-push-block-main.sh` to block direct pushes to `main`.

If you need an emergency one-off bypass:

```bash
PRISM_ALLOW_MAIN_PUSH=1 git push origin <ref>
```

## License

See [LICENSE](LICENSE).
