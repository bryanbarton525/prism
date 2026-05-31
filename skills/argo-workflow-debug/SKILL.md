---
name: argo-workflow-debug
description: Debug Argo Workflow failures with argo CLI by tracing nodes, retries, template errors, and timing. Trigger on failed/hanging workflows, extract-step errors, or regression after upgrades.
compatibility: Requires workflow visibility; collector can auto-download argo CLI (ARGO_VERSION / ARGO_BIN / PRISM_BIN_DIR).
metadata:
  prism-agents: argo
---

# Argo workflow debug

## Inputs expected

- Workflow name/UID, namespace, and failing step or template hint.
- Optional run window (latest run, last N failures).

## Command workflow (argo)

1. Inspect workflow status and node graph.
   - `argo list -n <ns>`
   - `argo get <workflow> -n <ns>`
2. Inspect failed nodes and retry behavior.
   - `argo get <workflow> -n <ns> -o json`
   - identify `phase`, `message`, retry count, failing template.
3. Inspect logs for failing steps.
   - `argo logs <workflow> -n <ns>`
   - `argo logs <workflow> -n <ns> --node-field-selector phase=Failed`
4. Correlate likely failure class.
   - bad inputs/config key missing
   - upstream service/API version mismatch
   - transient dependency timeout
   - template/runtime image issue

## Output requirements

- `summary`: failure locus and likely cause.
- `findings`: failing node/template, first explicit error, retry pattern, impact.
- `artifacts`: node/log snippets with identifiers.
- `confidence`: high only with explicit node/log error text.

## Guardrails

- Do not terminate/resubmit/rerun workflows unless explicitly requested.
