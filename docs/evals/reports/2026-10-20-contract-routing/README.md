# Response contracts and optional routing

Fresh results from the fixed implementation. Laya is optional: original mode
uses the model-driven tool loop without contacting Laya. First mode binds
execution to a Laya-selected authorized server/tool before generation.

![All-check pass rates](comparison.png)

![Observed latency](latency.png)

| Model and mode | All-check passes |
|---|---:|
| llama3.1:8b / original | 8/18 |

These are small synthetic integration probes, not a ranking-quality benchmark.
Expected answers stay in graders; models receive schemas and replayed tool evidence.
Failures remain in the data. No failing run is an approved baseline.
The historical v0.1.32 report used different contracts and advisory routing;
its pass rates are not a controlled before/after comparison.

## Failed checks

| Model and mode | Check | Failed trials |
|---|---|---:|
| llama3.1:8b / original | `decision_schema` | 7 |
| llama3.1:8b / original | `mcp_response` | 7 |
| llama3.1:8b / original | `model_identity` | 2 |
| llama3.1:8b / original | `outcome:0:/decision/accessible` | 2 |
| llama3.1:8b / original | `outcome:0:/decision/issue_id` | 2 |
| llama3.1:8b / original | `outcome:0:/decision/project_id` | 3 |
| llama3.1:8b / original | `outcome:1:/decision/project_name` | 3 |
| llama3.1:8b / original | `outcome:1:/decision/status` | 5 |
| llama3.1:8b / original | `outcome:2:/decision/health` | 3 |
| llama3.1:8b / original | `outcome:2:/decision/reason` | 2 |
| llama3.1:8b / original | `required_tool:projects/get_project` | 3 |
| llama3.1:8b / original | `required_tool:tracker/get_issue` | 3 |
| llama3.1:8b / original | `response_schema` | 9 |
| llama3.1:8b / original | `structured_decision` | 6 |
| llama3.1:8b / original | `tool_sequence` | 3 |

## Reproduction and provenance

See [the guarded runner guide](../../../../evals/README.md). Exact commands,
binary revisions, model digests, generation settings, memory samples, and
The agent also has a 60-second per-invocation latency budget.
CLI-validated raw reports are retained in `data/`. Runs use one resident
small model at a time, unload between blocks, and stop at guard thresholds.

Regenerate this page and graph with:

```sh
python3 -m pip install matplotlib
python3 docs/evals/reports/2026-10-20-contract-routing/analyze.py
```
