# Benchmark at scale and monthly projections

This document explains the **at-scale** benchmark scenarios, how committed results feed monthly savings projections, and how to refresh numbers for your team.

## Scenarios

| ID | Purpose | Delegations | Extra context |
|----|---------|-------------|---------------|
| `homelab-release-incident` | Baseline all-skill incident | 8 | None |
| `homelab-release-incident-at-scale` | Enterprise session size | 10 (+2 codegen) | Cursor rules, runbooks, chat history |
| `codegen-helper-task` | Package helper offload | 1 (`go-helper`) | Full skill library in baseline only |

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

**Why input reduction grows at scale:** baseline orchestrator input grows with rules/history/skills; delegated synthesis stays ~flat because agents return compact JSON summaries.

## Monthly projection

```bash
prism benchmark project
prism benchmark project --json
```

### Orchestrator model matrix

Use this format as the headline benchmark view: explicit before/after costs for one engineer, one task/day.

**Feature benchmark (1 engineer):** `17,922 in / 812 out` without Pulse -> `485 in / 442 out` with Pulse (**97.3% input reduction**).

| Model | Without Pulse ($/task) | With Pulse ($/task) | Saved/task | Saved/day | Saved/month (30 tasks) | Saved/year (365 tasks) |
|---|---:|---:|---:|---:|---:|---:|
| `gpt-5.4` | $0.0570 | $0.0078 | $0.0491 | $0.0491 | $1.47 | $17.94 |
| `gpt-5.5` | $0.1140 | $0.0157 | $0.0983 | $0.0983 | $2.95 | $35.87 |
| `claude-opus-4.7` | $0.1099 | $0.0135 | $0.0964 | $0.0964 | $2.89 | $35.20 |
| `claude-opus-4.6` | $0.1099 | $0.0135 | $0.0964 | $0.0964 | $2.89 | $35.20 |
| `claude-sonnet-4.6` | $0.0660 | $0.0081 | $0.0579 | $0.0579 | $1.74 | $21.12 |

Additional benchmark task token baselines:

| Task | Without Pulse tokens (in / out) | With Pulse tokens (in / out) | Input reduction |
|---|---|---|---:|
| Feature delivery (notification preferences) | `17,922 / 812` | `485 / 442` | 97.3% |
| At-scale incident | `5,734 / 580` | `986 / 545` | 82.8% |
| Codegen helper | `2,518 / 420` | `173 / 380` | 93.1% |

**Pricing sources (May 2026):** [OpenAI API pricing](https://openai.com/api/pricing/) (GPT-5.4, GPT-5.5, GPT-4.1 baseline); [Anthropic pricing](https://www.anthropic.com/pricing) (Opus 4.6/4.7, Sonnet 4.6).

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
