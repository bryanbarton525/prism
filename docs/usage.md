# Prism usage

## Runtime model

The Prism executable is a self-contained release bundle. Its embedded filesystem includes agent specifications, constitutions, skills, and every supporting reference, script, and eval beneath those skills. Run `prism version --json` to inspect the executable version, bundle version, content digest, VCS revision, and dirty state.

`--agent-dir` and `--skills-dir` are development overrides. They change `bundle_mode` to `development` and produce a fresh digest.

## Workspace selection

The workspace is repository data an agent may inspect; it is unrelated to Prism's installation directory.

For direct CLI commands, a repository-aware operation uses `--root` when supplied and otherwise uses the current directory. The MCP server can start with no root:

```bash
prism mcp serve
```

Repository-aware MCP calls may supply:

```json
"workspace": { "root": "/absolute/path/to/repository" }
```

Prism resolves an MCP workspace in this order:

1. Explicit `workspace.root` in the call.
2. The single root advertised by the MCP client.
3. Server-level `--root` fallback.
4. An error only when the chosen specialist requires repository access.

If a client advertises multiple roots, the call must select one explicitly. Local paths are canonicalized before repository plugins are created. Each filesystem plugin receives an `fs.FS` rooted at that directory, preventing traversal outside it.

## Host installation

Run `prism install` for the guided flow. It displays the release and digest, prompts for bundled skills and specialists, detects hosts, selects project/global scope and link/copy mode, then separately selects user/project runtime scope, optional skill sources, and optional agent import/copy. It previews both transactions and asks for confirmation.

```bash
prism install --project --all --yes
prism install --global --target codex --skill prism-mcp-orchestrator --yes
prism install --target claude --specialist kubectl --skill kubectl-triage --yes
prism install --all --copy --dry-run
prism install status --global
prism uninstall --global
```

| Host | Project specialists | Global specialists | MCP configuration |
|---|---|---|---|
| Codex | `.codex/agents/*.toml` | `~/.codex/agents/*.toml` | `.codex/config.toml` or `~/.codex/config.toml` |
| Copilot | `.github/agents/*.agent.md` | `~/.copilot/agents/*.agent.md` | `.vscode/mcp.json` |
| Antigravity | `.agents/agents/*.md` | `~/.gemini/config/agents/*.md` | Gemini/Antigravity settings |
| Claude Code | `.claude/agents/*.md` | `~/.claude/agents/*.md` | `.mcp.json` or `~/.claude.json` |
| OpenCode | `.opencode/agents/*.md` | `~/.config/opencode/agents/*.md` | `opencode.json` |

Project skills always have a universal `.agents/skills` copy. Other host directories link to it by default and fall back to copies when links are unavailable. `--copy` requests copies explicitly.

Generated specialist files are adapters, not copies of Prism frontmatter. They preserve identity and the allowed skill list, then instruct the host to delegate through Prism MCP. The authoritative constitution, model, and tool policy remain embedded in Prism.

The scope manifest is `.prism/install.json`. Upgrades touch only its recorded paths. Prism refuses unmanaged collisions unless `--force` is used, backs up changed host configuration, removes stale managed paths, and rolls back touched paths on failure.

`prism install --runtime-only` initializes only runtime-extension state. It does
not download Graphify, create an index, register an endpoint, or alter an
existing `graphify.yaml` binding. The same is true of unattended installation
paths (`--yes`, `--all`) and install previews (`--dry-run`).

`--runtime-skill-source` accepts an explicitly selected local directory or a
GitHub source supported by the bounded resolver. Multiple discovered skills
require `--runtime-skill-all` or one or more `--runtime-skill-name` values.
`--runtime-agent-source` requires an explicit `--runtime-agent-model`; source
model settings are never adopted as an execution target. A bundled agent can
be made independent with `--runtime-agent-copy ID --runtime-agent-as NEW-ID`.
`--runtime-replace` replaces only matching managed runtime entries. These
runtime actions are never selected by `--yes` or `--all`.
Per-agent MCP access is intentionally unchanged by import and copy; configure
it explicitly with `prism --state-dir STATE mcp access agent set AGENT ...` so
unattended setup never grants a capability.

Hosts receive an absolute `--state-dir` in their generated MCP command. On
startup Prism loads that selected state directory's `config.env`, after
explicit environment variables, so a host launched elsewhere observes the
same runtime configuration as setup. Global host installation cannot target a
project runtime.

## Graphify repository investigation

Graphify is optional and disabled until an operator records an exact binding.
Prism never downloads its executable, starts a service during setup or doctor,
or builds an index. The bundled `repo-investigator` is intended only for
repository architecture, cross-component relationship, dependency-path, and
change-impact investigations. Hosts delegate it through Prism `run_agent`;
they do not receive its internal Graphify tool instructions. Keep ordinary
one-file reads and simple symbol searches in the parent.

