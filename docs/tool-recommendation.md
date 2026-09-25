# Local tool recommendation

Prism can suggest downstream MCP tools for an agent's task. The public MCP tool is `recommend_tools` with `agent_id`, `task`, optional `top_k` (1–10), and optional `workspace` for Graphify's repository investigator. A recommendation is advisory; the existing tool call path checks access again.

The default score uses Potion embeddings when its model is installed. Set `PRISM_TOOL_RECOMMEND_MODEL=onnx` to use MiniLM embeddings through Hugot's pure Go backend. If the selected model is unavailable, Prism reports that and uses lexical overlap for the public recommendation call. Model artifacts are downloaded only by an explicit setup command:

```sh
prism --state-dir /path/to/state models setup potion
prism --state-dir /path/to/state models setup onnx
prism --state-dir /path/to/state models status
```

The setup commands use pinned upstream revisions and SHA-256 checksums. Potion is `minishlab/potion-base-32M` (MIT); MiniLM is `sentence-transformers/all-MiniLM-L6-v2` (Apache 2.0). Prism's Potion loader and tokenizer are adapted from Pulse's Apache-2.0-licensed implementation; see the bundled third-party license notice. Runtime inference needs neither Python nor cgo.

Automatic shortlist injection is off by default. Set `PRISM_TOOL_RECOMMEND_AGENTS` to a comma separated list of agent IDs, for example `linear`. Prism calls the same recommendation path before those agents' MCP loops, within a 500 ms routing budget. It omits suggestions when there is insufficient context room. Graphify already has a four tool catalog and does not need automatic discovery; its catalog can be recommended through `recommend_tools` with a workspace.

An optional local Kev server can score the top embedding candidates. Configure `PRISM_KEV_URL=http://127.0.0.1:8009` and, if the server requires a bearer token, `PRISM_KEV_API_KEY_ENV` with the name of the environment variable holding that token. Prism never installs or starts Kev. A failed Kev call leaves the embedding order in place. After an ambiguous hard timeout, Kev stays disabled until Prism restarts.

Agent tool loops keep large tool results in memory for that run. The model receives a small preview and an opaque `result_id`, then can call `read_tool_result` with an offset and limit. Results are deleted when the run ends. A per-result and per-run limit can make retained output incomplete; that status is shown in the preview and reads. Prism includes tool definitions and history in its context estimate and can replace older previews with their read references.

To run the real-model and local MCP tests after setup:

```sh
PRISM_TEST_MODEL_STATE=/path/to/state go test ./internal/toolmodel ./internal/app ./internal/downstreammcp -count=1
CGO_ENABLED=0 go build ./cmd/prism
```

The 25% median end-to-end latency improvement remains an evaluation target. The single-task live benchmark below does not establish it; a representative task set with tool-call counts is still needed.

## Local validation on 2026-09-24

These measurements used an AMD Ryzen 5 7600X host, the installed pinned Potion and MiniLM artifacts, a real local Streamable HTTP MCP fixture, and the cluster's SGLang gateway serving `Qwen3-8B-AWQ`. The fixture returned an issue with known ID `ENG-731` and title `Blue Canary`; a completed run counted as correct only if its final summary contained both values. The SGLang deployment permits one running request, so the two-way measurements primarily exercise queueing and Prism concurrency, not additional model parallelism.

The SGLang health, chat, stream, and structured-output contract tests passed. A real Prism agent run with a Potion shortlist called the MCP tool and returned the correct issue. A 50-tool ranking test placed `find_issue` within the top five for both Potion and MiniLM. Repository-wide tests, affected-package race tests, `go vet`, CGO-free builds, and cross-platform builds passed.

| Completed-task mode | Sequential mean, 6 runs | Two-way throughput time/op, 6 runs | Correct runs |
| --- | ---: | ---: | ---: |
| No automatic shortlist | 1.975 s | 2.046 s | 12/12 |
| Potion shortlist | 1.876 s | 1.904 s | 12/12 |
| ONNX MiniLM shortlist | 2.044 s | 2.030 s | 12/12 |

The one-tool task is a correctness and overhead check, not a representative discovery benchmark. Potion was about 5% faster than baseline sequentially and 7% faster in the two-way run, well short of the 25% target. ONNX did not improve completed-task time in this fixture. These small samples are not a claim of statistically significant improvement or reduced tool-call count.

For a warm 50-tool catalog, three repeated two-second load samples per configuration yielded these ranges (`ns/op` in parallel is throughput-normalized, not per-request latency):

| Recommender | Sequential | Four-way load | Allocated/call |
| --- | ---: | ---: | ---: |
| Potion | 84–89 µs | 23–24 µs/op | ~59 KB |
| ONNX MiniLM | 60–63 ms | 62–63 ms/op | ~42 MB |

Potion is the faster default on this host. MiniLM's pure-Go ONNX path works and ranks the expected tool, but its per-query allocation and serialized inference make it unsuitable as the default fast path here.

Kev was also run as a **real local server**, using upstream `jaredpalmer/kev-0.8b` on CPU, not a mock. Prism's live recommendation call passed with two candidates (~0.43–0.66 s) and with 20 candidates (~3.35 s). A separate four-call 20-candidate load run averaged 2.747 s/call sequentially and 2.840 s/op at two-way load, with 8/8 Kev-ranked responses. The latter fits the explicit five-second Kev exchange limit but exceeds the automatic shortlist's 500 ms budget, so this CPU configuration is useful for explicit decision requests, not latency-sensitive automatic injection. The local Kev test checkout was outside this repository and is not part of the Prism build.

Reproduce the live tests by setting `PRISM_TEST_MODEL_STATE` to a state directory prepared with both `prism models setup` commands, `PRISM_LIVE_SGLANG_URL` and `PRISM_LIVE_SGLANG_MODEL` to a reachable OpenAI-compatible SGLang endpoint, and optionally `PRISM_LIVE_KEV_URL` to a running local Kev server. Then run:

```sh
go test -tags=integration ./internal/app -run 'TestLiveSGLang|TestRecommendToolsWithLiveKev' -count=1 -v
go test -tags=integration ./internal/app -run '^$' -bench '^BenchmarkLiveSGLangIssueLookup$' -benchtime=6x -cpu=2
go test ./internal/app -run '^$' -bench '^BenchmarkRecommendToolsLoad$' -benchtime=2s -count=3 -cpu=1,4 -benchmem
go test -tags=integration ./internal/app -run '^$' -bench '^BenchmarkLiveKevRecommendation$' -benchtime=4x -cpu=2
```

For the runtime contract tests, separately set `PRISM_LLM_CONTRACT_BASE_URL`, `PRISM_LLM_CONTRACT_MODEL`, and `PRISM_LLM_CONTRACT_ENGINE=sglang`, then run `go test -tags=integration ./internal/llm/runtime -run TestContract -count=1 -v`.
