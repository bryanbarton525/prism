# 5. Testing and validation

[← Architecture](architecture.md) · [Contents](README.md) · [Troubleshooting →](troubleshooting.md)

Test each layer independently, then run a complete task. A healthy HTTP endpoint
does not establish useful rankings or correct specialist behavior.

## Operator checks

```bash
prism version --json
prism agent list
prism models status
curl -fsS http://127.0.0.1:8010/health
systemctl --user list-unit-files 'prism-laya-*.service'
```

For managed Laya, expect `status: "ok"`, `device: "cpu"`, a loaded English
checkpoint, and its pinned revision. Check your exact unit with
`systemctl --user status UNIT_NAME`. `prism models status` checks embedding
artifacts; it does not report Laya readiness. `prism config doctor` probes
Ollama and registry health, so use runtime-specific checks for SGLang/vLLM.

From an MCP host, call `recommend_tools` with a known lookup task and inspect
the first tool, `score_kind`, `model_identity`, and warnings. A `laya_choice`
result establishes that the decision adapter was used. A lexical or embedding
score with a warning means the fallback path was used.

## Deterministic development suite

From a checkout with Go installed:

```bash
bash scripts/ci-check.sh
```

This verifies Go module integrity, runs the normal tests and mock benchmark,
runs `go vet`, and builds Prism. It requires no live model or Kubernetes cluster.
To avoid picking up a developer machine's saved runtime configuration, select
an empty temporary state for the suite:

```bash
TEST_STATE=$(mktemp -d)
PRISM_STATE_DIR="$TEST_STATE" bash scripts/ci-check.sh
```

Use a targeted loop while editing:

```bash
PRISM_STATE_DIR="$TEST_STATE" go test \
  ./internal/toolmodel ./internal/config ./internal/cli ./internal/app -count=1
```

Laya unit coverage includes request shape, candidate order, malformed or missing
probabilities, normalization, authorized catalogs, failure fallback, saved
migration settings, service pins, readiness, and dry-run behavior. Book tests
check navigation and command examples against the CLI.

## Real Laya inference

Start your managed Laya service, then select the endpoint and a state with
MiniLM installed. This block assumes default user state:

```bash
export PRISM_LIVE_LAYA_URL=http://127.0.0.1:8010
export PRISM_TEST_MODEL_STATE="$HOME/.prism"
go test -tags=integration ./internal/app \
  -run '^TestRecommendToolsWithLiveLaya$' -count=1 -v
```

The test uses authorized fixture catalogs, the installed ONNX model, and actual
Laya inference. It checks that `find_issue` ranks first with two tools and a
50-tool catalog narrowed to 20 candidates. Fixture tool execution is forbidden.
This tests the full retrieval and reranking path, including cold catalog costs.
It does not establish production accuracy on arbitrary tasks.

## Complete Laya + offload model + MCP task

Point at your served SGLang model (replace both values):

```bash
export PRISM_LIVE_SGLANG_URL=http://127.0.0.1:30000/v1
export PRISM_LIVE_SGLANG_MODEL=YOUR_SERVED_MODEL
go test -tags=integration ./internal/app \
  -run '^TestLiveLayaSGLangIssueLookup$' -count=1 -v
```

Keep the two Laya/model-state variables from the previous block. This test first
makes an explicit Laya recommendation, then passes it into a real specialist
run backed by a local HTTP MCP fixture. Success requires the exact `ENG-731`
ID and `Blue Canary` title. Creating an issue fails the test. It does not mutate
a real issue tracker. It exercises explicit recommendation use, not automatic
500 ms injection or a production Linear deployment.

Without live variables, these optional tests skip. CI relies on deterministic
coverage. Compare warmed HTTP inference with completed-task time separately;
cold embeddings, catalog discovery, and model serving add their own latency.
See [measured evaluation](../laya-evaluation.md) for methodology and limitations.
