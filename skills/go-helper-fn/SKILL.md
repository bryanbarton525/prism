---
name: go-helper-fn
description: Draft a single Go helper function or small utility from provided signatures and call-site context. Use when the orchestrator needs one focused function while building a package.
compatibility: Requires Go source excerpts and package context from the orchestrator.
metadata:
  prism-agents: go-helper
---

# Go helper function

1. Read the target signature, callers, and error-handling patterns in the evidence.
2. Implement one exported or unexported helper — no unrelated helpers in the same response.
3. Return JSON with compact `summary`/`findings` and full code in `code` or `artifacts`.
4. Note tests the orchestrator should add (do not generate large test files unless asked).
