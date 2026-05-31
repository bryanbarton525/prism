---
name: go-package-scaffold
description: Draft concise Go package scaffolding (doc comments, core types, constructor stubs). Trigger on "create package", "bootstrap module folder", or "initial service skeleton".
compatibility: Requires module path, package name, and API sketch from the orchestrator.
metadata:
  prism-agents: go-scaffold
---

# Go package scaffold

1. Parse package path, target API, and dependencies.
2. Produce one primary scaffold file (`doc.go`, `types.go`, or `service.go` stub).
3. Include package comment and exported symbol doc stubs.
4. Keep placeholders explicit (`TODO`) only where caller context is missing.
5. Provide follow-up file suggestions in `findings`.
6. Include quick validation hints when appropriate:
   - `go test ./...`
   - `go list ./...`
