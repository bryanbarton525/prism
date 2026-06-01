---
name: gh-actions-diagnostics
description: Diagnose GitHub Actions failures with gh by tracing failing runs, jobs, and step logs. Trigger on CI red/failing checks, flaky workflows, rerun decisions, or release pipeline breakages.
compatibility: Requires GitHub CLI and workflow log visibility; collector can auto-download gh into ~/.cache/prism/bin (set GH_VERSION / GH_BIN / PRISM_BIN_DIR).
metadata:
  prism-agents: github-cli
---

# GH Actions diagnostics

## Inputs expected

- Workflow name, run URL/ID, branch/SHA, or PR number.
- Desired scope (latest failure vs historical flake pattern).

## Command workflow (gh CLI)

1. Locate candidate runs.
   - `gh run list --limit 20`
   - `gh run list --branch <branch> --limit 20`
   - `gh run list --workflow "<workflow>" --limit 20`
2. Inspect failing run details and jobs.
   - `gh run view <run-id>`
   - `gh run view <run-id> --json jobs,conclusion,createdAt,updatedAt,headSha,event,workflowName`
3. Pull failing logs.
   - `gh run view <run-id> --log-failed`
   - For full context: `gh run view <run-id> --log`
4. Pattern-match root causes.
   - Infra/transient: timeouts, network, rate limits.
   - Deterministic: test assertion, migration mismatch, unknown flags.
   - Config drift: missing secrets/env/permissions.
5. Recommend next action.
   - rerun safe? (`transient`)
   - patch required? (`deterministic/config`)
   - handoff target (k8s/argo/docs) if failure is downstream.

## Output requirements

- `summary`: short root-cause hypothesis with confidence.
- `findings`: failing job, failing step, first concrete error, recurrence signal.
- `artifacts`: include run/job IDs and key log snippets.
- `confidence`: high when error is explicit and reproducible; medium/low otherwise.

## Guardrails

- Do not trigger reruns or workflow dispatch without explicit approval.
- Clearly separate observed evidence from hypotheses.
- If repo/run identifiers are missing, return `insufficient_evidence` with a callback request to parent for exact run URL/ID.
- Do not output fabricated file paths, run IDs, or generic placeholders as observed evidence.
