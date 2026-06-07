# Prism usage guide

This guide covers day-to-day use of the Prism CLI and MCP server as implemented today.

## Before you start

1. **Clone** the repository and `cd` into it (or pass `--root` to every command).
2. **Install** Go 1.25+ and the binary: `go install ./cmd/prism`
3. **Start Ollama** and confirm it responds: `curl http://127.0.0.1:11434/api/version`
4. **Pull models** used by your agents (see `model:` in `agents/*.md`), e.g.:
  ```bash
   ollama pull qwen3.5:9b
  ```
5. **Run doctor**:
  ```bash
   prism config doctor
  ```
   A `warn` on `agent_models` means Ollama is up but the tag in the agent spec is not installed yet.

## Configuration

Prism resolves paths relative to `**--root**` (default: current working directory). It also reads an optional `.env` file from the current working directory before checking environment variables.


| Setting      | Flag            | Environment variable |
| ------------ | --------------- | -------------------- |
| Project root | `--root`        | `PRISM_ROOT`         |
| Agent specs  | `--agent-dir`   | `PRISM_AGENT_DIR`    |
| Skills       | `--skills-dir`  | `PRISM_SKILLS_DIR`   |
| Ollama URL   | `--ollama-host` | `PRISM_OLLAMA_HOST`  |
| Local state  | `--state-dir`   | `PRISM_STATE_DIR`    |
| Event store  | `--event-store` | `PRISM_EVENT_STORE`  |
| Policy file  | `--policy-file` | `PRISM_POLICY_FILE`  |
| GitHub token | —               | `PRISM_GITHUB_TOKEN`, `PRISM_GH_TOKEN`, `GITHUB_TOKEN`, `GH_TOKEN` |
| Linear MCP URL | —            | `PRISM_LINEAR_MCP_URL` |


Example running from another directory:

```bash
prism --root ~/src/prism agent list
prism --root ~/src/prism config doctor
```

## Full local control-plane setup

Use this path when you want the OSS control-plane features enabled together:
signed bundle verification, policy enforcement, durable events, reports, and the
local dashboard.

```bash
# 1. Build/install the binary and verify local runtime dependencies.
go install ./cmd/prism
ollama pull qwen3.5:9b
prism --root "$(pwd)" config doctor

# 2. Pick one local state directory for registry sources, installed bundles,
#    and the default dashboard/event-store path.
export PRISM_STATE_DIR=.prism
mkdir -p "$PRISM_STATE_DIR"

# 3. Register a local registry source root. Sources may be local paths or
#    http(s) URLs. Manifest paths passed with --source are resolved under it.
prism registry source add local .
prism registry source list
prism registry sync

# 4. Verify and install a signed registry manifest.
prism bundle verify --source local testdata/bundles/k8s-core-triage/registry.json \
  --public-key testdata/bundles/k8s-core-triage/public_key.txt
prism bundle install --source local testdata/bundles/k8s-core-triage/registry.json \
  --dest-root . \
  --public-key testdata/bundles/k8s-core-triage/public_key.txt
prism bundle list

# 5. Validate policy before enforcing it on runs or MCP.
prism policy validate testdata/policies/k8s-readonly.yaml
prism policy test testdata/policies/k8s-readonly.yaml testdata/policies/k8s-readonly-cases.yaml

# 6. Run with event storage, policy, and bundle provenance.
echo "Investigate deployment checkout-api in namespace staging" | \
  prism --policy-file testdata/policies/k8s-readonly.yaml \
    --event-store "$PRISM_STATE_DIR/events.db" \
    run kubectl \
    --skills kubectl-triage,k8s-rollout-diagnostics \
    --bundle-id k8s-core-triage

# 7. Inspect events, reports, and dashboard.
prism --event-store "$PRISM_STATE_DIR/events.db" events summarize
prism --event-store "$PRISM_STATE_DIR/events.db" report usage
prism --event-store "$PRISM_STATE_DIR/events.db" dashboard serve --addr 127.0.0.1:8765
```

Open `http://127.0.0.1:8765` for the dashboard. The dashboard reads the same
SQLite event store as `events` and `report`; it will be empty until at least one
run or graph has been executed with `--event-store` or `PRISM_EVENT_STORE`.

