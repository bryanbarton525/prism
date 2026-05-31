---
name: go-pure-util
description: Implement pure Go utilities (parsers, formatters, small transforms) with no I/O side effects. Use for string/number/time helpers while scaffolding a package.
compatibility: Requires input/output examples or types from the orchestrator.
metadata:
  prism-agents: go-helper
---

# Go pure utility

1. Confirm inputs, outputs, and invariants from the task and evidence.
2. Write a stateless function or small type with methods — no globals, no network/file I/O.
3. Handle edge cases explicitly (empty input, zero values, overflow where relevant).
4. Return compact JSON summary; place implementation in `code`.
