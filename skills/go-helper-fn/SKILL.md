---
name: go-helper-fn
description: Implement one focused Go helper function from call-site context. Trigger on prompts like "add helper", "extract parser/validator", or "small utility for this package".
compatibility: Requires Go source excerpts and package context from the orchestrator.
metadata:
  prism-agents: go-helper
---

# Go helper function

1. Read target signature, callers, and error-handling style from evidence.
2. Implement one exported/unexported helper; avoid unrelated refactors.
3. Match local conventions (`fmt.Errorf("%w")`, naming, zero-value behavior).
4. Include edge-case handling and explicit failure modes.
5. If suitable, suggest quick validation commands:
   - `go test ./...`
   - `gofmt -w <file.go>`
6. Return compact JSON (`summary`, `findings`) and full helper code in `code`/`artifacts`.
