# Eino and DeepEval review

Reviewed 2026-10-20 against Eino commit
`0a5ca53b9d3bf7b661dccc863465ec568a421eea` and DeepEval commit
`727203702cddf68e4f01a6ef95b23d9cb9036654`, plus their current official docs.
This records design inspiration; neither project is added as a dependency and no
implementation code is copied.

## Eino: execution visibility and composition

Eino separates model/tool components from agent execution and graph composition.
Its callback interfaces expose component identity and lifecycle inputs, outputs,
and failures, with context propagated between a handler's lifecycle calls.
That is useful for locating a failure in a workflow rather than inspecting only
its final answer. See [Eino overview](https://github.com/cloudwego/eino) and
[agent callbacks](https://www.cloudwego.io/docs/eino/core_modules/eino_adk/adk_agent_callback/).

Applied in this change: model records include start time, duration, request,
response, usage, and errors. Downstream MCP records include server/tool identity,
arguments, start time, duration, full fixture response, and argument-match result.
The outer production MCP response is preserved too. These records distinguish
provider failure, tool-use failure, and final-answer failure without coupling the
agent runner to a particular tracing service.

Follow-up: extend Prism's existing run-level observer with component lifecycle
hooks, correlation IDs, and cancellation events. Keep observers optional and
propagate context; export the same component traces for production diagnostics
and evals. Evaluate graph-style typed composition only when a workflow needs
branching/state beyond Prism's existing bounded tool loop. Eino's checkpoint and
resume pattern is relevant to interactive approvals, but resuming partial model
trials needs its own accounting rules before it can support fair benchmarks.

## DeepEval: explicit test cases and diagnostic metrics

DeepEval separates test cases from metric execution. Tool correctness can check
names, input arguments, outputs, order, and exact matching. Metrics include a
score, threshold, and reason; component and trajectory evaluations give different
views of agent behavior. See [tool correctness](https://deepeval.com/docs/metrics-tool-correctness)
and [evaluation introduction](https://deepeval.com/docs/introduction).

Applied in this change:

- Each deterministic check now emits a metric name, score, threshold, pass/fail,
  and reason. Incorrect decision fields show expected and observed values.
- Suites can declare an exact `expected_tool_sequence`, using server-qualified
  tool identities. A new two-server fixture requires extracting a project ID
  from one result before requesting the project from another server.
- Actual tool execution evidence is kept separate from model-written artifacts;
  an artifact claiming a tool ran cannot satisfy a required bridge-call check.
- Dataset exports include one row per metric in `metrics.jsonl`, in addition to
  raw trial traces and per-case comparison scores.

DeepEval also models MCP server inventories and tools/resources/prompts called
with their actual results. Prism currently exposes downstream tools through its
bridge, so this suite evaluates that supported path. Add resource/prompt cases
when those primitives are exposed by Prism, using the same execution records.
See [DeepEval MCP evaluation](https://deepeval.com/docs/evaluation-mcp).

## Priorities after the initial baseline

1. Add representative fixtures from real Prism workflows, sanitized and reviewed,
   alongside the current synthetic probes. Keep a held-out comparison set separate
   from prompt-development cases; version and record both.
2. Add optional prose-quality and evidence-grounding judges for aspects that
   deterministic facts cannot cover. Record the judge's model, prompt, version,
   raw response, and calibration results. Judge failure must be visible; it must
   never turn into a passing functional check. No automatic paid-model fallback.
3. Add per-component token/latency summaries, error taxonomy, confidence intervals,
   and paired model comparisons when sample counts support meaningful claims.
   Keep functional gates separate from performance and subjective quality scores.
4. Add optional DeepEval export/integration as a separate analysis process if its
   metrics are useful. Prism's Go CLI and offline regression tests should remain
   usable without Python, hosted accounts, or a model judge.

The immediate decision is to adopt these execution and evaluation patterns in
Prism's existing interfaces. A framework migration would need a separate case
showing a benefit over the current model abstraction, MCP loop, and observers.