### Creating a registry today

Prism's registry install path consumes a signed registry manifest. A registry
source is a named local path or `http(s)` URL saved with `prism registry source
add`; `prism bundle verify --source <name>` and `prism bundle install --source
<name>` resolve the manifest path under that source.

A signed registry manifest must include:

- registry metadata: `registry_id`, `version`, `generated_at`, and optional `compat`
- one or more bundles with `id`, `version`, `channel`, `owner`, `risk_level`,
  `agents`, `skills`, `required_plugins`, and `files`
- file entries with path-safe `path` values and SHA-256 hashes
- an Ed25519 `signature` over the manifest payload
- a matching public key supplied to `prism bundle verify` or `prism bundle install`

The fixture at `testdata/bundles/k8s-core-triage/registry.json` is the canonical
working example.

Local configured source:

```bash
prism registry source add local .
prism bundle install --source local testdata/bundles/k8s-core-triage/registry.json \
  --dest-root . \
  --public-key testdata/bundles/k8s-core-triage/public_key.txt
```

Remote configured source:

```bash
prism registry source add platform https://registry.example.com/prism
prism bundle install --source platform k8s-core-triage/registry.json \
  --dest-root . \
  --public-key ./platform-registry-public-key.txt
```

Direct remote manifest URL:

```bash
prism bundle verify https://registry.example.com/prism/k8s-core-triage/registry.json \
  --public-key ./platform-registry-public-key.txt
```

For remote manifests, Prism downloads the manifest's declared files into a
temporary verification root, then runs the same Ed25519 signature, Prism-version
compatibility, SHA-256 checksum, and path-safety checks before installing. The
v1 CLI verifies and installs signed manifests; `bundle build`, `bundle sign`,
and remote publishing are planned lifecycle commands and are not implemented yet.

## CLI workflows

### Inspect agents

```bash
prism agent list
prism agent list --json

prism agent show github-cli
prism agent constitution kubectl --json
```

`agent show` prints frontmatter fields and the inline Markdown body. `agent constitution` prints only the **resolved** contract (from `constitution_path`, inline body, or `constitutions/<id>.md`).

### Run an agent

Skills are **required** and must be listed in the agent’s `allowed_skills`:

```bash
# From a file
prism run github-cli --skills gh-pr-triage --input ./task.md

# Multiple skills (order affects prompt assembly)
prism run kubectl --skills kubectl-triage,k8s-rollout-diagnostics --input task.md

# Stdin
prism run web-docs-search --skills docs-source-harvest --stdin <<'EOF'
Find the rate limit for the GitHub REST API.
EOF

# Piped stdin (no --stdin flag needed)
echo "Check sync health" | prism run argo --skills argo-sync-health

# Attribute a run to an installed bundle version for policy and reporting
echo "Investigate deployment checkout-api in namespace staging" | \
  prism --state-dir .prism run kubectl \
    --skills kubectl-triage,k8s-rollout-diagnostics \
    --bundle-id k8s-core-triage
```

**Output formats**

- `--format json` (default) — full `RunResult` envelope on stdout
- `--format markdown` — human-readable report

**Bundle attribution**

- `--bundle-id` records the bundle on the run and lets policy check allowed bundles.
- `--bundle-version` records an explicit version.
- If `--bundle-id` is set without `--bundle-version`, Prism resolves the version from installed bundle state under `--state-dir`.
- If the bundle is not installed and no version is supplied, the CLI fails closed instead of recording ambiguous provenance.

**Validation**

If a skill is not in `allowed_skills`, the run returns `status: validation_fail` without calling Ollama.

If the assembled prompt exceeds `context_budget`, the runtime truncates the system prompt and sets `context_budget_exceeded: true` in the result.

If the run exceeds `latency_budget_ms`, the context deadline may return `status: timeout`.

### Control-plane commands

Policy is optional and preserves existing behavior when unset:

```bash
prism policy validate testdata/policies/k8s-readonly.yaml
prism policy explain testdata/policies/k8s-readonly.yaml kubectl \
  --skills k8s-rollout-diagnostics \
  --plugins kubernetes
prism policy test testdata/policies/k8s-readonly.yaml testdata/policies/k8s-readonly-cases.yaml

echo "Investigate deployment checkout-api in namespace staging" | \
  prism route suggest
```

