# Homelab release incident — orchestrator brief

You are investigating a failed homelab platform upgrade (`v2.3.0` → `v2.4.0`).

**Symptoms reported**

1. GitHub PR **#42** ("Bump platform to v2.4.0") has failing checks and review comments.
2. GitHub Actions workflow **deploy-homelab** failed on the latest push.
3. Kubernetes namespace **payments** has pods in **CrashLoopBackOff**.
4. Deployment **payments-api** rollout is stalled at 1/3 ready.
5. Argo CD application **payments-api** is **OutOfSync** and **Degraded**.
6. Argo Workflow **nightly-etl** failed at the `extract` template.
7. Platform v2.4.0 docs mention API changes; release notes may explain regressions.

**Your deliverable**

Produce a single incident report with:

- Executive summary (root cause hypothesis)
- Findings per area (GitHub, Kubernetes, Argo CD, Argo Workflows, docs/release notes)
- Recommended next actions in priority order
- Confidence level (low / medium / high)

Use only the evidence provided. Do not invent resource names beyond those listed.
