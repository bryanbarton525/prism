# Agent skills

Prism agents are invoked with **explicit Agent Skills** attached to each run.
Skills narrow what the local model is allowed to assume and which procedures it
may follow, which reduces scope creep and improves accuracy on specialized work.

Skills in this repository follow the [Agent Skills specification](https://agentskills.io/specification),
including [frontmatter requirements](https://agentskills.io/specification#frontmatter).

## Directory layout

Each skill is a directory with a required `SKILL.md` file:

```text
skills/
|-- go-testing/
|   |-- SKILL.md
|   |-- references/
|   `-- scripts/
`-- README.md
```

Optional subdirectories (`scripts/`, `references/`, `assets/`) follow the spec.

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

The orchestrator (Cursor, Copilot, etc.) chooses which skills to attach based on
the subtask. Prism enforces the allowlist; it does not auto-attach every skill
an agent could use.
