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
