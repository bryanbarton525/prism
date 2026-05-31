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

Live benchmarks (Ollama `llama3.1:8b`, orchestrator priced as GPT-4.1):

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

`prism benchmark project` now outputs a comparison matrix for:
- `gpt-5.4`
- `gpt-5.5`
- `claude-opus-4.7`
- `claude-opus-4.6`
- `claude-sonnet-4.6`

Current showcase output (from committed benchmark fixtures + placeholder rates):

| Orchestrator model | Incident $/run | At-scale incident $/run | Codegen $/run | Monthly (enterprise) | Annual (enterprise) |
|--------------------|----------------|--------------------------|---------------|----------------------|---------------------|
| `gpt-5.4` | $0.01 | $0.01 | $0.01 | $14.40 | $172.80 |
| `gpt-5.5` | $0.01 | $0.01 | $0.01 | $14.40 | $172.80 |
| `claude-opus-4.7` | $0.01 | $0.01 | $0.01 | $14.40 | $172.80 |
| `claude-opus-4.6` | $0.01 | $0.01 | $0.01 | $14.40 | $172.80 |
| `claude-sonnet-4.6` | $0.01 | $0.01 | $0.01 | $14.40 | $172.80 |

Rates are configured in `testdata/benchmarks/orchestrator-models.yaml`. These values are currently placeholder baselines; replace with real provider pricing for production-accurate dollar comparisons.

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
