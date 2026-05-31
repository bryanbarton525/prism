# Benchmark showcase (generated)

Do not edit by hand. Regenerate with `prism benchmark project --write`.

### Orchestrator showcase matrix

**1 engineer, 1 task/day model (todo app request benchmark)**  
Token usage per task: **without Prism** `6,191 in / 811 out` -> **with Prism** `363 in / 1,072 out` (**94.1% input reduction**).  
Live run: `todo-spa-build` measured 2026-05-31 — `testdata/benchmarks/results.yaml`. Regenerate: `prism benchmark project --write`.

| Model | Without Prism ($/task) | With Prism ($/task) | Saved/task | Saved/day | Saved/month (30 tasks) | Saved/year (365 tasks) |
|---|---:|---:|---:|---:|---:|---:|
| `gpt-5.4` | $0.0276 | $0.0170 | $0.0107 | $0.0107 | $0.32 | $3.89 |
| `gpt-5.5` | $0.0553 | $0.0340 | $0.0213 | $0.0213 | $0.64 | $7.78 |
| `claude-opus-4.7` | $0.0512 | $0.0286 | $0.0226 | $0.0226 | $0.68 | $8.25 |
| `claude-opus-4.6` | $0.0512 | $0.0286 | $0.0226 | $0.0226 | $0.68 | $8.25 |
| `claude-sonnet-4.6` | $0.0307 | $0.0172 | $0.0136 | $0.0136 | $0.41 | $4.95 |

Quality parity (live rubric): baseline and Prism outputs both scored 10/10 on required deliverables (`index.html`, `styles.css`, `app.js`, `README`, localStorage + add/complete/delete behavior).

**Pricing sources (May 2026):**
- [OpenAI API pricing](https://openai.com/api/pricing/) — `gpt-5.4`, `gpt-5.5`
- [Anthropic pricing](https://www.anthropic.com/pricing) — `claude-opus-4.6`, `claude-opus-4.7`, `claude-sonnet-4.6`
- [Cursor pricing](https://cursor.com/pricing) — subscription seat plans (`Individual $20/mo`, `Teams $40/user/mo`)

**Cursor seat economics (todo benchmark, GPT-5.5-equivalent savings/task = $0.0213):**

| Cursor plan | Seat price | Saved/task | Workflows/month to offset seat |
|---|---:|---:|---:|
| Individual | $20/mo | $0.0213 | 938 |
| Teams | $40/user/mo | $0.0213 | 1877 |

Cursor plans are subscription-based (included usage + overage), not pure per-token billing, so these are break-even workflow examples rather than direct token-rate rows.
