# Platform v2.4.0 release notes (excerpt)

## Breaking changes

- **payments-api**: requires `--metrics-port-v2` and `payments.apiVersion: v2` in config.
- **ETL workflows**: `extract` template must reference API v2 endpoints.

## Fixes

- Argo CD sync performance improvements for homelab clusters.
