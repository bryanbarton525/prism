# Runbook index (abbreviated — full library loaded without MCP)

| ID | Title | Trigger |
|----|-------|---------|
| RB-1042 | Platform upgrade | version bump PR |
| RB-2101 | payments-api rollback | CrashLoopBackOff |
| RB-2102 | Argo manual sync | OutOfSync > 15m |
| RB-3301 | ETL workflow replay | extract failure |
| RB-4401 | GitHub deploy-homelab | CI red post-merge |

Each runbook section below expands to 400-800 tokens when pasted into orchestrator context.

## RB-1042 Platform upgrade

1. Verify release notes for breaking changes.
2. Open PR with chart + app version bump.
3. Run deploy-homelab on homelab before prod.
4. Validate Argo sync for all payment services.
5. Run nightly-etl smoke workflow.

## RB-2101 payments-api rollback

1. Identify failing flag or config key from pod logs.
2. Diff live vs desired Argo manifest.
3. Rollback deployment or revert PR.
4. Confirm 3/3 replicas ready.

## RB-2102 Argo manual sync

1. Capture diff summary.
2. Resolve OutOfSync resources individually.
3. Re-sync with prune disabled in homelab.

## RB-3301 ETL workflow replay

1. Inspect failed node inputs.
2. Validate API v2 endpoints in extract template.
3. Retry from failed step.

## RB-4401 deploy-homelab failure

1. Open GitHub Actions log for kubectl apply step.
2. Correlate with cluster events in payments namespace.
3. Block merge until green.

Duplicate index entries for scale padding — enterprise teams attach full runbook exports.

Service RB-5001 auth-gateway upgrade parallels RB-1042 with OAuth scope checks.
Service RB-5002 ledger-api migration requires job completion before sync.
Service RB-5003 notifications-worker topic ACL verification post upgrade.
Service RB-5004 audit-ingest bucket policy unchanged during platform bump.
Service RB-5005 rate-limiter redis key prefix v2 migration optional in homelab.

Contact tree: platform-oncall → architect → VP infra for SEV1 escalation.
Status page components: homelab-platform, payments-api, etl-workflows, github-ci.

Postmortem template sections: timeline, root cause, action items, lessons learned.
Each section guideline adds orchestrator tokens when loaded monolithically.

End runbook index padding.