The reviewed upstream reference is
[`Graphify-Labs/graphify` `v0.9.61`](https://github.com/Graphify-Labs/graphify/tree/v0.9.61)
at commit `fe66389083369c3159aa391117185c8f58b4d07c`. Its package is
`graphifyy[mcp]==0.9.61`, requires Python `>=3.10`, and provides
`graphify-mcp`; the MCP extra declares `mcp>=1,<3` and
`starlette>=1.3.1,<2`. Prism's checked tool contract is
`prism-graphify-mcp-v0.9.61`. The embedded
`skills/graphify-query/references/GRAPHIFY-RELEASE.json` records the release
asset digest, Apache-2.0 notice, exact source basis, and intentionally
unsupported dependency operations.

Prism does **not** create a Python environment, fetch a Python distribution,
or certify a platform: upstream declares the Python floor but gives Prism no
platform contract. Operators provision a compatible environment and index
outside Prism. The reviewed, deterministic code-only indexing command is:

```bash
python -m pip install "graphifyy[mcp]==0.9.61"
graphify extract "$PWD" --code-only --no-viz
graphify-mcp --graph "$PWD/graphify-out/graph.json"
```

The commands above are not run by Prism or by CI. `--code-only` is the pinned
upstream's local AST-only mode and does not use an LLM or API key;
`--no-viz` omits the unneeded visual output. `graphify-mcp --graph` receives
the absolute `graph.json` path when Prism launches it as a local downstream
server. Create those resources separately, register the MCP server separately,
then record the exact configuration:

```bash
# A service you operate. This writes only state-dir/graphify.yaml.
prism graphify setup --approve \
  --workspace "$PWD" --index "$PWD/graphify-out/graph.json" \
  --fingerprint "$SOURCE_FINGERPRINT" \
  --server graphify --endpoint-kind self-hosted

# A named managed endpoint requires both environment identity and version.
prism graphify setup --approve \
  --workspace "$PWD" --index "$PWD/graphify-out/graph.json" \
  --fingerprint "$SOURCE_FINGERPRINT" \
  --server graphify-prod --endpoint-kind managed \
  --environment production --environment-version 2026.09.1

prism graphify doctor --workspace "$PWD" --fingerprint "$SOURCE_FINGERPRINT"
```

`--upstream-version` and `--schema-version` default to, and reject values
other than, the reviewed upstream release and Prism contract above. For a
user-managed stdio endpoint, register the exact bounded command and bind it
without giving the model a `project_path` override:

```bash
prism mcp add graphify -- graphify-mcp --graph "$PWD/graphify-out/graph.json"
prism graphify setup --approve \
  --workspace "$PWD" --index "$PWD/graphify-out/graph.json" \
  --fingerprint "$SOURCE_FINGERPRINT" \
  --server graphify --endpoint-kind local --executable graphify-mcp
```

Use `--endpoint-kind local --executable /path/to/graphify-mcp` only for a
user-managed local executable. `self-hosted` identifies an endpoint you
operate; `managed` requires a named environment and immutable version pin.
Doctor checks recorded approval, schema support, upstream version metadata,
workspace/fingerprint binding, index presence, local executable availability,
and registration/transport of the named MCP server. It deliberately does not
contact or launch an endpoint, so it cannot trigger a dependency download or
index build. `graphify setup --dry-run` is configuration preview only.

`prism graphify remove --approve` deletes only Prism's configuration file. It
preserves all user-managed executables, indexes, endpoints, and MCP server
registrations.

At invocation Prism checks the pinned MCP schemas before offering the
specialist any tools, allows only `query_graph`, `get_node`, `get_neighbors`,
and `shortest_path`, denies upstream `project_path`, and bounds arguments,
rounds, response bytes, and source reads. Graph results are untrusted leads.
Recognizable source locations are read again through the selected workspace
and returned as `source_verification` artifacts alongside binding, upstream,
and contract provenance. A graph without a recognizable source location stays
explicitly unverified.

For an opt-in local smoke test that uses an operator-provisioned index and
offload model, see [`scripts/graphify-smoke.sh`](../scripts/graphify-smoke.sh).
It is excluded from required CI and refuses to run unless
`PRISM_GRAPHIFY_SMOKE=1` is set.

## Versioning and provenance

GitHub artifacts are built with the release tag injected through `-ldflags`. The MCP initialization response uses the same version. Every run result and stored event is stamped with bundle ID `prism`, the compiled version, digest, and bundle mode. Event-store bundle columns remain compatible with older databases.

## MCP contract

```json
{
  "agent_id": "kubectl",
  "skill_names": ["kubectl-triage"],
  "task": "Inspect the checkout rollout in namespace staging",
  "format": "json"
}
```

Add `workspace` only for repository-aware work or when selecting among multiple host roots. `bundle_id` and `bundle_version` are no longer accepted inputs. The old signed registry runtime and the `bundle`/`registry` commands are removed; `install_bundle` and `list_bundles` are not exposed over MCP.

## Model runtimes and control plane

Ollama is the default. Configure `PRISM_MODEL_RUNTIME_ENGINE`, `PRISM_MODEL_RUNTIME_BASE_URL`, `PRISM_MODEL_RUNTIME_API_KEY`, and `PRISM_MODEL_RUNTIME_MODEL` for another compatible runtime. See [model-runtime.md](model-runtime.md).

```bash
prism --policy-file prism-policy.yaml policy validate prism-policy.yaml
prism --event-store .prism/events.db events list
prism --event-store .prism/events.db dashboard serve --addr 127.0.0.1:8765
prism mcp server list
```
