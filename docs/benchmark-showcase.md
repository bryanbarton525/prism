# Benchmark showcase (generated)

Do not edit by hand. Regenerate with `prism benchmark project --write`.

**1 engineer · 12 tasks/day · 240 tasks/month · 2880 tasks/year** (`todo-spa-build` live benchmark, 2026-05-31)

Orchestrator tokens per task: **without Prism** `6,191 in / 811 out` → **with Prism** `363 in / 1,072 out` (**94.1% input reduction**)

| Model | Without ($/task) | With ($/task) | Daily (12 tasks, without / with) | Monthly (240 tasks, without / with) | Yearly (2880 tasks, without / with) |
|---|---:|---:|---:|---:|---:|
| `gpt-5.4` | $0.0276 | $0.0170 | $0.3312 / $0.2040 | $6.62 / $4.08 | $79.49 / $48.96 |
| `gpt-5.5` | $0.0553 | $0.0340 | $0.6636 / $0.4080 | $13.27 / $8.16 | $159.26 / $97.92 |
| `claude-opus-4.7` | $0.0512 | $0.0286 | $0.6144 / $0.3432 | $12.29 / $6.86 | $147.46 / $82.37 |
| `claude-opus-4.6` | $0.0512 | $0.0286 | $0.6144 / $0.3432 | $12.29 / $6.86 | $147.46 / $82.37 |
| `claude-sonnet-4.6` | $0.0307 | $0.0172 | $0.3684 / $0.2064 | $7.37 / $4.13 | $88.42 / $49.54 |

Pricing: [OpenAI](https://openai.com/api/pricing/) · [Anthropic](https://www.anthropic.com/pricing) · rates in `testdata/benchmarks/orchestrator-models.yaml`. Token counts: `testdata/benchmarks/results.yaml`. Regenerate: `prism benchmark project --write`.
