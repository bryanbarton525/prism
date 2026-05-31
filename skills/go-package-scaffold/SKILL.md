---
name: go-package-scaffold
description: Draft boilerplate for a new Go package file (doc comment, types, constants, constructor stubs). Use when the orchestrator is creating a package layout.
compatibility: Requires module path, package name, and API sketch from the orchestrator.
metadata:
  prism-agents: go-scaffold
---

# Go package scaffold

1. Parse the desired package path, public API, and dependencies from the task.
2. Produce one primary file (e.g. `doc.go` + types, or `service.go` stub) — not an entire module tree.
3. Include package comment and exported symbol godoc stubs.
4. List follow-up files in `findings`; put paste-ready content in `code`/`artifacts`.
