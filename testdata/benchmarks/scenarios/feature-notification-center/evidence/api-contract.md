# API contract notes

## Endpoint: read preferences

`GET /v1/users/{id}/notification-preferences`

Response sketch:

```json
{
  "user_id": "u_123",
  "categories": {
    "security": {"email": true, "sms": true, "in_app": true},
    "billing": {"email": true, "sms": false, "in_app": true},
    "product": {"email": true, "sms": false, "in_app": true},
    "incident": {"email": true, "sms": true, "in_app": true}
  },
  "digest": {"mode": "daily", "hour_local": 8},
  "quiet_hours": {"enabled": true, "start": "22:00", "end": "07:00", "timezone": "America/New_York"},
  "version": 12,
  "updated_at": "2026-05-31T12:00:00Z"
}
```

## Endpoint: update preferences

`PUT /v1/users/{id}/notification-preferences`

Request constraints:
- optimistic lock via `version`
- partial updates allowed for `digest` and `quiet_hours`
- category/channel map update is full replacement for deterministic behavior

Error shape:

```json
{
  "error": "validation_failed",
  "error_key": "quiet_hours.timezone_invalid",
  "message": "timezone must be an IANA timezone"
}
```

## Service/package guidance

Target package: `internal/notifications/preferences`

Suggested files:
- `types.go` (domain types)
- `service.go` (business logic)
- `store.go` (interface)
- `validate.go` (pure validation helpers)
- `http_handlers.go` (wire format mapping)

Migration note:
- backfill users without preferences with default profile at read time first,
  then run async migration to persist defaults.
