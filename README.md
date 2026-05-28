# Prism

Just as a prism separates light into distinct, pure colors, this project
separates work into small, specialized local agents.

Prism is a planning-stage Go project for running specialized sub-agents on
local Ollama models while keeping tools such as Cursor, Copilot, and other
LLMs in the role of orchestrator. The goal is to reduce operating and
development cost, limit main-model token use, and preserve useful latency by
delegating focused tasks to local agents with narrow constitutions.

## Planning artifacts

- [Implementation plan](docs/implementation-plan.md) - architecture,
  interface tradeoffs, agent boundaries, and Go implementation approach.
- [Tooling references](docs/tooling-references.md) - initial libraries,
  protocol references, and evaluation notes.
- [Agent constitutions](constitutions/README.md) - initial system contracts for
  local specialist agents.

## Initial direction

Prism should be implemented as a shared Go core with two thin interfaces:

1. A Cobra-based CLI for local development, scripting, and early validation.
2. An MCP server adapter for editor and agent integrations once the agent
   contract is stable.

This lets the project prove local Ollama agent behavior quickly without
locking the design to one integration surface.
