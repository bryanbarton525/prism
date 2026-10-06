# Laya evaluation for Prism decision reranking

Evaluated 2026-09-30 on this machine. Laya is a promising replacement for explicit tool recommendations **when the adapter uses a single categorical choice question**. Switching the current Kev endpoint to Laya without changing its question format substantially reduced ranking quality. The CPU configurations tested here do not establish a suitable replacement for Prism's 500 ms automatic shortlist path.

## Local results

The fixture contains 12 hand-authored English tasks and a shared 20-tool catalog covering issues, Kubernetes, GitHub, Argo CD, documentation and distracting tools. Each task specifies one directly appropriate next tool. No listed tools were executed. For the main comparison, every task was run in normal and reversed catalog order: 24 observations over **12 distinct tasks**, not 24 independent tasks.

| Decision path | Expected tool ranked first | Expected tool in top three | Median HTTP inference time |
| --- | ---: | ---: | ---: |
| Installed Kev, existing Prism binary questions | 22/24 | 24/24 | 3,373 ms |
| Laya English, one choice, head budget 512 | 22/24 | 24/24 | 1,135 ms |
| Laya typed-decisions, one choice, head budget 512 | 22/24 | 23/24 | 1,154 ms |
| Laya multilingual, one choice, head budget 512 | 8/24 | 15/24 | 417 ms |
| Laya English, one choice, head budget 384 | 21/24 | 23/24 | 986 ms |
| Laya English, one choice, head budget 256 | 13/24 | 18/24 | 696 ms |

The 512-token option budget used `max_len=1024`; the compact variants used `max_len=512`. English was 10/12 in normal order and 12/12 in reverse; typed-decisions was 11/12 in each. Kev was 11/12 in each. These observations show some order sensitivity and do not establish statistical equivalence or general production accuracy. The roughly 3x latency difference applies to this local short-task, 20-candidate fixture.

### Keeping the current Prism question format

Prism currently sends one independent `noul` question per candidate with instructions “Would this specific tool help complete the task?” and a candidate description under the `true` criterion. Laya accepts this request but performed poorly. The following runs used normal catalog order only:

| Path | First choice correct | Top-three hit | Median time, 20 candidates |
| --- | ---: | ---: | ---: |
| Kev | 11/12 | 12/12 | 3,348 ms |
| Laya English, unchanged request | 3/12 | 10/12 | 3,165 ms |
| Laya English, neutral A/B boolean labels | 5/12 | 9/12 | 3,149 ms |
| Laya multilingual, unchanged questions | 4/12 | 5/12 | 993 ms |
| Laya multilingual, neutral labels | 1/12 | 1/12 | 1,008 ms |
| Laya typed-decisions, unchanged questions | 5/12 | 8/12 | 2,919 ms |
| Laya typed-decisions, neutral labels | 5/12 | 9/12 | 2,829 ms |

Neutral labels preserve boolean semantics through `labels={false: B, true: A}`. They did not reliably fix this workload. Several incorrect rankings preferred `argo.sync_app` for unrelated read requests. These are ranking failures; Prism's execution access checks remain separate from advisory recommendations.

### Latency and client compatibility

Three warmed measurements per candidate count, using the issue lookup task:

| Candidates | Kev binary median | Laya English binary median |
| ---: | ---: | ---: |
| 1 | 239 ms | 202 ms |
| 2 | 365 ms | 325 ms |
| 5 | 813 ms | 716 ms |
| 10 | 1,464 ms | 1,404 ms |
| 20 | 3,198 ms | 3,029 ms |

All measured short-task binary calls fit Prism's five-second exchange limit. No 20-candidate English or typed-decision configuration fit the 500 ms automatic recommendation budget. The multilingual choice configuration did fit that budget, with materially weaker ranking results.

The existing Go integration tests `TestRecommendToolsWithLiveKevServer` and `TestRecommendToolsWithLiveKevTwentyCandidates` also passed against temporary Laya at `127.0.0.1:8010` (0.26 s and 2.29 s). These checks validate real HTTP decoding and the client limit; their fixture assertions do not establish ranking quality. Their current names and `kev-latest` identity are retained compatibility behavior, not truthful Laya provenance.

## Integration requirements

