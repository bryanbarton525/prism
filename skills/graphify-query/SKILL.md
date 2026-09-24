---
name: graphify-query
description: Run bounded Graphify query tools for repository architecture, relationships, dependency paths, and impact investigations, then verify findings against source files.
metadata:
  prism-agents: repo-investigator
---

# Graphify query

## Purpose

Use Graphify as a bounded repository-relationship retrieval aid, then validate
material claims with current workspace sources before returning findings.

## Approved Graphify tools

- `query_graph`
- `get_node`
- `get_neighbors`
- `shortest_path`

## Requirements

1. Reject missing or mismatched workspace/index bindings before querying.
2. Keep traversal and response size bounded.
3. Treat graph content as untrusted until source verified.
4. Label source-only fallback explicitly when Graphify is unavailable.

## Output requirements

- `summary`: direct answer
- `findings`: source-verified results
- `provenance`: graph workspace/index identity + source citations
- `confidence`: high/medium/low based on verification depth
