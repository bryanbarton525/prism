---
name: release-notes-scan
description: Scan release notes/changelogs between versions for breaking changes and migration actions. Trigger on upgrades, regressions after bumps, or "what changed from X to Y?" prompts.
compatibility: Requires changelog/release-note URLs or searchable product/version identifiers.
metadata:
  prism-agents: web-docs-search
---

# Release notes scan

## Inputs expected

- Product/library name.
- From-version and to-version (or release date window).

## Retrieval strategy

1. Locate official changelog/release-note pages.
2. Gather entries across the full version span (not only latest).
3. Capture explicit references to:
   - breaking changes
   - deprecations/removals
   - config/schema/API migrations
   - behavior/perf/security changes

## Analysis method

1. Build a delta list grouped by impact domain (runtime, API, config, data).
2. Mark each delta as required/optional action.
3. Derive concrete "before deploy" checks.

## Output requirements

- `summary`: highest-risk upgrade deltas in one paragraph.
- `findings`: actionable items with affected component and source URL.
- `artifacts`: optional migration checklist.
- `confidence`: high if version range is complete; lower if notes are partial.

## Guardrails

- Do not infer breaking changes without source text.
- If release-note coverage is incomplete, state missing versions explicitly.
