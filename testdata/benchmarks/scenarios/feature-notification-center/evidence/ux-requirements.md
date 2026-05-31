# UX requirements: Notification Preferences Center

## Product goals

- Give users confidence they are in control of alerts.
- Prevent alert fatigue with digest and quiet-hour controls.
- Keep incident/security notices visible and hard to disable accidentally.

## Screen shape

One page with four stacked sections:

1. Channel defaults
2. Category matrix
3. Digest settings
4. Quiet hours

## Category matrix

Rows: `security`, `billing`, `product`, `incident`  
Columns: `email`, `sms`, `in_app`

Behavior:
- `incident` always allows `in_app` and cannot be fully disabled.
- `security` requires at least one channel.
- `sms` requires verified phone.

## Digest settings

Allowed values:
- `instant`
- `hourly`
- `daily`
- `weekly`

Rules:
- `incident` and high severity `security` bypass digest.
- Digest applies to `product` and `billing` by default.

## Quiet hours

Fields:
- `enabled` (bool)
- `start_local_time` (`HH:MM`)
- `end_local_time` (`HH:MM`)
- `timezone` (IANA tz)

Rules:
- Quiet hours suppress non-critical pushes/sms.
- Critical `incident` notifications ignore quiet hours.
- If timezone invalid, block save with actionable error.

## Accessibility + copy

- Every toggle has clear label and hint text.
- Error messages map to stable error keys, not prose matching.
- Confirmation banner: "Notification preferences updated."
