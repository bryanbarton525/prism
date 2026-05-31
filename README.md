# Prism

**Keep Cursor (or any MCP host) as the orchestrator. Offload narrow work to local Ollama specialists.**

Prism is an MCP server + CLI that runs tool-specific agents on [Ollama](https://ollama.com/) — GitHub CI, Kubernetes, Argo, docs lookup, Go codegen — and returns compact JSON summaries. Your paid model sees a short brief, not every skill, constitution, and evidence dump.

## Why use it

- **Lower orchestrator cost** — measured **77–93% input token reduction** vs loading everything in one prompt ([benchmarks](docs/benchmark-scale.md))
- **Local specialists at $0** — Ollama runs absorb skill bodies and domain context; the orchestrator synthesizes from summaries
- **Editor stays in charge** — no autonomous swarm; you pick agent + skills per subtask
- **Progressive disclosure** — only attached [Agent Skills](https://agentskills.io/) per call, enforced by allowlists
- **Repo-native specs** — agents, skills, and constitutions are versioned Markdown in this tree

## Quick start

**Requires:** Go 1.22+, [Ollama](https://ollama.com/) at `http://127.0.0.1:11434`, model from agent specs (default `llama3.1:8b`).

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

## Use with Cursor

Add to `~/.cursor/mcp.json` (use the full path to your `prism` binary):

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

Reload MCP, then call **`run_agent`** with `agent_id`, `task`, and `skill_names`.

Available tools include:
- core: `list_agents`, `run_agent`, `get_constitution`, `doctor`
- prompt/resource compatibility: `list_prompts`, `get_prompt`, `list_resources`, `get_resource`

Typical flow: paste a **short brief** → delegate evidence-heavy subtasks to specialists → synthesize their compact summaries. Do not paste all skills and evidence into chat.

Full setup, flags, troubleshooting: **[docs/usage.md](docs/usage.md)**

## Built-in agents

| Agent | Use for |
|-------|---------|
| `github-cli` | PR triage, GitHub Actions failures |
| `kubectl` | Pod/rollout diagnostics |
| `argo` | Argo CD sync, workflow debug |
| `web-docs-search` | Docs harvest, release notes |
| `go-helper` | Small Go helpers and utilities |
| `go-scaffold` | Package boilerplate, test scaffolds |

Add your own under `agents/` and `skills/`. See [agents/README.md](agents/README.md) and [skills/README.md](skills/README.md).

## How it compares

| Capability | Prism | Claude subagents | Cursor Skills+MCP | CrewAI/LangGraph |
|------------|-------|------------------|-------------------|------------------|
| Orchestrator stays in editor | ✓ | ✓ | ✓ | ✗ |
| Specialist isolation | ✓ | ✓ | partial | ✓ |
| Local Ollama specialists | ✓ | ✗ | ✗ | optional |
| Token/cost benchmarks | ✓ | ✗ | ✗ | ✗ |
| Ops specialists (gh/k8s/argo) | ✓ | DIY | DIY | DIY |

Full breakdown: **[docs/comparison.md](docs/comparison.md)**

## Proof it saves tokens

Live benchmarks (Ollama `llama3.1:8b`, orchestrator priced as GPT-4.1 at [$2.00 / $8.00 per M](https://openai.com/api/pricing/) — see `testdata/benchmarks/rates.yaml`):

| Scenario | Input reduction | Savings/run |
|----------|-----------------|-------------|
| Incident (8 skills) | 77% | $0.005 |
| Incident at scale | 83% | $0.011 |
| Codegen helper | 93% | $0.006 |

```bash
prism benchmark run homelab-release-incident
prism benchmark project    # monthly/annual projection
```

### Orchestrator showcase matrix

**1 engineer model:** 1 task/day, 30-day month, 365-day year.  
**Token basis (without -> with Pulse):** Incident `3,547 -> 816`, At-scale incident `5,734 -> 986`, Codegen `2,518 -> 173`.

| Model | Task | Without Pulse ($/task) | With Pulse ($/task) | Daily (without / with) | Monthly 30 tasks (without / with) | Yearly 365 tasks (without / with) |
|---|---|---:|---:|---:|---:|---:|
| `gpt-5.4` | Incident | $0.0165 | $0.0100 | $0.0165 / $0.0100 | $0.50 / $0.30 | $6.04 / $3.66 |
| `gpt-5.4` | At-scale incident | $0.0230 | $0.0106 | $0.0230 / $0.0106 | $0.69 / $0.32 | $8.41 / $3.88 |
| `gpt-5.4` | Codegen | $0.0126 | $0.0061 | $0.0126 / $0.0061 | $0.38 / $0.18 | $4.60 / $2.24 |
| `gpt-5.5` | Incident | $0.0331 | $0.0201 | $0.0331 / $0.0201 | $0.99 / $0.60 | $12.08 / $7.33 |
| `gpt-5.5` | At-scale incident | $0.0461 | $0.0213 | $0.0461 / $0.0213 | $1.38 / $0.64 | $16.82 / $7.77 |
| `gpt-5.5` | Codegen | $0.0252 | $0.0123 | $0.0252 / $0.0123 | $0.76 / $0.37 | $9.19 / $4.48 |
| `claude-opus-4.7` | Incident | $0.0305 | $0.0174 | $0.0305 / $0.0174 | $0.92 / $0.52 | $11.15 / $6.35 |
| `claude-opus-4.7` | At-scale incident | $0.0432 | $0.0186 | $0.0432 / $0.0186 | $1.30 / $0.56 | $15.76 / $6.77 |
| `claude-opus-4.7` | Codegen | $0.0231 | $0.0104 | $0.0231 / $0.0104 | $0.69 / $0.31 | $8.43 / $3.78 |
| `claude-opus-4.6` | Incident | $0.0305 | $0.0174 | $0.0305 / $0.0174 | $0.92 / $0.52 | $11.15 / $6.35 |
| `claude-opus-4.6` | At-scale incident | $0.0432 | $0.0186 | $0.0432 / $0.0186 | $1.30 / $0.56 | $15.76 / $6.77 |
| `claude-opus-4.6` | Codegen | $0.0231 | $0.0104 | $0.0231 / $0.0104 | $0.69 / $0.31 | $8.43 / $3.78 |
| `claude-sonnet-4.6` | Incident | $0.0183 | $0.0104 | $0.0183 / $0.0104 | $0.55 / $0.31 | $6.69 / $3.81 |
| `claude-sonnet-4.6` | At-scale incident | $0.0259 | $0.0111 | $0.0259 / $0.0111 | $0.78 / $0.33 | $9.45 / $4.06 |
| `claude-sonnet-4.6` | Codegen | $0.0139 | $0.0062 | $0.0139 / $0.0062 | $0.42 / $0.19 | $5.06 / $2.27 |

**Pricing sources (May 2026):**
- [OpenAI API pricing](https://openai.com/api/pricing/) — `gpt-5.4`, `gpt-5.5`
- [Anthropic pricing](https://www.anthropic.com/pricing) — `claude-opus-4.6`, `claude-opus-4.7`, `claude-sonnet-4.6`

Details, scenarios, and Cursor A/B steps: **[docs/benchmark-scale.md](docs/benchmark-scale.md)** · **[docs/benchmark-homelab-incident.md](docs/benchmark-homelab-incident.md)**

## Documentation

| Doc | What's inside |
|-----|---------------|
| [docs/usage.md](docs/usage.md) | CLI/MCP reference, examples, troubleshooting |
| [docs/comparison.md](docs/comparison.md) | Landscape vs Claude, Cursor, frameworks |
| [docs/benchmark-scale.md](docs/benchmark-scale.md) | At-scale scenarios, monthly projections |
| [docs/implementation-plan.md](docs/implementation-plan.md) | Architecture and design |
| [docs/success-metrics.md](docs/success-metrics.md) | Benchmark targets and report format |

## Develop

```bash
go test ./...
go build -o prism ./cmd/prism
```

## License

See [LICENSE](LICENSE).
