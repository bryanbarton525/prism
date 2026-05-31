# Test plan notes

## Table-driven validation suite

Cases:
- valid default preferences
- invalid digest mode
- security all channels off
- incident in_app false
- sms enabled with unverified phone
- quiet hours enabled with bad timezone
- quiet hours equal start/end
- quiet hours crossing midnight (valid)

## Service update tests

- version mismatch returns conflict
- migration fallback returns defaults when no row exists
- update applies deterministic normalization
- update preserves unchanged categories on partial quiet-hours update

## Handler tests

- decode failure -> 400
- validation errors -> 422 + `error_key`
- success -> 200 with incremented version