Durable event storage is opt-in for runs with `--event-store` or `PRISM_EVENT_STORE`:

```bash
prism --event-store .prism/events.db run github-cli --skills gh-pr-triage --stdin
prism events list --limit 20
prism events export --format csv
prism events summarize
```

Skill, bundle, registry, graph, dashboard, and report commands:

```bash
prism skill lint
prism skill test k8s-rollout-diagnostics
prism skill benchmark --max-chars 24000

prism registry source add local .
prism registry source list
prism registry sync

prism bundle verify --source local testdata/bundles/k8s-core-triage/registry.json \
  --public-key testdata/bundles/k8s-core-triage/public_key.txt
prism --state-dir .prism bundle install --source local testdata/bundles/k8s-core-triage/registry.json \
  --dest-root . \
  --public-key testdata/bundles/k8s-core-triage/public_key.txt
prism bundle list

prism graph validate testdata/graphs/k8s-rollout-investigation.yaml
prism graph run testdata/graphs/k8s-rollout-investigation.yaml

prism dashboard serve
prism report usage
prism report savings --format json
prism report adoption
prism report bundles
```

The local event store records run and graph metadata, not raw prompts, raw logs, or raw evidence.

For a deterministic local control-plane smoke test that avoids live model calls:

```bash
scripts/local-acceptance.sh
```

### Diagnostics

```bash
prism config doctor
prism config doctor --json
```

Checks include:

- `ollama_connectivity` — ping `/api/version`
- `ollama_models` — list tags from `/api/tags`
- `agent_models` — warn when a spec’s `model` is not in the local list
- `agent_registry` — loaded agent count and IDs
- `skill_registry` — skills passing structure validation

## Programmatic integrations

Prism's primary interfaces are still the CLI and MCP server, but the repository
also exposes a few Go packages for integrations that need stable contracts
without scraping command output.

### `pkg/observe`

- `Metadata` carries optional caller context such as `actor_id`,
  `workspace_id`, `source`, and `correlation_id`.
- `RunEvent` is a stable event shape emitted after each `Runner.Run` call.
- `Sink` is a small interface you can implement and pass through
  `app.Config.EventSink`.

The shared runner emits one event for successful runs, validation failures,
timeouts, and model/runtime errors. CLI calls tag `source: cli`; MCP calls tag
`source: mcp`.

### `pkg/registry`

- `Manifest` defines a signed registry document for agent/skill bundles.
- `VerifySignature` checks the Ed25519 signature over the manifest payload.
- `VerifyCompat` checks the manifest's Prism version bounds.
- `VerifyManifest` runs signature, compatibility, and file integrity checks together.
- `VerifyFiles` checks SHA-256 digests for bundle files under a source root.
- `Install` is the safe install entrypoint: it verifies signature, compatibility,
  and file hashes before copying files into the destination root with path-safety checks.

The CLI `prism bundle install` uses this same fail-closed path for signed registry
manifests. Local installed-bundle state is recorded only after verification and
copying both succeed.

This is useful for controlled distribution of approved `agents/` and `skills/`
content while still relying on Prism's existing `--agent-dir` and `--skills-dir`
resolution.

### `pkg/report`

- `MonthlyProjectionReport` returns the structured benchmark projection.
- `MonthlyProjectionJSON` returns the same projection as indented JSON.
- `MonthlyProjectionMarkdown` returns the human-readable report.

These helpers wrap the existing benchmark projection logic in
`internal/benchmark` so external tooling can reuse it directly.

## MCP server

### How it works

- Command: `prism mcp serve`
- Transport: **stdio** (JSON-RPC)
- Logging: **stderr** only (`[prism-mcp] ...`)
- Same `AgentRunner` as the CLI — plugin evidence and results match CLI JSON shapes

Always pass `**--root`** to the absolute path of this repository when the MCP host’s working directory is not the repo root.

### MCP host configuration

Example for Cursor (`~/.cursor/mcp.json`). Other MCP-compatible editors use equivalent server config — check your host's MCP documentation for file location and format.

