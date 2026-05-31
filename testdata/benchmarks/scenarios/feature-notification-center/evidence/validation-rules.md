# Validation rules for preference updates

## Core invariants

1. At least one channel enabled for `security`.
2. `incident` must keep `in_app=true`.
3. `sms=true` requires `phone_verified=true`.
4. `digest.mode` in {`instant`, `hourly`, `daily`, `weekly`}.
5. Quiet hours require valid IANA timezone if enabled.
6. Quiet-hours interval must not be zero-length.
7. Quiet-hours crossing midnight is allowed.

## Error keys

- `category.security.no_channel_enabled`
- `category.incident.in_app_required`
- `channel.sms.phone_unverified`
- `digest.mode.invalid`
- `quiet_hours.timezone_invalid`
- `quiet_hours.window_invalid`

## Helper direction

Expose pure helper:

`ValidateUpdate(current Preferences, req UpdateRequest, phoneVerified bool) []ValidationError`

Requirements:
- deterministic ordering of errors
- no DB/network calls
- safe for direct table-driven unit tests
