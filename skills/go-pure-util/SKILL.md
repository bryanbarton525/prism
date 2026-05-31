---
name: go-pure-util
description: Build pure Go parsers/formatters/transforms with deterministic behavior and no I/O. Trigger on prompts for stateless utility logic and edge-case-safe helpers.
compatibility: Requires input/output examples or types from the orchestrator.
metadata:
  prism-agents: go-helper
---

# Go pure utility

1. Confirm input/output contract and invariants from provided evidence.
2. Write stateless code only (no globals, network, filesystem, env dependencies).
3. Handle edge cases explicitly (empty input, malformed values, bounds/overflow).
4. Note complexity/allocation tradeoffs when non-trivial.
5. Suggest local validation commands:
   - `go test ./...`
   - `go test ./... -run <UtilityName>`
6. Return compact JSON summary and full implementation in `code`.
