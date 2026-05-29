# Agent specifications

Each Prism agent is defined as **Markdown with YAML frontmatter**. The runtime
loads every `agents/*.md` file (except `README.md`) at startup via `prism agent
list`, `prism run`, and MCP tools.

The frontmatter is the machine-readable spec; the Markdown body is the
constitution (behavior contract) unless `constitution_path` points elsewhere.

## Required frontmatter

| Field | Purpose |
| --- | --- |
| `id` | Stable identifier (matches file stem). |
| `name` | Display name. |
| `description` | When the orchestrator should delegate here. |
| `model` | Default Ollama model. |
| `context_budget` | Max input size for the local model. |
| `allowed_skills` | Skill `name` values this agent may attach at run time. |
| `latency_budget_ms` | Benchmark and runtime latency budget. |

## Run-time skill attachment

Invocations must pass one or more skills from `allowed_skills`. Skills follow
the [Agent Skills specification](https://agentskills.io/specification#frontmatter)
under `skills/<name>/SKILL.md`.

Prism does not load the full skill library into every prompt-only skills named
on that run.

## Migration note

Initial constitutions live in `constitutions/` while this directory is populated.
New work should add `agents/<id>.md` files; constitutions can be merged into the
spec body or referenced via `constitution_path` until migration completes.


## Initial targeted agent specs

- `agents/github-cli.md`
- `agents/web-docs-search.md`
- `agents/kubectl.md`
- `agents/argo.md`

Each spec uses Markdown + YAML frontmatter and references a matching
constitution plus an `allowed_skills` list.
