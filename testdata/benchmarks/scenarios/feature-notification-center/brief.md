# Feature brief — Notification Preferences Center

You are implementing a production feature for a Go service and a web UX:
**Notification Preferences Center**.

Goal: allow each user to control channel + category notifications, digest timing,
and quiet hours with clear API contracts and test coverage.

## Requested scope

1. UX structure for a settings screen:
   - categories (`security`, `billing`, `product`, `incident`)
   - channels (`email`, `sms`, `in_app`)
   - digest options (`instant`, `hourly`, `daily`, `weekly`)
   - quiet hours (`start`, `end`, `timezone`)
2. Backend package skeleton under `internal/notifications/preferences`
3. API contract for
   `GET /v1/users/{id}/notification-preferences` and
   `PUT /v1/users/{id}/notification-preferences`
4. Validation helper behavior and table-driven test plan
5. Migration notes for existing default notification settings

## Deliverable

Produce a concise implementation plan with:
- architecture decisions
- API shape and data model
- test strategy
- migration and rollout order

Do not propose unrelated refactors.
