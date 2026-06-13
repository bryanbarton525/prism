# Prism Test Data

This directory contains committed fixtures used by CLI smoke tests, package
tests, documentation examples, and local acceptance flows.

Current fixture groups:

- `benchmarks/`: golden benchmark scenarios, pricing, model, and scale inputs.
- `bundles/`: signed registry and bundle fixtures for managed install tests.
- `graphs/`: bounded graph definitions for validation and run tests.
- `policies/`: Prism policy files plus fixture-driven policy test suites.

There are intentionally no `testdata/agents` or `testdata/skills` directories
right now. Tests that need temporary agents or skills create isolated fixtures
under `t.TempDir()`, while integration and acceptance examples exercise the
repo's real `agents/` and `skills/` trees. Add `testdata/agents` or
`testdata/skills` only when a test needs stable miniature fixtures that should
not use the production examples.