```json
{
  "mcpServers": {
    "prism": {
      "command": "/Users/you/go/bin/prism",
      "args": [
        "mcp",
        "serve",
        "--root",
        "/Users/you/src/prism"
      ],
      "env": { "PRISM_OLLAMA_HOST": "http://127.0.0.1:11434" }
    }
  }
}
```

After saving, reload MCP servers in your editor settings. The **prism** server should list:
- Core tools: `list_agents`, `run_agent`, `get_constitution`, `doctor`, `suggest_route`, `run_graph`, `explain_policy`, `list_policies`, `list_mcp_servers`, `list_mcp_server_tools`, `call_mcp_tool`
- Compatibility tools: `list_prompts`, `get_prompt`, `list_resources`, `get_resource`

For a local Gemini MCP config, this repository also includes a helper at `scripts/install_mcp.py`. Review the paths in the script first, then run it from the repo root with `python3 scripts/install_mcp.py`.

### Remote `--root` (git URL)

`--root` accepts a github.com URL in addition to a local path. When a URL is given, Prism reads files dynamically via the GitHub Contents API without cloning the repository.

**Supported URL formats:** `https://github.com/owner/repo`, `git@github.com:owner/repo.git`, `https://github.com/owner/repo/tree/branch`

```json
{
  "mcpServers": {
    "prism": {
      "command": "/Users/you/go/bin/prism",
      "args": [
        "mcp",
        "serve",
        "--root",
        "https://github.com/bryanbarton525/prism"
      ],
      "env": {
        "PRISM_OLLAMA_HOST": "http://127.0.0.1:11434",
        "GITHUB_TOKEN": "ghp_yourtokenhere"
      }
    }
  }
}
```

**Requirements and behaviour:**

- Set `PRISM_GITHUB_TOKEN`, `PRISM_GH_TOKEN`, `GITHUB_TOKEN`, or `GH_TOKEN` in the environment or local `.env` file. `PRISM_GITHUB_TOKEN` is the Prism-native name; the others are accepted aliases.
- If neither token is set or the API is inaccessible, Prism falls back to `git clone --depth 1 <url> <tmpdir>` (requires `git` on `PATH`).
- The fallback temp directory is removed when the process exits.
- `--agent-dir` and `--skills-dir` still override subdirectory paths if set explicitly.

### Runtime plugins

Agent specs may declare a `tools:` allowlist. Prism resolves those names through the native runtime plugin registry, collects bounded read-only evidence before prompt assembly, and includes that evidence in both the specialist prompt and the returned artifacts. If a policy sets `max_evidence_bytes`, oversized evidence is blocked before the prompt is assembled or the model is called.

Built-in v1 plugins:

- `kubernetes` / alias `kubectl`: native client-go diagnostics for namespace, pod, deployment, service, event, EndpointSlice, HTTPRoute, and server-version evidence.
- `github`: repo-local `.github` workflow/template metadata. This does not call the live GitHub API.
- `localdocs` / alias `docs`: bounded search over `docs/`, README, and Markdown files.
- `filesystem` / alias `fs`: bounded read-only search over common repo text files.
- `goproject` / alias `go`: bounded Go project metadata from `go.mod` and package paths.
- `linear`: Linear MCP setup context, issue-key extraction, and operation hints. This plugin does not execute Linear writes.
- `mcp`: compact inventory for configured downstream MCP servers. This lets a local specialist inspect downstream tool names without loading every downstream tool schema into the parent orchestrator.

The `kubectl` agent declares:

```yaml
tools:
  - kubernetes
```

That means Prism uses Kubernetes client-go APIs and does not shell out to `kubectl` for runtime evidence. Results are labeled `runtime-plugin:kubernetes`; structured evidence packs are returned as `evidence-pack:*` artifacts.

The `linear` agent declares:

```yaml
tools:
  - linear
```

That means Prism adds bounded Linear MCP context before the local specialist runs. Configure an authenticated Linear MCP server in the parent MCP host for live searches and mutations. Prism's native plugin returns proposed create/edit/comment/archive actions and evidence artifacts; it does not create or edit Linear issues itself.

### Downstream MCP clients

