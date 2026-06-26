# Prism

![Prism logo](docs/img/prism-logo.png)

**Keep your AI editor or MCP host as the orchestrator. Offload narrow, governed work to local or self-hosted specialists.**

Prism is an open-source, local-first AI offload control plane. It runs tool-specific agents on a configurable model runtime — [Ollama](https://ollama.com/) by default, plus SGLang and vLLM-compatible endpoints — for GitHub CI, Kubernetes, Argo, Linear issue workflows, docs lookup, Go codegen, and more. Your paid model sees a short brief, not every skill, constitution, runbook, and evidence dump.

## Why use it

- **Lower orchestrator cost** — **94% orchestrator input reduction** on a live todo-app coding task ([benchmarks](#proof-it-saves-tokens))
- **Local specialists at $0** — local or homelab model runtimes absorb skill bodies and domain context; the orchestrator synthesizes from summaries
- **Editor stays in charge** — no autonomous swarm; you pick agent + skills per subtask
- **Progressive disclosure** — only attached [Agent Skills](https://agentskills.io/) per call, enforced by allowlists
- **Native runtime plugins** — read-only evidence collectors, such as Kubernetes diagnostics, run through Prism plugins instead of ad hoc shell calls
- **Governed control-plane core** — optional policy files, local event storage, route suggestions, bounded graphs, signed bundle install, reports, and dashboard commands
- **Repo-native specs** — agents, skills, and constitutions are versioned Markdown in this tree

## Product direction

Prism is evolving into a local-first AI offload control plane for engineering teams. See `docs/product/prism-oss-product-definition.md`.

## Quick start

**Requires:** Go 1.25+ and a model runtime. The simplest path is [Ollama](https://ollama.com/) at `http://127.0.0.1:11434` with the model from agent specs (default `qwen3.5:9b`). Set `PRISM_MODEL_RUNTIME_*` to target Ollama explicitly, SGLang, vLLM, or another OpenAI-compatible endpoint.

```bash
go install ./cmd/prism
ollama pull qwen3.5:9b

cd /path/to/prism          # or pass --root everywhere
prism config doctor        # Ollama + agent registry check
prism agent list
prism route suggest --task "Investigate deployment checkout-api in namespace staging"

# CLI: one specialist run
echo "Summarize PR #42 CI status" | \
  prism run github-cli --skills gh-pr-triage
```

Optional control-plane features are enabled explicitly:

```bash
prism --policy-file testdata/policies/k8s-readonly.yaml policy validate testdata/policies/k8s-readonly.yaml
prism policy test testdata/policies/k8s-readonly.yaml testdata/policies/k8s-readonly-cases.yaml
prism registry source add local .
prism bundle verify --source local testdata/bundles/k8s-core-triage/registry.json \
  --public-key testdata/bundles/k8s-core-triage/public_key.txt
prism --state-dir .prism bundle install --source local testdata/bundles/k8s-core-triage/registry.json \
  --dest-root . \
  --public-key testdata/bundles/k8s-core-triage/public_key.txt
prism --event-store .prism/events.db events list
prism --event-store .prism/events.db dashboard serve --addr 127.0.0.1:8765
```

The dashboard reads the configured SQLite event store, so run at least one
`prism run` or `prism graph run` with `--event-store` before expecting usage
data. For the full registry, bundle, policy, event-store, dashboard, and MCP
setup path, see [docs/usage.md](docs/usage.md#full-local-control-plane-setup).

For a deterministic local control-plane smoke test:

```bash
scripts/local-acceptance.sh
```

## Use with an MCP host

Register Prism as an MCP server in your AI editor. Example for Cursor (`~/.cursor/mcp.json`; other MCP hosts use equivalent config — see [docs/usage.md](docs/usage.md)):

Use the full path to your `prism` binary. This example targets Ollama through the runtime registry:

```json
{
  "mcpServers": {
    "prism": {
      "command": "/absolute/path/to/prism",
      "args": ["mcp", "serve", "--root", "/absolute/path/to/prism"],
      "env": {
        "PRISM_MODEL_RUNTIME_ENGINE": "ollama",
        "PRISM_MODEL_RUNTIME_BASE_URL": "http://127.0.0.1:11434",
        "PRISM_MODEL_RUNTIME_MODEL": "qwen3.5:9b"
      }
    }
  }
}
```

`--root` also accepts a github.com URL — Prism reads files directly via the GitHub Contents API (set `PRISM_GITHUB_TOKEN`, `PRISM_GH_TOKEN`, `GITHUB_TOKEN`, or `GH_TOKEN` to avoid rate limits for public repos; required for private repos). It falls back to `git clone` if the API is inaccessible:

```json
{
  "mcpServers": {
    "prism": {
      "command": "/absolute/path/to/prism",
      "args": ["mcp", "serve", "--root", "https://github.com/bryanbarton525/prism"],
      "env": {
        "PRISM_MODEL_RUNTIME_ENGINE": "ollama",
        "PRISM_MODEL_RUNTIME_BASE_URL": "http://127.0.0.1:11434",
        "PRISM_MODEL_RUNTIME_MODEL": "qwen3.5:9b",
        "GITHUB_TOKEN": "ghp_yourtokenhere"
      }
    }
  }
}
```

Reload MCP servers in your editor, then call `**run_agent**` with `agent_id`, `task`, and `skill_names`.

### Install agent instructions

Teach your coding agent how to use Prism by writing a Prism block into its
instruction file. The default target is [`AGENTS.md`](https://agents.md) — the
open standard read by Codex, Cursor, Aider, Gemini, OpenCode and others — with
dedicated targets for agents that read their own file instead:

```bash
prism instructions install            # default: AGENTS.md
prism instructions install -t copilot # .github/copilot-instructions.md
prism instructions install --all      # AGENTS.md, Copilot, Claude, Gemini, Cursor
prism instructions list               # show supported targets
prism instructions uninstall --all    # remove the Prism block everywhere
```

The block is delimited by sentinel comments, so re-running `install` updates it
in place without disturbing surrounding content.

For SGLang, vLLM, fallback runtime, and live contract examples, see [docs/model-runtime.md](docs/model-runtime.md). For local Gemini MCP setup, review and run `scripts/install_mcp.py` from the repo root.

Available tools include:

- core: `list_agents`, `run_agent`, `get_constitution`, `doctor`
- control plane: `suggest_route`, `run_graph`, `explain_policy`, `list_policies`, `list_bundles`, `install_bundle`, `get_usage_summary`, `get_skill_health`
- downstream MCP: `list_mcp_servers`, `list_mcp_server_tools`, `call_mcp_tool`
- prompt/resource compatibility: `list_prompts`, `get_prompt`, `list_resources`, `get_resource`

Typical flow: paste a **short brief** → delegate evidence-heavy subtasks to specialists → synthesize their compact summaries. Do not paste all skills and evidence into chat.

When an agent declares `tools:` in its spec, Prism resolves those names through its runtime plugin registry before the local model runs. Built-in v1 plugins are read-only and bounded: `kubernetes`, `github` (repo-local `.github` metadata), `localdocs`, `filesystem`, `goproject`, `linear` (Linear MCP setup context), and `mcp` (configured downstream MCP inventory). The Kubernetes agent declares `kubernetes`, so Prism collects bounded read-only cluster evidence with the native Kubernetes client and returns it as `runtime-plugin:kubernetes` plus structured `evidence-pack:*` artifacts for the specialist to analyze.

Prism can also act as a bounded MCP client to downstream servers. Configure one under `prism mcp server ...`, then use Prism MCP tools `list_mcp_servers`, `list_mcp_server_tools`, and `call_mcp_tool`. Agents that declare `tools: [mcp]` can also let a tool-capable local model call that bridge during the run, so the specialist owns bulky downstream tool surfaces without loading every downstream schema into the parent model.

Ready-to-copy sample configs (`config.env`, `mcp-servers.yaml`, `prism-policy.json`) live in **[examples/](examples/README.md)**.

Runs can also be attributed to signed installed bundles. Pass `--bundle-id` on `prism run` (and optionally `--bundle-version`) or include `bundle_id`/`bundle_version` in MCP `run_agent` calls; policy can check the bundle ID and the event store records the bundle version for dashboard and report summaries.

Full setup, flags, troubleshooting: **[docs/usage.md](docs/usage.md)**

## Built-in agents


| Agent              | Use for                                          |
| ------------------ | ------------------------------------------------ |
| `github-cli`       | PR triage, GitHub Actions failures               |
| `kubectl`          | Kubernetes pod/rollout diagnostics via native plugin |
| `linear`           | Linear issue/project/cycle workflows via MCP offload |
| `argo`             | Argo CD sync, workflow debug                     |
| `web-docs-search`  | Docs harvest, release notes                      |
| `go-helper`        | Small Go helpers and utilities                   |
| `go-scaffold`      | Package boilerplate, test scaffolds              |
| `frontend-builder` | Vanilla HTML/CSS/JS UI subtasks                  |


Add your own under `agents/` and `skills/`. See [agents/README.md](agents/README.md) and [skills/README.md](skills/README.md).

## Extension points

Prism also exposes a few OSS packages for repo-local integrations and wrappers:

- `pkg/observe` — stable run-event contract plus a sink interface for capturing agent invocation metadata
- `pkg/policy` — stable policy request and decision contracts
- `pkg/evidence` — bounded runtime evidence pack contracts
- `pkg/graph` — bounded graph definition and result contracts
- `pkg/bundle` — managed bundle metadata contracts
- `pkg/registry` — signed agent/skill bundle verification plus fail-closed install helpers that enforce signature, Prism-version compatibility, and file integrity
- `pkg/report` — benchmark projection exports in JSON, Markdown, or structured Go types

These packages are intended for external tooling that wants to build around Prism without scraping CLI output or reimplementing core verification logic.

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

Current verified benchmark-path result (2026-06-14 mock harness): `go test -tags mock ./internal/benchmark/... -run TestHomelabReleaseIncident_Mock -v` produced `92.8%` input reduction and `$0.0176` net savings from the real `Compare()` path.

More scenarios: **[docs/benchmark-scale.md](docs/benchmark-scale.md)**

## Documentation


| Doc                                                        | What's inside                                |
| ---------------------------------------------------------- | -------------------------------------------- |
| [docs/usage.md](docs/usage.md)                             | CLI/MCP reference, examples, troubleshooting |
| [docs/comparison.md](docs/comparison.md)                   | Landscape vs Claude, Cursor, frameworks      |
| [docs/benchmark-scale.md](docs/benchmark-scale.md)         | At-scale scenarios, monthly projections      |
| [docs/workflow-token-diagrams.md](docs/workflow-token-diagrams.md) | Why token savings happen (visual diagrams) |
| [docs/tooling-references.md](docs/tooling-references.md)   | Libraries, protocols, and implementation references |
| [docs/blog-prism-launch.md](docs/blog-prism-launch.md)     | Launch post / product narrative              |
| [docs/implementation-plan.md](docs/implementation-plan.md) | Architecture and design                      |
| [docs/success-metrics.md](docs/success-metrics.md)         | Benchmark targets and report format          |


## Develop

```bash
bash scripts/ci-check.sh
go build -o prism ./cmd/prism
```

`scripts/ci-check.sh` is the deterministic release gate: module verification, normal tests, mock benchmark coverage, `go vet`, and CLI build. Live Ollama benchmarks are opt-in:

```bash
go test -tags live ./internal/benchmark -run TestHomelabReleaseIncident -count=1 -timeout=20m
go test -tags docsgen ./internal/benchmark -run TestWriteShowcaseDocs -count=1
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

Commit hooks include `scripts/ci-check.sh` and basic file checks. The pre-push hook in `.pre-commit-config.yaml` runs `scripts/hooks/pre-push-block-main.sh` to block direct pushes to `main`.

If you need an emergency one-off bypass:

```bash
PRISM_ALLOW_MAIN_PUSH=1 git push origin <ref>
```

## License

See [LICENSE](LICENSE).
