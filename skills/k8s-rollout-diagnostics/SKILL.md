---
name: k8s-rollout-diagnostics
description: Diagnose rollout failures with version-aware kubectl rollout/status/history + events. Trigger on stalled rollouts, ProgressDeadlineExceeded, readiness failures, or bad releases.
compatibility: Requires kubectl (version-matched to cluster) and deployment read access; collector auto-downloads kubectl by version (KUBECTL_VERSION / KUBECTL_BIN / PRISM_BIN_DIR).
metadata:
  prism-agents: kubectl
---

# K8s rollout diagnostics

## Inputs expected

- Namespace + deployment/statefulset name.
- Target revision/version if known.
- **Required:** cluster version (e.g., `v1.29.x`) to align diagnostics and command behavior.
- Optional: explicit kubectl binary (e.g., `kubectl-1.29`) if multiple clients exist.

## Command workflow (kubectl)

1. Check rollout status and high-level condition.
   - `kubectl version --client`
   - `kubectl version`
   - `kubectl rollout status deploy/<name> -n <ns>`
   - `kubectl describe deploy <name> -n <ns>`
2. Inspect rollout history and revision diffs.
   - `kubectl rollout history deploy/<name> -n <ns>`
   - `kubectl rollout history deploy/<name> --revision=<n> -n <ns>`
3. Inspect pods for new ReplicaSet.
   - `kubectl get rs -n <ns>`
   - `kubectl get pods -n <ns> -l app=<label> -o wide`
4. Identify blocking signals.
   - readiness/liveness failures
   - image pull/auth errors
   - config/env/flag regressions
   - quota/scheduling constraints
5. Capture safe next checks.
   - "check config key X", "compare revision N vs N-1", "verify image tag".

## Output requirements

- `summary`: rollout state + likely blocker.
- `findings`: current revision, ready counts, first blocking condition, impact.
- `artifacts`: rollout/event snippets and key command outputs.
- `confidence`: high only when blocker is explicit.

## Guardrails

- No rollback/restart/apply actions unless explicitly requested.
- Separate observed rollout status from speculative causes.
- If cluster version is not provided, request it before final diagnosis.
