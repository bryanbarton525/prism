# Success metrics

Prism succeeds when delegation to local Ollama agents **reduces orchestrator
token use and cost** without breaking task quality or blowing latency budgets.
Metrics must be **executable and reproducible**, not slide-deck estimates.

## Measurement model

Each benchmark scenario records:

| Field | Description |
| --- | --- |
| `scenario_id` | Stable fixture identifier. |
| `mode` | `orchestrator_only` or `prism_delegated`. |
| `orchestrator_tokens` | Prompt + completion tokens for the main model (or estimate from transcript). |
| `local_tokens` | Prompt + completion tokens (or estimates) for Prism/Ollama runs. |
| `wall_clock_ms` | End-to-end time for the scenario. |
| `pass` | Whether outputs meet fixture assertions. |
| `estimated_cost_usd` | Derived from configured per-model rates (see below). |

### Cost and time savings report

Benchmark runs emit a summary report (JSON + human-readable markdown) with:

- **Token reduction** - percent decrease in orchestrator **input** tokens vs baseline (primary savings driver).
- **Cost delta** - orchestrator-only estimated cost minus Prism-delegated
  estimated cost (including local compute treated as $0 or configurable).
- **Time delta** - wall-clock difference; Prism may be slower per step but
  cheaper overall when the orchestrator does less work.
- **Pass-rate** - fraction of scenarios that pass assertions in each mode.
- **Latency compliance** - fraction of delegated runs within per-agent budgets.
- **Monthly projection** - `prism benchmark project` extrapolates committed
  per-run results using volume profiles in `scale-profiles.yaml`.

Example report shape:

```json
{
  "baseline": "orchestrator_only",
  "delegated": "prism_delegated",
  "scenarios": 12,
  "token_reduction_percent": 42.5,
  "orchestrator_cost_usd": 0.18,
  "delegated_orchestrator_cost_usd": 0.09,
  "local_cost_usd": 0.0,
  "net_cost_savings_usd": 0.09,
  "median_wall_clock_ms": { "baseline": 8200, "delegated": 9400 },
  "pass_rate": { "baseline": 1.0, "delegated": 0.92 },
  "latency_budget_compliance": 0.95
}
```

Cost rates live in test configuration (for example `testdata/benchmarks/rates.yaml`)
so CI does not depend on live billing APIs. Committed live measurements live in
`testdata/benchmarks/results.yaml`; monthly extrapolation in `scale-profiles.yaml`.

Verified benchmark-path measurement (2026-06-14, mock harness): `go test -tags mock ./internal/benchmark/... -run TestHomelabReleaseIncident_Mock -v` produced `token_reduction_percent=92.8%` and `net_cost_savings_usd=$0.0176` from the real `Compare()` path.

### Example monthly projection (2025-05-31)

| Profile | Monthly savings | Annual savings |
| --- | --- | --- |
| Solo developer | $0.29 | $3.48 |
| Platform team | $1.56 | $18.72 |
| Enterprise SRE | $14.40 | $172.80 |

Run `prism benchmark project` after updating `results.yaml`.

## Targets (initial)

These are starting targets for the first golden benchmark suite. Tune per
project once baselines exist.

| Metric | Target | Notes |
| --- | --- | --- |
| Orchestrator token reduction | **>= 35%** median across golden scenarios | Measured vs `orchestrator_only` on the same fixtures. |
| Pass-rate (delegated) | **>= 90%** | Must not regress more than 5 points vs baseline without review. |
| Latency budget compliance | **>= 95%** | Per-agent `latency_budget_ms` in agent spec frontmatter. |
| Net cost savings | **> 0** on golden suite | After configured orchestrator pricing; local run cost defaults to $0. |

### Default per-agent latency budgets

| Agent | `latency_budget_ms` |
| --- | --- |
| GitHub CLI agent | 30000 |
| Web/docs search agent | 35000 |
| Kubernetes agent (`kubectl`) | 45000 |
| Argo agent | 45000 |

## Test types

### 1. Benchmark suite (`go test ./internal/benchmark/...`)

- Runs golden scenarios in both `orchestrator_only` and `prism_delegated` modes.
- Records token estimates, timing, pass/fail, and cost fields.
- Fails CI if any **target** row above is breached (configurable thresholds in
  `testdata/benchmarks/thresholds.yaml`).

### 2. Agent + skill contract tests

- Parse every `agents/*.md` spec (Markdown + YAML frontmatter).
- Validate referenced skills exist and satisfy Agent Skills frontmatter rules.
- Golden-test prompt assembly: spec + constitution + **only attached skills**.

### 3. Integration tests (opt-in)

- Real Ollama runs behind `-tags=integration` for smoke validation, not for
  every CI job unless a runner provides Ollama.

## When to run

- **Every PR** - unit + contract + benchmark tests (mocked Ollama where needed).
- **Nightly or manual** - integration tests with real Ollama and full token
  accounting from API responses when available.

## Related documents

- [Implementation plan](implementation-plan.md) - architecture and milestones.
- [Agent skills](../skills/README.md) - skill layout and invocation rules.
