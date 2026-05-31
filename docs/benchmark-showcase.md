# Benchmark showcase (generated)

Do not edit by hand. Regenerate with `prism benchmark project --write`.

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
