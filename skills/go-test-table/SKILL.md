---
name: go-test-table
description: Scaffold idiomatic table-driven tests for one Go function/method. Trigger on "add tests", "cover edge cases", or "write _test.go for this helper".
compatibility: Requires the function under test and example cases from the orchestrator.
metadata:
  prism-agents: go-scaffold
---

# Go table-driven test scaffold

1. Identify function under test and package path.
2. Emit `_test.go` scaffold with `tests := []struct{...}` and `t.Run`.
3. Include 3-6 cases: happy path, edge cases, and explicit error behavior.
4. Keep tests deterministic (no clock/network/fs unless provided mock seams).
5. Include minimal command hints:
   - `go test ./... -run <TestName>`
   - `go test ./...`
6. Return full test file in `code`; keep `summary` to one sentence.
