# Benchmark showcase (generated)

Do not edit by hand. Regenerate with `prism benchmark project --write`.

### Executive benchmark view

**Workload assumption (per engineer):** 4 substantial coding tasks/day, 80 tasks/month, 960 tasks/year.

**Task definition:** one completed coding request equal to `todo-spa-build` (live run on 2026-05-31), including implementation output + README.

Orchestrator token footprint per task: **without Prism** `6,191 in / 811 out` → **with Prism** `363 in / 1,072 out` (**94.1% input reduction**).

| Model | Monthly cost without Prism | Monthly cost with Prism | Monthly savings | Annual savings |
|---|---:|---:|---:|---:|
| `gpt-5.4` | $2.21 | $1.36 | $0.85 | $10.18 |
| `gpt-5.5` | $4.42 | $2.72 | $1.70 | $20.45 |
| `claude-opus-4.7` | $4.10 | $2.29 | $1.81 | $21.69 |
| `claude-opus-4.6` | $4.10 | $2.29 | $1.81 | $21.69 |
| `claude-sonnet-4.6` | $2.46 | $1.38 | $1.08 | $12.96 |

| Model | Without ($/task) | With ($/task) | Savings/task | Daily savings |
|---|---:|---:|---:|---:|
| `gpt-5.4` | $0.0276 | $0.0170 | $0.0107 | $0.0424 |
| `gpt-5.5` | $0.0553 | $0.0340 | $0.0213 | $0.0852 |
| `claude-opus-4.7` | $0.0512 | $0.0286 | $0.0226 | $0.0904 |
| `claude-opus-4.6` | $0.0512 | $0.0286 | $0.0226 | $0.0904 |
| `claude-sonnet-4.6` | $0.0307 | $0.0172 | $0.0136 | $0.0540 |

Pricing sources: [OpenAI](https://openai.com/api/pricing/) and [Anthropic](https://www.anthropic.com/pricing/) list rates configured in `testdata/benchmarks/orchestrator-models.yaml`. Token counts come from `testdata/benchmarks/results.yaml`. Regenerate with `prism benchmark project --write`.
