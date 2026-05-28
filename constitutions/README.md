# Prism agent constitutions

Prism constitutions define the stable behavior contract for local specialist
agents. They are intentionally narrower than general system prompts: each one
states what the agent is for, what it must not do, what context it needs, and
how it should report results back to the primary orchestrator.

**Target format:** each agent will be defined under `agents/<id>.md` as Markdown
with YAML frontmatter (spec) plus a constitution body. These files are the initial
constitution-only artifacts during migration. See [agent specifications](../agents/README.md)
and [agent skills](../skills/README.md).


## Initial agents

- [Researcher](researcher.md) - summarize supplied references and local context.
- [Planner](planner.md) - decompose tasks into implementation steps and risks.
- [Implementer](implementer.md) - propose focused code changes from bounded
  context.
- [Test Designer](test-designer.md) - design validation coverage and commands.
- [Reviewer](reviewer.md) - identify correctness, safety, and maintainability
  risks.

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
  "agent_id": "planner",
  "status": "ok",
  "summary": "",
  "findings": [],
  "artifacts": [],
  "confidence": "medium"
}
```
