---
name: go-test-table
description: Scaffold table-driven Go tests for one function or method. Use when the orchestrator is adding tests alongside new package code.
compatibility: Requires the function under test and example cases from the orchestrator.
metadata:
  prism-agents: go-scaffold
---

# Go table-driven test scaffold

1. Identify the function under test and its package from the task.
2. Emit `_test.go` skeleton with `tests := []struct{ name string; ... }`.
3. Include 3–6 cases covering happy path, edge, and error paths when applicable.
4. Use `t.Run(tt.name, ...)` and leave TODO only where caller context is missing.
5. Return full test file in `code`; keep `summary` to one sentence.