Prism can also act as a bounded MCP client to downstream MCP servers. This is useful when a specialist should own a bulky tool surface, such as Linear issues/projects/comments, without forcing the parent model to carry all downstream tool schemas.

Configuration is stored in `mcp-servers.yaml` under `--state-dir`.

For Linear, use command transport through `mcp-remote` until Prism's Go SDK path supports Streamable HTTP directly:

```bash
prism mcp server add-command linear npx -y mcp-remote https://mcp.linear.app/mcp
prism mcp server list
prism mcp server tools linear
```

The Linear OAuth/token flow is handled by the downstream MCP command or its auth cache. Prism does not store Linear API tokens.

Call one downstream tool through Prism:

```bash
prism mcp server call linear search_issues --args-json '{"query":"checkout rollout"}'
```

The same bridge is exposed over Prism MCP:

- `list_mcp_servers`
- `list_mcp_server_tools`
- `call_mcp_tool`

Use `call_mcp_tool` only after policy and user approval for write-oriented downstream actions.

When an agent declares `tools: [mcp]`, Prism also exposes these bridge functions to the local Ollama chat request as tool definitions. Tool-capable local models may call `list_mcp_servers`, `list_mcp_server_tools`, and `call_mcp_tool` during the specialist run; Prism executes bounded downstream calls, appends the tool result, and asks the model for the final compact result envelope.

### Tool reference

#### `list_agents`

No parameters. Returns agent id, name, description, model, `allowed_skills`, and `latency_budget_ms`.

#### `get_constitution`

```json
{ "agent_id": "github-cli" }
```

Returns `agent_id`, `source` (`path` | `body` | `legacy`), optional `path`, and `text`.

#### `doctor`

No parameters. Same information as `prism config doctor` (JSON).

#### `run_agent`

```json
{
  "agent_id": "github-cli",
  "task": "Describe what to investigate.",
  "skill_names": ["gh-pr-triage"],
  "format": "json",
  "bundle_id": "team-github-triage",
  "bundle_version": "0.1.0"
}
```

- `skill_names` is required (at least one entry).
- `format` is optional (`json` or `markdown`).
- `bundle_id` and `bundle_version` are optional; use them when attributing a run to an installed, policy-approved bundle.

Response is a `RunResult` object (see README).

#### `suggest_route`

```json
{
  "task": "Investigate deployment checkout-api in namespace staging",
  "source": "mcp"
}
```

Returns a deterministic recommendation with `agent_id`, `skill_names`, reason, risk, and policy-shaped decision metadata.

#### `run_graph`

```json
{
  "graph": {
    "id": "k8s-rollout-investigation",
    "version": 1,
    "nodes": {
      "analyze": {
        "agent": "kubectl",
        "skills": ["k8s-rollout-diagnostics"],
        "task": "Investigate deployment checkout-api in namespace staging."
      }
    }
  }
}
```

Runs a bounded graph through the same shared runner as CLI and `run_agent`. When a policy file is configured, Prism prechecks graph size/depth plus each node's agent, skills, and plugins before execution. V1 graph execution is sequential; `max_parallel` must be `0` or `1`, retries are not yet implemented, and node artifacts are returned in the graph result as bounded typed artifacts.

#### `explain_policy`

```json
{
  "agent_id": "kubectl",
  "skills": ["k8s-rollout-diagnostics"],
  "plugins": ["kubernetes"],
  "source": "mcp"
}
```

Returns the configured policy decision. If no policy file is configured, the decision is `allow` with reason `no policy configured`.

#### `list_policies`

No parameters. Reports whether the MCP server started with a configured policy.

#### `list_prompts` / `get_prompt`

Compatibility tools that expose reusable prompt templates for accurate `run_agent` calls
in MCP hosts that do not support native MCP prompts yet.

```json
{ "prompt_id": "k8s_incident_triage", "variables": { "namespace": "payments" } }
```

Notable built-in prompt IDs:

- `run_agent_json_call` — generate a valid `run_agent` JSON payload
- `prism_delegation_playbook` — full delegation decision + call sequence (`list_agents` -> resources/prompts -> `run_agent` -> parent synthesis)
- `github_pr_triage`, `k8s_incident_triage`, `argo_failure_debug`, `go_codegen_helper` — domain templates

#### `list_resources` / `get_resource`

