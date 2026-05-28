---
name: k8s-rollout-diagnostics
description: Diagnose deployment and rollout issues using kubectl rollout/status/history patterns. Use when releases stall, crash-loop, or fail readiness.
compatibility: Requires kubectl and deployment access.
metadata:
  prism-agents: kubectl
---

# K8s rollout diagnostics

1. Check rollout status and history.
2. Correlate failures with events and readiness signals.
3. Report likely causes and safe follow-up commands.
