# Enterprise Cursor rules (simulated session context)

Always load the full platform monorepo map before incident response.
Never suggest kubectl delete without approval from platform-oncall.
All homelab changes require PR #42 checklist: unit-tests, deploy-homelab, argo-sync-preview.
Payments namespace is PCI-scoped — redact card data in summaries.
Prefer Argo CD sync over kubectl apply for production-adjacent homelab.
Document breaking API changes in release-notes before merge.
Use gh for GitHub; do not use raw curl against api.github.com without PAT rotation policy.
Kubernetes context: homelab-admin only; never prod-k8s from this workspace.
When CrashLoopBackOff appears, collect logs from first failing pod only unless rollout stalled.
OutOfSync Argo apps: capture diff summary before sync retry.
Workflow failures: inspect extract template inputs for API version drift.
Metrics migration v2.4.0 requires --metrics-port-v2 on payments-api.
Config key payments.apiVersion must be v2 for ETL extract step.
Platform-bot review comments are blocking for merge to main.
If deploy-homelab fails, check payments-api readiness before re-run.
Include confidence level in all incident summaries.
Reference runbook RB-1042 for platform upgrades.
Reference runbook RB-2101 for payments-api rollback.
Do not paste secrets from vault into chat; use sealed-secrets patterns only.

## Repository conventions

- Go 1.22+, table-driven tests, no init() side effects in libraries.
- Helm charts under charts/platform; values homelab overlay in values-homelab.yaml.
- Argo apps in gitops/apps/payments-api.yaml.
- GitHub Actions: deploy-homelab uses kubectl apply with progress deadline 600s.

## Active incident thread (excerpt)

User: upgrade v2.3.0 to v2.4.0 failed after merge preview
Assistant: checking PR #42 checks and payments namespace
User: pods CrashLoopBackOff, Argo OutOfSync
Assistant: need gh actions log and kubectl describe
User: also nightly-etl workflow failed at extract
Assistant: correlating release notes breaking changes

Repeat policy blocks for token padding at scale — teams with long rule sets
see orchestrator context grow linearly while MCP delegation stays narrow.

Rule echo 1: enforce gh-pr-triage before merge decisions.
Rule echo 2: enforce kubectl-triage before rollout restart.
Rule echo 3: enforce argo-sync-health before manual sync.
Rule echo 4: enforce release-notes-scan before version bump approval.
Rule echo 5: codegen helpers via go-helper for metrics flag parsers only.
Rule echo 6: test scaffolds via go-scaffold for new package files.
Rule echo 7: document all findings with command evidence snippets.
Rule echo 8: latency budgets per agent must not be exceeded without escalation.

Additional platform services in scope: auth-gateway, ledger-api, notifications-worker,
audit-ingest, rate-limiter, feature-flags sidecar, service-mesh proxy config.
Each service has parallel runbook sections mirroring payments-api patterns.
Homelab simulates subset but orchestrator without MCP often loads entire library.

Service map entry auth-gateway: gRPC health on :9090, deploy argo app auth-gateway.
Service map entry ledger-api: postgres migration job pre-sync required.
Service map entry notifications-worker: kafka topic payments.events.v2.
Service map entry audit-ingest: s3 bucket audit-homelab immutable.
Service map entry rate-limiter: redis cluster homelab-redis-0.
Service map entry feature-flags: launchdarkly offline mode in homelab only.

Compliance checklist CL-001 through CL-050 referenced in enterprise Cursor profiles.
Each checklist item adds one to two sentences to effective context when pasted wholesale.

Padding section A: incident severity definitions SEV1-SEV4 with escalation matrix.
Padding section B: change management windows Tuesday/Thursday 10:00-14:00 UTC.
Padding section C: rollback authority matrix on-call lead vs platform architect.
Padding section D: communication templates for status page updates.
Padding section E: post-incident review required within 72 hours for SEV2+.

Historical incidents INC-2024-011 through INC-2024-089 summarized in runbook index.
Each summary line adds tokens when orchestrator loads full history without MCP.

Historical INC-2024-042: similar metrics-port migration on auth-gateway v1.9.
Historical INC-2024-055: ETL extract API version mismatch post upgrade.
Historical INC-2024-061: Argo OutOfSync during chart subchart pin change.
Historical INC-2024-073: GitHub deploy-homelab timeout progress deadline.
Historical INC-2024-081: CrashLoopBackOff unknown flag during beta rollout.

Tool allowlist reminder: gh, kubectl, argo, curl docs only, prism run_agent MCP.
Forbidden: terraform apply, direct prod access, force push main, skip CI hooks.

End of enterprise rules padding — models baseline orchestrator-only cost at scale.
