# Agent skills

Prism loads skills from `skills/<name>/` when you pass `--skills` on the CLI or
`skill_names` to MCP `run_agent`. Use `prism config doctor` to validate layout.

See [docs/usage.md](../docs/usage.md) for examples.

## Install from repository via skills CLI

Use the skills ecosystem CLI to discover/install skills from this repo:

```bash
npx skills add github.com/bryanbarton525/prism -l
npx skills add github.com/bryanbarton525/prism --skill prism-mcp-orchestrator -g -y
```

The `prism-mcp-orchestrator` skill teaches a parent model when delegation is
worth it and how to call Prism MCP tools/resources/prompts in the correct order.
It lives alongside the specialist skills in `skills/` for standard discovery,
but is **not** listed in any agent `allowed_skills` — only your editor/host
orchestrator should install it.

Prism agents are invoked with **explicit Agent Skills** attached to each run.
Skills narrow what the local model is allowed to assume and which procedures it
may follow, which reduces scope creep and improves accuracy on specialized work.

Skills in this repository follow the [Agent Skills specification](https://agentskills.io/specification),
including [frontmatter requirements](https://agentskills.io/specification#frontmatter).
Authoring should also follow the
[Anthropic skill-creator baseline](https://github.com/anthropics/skills/blob/main/skills/skill-creator/SKILL.md)
for practical writing patterns, eval-first iteration, and trigger-friendly
descriptions.

## Directory layout

Each skill is a directory with a required `SKILL.md` file:

```text
skills/
|-- gh-pr-triage/
|   |-- SKILL.md
|   |-- references/
|   `-- scripts/
`-- README.md
```

Prism requirement: every skill directory must include both `references/` and
`scripts/` so the runtime can pass focused documentation, helper CLIs, and
deterministic data-collection logic to the local agent. `assets/` remains
optional.

## Required per-skill structure

Every `skills/<name>/` directory must contain:

- `SKILL.md`
- `references/REFERENCE.md` (or equivalent focused docs)
- `scripts/` with one or more executable helpers for repeatable data gathering

This is a hard project rule for Prism, not just a recommendation.

## Authoring baseline

When writing or revising Prism skills, use the Anthropic `skill-creator`
guidance as the default pattern:

1. Capture intent and triggering contexts clearly.
2. Write or revise `SKILL.md` with strong "when to use" language in
   `description`.
3. Define realistic test prompts and iterate skill quality with evals.
4. Keep instructions concise and move bulky material to `references/`.

Prism-specific constraint: even with this authoring pattern, run-time behavior
is still enforced by `allowed_skills` in agent specs and explicit `skill_names`
on each invocation.

## `SKILL.md` frontmatter

Required fields per the Agent Skills spec:

- `name` - lowercase hyphenated identifier matching the directory name.
- `description` - what the skill does and when to use it (max 1024 characters).

Optional fields Prism may honor when present:

- `license`
- `compatibility` - environment requirements (for example Ollama, local repo access).
- `metadata` - arbitrary string key-value pairs (for example `prism-agent: reviewer`).
- `allowed-tools` - experimental space-separated tool pre-approval list.

Example:

```markdown
---
name: go-testing
description: Design focused Go tests, table-driven cases, and validation commands. Use when adding or updating Go test coverage.
compatibility: Requires local repository read access and standard Go toolchain.
metadata:
  prism-agents: test-designer reviewer
---

# Go testing skill

...
```

## How Prism uses skills

1. **Discovery** - Prism loads `name` and `description` from every skill under
   `skills/` (or configured skill roots) for orchestrator selection.
2. **Invocation** - Each `prism run` or MCP `run_agent` call must include one
   or more skill IDs. Prism validates them against the target agent's
   `allowed_skills` list in the agent spec.
3. **Progressive disclosure** - Prism injects skill metadata first, then the
   full `SKILL.md` body only for skills attached to that run (not the entire
   skill library).
4. **Scope control** - Agents refuse tasks that require capabilities outside the
   attached skills and their constitution.

The orchestrator (your AI editor) chooses which skills to attach based on
the subtask. Prism enforces the allowlist; it does not auto-attach every skill
an agent could use.
