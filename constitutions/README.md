# Prism agent constitutions

Prism constitutions define the stable behavior contract for local specialist
agents. They are intentionally narrower than general system prompts: each one
states what the agent is for, what it must not do, what context it needs, and
how it should report results back to the primary orchestrator.

**Runtime resolution:** for each agent, Prism uses (1) `constitution_path` in the
agent spec, (2) the spec Markdown body, or (3) `constitutions/<id>.md` as a
legacy fallback. Inspect with `prism agent constitution <id>`.

Agent specs live under `agents/<id>.md`. This directory holds constitution-only
artifacts during migration. See [agent specifications](../agents/README.md),
[agent skills](../skills/README.md), and [docs/usage.md](../docs/usage.md).


## Initial agents

- [GitHub CLI](github-cli.md) - inspect PRs, CI runs, commit metadata, and logs using `gh`.
- [Web/docs search](web-docs-search.md) - gather source-backed documentation findings.
- [Kubernetes kubectl](kubectl.md) - inspect pods, events, rollouts, and cluster state.
- [Argo](argo.md) - inspect Argo CD/Workflows sync status and diagnostics.

## Shared rules

All Prism agents must:

1. Stay inside their specialty and **only use Agent Skills attached to the run**.
2. Prefer concise, structured output over conversational prose.
3. State uncertainty and missing context explicitly.
4. Avoid pretending to have run tools or inspected files they were not given.
5. Return evidence and reasoning that the primary orchestrator can verify.
6. Avoid direct mutation of the repository until the runtime has explicit,
   audited write-tool support.
7. Refuse work that requires skills not passed on the invocation (scope control).

## Suggested output envelope

Agents may use different domain-specific sections, but they should fit inside
the shared result envelope described in the implementation plan:

```json
{
  "agent_id": "github-cli",
  "status": "ok",
  "summary": "",
  "findings": [],
  "artifacts": [],
  "confidence": "medium"
}
```
