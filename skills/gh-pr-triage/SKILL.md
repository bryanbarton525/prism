---
name: gh-pr-triage
description: Triage a GitHub PR for merge readiness using gh (checks, reviews, blockers, changed files). Trigger on prompts about PR health, blocked merges, review status, or "is this ready?".
compatibility: Requires GitHub CLI and repository read permissions; collector can auto-download gh into ~/.cache/prism/bin (set GH_VERSION / GH_BIN / PRISM_BIN_DIR).
metadata:
  prism-agents: github-cli
---

# GH PR triage

## Inputs expected

- PR number, URL, branch, or commit hint.
- Repository context (`owner/repo`) if not inferable from cwd.
- Optional focus: checks, reviewers, risky file changes, mergeability.

## Command workflow (gh CLI)

1. Resolve PR identity.
   - `gh pr view <pr> --json number,title,state,isDraft,mergeStateStatus,reviewDecision,headRefName,baseRefName,author`
2. Inspect checks and CI status.
   - `gh pr checks <pr>`
   - `gh pr view <pr> --json statusCheckRollup`
3. Inspect review and conversation blockers.
   - `gh pr view <pr> --json reviews,comments,latestReviews`
4. Inspect file-level risk quickly.
   - `gh pr diff <pr> --name-only`
   - Highlight infra/security/migration paths (`deploy/`, `charts/`, `db/`, auth).
5. If CI is failing, hand off to `gh-actions-diagnostics` with failing run IDs.

## Output requirements

- `summary`: one sentence verdict (ready / blocked / uncertain).
- `findings`: 3-6 concise bullets with evidence.
- `artifacts`: include command snippets or key command outputs.
- `confidence`: high/medium/low based on evidence coverage.

## Guardrails

- Read-only diagnostics only; do not merge, approve, or edit PR state.
- If permissions are missing, report exactly which command failed.
- If `repo` or PR identifier is missing, return `insufficient_evidence` and request callback to parent with required IDs.
- Never emit placeholder paths/IDs (for example `<pr>`, `<path>`) as factual findings.
