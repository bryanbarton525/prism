# Repeatable model and Laya evaluations

Run from the repository root. Store results in a persistent directory, not `/tmp`.
The runner requires Python 3, macOS `memory_pressure`, a local Ollama server, and
an installed Prism CLI. No model downloads or configuration changes occur.

## Memory-bounded model matrix

```sh
python3 -m unittest discover -s evals -p 'test_*.py'
python3 evals/run_matrix.py \
  --model qwen3.5:9b --model llama3.1:8b \
  --repeat 3 --max-tokens 4096 \
  --output-dir "$HOME/prism-eval-results/ollama-small-models-001"
```

Each model runs the six bundled MCP-response cases three times. The wrapper uses
one model at a time, retains failed trials, validates each report with
`prism eval export`, and unloads that model before moving on. An existing output
directory is refused. A local file lock prevents two copies of this wrapper from
running together. Other applications can still submit Ollama work; close those
inference sessions before testing.

Defaults reject model artifacts larger than 8 GiB, refuse to start if Ollama
already has resident models, and stop if macOS reports less than 40% memory free
or Ollama reports more than 12 GiB resident. Runtime memory is sampled every two
seconds. Artifact size is **not** a runtime memory estimate; these checks are
conservative safeguards, not a hard OS memory limit. Leave the large 27B/31B
models out until their memory requirements have been measured separately.

On a guard failure, command error, Ctrl-C, or SIGTERM, the wrapper stops the
child and requests model unloading. SIGKILL, a crash, or a reboot cannot run
cleanup. Verify afterward with:

```sh
curl --fail http://127.0.0.1:11434/api/ps
memory_pressure -Q
```

Outputs include CLI identity, installed model digests, commands, memory samples,
full reports, incremental trial journals, export datasets, logs, and checksums.
An interrupted run retains its completed trials; never merge those into a
complete run or silently retry failures. Use a new output directory for reruns.
`eval run` retains functional failures as data; use `eval test` for a failing
regression gate. See [baseline workflow](../docs/evals/README.md).

## Laya on the homelab GPU

Laya is a System 1 decision model. It ranks tool candidates; the Ollama model
still generates the answer and performs tool calls. Its verified DNS origin is:

```sh
curl --fail https://laya-gpu.iambarton.com/health
```

Health should identify the English checkpoint on CUDA, with zero CPU fallbacks.
The homelab repository contains `clusters/namespace-laya/deployment.yaml`,
`https-route.yaml`, and `verification.md`; DNS resources are under
`clusters/laya-dns/`. The local homelab checkout may lag origin, so inspect
current GitHub files without resetting local changes.

An existing live integration test exercises the production Laya adapter with
both two-tool and fifty-tool catalogs. It performs remote decision inference
and local embedding retrieval, without loading an Ollama generative model:

```sh
PRISM_LIVE_LAYA_URL=https://laya-gpu.iambarton.com \
PRISM_TEST_MODEL_STATE="$HOME/.prism" \
go test -tags integration ./internal/app \
  -run '^TestRecommendToolsWithLiveLaya$' -count=1 -v
```

This requires the installed MiniLM assets under `~/.prism/models/minilm`.
A pass requires `score_kind=laya_choice`, `model_identity=laya-english`, no
warnings, and `find_issue` ranked first. It tests retrieval/ranking integration,
not generative response consistency.

## Recorded Laya off/on comparison

The v0.1.31 released harness does not enable Laya. Build this branch's CLI to
use the new explicit `--laya-url` flag; use that same binary for both arms:

```sh
go build -o "$HOME/prism-eval-results/prism-eval" ./cmd/prism
python3 evals/run_matrix.py \
  --prism "$HOME/prism-eval-results/prism-eval" \
  --model llama3.1:8b --repeat 3 --max-tokens 4096 \
  --output-dir "$HOME/prism-eval-results/laya-off-001"
python3 evals/run_matrix.py \
  --prism "$HOME/prism-eval-results/prism-eval" \
  --model llama3.1:8b --repeat 3 --max-tokens 4096 \
  --laya-url https://laya-gpu.iambarton.com \
  --output-dir "$HOME/prism-eval-results/laya-on-001"
```

Laya is optional. Without `--laya-url`, evaluations use the original model-driven
tool loop and do not load or contact Laya. With an endpoint, `--laya-mode first`
(the default) selects an authorized server/tool before generation, binds the
execution target, validates arguments, and records each decision and execution.
A configured service failure fails the route; it does not silently fall back.
`--laya-mode advisory` explicitly reproduces legacy suggestions.

The bundled `mcp-response-v2` suite supplies decision schemas without expected
answers. Grader `mcp-outcomes-v2` checks the structured result delivered through
MCP, schema validity, fixture outcomes, and bound route traces (`laya_route`).
Baselines cannot mix routing modes or grader versions. These small catalogs test
integration and response contracts, not general routing quality. Historical
v0.1.32 reports used advisory routing and remain diagnostic evidence.

The first live ranking check on 2026-10-20 failed: the two-candidate request
exceeded the adapter's timeout, and the fifty-candidate request received HTTP
503. Both fell back to MiniLM scores. The pod was running on `omarchy` and health
reported English/CUDA with zero CPU fallbacks. `nvidia-smi` reported 15,051 MiB
used out of 16,311 MiB; that snapshot does not establish the cause of the timeout.
Resolve decision-serving readiness before running the Laya-on generation arm.

## Interrupted first collection

The initial collection used v0.1.31 with five models, including 27B and 31B
models. The user reported memory overload. Afterward no eval processes or
resident Ollama models remained, and macOS reported 92% memory free. The
worktree, raw traces, and exploratory control results in `/private/tmp` were
missing. Previously observed counts cannot be audited and are not a published
dataset or an approved baseline. Reruns must use the persistent output and
memory safeguards above. No claim is made about the precise cause of the
memory incident without retained telemetry.
