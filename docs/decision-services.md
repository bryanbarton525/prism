# Configure Laya or Jev: HTTP and MCP

Prism's built-in tool reranker calls `POST /v1/systemone` directly. A downstream
MCP registration instead exposes decision tools that an authorized agent can
call. Registering MCP does not change the reranker's endpoint or adapter.
Laya and Jev return typed decisions; the generative model and Prism handle
general tool-call arguments and execution.

## Built-in reranking over HTTP

For the homelab GPU service, run:

```bash
prism install --runtime-only --runtime-scope user \
  --decision-service laya --decision-url https://laya-gpu.iambarton.com
```

The corresponding settings in `~/.prism/config.env` are:

```dotenv
PRISM_DECISION_URL="https://laya-gpu.iambarton.com"
PRISM_DECISION_MODEL="laya-english"
PRISM_DECISION_API_KEY_ENV=""
```

This hostname requires access to the homelab LAN or its private network.
Remote endpoints require HTTPS; local HTTP must use loopback. Check GPU health:

```bash
curl --fail https://laya-gpu.iambarton.com/health
```

Expect `status: "ok"`, `checkpoint_devices.english: "cuda"`, and no CPU
fallbacks. For a local CPU installation, see [managed Laya](book/installation.md#managed-laya).

To select hosted Jev instead, provide a TypeSafe API key in the environment
of both setup and the running Prism process:

```bash
# Set TYPESAFE_API_KEY through your environment or secret manager first.
prism install --runtime-only --runtime-scope user \
  --decision-service jev --decision-url https://api.typesafe.ai \
  --decision-key-env TYPESAFE_API_KEY
```

This selects `jev-latest`. Supply the origin without `/v1/systemone`; Prism
appends that path. MiniLM/Potion candidate retrieval is configured separately
with `--tool-model`; neither command installs a generative model.
See the [TypeSafe API reference](https://docs.typesafe.ai/api).

Reconnect running Prism processes after configuration changes. Explicit process
environment settings override saved settings; remove an old
`PRISM_DECISION_URL` override if the saved endpoint appears ineffective.

## Laya MCP backed by the cluster

Use a separate wrapper environment so the managed CPU service keeps its pinned
dependencies. Requires `uv`, Python 3.13 support, and network access for installation.

```bash
uv venv --python 3.13 "$HOME/.local/share/prism-laya-mcp"
uv pip install --python "$HOME/.local/share/prism-laya-mcp/bin/python" \
  torch --index-url https://download.pytorch.org/whl/cpu
uv pip install --python "$HOME/.local/share/prism-laya-mcp/bin/python" \
  'laya[mcp]==0.4.8'

prism mcp add laya --transport command --timeout-ms 60000 -- \
  env LAYA_BASE_URL=https://laya-gpu.iambarton.com \
  "$HOME/.local/share/prism-laya-mcp/bin/laya-mcp-server"

prism mcp tools laya --schema
prism mcp call laya laya_status
```

The local stdio wrapper forwards predictions to the cluster. Laya 0.4.8 supports
`LAYA_BASE_URL`; the managed 0.3.22 MCP launcher lacks this mode. Keep inference
on the existing server and explicitly select `model: "english"` in prediction
calls. For example:

```bash
prism mcp call laya laya_predict --args-json '{
  "model": "english",
  "state": {"body": "I was charged twice."},
  "questions": {"department": {
    "type": "choice",
    "instructions": "Which team should handle this request?",
    "criteria": {"billing": "Invoices and refunds", "technical": "Bugs and outages"}
  }}
}'
```

`laya_preset` supplies built-in questions; custom predictions still supply choices.
Remote mode does not support `laya_shortlist` or `laya_tournament`.
See [Laya MCP setup](https://nandhakishorm.github.io/laya/cli-mcp/) and the
[versioned remote implementation](https://github.com/NandhaKishorM/laya/blob/v0.4.8/laya/mcp/server.py).

## Optional adapted tool-routing MCP

The homelab also serves a separately adapted English choice checkpoint at
`https://laya-routing.iambarton.com`. Its model artifact SHA-256 is
`1920ee68f11b2d657b8500448f31c0ff8d56e8eca2913e978991ae3c7bb7c991`.
Stock `laya-gpu.iambarton.com` retains the original revision `55cf4c4…`.
Both endpoints execute on CUDA; startup requires the pinned identities and zero
CPU fallbacks. This optional endpoint requires the homelab LAN/private network.

Register a second stdio wrapper using the environment installed above:

```bash
prism mcp add laya-routing --transport command --timeout-ms 15000 -- \
  env LAYA_BASE_URL=https://laya-routing.iambarton.com \
  "$HOME/.local/share/prism-laya-mcp/bin/laya-mcp-server"
prism mcp tools laya-routing --schema
prism mcp call laya-routing laya_status
```

Grant access with the same agent/default MCP policy used for other servers,
including existing servers when replacing an allowlist. Reconnect the MCP host.
The official wrapper requires an object for `laya_predict.state`; pass
`{"text": "TASK:\n<task>\nCOMPLETED TOOL EVIDENCE:\n<actual results>"}`.
The specialized server unwraps only that exact one-field string envelope, for
single and batch predictions. Other structured objects retain their structure.
HTTP also accepts a plain string; the general stock endpoint keeps SDK behavior.

For the tested next-tool protocol, supply `model: "english"`, `max_len: 1024`,
`head_max_len: 512`, and a choice catalog keyed `tool_0`, `tool_1`, etc., with
values `server.tool: description`. Before execution, the question is:

> Choose the first tool call required by the task. Obtain prerequisite identifiers before following linked records.

After a completed result, the question is:

> Choose the next action needed for the task using the completed results. Finish only after the requested records have been retrieved.

Include a finish alternative only after a completed result. Keep relevant
catalogs small (training covered 2–8 choices). This model is specialized for
English tool-routing choice questions; general `noul`, `score`, language routing,
and built-in presets belong on the stock endpoint. Confidence is not calibrated.
The parent still creates arguments and checks authorization; Laya returns a
choice, not an MCP invocation.

Prism's experimental bound HTTP path can target this same origin explicitly:

```bash
prism eval run --suite evals/mcp-response-v2/suite.yaml \
  --model Qwen3-8B-AWQ --repeat 3 \
  --laya-url https://laya-routing.iambarton.com --laya-mode first \
  --output routing-check.json
```

The argument-binding and first/next-question fixes are in PR #49, tested with
its recorded clean development binary. The installed published v0.1.35 binary
retains its released behavior until these changes are merged and released.
Registering the MCP server does not install or enable that HTTP routing loop.
See the [bound-routing repair report](evals/reports/2026-10-20-bound-routing-repair/README.md)
for the training corpus, stage-by-stage comparisons, limitations, and artifacts.

## Jev MCP backed by TypeSafe

This example uses the **community-maintained**
[blakestone-x/jev-mcp wrapper, pinned to v0.2.1](https://github.com/blakestone-x/jev-mcp/tree/v0.2.1),
not a Prism-bundled or official TypeSafe MCP server. Requires `uvx`, Git, and a
TypeSafe API key. Initial launch downloads its package and dependencies.

```bash
# Set TYPESAFE_API_KEY in the environment that launches Prism first.
prism mcp add jev --transport command --timeout-ms 90000 \
  --env-from TYPESAFE_API_KEY=TYPESAFE_API_KEY -- \
  uvx --from git+https://github.com/blakestone-x/jev-mcp@v0.2.1 jev-mcp

prism mcp tools jev --schema
```

`--env-from` saves the environment variable reference, not its secret value.
Inspect the discovered schema before calling `jev_ask`, `jev_classify`, or
another wrapper tool. Calls use the hosted TypeSafe service and its usage
charges. The wrapper's tool names and inputs differ from Laya's.

## Grant agent access

Registration does not grant an agent access. For an existing agent, explicitly
select the intended server, for example:

```bash
prism mcp access agent set reviewer --mode custom --server laya
# Or select Jev:
# prism mcp access agent set reviewer --mode custom --server jev
```

Replace `reviewer` with your agent ID. This sets its custom server list; include
any other required servers with additional `--server` flags. Reconnect the host
after setup. CLI `mcp tools` and `mcp call` commands above are operator checks;
successful checks do not establish agent authorization.

To expose either wrapper directly to another MCP host, use the same executable,
arguments, and environment in that host's stdio configuration. The cluster's
Laya HTTPS origin is an HTTP decision API, not a streamable HTTP MCP endpoint.

## Verified local pattern

On 2026-10-17, a local Laya 0.4.8 wrapper with MCP 2.3.0 successfully
exposed nine tools to Prism. A fresh `prism mcp serve` session accepted
`call_mcp_tool(server="laya", tool="laya_predict", ...)` for the duplicate-charge
example above and returned `billing` with probability 0.9855 and reported
latency 25.385 ms. `laya_status` confirmed the remote English checkpoint ran on
CUDA with zero CPU fallbacks. This verifies the MCP transport and decision
pattern; it does not test an autonomous specialist's choice to invoke the tool.

The built-in reranker remains configured through the direct HTTP adapter. MCP
is an additional callable capability, not a replacement reranking backend.
