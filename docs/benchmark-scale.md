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

### Latest live run (2025-05-31, llama3.1:8b)

| Scenario | Baseline input | Delegated input | Input reduction | Savings/run |
|----------|----------------|-----------------|-----------------|-------------|
| homelab-release-incident | 3,547 | 816 | 77.0% | $0.0052 |
| homelab-release-incident-at-scale | 5,734 | 986 | 82.8% | $0.0106 |
| codegen-helper-task | 2,518 | 173 | 93.1% | $0.0061 |

**Why input reduction grows at scale:** baseline orchestrator input grows with rules/history/skills; delegated synthesis stays ~flat because agents return compact JSON summaries.

## Monthly projection

```bash
prism benchmark project
prism benchmark project --json
```

### Orchestrator model matrix

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
