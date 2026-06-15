# Benchmark: homelab release incident

This scenario exercises **all four Prism agents** and **all eight skills** in one mock post-upgrade incident. Use it to compare **orchestrator-only** (no MCP) vs **Prism-delegated** (MCP) workflows.

## Scenario

**ID:** `homelab-release-incident`

A homelab platform upgrade (`v2.3.0` → `v2.4.0`) causes:

- GitHub PR #42 CI failure and review blockers
- `payments-api` CrashLoopBackOff and stalled rollout
- Argo CD `payments-api` OutOfSync / Degraded
- Argo Workflow `nightly-etl` failed at `extract`
- Breaking API changes in v2.4.0 docs and release notes

Fixtures live under:

```text
testdata/benchmarks/scenarios/homelab-release-incident/
  brief.md              # orchestrator task
  evidence/             # mock gh/k8s/argo/docs data
  tasks/                # per-delegation prompts
  responses/            # mock Ollama outputs (CI)
```

## Automated comparison (recommended first)

From the repo root:

```bash
go test ./internal/benchmark/ -run HomelabReleaseIncident -v
```

Or run the CLI report:

```bash
prism benchmark run homelab-release-incident
prism benchmark run homelab-release-incident --json
prism benchmark run homelab-release-incident --output /tmp/prism-benchmark.md
```

**Live Ollama** is the default (~8 agent calls, ~60–90s):

```bash
prism benchmark run homelab-release-incident
prism benchmark run homelab-release-incident --json
prism benchmark run homelab-release-incident --output /tmp/prism-benchmark.md
```

Use `--mock` for offline CI simulation (canned responses, no Ollama).

### Verified benchmark-path results (2026-06-14)

The real `Compare()` benchmark path now reports the following verified mock-harness numbers from `go test -tags mock ./internal/benchmark/... -run TestHomelabReleaseIncident_Mock -v`:

| Mode | Orchestrator input | Pass |
|------|-------------------|------|
| Orchestrator only | 9,505 tokens | ✓ |
| Prism delegated | 684 tokens | ✓ |
| **Input reduction** | **92.8%** | |
| **Savings/run** | **$0.0176** | |

These numbers are the current benchmark-path measurement from the real comparison logic; the older fixture table in this doc is superseded by the verified `Compare()` output above.

At-scale variant (`homelab-release-incident-at-scale`): **82.8%** input reduction, **$0.0106**/run — see [benchmark-scale.md](benchmark-scale.md).

Monthly projection: `prism benchmark project`

### What the numbers mean

| Mode | Models tokens counted | What happens |
|------|----------------------|--------------|
| `orchestrator_only` | Main model only | One huge prompt: brief + all evidence + all 8 skills + all constitutions |
| `prism_delegated` | Main model + local | Brief + 8 narrow agent runs + short synthesis; local tokens on Ollama |

Expect **lower orchestrator tokens** with MCP and **higher wall-clock** in mock/live delegated mode (eight sequential local calls).

## Manual MCP host comparison

### A. Without MCP (baseline)

1. Open a **new chat** in your AI editor (no Prism tools).
2. Paste the contents of:
   - `testdata/benchmarks/scenarios/homelab-release-incident/brief.md`
   - Every file under `evidence/`
   - Optional: all `skills/*/SKILL.md` and `constitutions/*.md` (mirrors “load everything”).
3. Ask:

   > Write the homelab incident report per the brief. Include PR #42, CI, CrashLoopBackOff, OutOfSync, workflow failure, and breaking changes.

4. Note **time to complete** and approximate **context size** (token usage if visible).

### B. With MCP (Prism delegated)

1. Enable the **prism** MCP server in your editor.
2. Paste only `brief.md` (do not paste all evidence).
3. Run **eight** `run_agent` calls — one per row:

| Agent | skill_names | Task hint |
|-------|-------------|-----------|
| `github-cli` | `gh-pr-triage` | Analyze PR #42; evidence in scenario gh-pr.json |
| `github-cli` | `gh-actions-diagnostics` | Diagnose deploy-homelab failure |
| `kubectl` | `kubectl-triage` | CrashLoopBackOff in payments namespace |
| `kubectl` | `k8s-rollout-diagnostics` | payments-api rollout stalled |
| `argo` | `argo-sync-health` | payments-api OutOfSync |
| `argo` | `argo-workflow-debug` | nightly-etl failed at extract |
| `web-docs-search` | `docs-source-harvest` | v2.4.0 migration for payments-api |
| `web-docs-search` | `release-notes-scan` | breaking changes in v2.4.0 |

   For each call, paste the matching **evidence file** content into the `task` field (or ask the editor to read it from the repo).

4. Ask the editor to **synthesize** the eight summaries into one incident report (same structure as the brief).

5. Compare to baseline: orchestrator context size, total time, and report quality.

### C. Record results

Fill in:

| Metric | Without MCP | With MCP |
|--------|-------------|----------|
| Wall clock (end-to-end) | | |
| Orchestrator tokens (est.) | | |
| Local/Ollama tokens | 0 | |
| Report passes checklist | | |

Checklist phrases (from scenario assertions): `PR #42`, `CI`, `CrashLoopBackOff`, `OutOfSync`, `workflow`, `breaking`, `payments-api`.

## Thresholds

CI targets in `testdata/benchmarks/thresholds.yaml`:

- Orchestrator token reduction ≥ 35%
- Delegated pass rate ≥ 90%
- Latency budget compliance ≥ 95% (per-agent `latency_budget_ms`)

Pricing assumptions: `testdata/benchmarks/rates.yaml`.

## Related

- [Success metrics](success-metrics.md)
- [Usage guide](usage.md)
