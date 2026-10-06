# 1. Quick start

[Contents](README.md) · [Next: Installation →](installation.md)

## Get the executable

Download the asset for your platform from [Prism releases](https://github.com/bryanbarton525/prism/releases),
extract it, and place `prism` on `PATH`. To build the branch you are developing,
use Go 1.25 or newer (the exact requirement is in `go.mod`):

```bash
git clone https://github.com/bryanbarton525/prism.git
cd prism
go build -o ./prism ./cmd/prism
./prism version --json
```

Move the built executable to a directory on your `PATH` before continuing.
A development build reports its revision and dirty state. The executable embeds
its agents, constitutions, skills, and supporting resources; running it does not
require keeping this checkout nearby.

## Connect your offload model

You need an existing inference server. This template uses SGLang; replace the
URL and model ID with the exact values served by your deployment:

```bash
export PRISM_MODEL_RUNTIME_ENGINE=sglang
export PRISM_MODEL_RUNTIME_BASE_URL=http://127.0.0.1:30000/v1
export PRISM_MODEL_RUNTIME_MODEL=YOUR_SERVED_MODEL

curl -fsS "$PRISM_MODEL_RUNTIME_BASE_URL/models"
prism install --runtime-only --runtime-scope user \
  --primary-engine "$PRISM_MODEL_RUNTIME_ENGINE" \
  --primary-url "$PRISM_MODEL_RUNTIME_BASE_URL" \
  --primary-model "$PRISM_MODEL_RUNTIME_MODEL"
```

The installer probes the runtime and saves its settings in user state. If your
server requires a token, export it in an environment variable and pass its name
with `--primary-api-key-env`. The installer stores the variable name.
For Ollama or vLLM, see [runtime configuration](../model-runtime.md).

## Complete a small task

```bash
printf '%s\n' 'Write a pure Go function that returns the larger of two integers.' | \
  prism run go-helper --skills go-helper-fn --format json
```

Look for an `ok` result and a function matching the task. This exercises the
agent bundle, skill attachment, prompt assembly, and your served model. It
requires no downstream MCP server and writes no external issue or message.

## Connect an editor or MCP host

Preview before selecting your host and scope:

```bash
prism install --global --target codex --all --dry-run
prism install --global --target codex --all --yes
prism install status --global
```

Supported targets also include `claude`, `copilot`, `antigravity`, and `opencode`.
Reconnect your host's Prism MCP connection. It can then use `list_agents` and
`run_agent` to delegate specialist work. `--all` installs bundled integrations;
it does not select optional Python runtimes or download model weights.

## Add tool ranking when you need it

On Linux with a systemd user manager, install MiniLM retrieval and Laya choice
reranking explicitly:

```bash
prism install --runtime-only --runtime-scope user \
  --tool-model onnx --decision-service install-laya
prism models status
```

This downloads dependencies and weights. It does not register tool servers or
grant agents access to them. Continue to [worked examples](examples.md) for an
authorized downstream catalog and explicit `recommend_tools` calls.
