---
name: kubectl-triage
description: Triage Kubernetes incidents with version-aware kubectl (pods, events, logs, describe). Trigger on CrashLoopBackOff, Pending pods, probe failures, namespace health, or workload instability.
compatibility: Requires kubectl (version-matched to cluster) and cluster read permissions; collector auto-downloads kubectl by version (KUBECTL_VERSION / KUBECTL_BIN / PRISM_BIN_DIR).
metadata:
  prism-agents: kubectl
---

# kubectl triage

## Inputs expected

- Namespace and workload hints (deployment/pod/app label).
- Symptom and time window (e.g., CrashLoopBackOff in last 30m).
- **Required:** cluster version (e.g., `v1.28.7`) and distribution/context (EKS/GKE/AKS/on-prem).
- Optional: requested kubectl client binary (e.g., `kubectl-1.28`) when multiple clients are installed.

## Command workflow (kubectl)

1. Confirm context and namespace scope.
   - `kubectl version --client`
   - `kubectl version`
   - `kubectl config current-context`
   - `kubectl get ns`
2. Snapshot workload health.
   - `kubectl get pods -n <ns> -o wide`
   - `kubectl get deploy,rs,svc -n <ns>`
3. Inspect failing pod(s).
   - `kubectl describe pod <pod> -n <ns>`
   - `kubectl logs <pod> -n <ns> --previous`
   - `kubectl logs <pod> -n <ns> --tail=200`
4. Correlate with events and probe failures.
   - `kubectl get events -n <ns> --sort-by=.lastTimestamp`
5. Distinguish infra vs app failure.
   - Scheduling/image pull/cni/node pressure vs app crash/config/runtime error.

## Output requirements

- `summary`: primary failure mode in one sentence.
- `findings`: 3-6 bullets with concrete resource names, reasons, and timestamps.
- `artifacts`: key event/log lines or commands used.
- `confidence`: based on directness of evidence.

## Guardrails

- Read-only triage: no delete, patch, rollout restart, or exec by default.
- If namespace/context is unclear, stop and request clarification.
- If cluster version is missing, return `validation_fail` guidance asking for exact cluster version.
- If context + namespace + workload identifiers are not available, return `insufficient_evidence` and callback to parent for missing fields.
- Do not use placeholder pod/deployment names in findings.