Compatibility tools that expose Prism resources (tool contracts, orchestration guide,
agents index, constitutions) in hosts that do not support native MCP resources.

Example URIs:
- `prism://resource/tooling/run_agent`
- `prism://resource/tooling/orchestration-guide`
- `prism://resource/agents/index`
- `prism://resource/agent/github-cli/constitution`

### Test without an MCP host

**MCP Inspector** (browser UI):

```bash
cd /path/to/prism
npx @modelcontextprotocol/inspector prism mcp serve --root "$(pwd)"
```

Suggested order:

1. Connect / initialize
2. `doctor`
3. `list_agents`
4. `get_constitution` with `agent_id: github-cli`
5. `run_agent` with a short task (requires Ollama + model)

## Adding agents and skills

### New agent

1. Create `agents/<id>.md` with required frontmatter (see [agents/README.md](../agents/README.md)).
2. Put the constitution in the file body, or set `constitution_path`, or add `constitutions/<id>.md`.
3. List skill names in `allowed_skills`.
4. Optionally list native runtime plugins in `tools` when the agent needs Prism-collected evidence.
5. Verify: `prism agent show <id>` and `prism config doctor`.

### New skill

1. Create `skills/<name>/` with:
  - `SKILL.md` (frontmatter `name` must match directory name)
  - `evals/smoke.yaml`
  - `references/REFERENCE.md`
  - `scripts/collect.sh`
2. Add `<name>` to an agent’s `allowed_skills`.
3. Verify: `prism skill test <name>` and `go test ./internal/benchmark/...`

### Publish/install via skills.sh CLI

Prism skills can be distributed with the skills CLI:

```bash
# from a public GitHub source
npx skills add github.com/bryanbarton525/prism -l --full-depth
npx skills add github.com/bryanbarton525/prism --skill prism-mcp-orchestrator --full-depth
```

`prism-mcp-orchestrator` lives under `skills/` like the others (standard
`npx skills` discovery). It is for host-orchestrator behavior only — no Prism
agent lists it in `allowed_skills`.

## Troubleshooting


| Problem                      | Likely cause                           | Fix                                                                        |
| ---------------------------- | -------------------------------------- | -------------------------------------------------------------------------- |
| `no agents found`            | Wrong `--root` or cwd                  | `cd` to repo or set `--root`                                               |
| MCP tools missing in editor | Bad `command` path or MCP not reloaded | Use absolute path to `prism`; reload MCP in host settings |
| `validation_fail` for skills | Skill not in `allowed_skills`          | Check `prism agent show <id>`                                              |
| `error` / timeout on run     | Ollama down or slow                    | `prism config doctor`; increase `latency_budget_ms`                        |
| Empty or poor model output   | Wrong/missing model                    | `ollama list`; pull or edit `model:` in spec                               |
| MCP hangs                    | Inspecting stdout                      | Use Inspector or your MCP host; don’t run `mcp serve` interactively in a terminal |


## Benchmark comparison (no MCP vs MCP)

Run the all-skill mock incident and print a comparison report:

```bash
prism benchmark run homelab-release-incident
prism benchmark run homelab-release-incident-at-scale   # enterprise context padding
prism benchmark project                               # monthly/annual savings
```

`prism benchmark project` emits the headline with-vs-without orchestrator cost matrix
from the live `todo-spa-build` benchmark in `testdata/benchmarks/results.yaml`,
priced across GPT 5.4/5.5 and Claude Opus/Sonnet variants in
`testdata/benchmarks/orchestrator-models.yaml`.

See [benchmark-homelab-incident.md](benchmark-homelab-incident.md) for the full scenario and [benchmark-scale.md](benchmark-scale.md) for monthly projections.

## Related docs

- [Comparison / landscape](comparison.md) — vs Claude subagents, Cursor, frameworks
- [Implementation plan](implementation-plan.md) — architecture and future milestones
- [Success metrics](success-metrics.md) — benchmark goals
- [Benchmark: homelab incident](benchmark-homelab-incident.md) — eight-delegation A/B scenario
- [Benchmark at scale](benchmark-scale.md) — monthly savings projection
- [Tooling references](tooling-references.md) — SDKs and specs
