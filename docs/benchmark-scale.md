# Benchmark at scale and monthly projections

This document explains the **at-scale** benchmark scenarios, how committed results feed monthly savings projections, and how to refresh numbers for your team.

## Scenarios

| ID | Purpose | Delegations | Extra context |
|----|---------|-------------|---------------|
| `homelab-release-incident` | Baseline all-skill incident | 8 | None |
| `homelab-release-incident-at-scale` | Enterprise session size | 10 (+2 codegen) | Cursor rules, runbooks, chat history |
| `codegen-helper-task` | Package helper offload | 1 (`go-helper`) | Full skill library in baseline only |
| `todo-spa-build` | Real coding task (todo SPA) | 3 (`frontend-builder`) | Eng standards, review notes, chat thread |

### At-scale padding

`homelab-release-incident-at-scale` loads three files **only in orchestrator-only mode** (simulating what Cursor ingests without MCP):

- `context/cursor-rules.md` — workspace rules and compliance blocks
- `context/runbook-index.md` — expanded runbook library
- `context/chat-history.md` — multi-turn incident thread export

Local Prism agents never receive this padding — they get narrow task + evidence only.

### Codegen scenario

`codegen-helper-task` models a common pattern: the orchestrator is building `internal/metrics` and offloads `ParseLabels` to `go-helper` instead of loading all 12 skills into one prompt.

## Committed results

Live measurements are stored in `testdata/benchmarks/results.yaml`:

```bash
# Refresh after model or fixture changes
prism benchmark run homelab-release-incident
prism benchmark run homelab-release-incident-at-scale
prism benchmark run codegen-helper-task
# Edit results.yaml with output token counts and costs from --json
```

Pricing assumptions: `testdata/benchmarks/rates.yaml` (default orchestrator GPT-4.1, local $0).

### Latest live runs (2025-05-31 and 2026-05-31, llama3.1:8b)

| Scenario | Baseline input | Delegated input | Input reduction | Savings/run |
|----------|----------------|-----------------|-----------------|-------------|
| homelab-release-incident | 3,547 | 816 | 77.0% | $0.0052 |
| homelab-release-incident-at-scale | 5,734 | 986 | 82.8% | $0.0106 |
| codegen-helper-task | 2,518 | 173 | 93.1% | $0.0061 |
| feature-notification-center | 17,922 | 485 | 97.3% | $0.0378 |
| todo-spa-build | 6,191 | 363 | 94.1% | $0.0096 |

**Why input reduction grows at scale:** baseline orchestrator input grows with rules/history/skills; delegated synthesis stays ~flat because agents return compact JSON summaries.

Todo scenario quality parity check (live run): both baseline and Prism outputs matched 10/10 rubric checks (`index.html`, `styles.css`, `app.js`, `README`, `localStorage`, add/complete/delete flows, event listeners, todo list UI), with both responses including full code fences.

## Monthly projection

```bash
prism benchmark project
prism benchmark project --json
```

### Orchestrator model matrix

Use this format as the headline benchmark view: explicit before/after costs for one engineer, one task/day.

**Todo benchmark (1 engineer):** `6,191 in / 811 out` without Prism -> `363 in / 1,072 out` with Prism (**94.1% input reduction**).

| Model | Without Prism ($/task) | With Prism ($/task) | Saved/task | Saved/day | Saved/month (30 tasks) | Saved/year (365 tasks) |
|---|---:|---:|---:|---:|---:|---:|
| `gpt-5.4` | $0.0276 | $0.0170 | $0.0107 | $0.0107 | $0.32 | $3.89 |
| `gpt-5.5` | $0.0553 | $0.0340 | $0.0213 | $0.0213 | $0.64 | $7.78 |
| `claude-opus-4.7` | $0.0512 | $0.0286 | $0.0226 | $0.0226 | $0.68 | $8.25 |
| `claude-opus-4.6` | $0.0512 | $0.0286 | $0.0226 | $0.0226 | $0.68 | $8.25 |
| `claude-sonnet-4.6` | $0.0307 | $0.0172 | $0.0136 | $0.0136 | $0.41 | $4.95 |

Additional benchmark task token baselines:

| Task | Without Prism tokens (in / out) | With Prism tokens (in / out) | Input reduction |
|---|---|---|---:|
| Todo SPA build | `6,191 / 811` | `363 / 1,072` | 94.1% |
| Feature delivery (notification preferences) | `17,922 / 812` | `485 / 442` | 97.3% |
| At-scale incident | `5,734 / 580` | `986 / 545` | 82.8% |
| Codegen helper | `2,518 / 420` | `173 / 380` | 93.1% |

**Pricing sources (May 2026):** [OpenAI API pricing](https://openai.com/api/pricing/) (GPT-5.4, GPT-5.5, GPT-4.1 baseline); [Anthropic pricing](https://www.anthropic.com/pricing) (Opus 4.6/4.7, Sonnet 4.6); [Cursor pricing](https://cursor.com/pricing) (subscription seats).

Cursor subscription break-even (todo benchmark, GPT-5.5-equivalent savings/task = $0.0213):

| Cursor plan | Seat price | Workflows/month to offset seat |
|---|---:|---:|
| Individual | $20/mo | 939 |
| Teams | $40/user/mo | 1,878 |

Profiles in `testdata/benchmarks/scale-profiles.yaml` (default orchestrator: GPT-4.1 at [$2/$8 per M](https://openai.com/api/pricing/)):

| Profile | Incidents/mo | Codegen/mo | Context multiplier | Monthly savings | Annual savings |
|---------|--------------|------------|--------------------|-----------------|--------------|
| `solo_developer` | 4 | 25 | 1.0× | $0.15 | $1.80 |
| `platform_team` | 12 | 120 | 2.5× | $1.83 | $21.96 |
| `enterprise_sre` | 40 | 400 | 5.0× | $12.29 | $147.48 |

### How projection works

1. **Incidents** — uses `homelab-release-incident` (solo) or `homelab-release-incident-at-scale` (team/enterprise when multiplier ≥ 2.5).
2. **Codegen tasks** — uses `codegen-helper-task` per helper/scaffold delegation.
3. **Context multiplier** — scales baseline orchestrator **input** cost linearly; delegated orchestrator input stays near measured flat synthesis size.
4. **Net savings** — `(scaled baseline cost − delegated cost) × monthly volume` per task type.

Orchestrator input tokens avoided/month (solo profile): ~69k. Enterprise profile: ~5.8M tokens/month.

### Customize for your team

Edit `scale-profiles.yaml`:

```yaml
profiles:
  my_team:
    description: Our on-call + feature squad
    incidents_per_month: 8
    codegen_tasks_per_month: 80
    context_size_multiplier: 3.0
```

Then run `prism benchmark project`.

## CI vs live

| Command | Use |
|---------|-----|
| `go test -tags mock ./internal/benchmark/ -run Mock` | Fast CI, ≥35% input reduction gate |
| `go test ./internal/benchmark/ -run HomelabReleaseIncident` | Live Ollama smoke (~90s) |
| `prism benchmark run --mock` | Offline report generation |

## Related

- [Homelab incident benchmark](benchmark-homelab-incident.md)
- [Success metrics](success-metrics.md)
- [Usage guide](usage.md)
