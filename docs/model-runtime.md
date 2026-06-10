# Prism model runtime

Prism's model runtime layer lets specialist workflows depend on one internal
interface instead of provider-specific clients. The first Prism-only runtime
targets the OpenAI-compatible `chat/completions` surface used by SGLang and
vLLM.

This is not full OpenAI API compatibility. The adapter supports the pieces Prism
uses: health checks, non-streaming chat, SSE streaming chat, and JSON-schema
structured output.

## Architecture

The runtime packages live under `internal/llm`:

- `internal/llm/runtime`: config structs, `ModelRuntime`, normalized errors,
  OpenAI-compatible adapter, streaming, and structured output.
- `internal/llm/fallback`: optional primary/fallback wrapper.
- `internal/llm`: runtime factory.

Prism startup builds the configured runtime once and injects it into the shared
agent runner. Existing Ollama behavior remains the default when no runtime is
configured.

## Engines

SGLang is the preferred default for Prism's next local-specialist runtime
because it emphasizes serving efficiency and prefix-cache friendly workflows.
The prompt builder keeps stable instructions before volatile task evidence so
SGLang-style stable-prefix reuse has a clean layout.

vLLM remains supported because many teams already run OpenAI-compatible vLLM
endpoints and it is a strong general-purpose serving runtime.

## Configuration

Primary SGLang:

```bash
export PRISM_MODEL_RUNTIME_ENGINE=sglang
export PRISM_MODEL_RUNTIME_BASE_URL=http://localhost:30000/v1
export PRISM_MODEL_RUNTIME_API_KEY=EMPTY
export PRISM_MODEL_RUNTIME_MODEL=Qwen/Qwen3-Coder
```

Primary SGLang with vLLM fallback:

```bash
export PRISM_MODEL_RUNTIME_ENGINE=sglang
export PRISM_MODEL_RUNTIME_BASE_URL=http://localhost:30000/v1
export PRISM_MODEL_RUNTIME_MODEL=Qwen/Qwen3-Coder

export PRISM_MODEL_RUNTIME_FALLBACK_ENGINE=vllm
export PRISM_MODEL_RUNTIME_FALLBACK_BASE_URL=http://localhost:8000/v1
export PRISM_MODEL_RUNTIME_FALLBACK_MODEL=Qwen/Qwen3-Coder
```

Local endpoints:

- SGLang: `http://localhost:30000/v1`
- vLLM: `http://localhost:8000/v1`

## Streaming

Streaming uses `POST /v1/chat/completions` with `"stream": true` and parses
server-sent events shaped as:

```text
data: {...}
data: [DONE]
```

The parser tolerates blank lines, comments, unknown SSE fields, and
provider-specific extra JSON fields. It emits delta, done, and error events
through Prism's internal stream event type.

If a fallback runtime is configured, fallback can happen only before a stream
channel is returned. Once partial stream output may have reached the caller,
Prism does not transparently restart on the fallback runtime.

## Structured Output

Structured requests use OpenAI-compatible JSON schema response format:

```json
{
  "response_format": {
    "type": "json_schema",
    "json_schema": {
      "name": "evidence_summary",
      "strict": true,
      "schema": {}
    }
  }
}
```

The adapter parses assistant content as JSON, preserves the raw content, and
returns normalized parse errors when the content is not valid JSON.

The first structured handoff is `internal/handoff.GenerateRepoScanSummary`,
which returns a typed `EvidenceSummary` for repo-scan specialist output. Existing
unstructured behavior remains available because the shared runner only uses the
new runtime when configured.

## Fallback

Fallback triggers for timeout, unavailable, and provider errors. It does not
trigger for invalid requests, unauthorized responses, rate limits, or parse
errors.

Health returns healthy when the primary is healthy. If the primary is unhealthy
and the fallback is healthy, health returns healthy with detail explaining that
fallback is carrying the runtime.

## Prompt Prefix Layout

`internal/prompt.Builder` renders stable sections before volatile sections,
regardless of add order:

```text
stable system instructions
stable specialist role
stable schema/output contract
stable repo/project rules
---
volatile user task
volatile retrieved evidence
volatile logs/tool results
```

Use this convention for SGLang-friendly prompt-prefix reuse.

## Testing

Unit tests:

```bash
make llm-test
go test ./...
```

Optional SGLang contract tests:

```bash
PRISM_LLM_CONTRACT_MODEL=Qwen/Qwen3-Coder make llm-contract-sglang
```

Optional vLLM contract tests:

```bash
PRISM_LLM_CONTRACT_MODEL=Qwen/Qwen3-Coder make llm-contract-vllm
```

The integration tests are behind `-tags=integration` and skip cleanly when the
required environment variables are missing. They log engine and base URL without
printing API keys.

## Compatibility Notes

- The adapter targets the OpenAI-compatible `chat/completions` API surface used
  by Prism.
- Tool calling is not exposed through the new runtime in this pass.
- Structured output support depends on the serving engine honoring
  `response_format`.
- Normal `go test ./...` does not require a running model server.
