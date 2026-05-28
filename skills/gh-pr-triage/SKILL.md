---
name: gh-pr-triage
description: Triage pull requests with gh by collecting status, changed files, review signals, and merge blockers. Use whenever the task mentions PR health, review state, or merge readiness.
compatibility: Requires GitHub CLI access and repository read permissions.
metadata:
  prism-agents: github-cli
---

# GH PR triage

1. Gather PR metadata, status checks, and review state.
2. Identify blockers and unresolved checks.
3. Return concise findings with command evidence.
