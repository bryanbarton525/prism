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
- `internal/llm`: the runtime registry/factory and engine adapters.
- `internal/llm/fallback`: optional primary/fallback wrapper.

Prism startup builds the configured runtime once and injects it into the shared
agent runner. When no explicit runtime is configured, Prism builds an
Ollama-backed `ModelRuntime` from the legacy Ollama host setting, so agent chat
uses the same runtime abstraction for Ollama, SGLang, and vLLM.

## Engines

The default registry includes:

- `ollama`: Prism's default local specialist runtime, implemented as an Ollama
  adapter behind `ModelRuntime`.
- `sglang`: OpenAI-compatible SGLang `/v1/chat/completions`.
- `vllm`: OpenAI-compatible vLLM `/v1/chat/completions`.

SGLang is the preferred default for Prism's next local-specialist runtime
because it emphasizes serving efficiency and prefix-cache friendly workflows.
The prompt builder keeps stable instructions before volatile task evidence so
SGLang-style stable-prefix reuse has a clean layout.

vLLM remains supported because many teams already run OpenAI-compatible vLLM
endpoints and it is a strong general-purpose serving runtime.

## Configuration

### Select and inspect the offload model

```bash
prism model set
prism model set qwen3.6:27b --engine ollama
prism model set my-served-model --engine sglang --url http://server:30000/v1
prism model set my-served-model --engine vllm --url http://server:8000/v1 \
  --api-key-env LOCAL_MODEL_API_KEY
prism model status
prism model status --check
prism model status --json
```

Without a model argument, `set` opens a terminal picker for the engine, endpoint,
and advertised models. Use ↑/↓ and Enter, or choose **Other** to type a model ID.
Review the target and select Save; Ctrl-C or Cancel leaves configuration unchanged.
The picker requires terminal input/output and never opens with `--json`. Scripts
must provide a model argument. Authentication can be selected with `--api-key-env`.

`set` saves the shared target to the selected runtime's `config.env`. The default
is `~/.prism/config.env`; `--state-dir /path/to/state` selects isolated runtime
state. It preserves unrelated settings and fallback configuration. If
`PRISM_CONFIG_FILE` selects another file, align it with that state before saving.

Omitting `--engine` and `--url` keeps the current target. Without an explicit
runtime, Prism defaults to Ollama and the configured Ollama host. Changing to
Ollama defaults to that Ollama host; changing to SGLang or vLLM requires `--url`.
Model names do not infer an engine. On an engine or endpoint change, previous
saved primary credentials are cleared; pass `--api-key-env` for the new target.
The CLI stores the environment variable name, never the supplied secret value.

Before saving, Prism queries `/api/tags` for Ollama or `/v1/models` for
SGLang/vLLM. The check verifies advertised availability, not inference readiness;
it does not generate text or download weights. Missing models, bad responses,
authorization failures, and unreachable servers leave configuration unchanged.

```bash
prism model set future-model --engine ollama --skip-check  # offline, unverified
prism model set qwen3.6:27b --engine ollama --dry-run       # preview; no write/probe
```

An interactive dry run may fetch the model list to populate the picker, but
never saves. `--skip-check` skips discovery and lets you type the model ID.

`status` reports effective settings, their sources, saved settings hidden by
overrides, the selected state/configuration path, and fallback configuration.
It is offline unless `--check` is supplied. Check failures return nonzero.
Environment variables take precedence over workspace `.env`, followed by saved
runtime configuration and defaults. Saving does not remove higher-priority
overrides, and neither command edits Codex or Claude configuration. A terminal
process cannot see overrides supplied only by its host's MCP registration.
Reconnect running Prism MCP servers after changing saved settings.

`agent list` and `agent show` distinguish the **declared model** in the agent
specification from the **effective model** selected by runtime configuration.
Existing JSON `model`/`spec.model` fields retain their declared meaning; new fields
describe the effective target, source, and runtime conflicts. Managed agents with
runtime drift remain blocked until explicitly reconciled, for example:

```bash
prism agent model set my-imported-agent --use-configured-model
```

That command requires a managed agent ID, not a model ID; it cannot edit bundled
agents. `prism models setup/status` manages tool embeddings rather than the
offload model. `prism config doctor` checks the selected runtime and effective
model requirements, so an inactive Ollama instance is not required for SGLang.

The installer remains available for broader runtime setup:

```bash
prism install --runtime-only --runtime-scope user \
  --primary-engine ollama --primary-url http://127.0.0.1:11434 \
  --primary-model qwen3.6:27b
```

### Environment configuration

Explicit Ollama runtime:

```bash
export PRISM_MODEL_RUNTIME_ENGINE=ollama
export PRISM_MODEL_RUNTIME_BASE_URL=http://127.0.0.1:11434
export PRISM_MODEL_RUNTIME_MODEL=qwen3.5:9b
```

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

When `PRISM_MODEL_RUNTIME_MODEL` is set, the runtime sends that model ID to the
provider even if an agent spec has its own legacy `model` value. This lets one
MCP/server installation retarget all specialists to Ollama, SGLang, vLLM, or a
compatible gateway without editing every agent file.

Local endpoints:

- Ollama: `http://127.0.0.1:11434`
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
- Prism uses provider tool calls for its bounded downstream-MCP bridge and the
  fixed `repo-investigator` Graphify workflow. Providers that emit tool calls
  as fenced JSON text are recovered only when the named tool was offered; no
  arbitrary text command is executed.
- Structured output support depends on the serving engine honoring
  `response_format`.
- Normal `go test ./...` does not require a running model server.
