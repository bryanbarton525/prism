# Agent specifications

Each Prism agent is defined as **Markdown with YAML frontmatter**. Release
builds load the immutable embedded `agents/*.md` bundle (except `README.md`);
development overrides and managed extensions use the same format.

The frontmatter is the machine-readable spec; the Markdown body is the
constitution (behavior contract) unless `constitution_path` points elsewhere.

## Required frontmatter

| Field | Purpose |
| --- | --- |
| `id` | Stable identifier (matches file stem). |
| `name` | Display name. |
| `description` | When the orchestrator should delegate here. |
| `model` | Selected execution model for the configured local or self-hosted runtime. |
| `context_budget` | Max input size for the local model. |
| `allowed_skills` | Skill `name` values this agent may attach at run time. |
| `latency_budget_ms` | Benchmark and runtime latency budget. |

## Optional frontmatter

| Field | Purpose |
| --- | --- |
| `tools` | Runtime plugin allowlist. Names resolve through Prism's plugin registry, for example `kubernetes`. |

## Run-time skill attachment

Bundled-agent invocations must pass one or more skills from `allowed_skills`.
User-managed agents may explicitly declare an empty allowlist and run with no
skills; any supplied skill must still appear in the allowlist. Skills follow
the [Agent Skills specification](https://agentskills.io/specification#frontmatter)
under `skills/<name>/SKILL.md`.

Prism does not load the full skill library into every prompt; it loads only
skills named on that run.

Managed imports require an explicit execution target via `--model`,
`--use-configured-model`, or a digest-bound import configuration. Claude Code
Markdown and Codex TOML inputs are translated into this format; unsupported
source fields must be mapped or explicitly omitted. A referenced constitution
is copied into the immutable managed package so it never resolves against the
release bundle accidentally. See [runtime extension management](../docs/usage.md#runtime-extension-management).

## Runtime plugin evidence

When an agent declares `tools:`, Prism collects bounded read-only evidence from
those plugins before the local model runs. The evidence is added to the prompt
and returned as artifacts such as `runtime-plugin:kubernetes`.

The current built-in Kubernetes plugin uses native client-go APIs. The
`kubectl` agent name is kept for familiarity, but its runtime evidence is not
collected by shelling out to the `kubectl` CLI.

## Migration note

Initial constitutions live in `constitutions/` while this directory is populated.
New work should add `agents/<id>.md` files; constitutions can be merged into the
spec body or referenced via `constitution_path` until migration completes.


## Registered agents

All agents are live and loaded at startup:

- `agents/github-cli.md` — PR triage, GitHub Actions failures
- `agents/kubectl.md` — Kubernetes pod/rollout diagnostics via native plugin
- `agents/linear.md` — Linear issue/project/cycle workflows via MCP offload
- `agents/argo.md` — Argo CD sync, workflow debug
- `agents/web-docs-search.md` — docs harvest, release notes
- `agents/go-helper.md` — small Go helpers and pure utilities
- `agents/go-scaffold.md` — package boilerplate and test scaffolds
- `agents/frontend-builder.md` — vanilla HTML/CSS/JS UI subtasks
- `agents/repo-investigator.md` — bounded repository architecture and relationship investigation through Prism

Each spec uses Markdown + YAML frontmatter and references a matching
constitution plus an `allowed_skills` list.
