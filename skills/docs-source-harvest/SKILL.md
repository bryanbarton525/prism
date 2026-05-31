---
name: docs-source-harvest
description: Harvest authoritative docs for a target library/service and extract implementation-ready guidance with citations. Trigger on "how do I use/configure X" and API/reference requests.
compatibility: Requires internet access or provided doc URLs; prefers official docs and release pages.
metadata:
  prism-agents: web-docs-search
---

# Docs source harvest

## Inputs expected

- Product/library name and version (if known).
- Question focus: setup, API usage, config, auth, migration, limits.

## Retrieval strategy

1. Prioritize official docs and reference pages.
2. Prefer version-matched docs for the caller's stack.
3. Use secondary sources only when official docs are silent.
4. Capture exact URLs for every material claim.

## Extraction checklist

- Minimal "how-to" path for the request.
- Required flags/fields and defaults.
- Version constraints and incompatibilities.
- Security/auth caveats.

## Output requirements

- `summary`: direct answer in 1-2 sentences.
- `findings`: concise bullets with one citation each.
- `artifacts`: optional snippet blocks (commands/config examples).
- `confidence`: based on source quality and version match.

## Guardrails

- No uncited recommendations for critical configuration.
- Explicitly call out gaps when authoritative docs are missing.
