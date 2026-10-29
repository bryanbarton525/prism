# MCP response efficiency

`call_mcp_tool` returns its complete bounded result in MCP `structuredContent`.
By default its text content list is empty, avoiding a second serialized copy of
that same envelope. Hosts must consume structured output. For a host that only
reads MCP text, pass `include_text: true` to also receive the legacy text copy.
The top-level MCP `isError` and the result's truncation markers are preserved.
This option applies to the MCP bridge; CLI JSON output remains a single envelope.

Within the downstream envelope, a text block that parses as the same complete
JSON value as structured content is omitted when the structured value fits the
response budget. Distinct prose, extra trailing text, different fields, and
truncated previews remain. Retained evidence also stores equivalent JSON once.
Numeric comparisons preserve precision rather than converting through float64.
No ticket fields are selected or discarded to achieve these reductions.

## Reproducible fixture comparison

`TestTicketResultAppearsOnceOnWire` calls a real HTTP MCP fixture through Prism's
MCP server and checks the host-visible result. It also reconstructs the old
v0.1.36 serialization using the identical fixture. This is a synthetic ticket,
not a replay of BBA-30/BBA-31 and not a measurement of billed model tokens.

| Serialized MCP response | Bytes | o200k_base tokens |
|---|---:|---:|
| v0.1.36 envelope | 5,468 | 1,110 |
| Single structured result | 1,360 | 263 |

The fixture reduction is 76.3% in serialized response tokens. Discovery, schemas,
authentication, requests, and conversation history are excluded. The added optional
input flag modestly increases tool schema size; these response counts exclude it.
No latency or live connector performance claim follows from this test.

```sh
mkdir -p /tmp/prism-response-samples
PRISM_RESPONSE_SAMPLE_DIR=/tmp/prism-response-samples \
  go test ./internal/mcp -run TestTicketResultAppearsOnceOnWire -count=1 -v
uv run --no-project --with tiktoken python - <<'PY'
from pathlib import Path
import tiktoken
encoding = tiktoken.get_encoding('o200k_base')
for name in ['before', 'after']:
    data = Path('/tmp/prism-response-samples', name + '.json').read_text()
    print(name, len(data.encode()), len(encoding.encode(data)))
PY
```

## Live connector checks (2026-10-28)

Tested installed v0.1.36 against patched commit dc3db8a, using fresh MCP hosts.
Every read was repeated three times per path, alternating path order. All four
responding registrations returned identical decoded data across all nine calls.
Counts below use o200k_base on compact serialized SDK results (aliases enabled,
null optional fields omitted), excluding requests, discovery, authentication,
schemas and conversation history. These are serialization counts, not billing.

| Server / read tool | Direct MCP | v0.1.36 bridge | Patched bridge |
|---|---:|---:|---:|
| Linear / get_issue BBA-31 | 998 | 2,268 | 1,006 |
| Kubescape / list_frameworks | 116 | 301 | 126 |
| Laya / laya_status | 569 | 1,370 | 579 |
| Laya routing / laya_status | 607 | 1,448 | 618 |

Each count was identical in all three repetitions. No call in these four groups
reported an MCP error. This covers the selected tools, not every tool or every
possible response shape. Direct calls used the same registered downstream command
and arguments, bypassing Prism; they were not Codex Apps calls.

A separate repeat of the original creation comparison used the actual Codex Apps
Linear connector and the patched Prism MCP bridge with identical team, title and
description. BBA-32 and BBA-33 were created successfully and then marked Done.

| Creation measurement | Codex Apps | Patched Prism |
|---|---:|---:|
| Request tokens | 70 | 81 |
| Response tokens | 367 | 374 |
| Combined tokens | 437 | 455 |
| Observed call seconds | 0.913 | 1.382 |

Creation responses have different generated IDs and timestamps. Codex Apps counts
use its returned envelope including null structuredContent; Prism counts use the
SDK representation described above. The totals represent those observable paths,
not an assertion about identical underlying transport envelopes. One creation per
path does not establish a latency difference. No fields were projected away.

Finance failed during discovery with a 45-second connection timeout. Its HTTPS
MCP endpoint returned 401 to an unauthenticated request, indicating the endpoint
was reachable but authentication was required. No successful Finance response was
available for a token or correctness comparison. This is excluded from savings
claims and must be retested after its connection is restored.

Private raw responses, schemas, exact commands, timing records, build, analysis
script and summary are retained locally under
`/home/bbarton/prism-eval-results/2026-10-28-mcp-efficiency/`.
