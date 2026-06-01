# Tooling references

These references anchor the Prism implementation. Versions and APIs should be
re-checked before major dependency upgrades.

## Go CLI

- Cobra: <https://github.com/spf13/cobra>
  - Use for `prism` command routing, subcommands, flags, help text, and shell
    completion.
  - Keep command handlers thin; they should translate CLI input into core
    `AgentRunner` requests.

## Model Context Protocol

- MCP specification and docs: <https://modelcontextprotocol.io/>
- Go MCP SDK: <https://github.com/modelcontextprotocol/go-sdk>
  - Powers `prism mcp serve` (`internal/mcp/`).
  - MCP tool names and schemas align with CLI JSON result envelopes.
  - Protocol-specific code stays in `internal/mcp/` so SDK churn does not leak
    into the core runtime.

## Local model runtime

- Ollama: <https://ollama.com/>
- Ollama API documentation: <https://github.com/ollama/ollama/blob/main/docs/api.md>
  - Use `http://127.0.0.1:11434` as the default host.
  - Start with health, model listing, and chat/generate calls.
  - Prefer explicit timeouts and context cancellation on every request.

## Runtime plugins

- Kubernetes client-go: <https://github.com/kubernetes/client-go>
  - Powers the built-in `kubernetes` runtime plugin under `internal/plugins/`.
  - Use typed clients for core resources such as namespaces, pods,
    deployments, services, events, and EndpointSlices.
  - Use the dynamic client for optional APIs such as Gateway API HTTPRoutes.
  - Keep plugin calls read-only unless a future agent explicitly declares and
    audits write capabilities.

## Agent Skills

- Agent Skills specification: <https://agentskills.io/specification>
- Frontmatter rules: <https://agentskills.io/specification#frontmatter>
- Anthropic skill-creator reference: <https://github.com/anthropics/skills/blob/main/skills/skill-creator/SKILL.md>
  - Use this as the writing and iteration baseline for `SKILL.md` quality
    (clear trigger descriptions, realistic eval prompts, and iterative refinement).
  - Each skill is a directory with `SKILL.md` (YAML frontmatter + Markdown body).
  - Required fields: `name`, `description`.
  - Prism passes **explicit skill names** on every `run`; the runtime loads only
    those skills and validates them against the agent's `allowed_skills` list.
  - Prefer progressive disclosure: metadata for discovery, full `SKILL.md` only
    when a skill is attached to a run.

## Go standard library

Use the standard library for the first implementation wherever practical:

- `context` for cancellation and deadlines.
- `encoding/json` for request and result envelopes.
- `net/http` for the initial Ollama client.
- `os` and `path/filepath` for config and constitution loading.
- `testing` for unit tests and golden prompt tests.

## Evaluation checklist

Before adding a dependency, confirm that it:

1. solves a concrete problem in the current milestone,
2. has active maintenance and a compatible license,
3. does not force Prism's core runtime to depend on an adapter concern, and
4. can be tested without requiring a running editor integration.
