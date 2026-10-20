# Model response evaluations

Latest repeated cluster results, controls, raw traces, and graphs:
[cross-runtime Laya and task-protocol evaluation](reports/2026-10-20-cross-runtime/README.md).
The earlier [optional-routing comparison](reports/2026-10-20-contract-routing/README.md)
remains a single-pass smoke dataset, not an approved consistency baseline.

For a memory-guarded Ollama runner and recorded Laya off/on commands, see
[the reproducible evaluation guide](../../evals/README.md).

`prism eval` measures functional correctness and response consistency through
Prism's production `run_agent` MCP adapter, bundled agent prompts, and model/tool
loop. Local HTTP MCP servers replay synthetic downstream responses. The run
never uses configured downstream servers, workspace files, or native service
credentials. The configured primary model endpoint is live (and the explicitly selected Laya endpoint when `--laya-url` is used); no fallback is
used. `--model` overrides the requested model for this evaluation only.

The default `mcp-response-v2` suite supplies explicit decision schemas. Grader
`mcp-outcomes-v2` validates structured delivery and evidence-based outcomes.
Laya is optional: omit `--laya-url` to retain the original model-driven loop.
An explicit endpoint enables bound routing; `--laya-mode advisory` selects the
legacy advisory path. Baselines cannot mix these modes.

For a production-path lexical comparator, use `--lexical-advice` instead of
`--laya-url`. It enables the same recommendation injection with word-overlap
scores, without an embedding model or Laya call. The actual recommendation trace
and a `lexical_applied` check are retained. Control reports and baselines record
`control_mode: "lexical"` and artifact version 2; older CLIs reject them, and
comparisons cannot mix lexical, plain, or Laya modes. Existing non-control
artifacts remain version 1. Build the current checkout if the installed CLI
does not list this flag.

The versioned suite covers issue lookup, empty results, permission
errors, conflicting historical evidence, instructions embedded in tool data,
and ordered lookups across two servers.
These are MCP workflow probes, not claims of general model intelligence. Add
representative prompts and fixtures as new cases; changes require a new reviewed
baseline. Existing skill smoke YAML and the incident benchmark remain separate.

## Run loops and approve a baseline

Using the configured runtime and the suite embedded in the binary (or select a
custom YAML file with `--suite`):

```sh
prism eval run --model qwen3.5:9b,llama3.1:8b --repeat 5 --output comparison.json
prism eval baseline create comparison.json --model qwen3.5:9b --output baseline.json
prism eval test --baseline baseline.json --model llama3.1:8b --output candidate.json
```

Use exact model IDs served by the endpoint. Each response records the served ID;
an unexpected served ID fails validation. All models in one run use the same
engine and endpoint. To compare different engines/endpoints, use separate runtime
configuration environments and run reports, then `eval compare` against the same
baseline. Configuration is never saved by evaluation commands. For example:

```sh
PRISM_MODEL_RUNTIME_ENGINE=ollama \
PRISM_MODEL_RUNTIME_BASE_URL=http://127.0.0.1:11434 \
prism eval run --model qwen3.5:9b --output ollama-run.json
```

Default trials: three per model/case, temperature 0.1, 1024 output tokens per
model call, 90-second timeout per complete agent trial. Model order is interleaved
by repetition and case; no failures are retried away. Every trial contributes to
the denominator, including timeouts and malformed responses. A crash retains
completed trials in `<output>.trials.jsonl`. Reports and baselines cannot overwrite
existing files. Cancelled/incomplete reports cannot become baselines.

Baseline creation is explicit and requires at least three trials per case, all
passing and semantically consistent. This strict initial policy prevents poor
responses from becoming an accepted standard. A baseline records suite and grader
hash/version, source report hash, generation settings, and per-case scores/sample
counts. Keep the approved report alongside its baseline for review.

`eval test` inherits baseline generation settings and runs at least its sample
count. It returns a nonzero status for regressions. Suite, grader, and generation
settings must match. Prompt/bundle updates are allowed: their identities and full
model requests are recorded so their behavior can be tested against existing
outcome requirements. Review and deliberately create a new baseline when changing
suite expectations; do not automatically regenerate it in CI.

For reports already collected:

```sh
prism eval compare baseline.json candidate.json --model llama3.1:8b
```

## Scoring

The deterministic grader checks the response JSON envelope, exact typed decision
facts via JSON pointers, successful required MCP calls with expected arguments,
optional exact server/tool call sequences, forbidden tool attempts, successful
MCP/agent completion, and requested versus
served model identity. Expected facts and argument values are never included in
the model context by the harness. Tool schemas and descriptions are visible.

Semantic consistency is the fraction of all trials matching the most common
signature of declared decision fields. Prose can vary. Invalid/missing decisions
have no signature. Consistently wrong answers can have high consistency and zero
correctness; both scores must pass. The initial rubric does not grade prose
quality or detect every possible unsupported statement outside decision fields.

`go test ./internal/eval ./internal/cli` includes deterministic integration tests
through the production MCP and downstream HTTP paths. Tests inject incorrect
facts, omitted tool use, invalid JSON, wrong arguments/model IDs, and semantic
drift; they also verify baseline compatibility, incomplete-run rejection, and
export integrity. These tests need no live model. Live regression tests use
`prism eval test` and are deliberately separate from ordinary offline CI.

## Publishable comparison data

```sh
prism eval export comparison.json --output comparison-dataset
```

This writes all trial traces (requests, responses, tokens, timings, tool calls,
errors, results, grades) as JSONL, scores as CSV, the suite, build/generation
metadata, a dataset card, and SHA-256 checksums. No external publication occurs.
Runtime authentication configuration and provider URLs are not collected; review
model outputs and error text before publishing. Synthetic fixtures use the
suite's declared license; generated output rights depend on model/provider terms.

Record runtime/server version, model weight digest/quantization, hardware, and
runtime sampling settings outside Prism before publishing results. Model names
alone do not identify weights. The harness records Prism revision, bundle digest,
context length, requested generation settings, and served IDs, but does not verify
server-side settings, weight identity, or hidden sampling defaults. Small trial
counts establish a practical functional baseline, not statistical significance.

Each check also records a score, threshold, and diagnostic reason; exported
`metrics.jsonl` makes these available for comparison analysis. See the
[Eino and DeepEval review](framework-review.md) for the design ideas applied here
and follow-up improvements.

## Captured results

- [Guarded Ollama and Laya comparison, v0.1.32](reports/2026-10-20-guarded-ollama/README.md): 72 retained trials, full traces, reproducible charts, and memory telemetry.
