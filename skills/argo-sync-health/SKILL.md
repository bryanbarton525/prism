---
name: argo-sync-health
description: Diagnose Argo CD app sync/health drift with argocd (and kubectl when needed). Trigger on OutOfSync, Degraded, failed sync hooks, or unexpected desired/live diffs.
compatibility: Requires Argo CD read access; collector can auto-download argocd CLI (ARGOCD_VERSION / ARGOCD_BIN / PRISM_BIN_DIR).
metadata:
  prism-agents: argo
---

# Argo sync health

## Inputs expected

- Application name and optional project/namespace.
- Symptom: OutOfSync, Degraded, sync failed, stale revision.

## Command workflow (argocd)

1. Inspect app summary and sync state.
   - `argocd app get <app>`
   - `argocd app list`
2. Inspect diff and resource-level drift.
   - `argocd app diff <app>`
   - `argocd app resources <app>`
3. Inspect app history and failed sync attempts.
   - `argocd app history <app>`
4. Correlate with Kubernetes-level evidence if needed.
   - `kubectl get events -n <ns> --sort-by=.lastTimestamp`
   - `kubectl describe deploy/<name> -n <ns>`
5. Categorize root cause.
   - Git desired/live drift
   - hook failure
   - policy/permission denial
   - downstream workload health failure

## Output requirements

- `summary`: one-sentence root-cause hypothesis.
- `findings`: sync status, health status, drifted resources, first hard error.
- `artifacts`: key argocd output snippets.
- `confidence`: tied to explicit drift/error evidence.

## Guardrails

- No `argocd app sync` or automated mutation unless explicitly requested.