1. Add a Laya decision-service identity and explicit checkpoint selection. Current Prism accepts only `kev-latest` or `jev-latest`; Laya silently treats these unrecognized names as automatic checkpoint routing. The existing client would therefore label Laya results as Kev. See [Prism client](../internal/toolmodel/kev.go) and [Laya model resolution](https://github.com/NandhaKishorM/laya/blob/6d942c92081fbc139e736bbd9ac0023223c29b7f/laya/serve.py).
2. Build one `choice` question whose criteria map candidate IDs to tool names and descriptions. Decode `answers.tool.probabilities`, with the reported `choice` used appropriately for ties. Those probabilities describe relative choices; they are not independent probabilities that each tool is useful. Preserve the existing authorized catalog and execution checks.
3. Set explicit token budgets and handle omitted state or truncated option descriptions. Shrinking the option budget reduced accuracy in this fixture. Long tasks, logs, large descriptions, multilingual tasks, no-relevant-tool cases and concurrent load remain untested.
4. Keep Laya optional for explicit recommendations until a larger, held-out workload supports enabling it automatically. No accuracy or calibration thresholds were fitted here. Startup reported an invalid published English choice temperature for 11+ options and clamped it; this evaluation measures ranking and does not validate probability calibration.
5. Pin package and weights and report Laya-specific provenance. Stock `laya-serve` uses PyTorch. Laya offers ONNX support, but the linked checkpoint repository does not ship ONNX artifacts; local export and a custom server/router integration require separate evaluation. See [source assessment](laya-source-assessment.md).

## Reproducibility and machine state

- Hardware: AMD Ryzen 5 7600X, 12 logical CPUs. Laya used CPU PyTorch with four intra-op threads. Kev used its previously installed CPU service configuration. This is a comparison of these actual deployments, not a controlled thread-for-thread microbenchmark. SGLang remained on the GPU; Laya did not allocate GPU memory.
- Package: `laya==0.3.22`, `torch==2.8.0+cpu`, `transformers==5.17.0`, Python 3.13. Full dependency freeze is recorded alongside the evaluation scripts.
- Checkpoint bundle: [`convaiinnovations/laya` at `55cf4c4ebb4ebe31b2550e8bdf3bd21b99753851`](https://huggingface.co/convaiinnovations/laya/tree/55cf4c4ebb4ebe31b2550e8bdf3bd21b99753851), root English and the `multilingual` and `typed-decisions` subfolders. The weights and runtime are Apache-2.0 according to their upstream licenses.
- Reviewed upstream source: [`6d942c92081fbc139e736bbd9ac0023223c29b7f`](https://github.com/NandhaKishorM/laya/tree/6d942c92081fbc139e736bbd9ac0023223c29b7f).
- Local isolated environment, scripts, raw responses and model cache: `/home/bbarton/.prism/evaluations/laya/`. Files: `evaluate.py`, `variants.py`, `choices.py`, `compact_choices.py`, `kev_reverse.py`, `requirements.lock.txt`, `results.json`, `variants.json`, `choices.json`, `compact_choices.json`, `kev_reverse.json`.
- Temporary server: loopback port 8010, CPU only, one resident checkpoint. `LAYA_REVISION` pinned the bundle SHA and `HF_HOME` isolated its cache. Download/load warm-ups were excluded from steady-state timing; cold switches remain recorded in `variants.json`.
- The temporary Laya server was stopped after evaluation. The installed Kev service and Prism runtime configuration were not replaced or edited. Downloaded evaluation artifacts remain available for reproduction.

To repeat, launch the isolated environment's `laya-serve` with `LAYA_HOST=127.0.0.1`, `LAYA_PORT=8010`, `LAYA_DEVICE=cpu`, `LAYA_THREADS=4`, `LAYA_MODELS=english`, `LAYA_MAX_LOADED=1`, `LAYA_PRELOAD=1`, `LAYA_REVISION` set to the bundle SHA above, `USE_TF=0` and the isolated `HF_HOME`. Run the evaluation scripts sequentially while the existing Kev service is listening on 8009. The scripts may load the other two checkpoint subfolders on their first calls.

## Migration on 2026-10-05

Prism now implements the categorical adapter described above and a managed CPU installer (`--decision-service install-laya`). Generic `PRISM_DECISION_*` settings select it; legacy Kev settings remain supported. The adapter uses the evaluated English checkpoint selection and token budgets, validates complete normalized probability distributions, preserves candidate order in JSON, and reports `laya_choice` / `laya-english` rather than a Kev identity. Managed readiness verifies the pinned revision separately. The September results and machine-state statements above describe that evaluation's state, before this migration.

On this machine the managed `prism-laya-c77e21491251.service` is enabled and active on port 8010, CPU only; Kev's corresponding unit is stopped and disabled. The installed local development binary reports `v0.1.25+laya` with a dirty working tree. The previous binary is retained at `/home/bbarton/.local/bin/prism.pre-laya`; source changes are not committed.

Validation passed for `internal/toolmodel`, `internal/config`, `internal/cli`, and `internal/app`, with `PRISM_STATE_DIR` pointing to an isolated test state so this machine's saved model configuration did not contaminate tests. Live `TestRecommendToolsWithLiveLaya` ranked `find_issue` first for both a two-tool authorized catalog (0.47 s) and a 50-tool catalog narrowed to 20 candidates (4.46 s, including cold MiniLM catalog embedding). The Laya recommendation plus real Qwen3-8B-AWQ/SGLang tool task returned `ENG-731` / `Blue Canary` in 3.12 s using a local MCP fixture; no external issue was created or changed. These timings are complete fixture paths and differ from the warmed HTTP-only measurements above. Automatic injection remains off.

Existing stdio MCP connections retain their original executable and in-memory settings; reconnect Prism or start a new host session to use the installed build. New Prism processes use the saved Laya settings. Unit and live test logs are retained at `/tmp/prism-laya-tests.log` and `/tmp/prism-laya-live-tests.log` for this session.
