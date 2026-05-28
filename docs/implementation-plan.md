# Prism implementation plan

## Problem statement

Prism lets a primary LLM orchestrator delegate focused work to local
specialist agents running on Ollama. The orchestrator remains responsible for
task selection, final judgment, and user communication. Prism provides the
local execution layer, agent definitions, prompts, tool references, and
structured results needed for that delegation.

The project should start with planning artifacts and then grow into a Go
application that can be exposed as either:

- a Cobra-based CLI for terminal use and automation, or
- an MCP server for editor and agent integrations.

## Goals

- Reduce paid model token usage by moving narrow subtasks to local models.
- Keep each local agent small, inspectable, and specialized.
- Preserve orchestration by the main LLM instead of building an autonomous
  multi-agent swarm.
- Make local agent behavior reproducible through versioned constitutions,
  prompt templates, model settings, and tool allowlists.
- Support both direct command-line invocation and MCP tool invocation from
  compatible clients.
- Keep the first implementation simple enough to run on a developer laptop.

## Non-goals for the first version

- Replacing the main LLM as the planner or final reviewer.
- Providing unrestricted shell, filesystem, browser, or network access to local
  agents.
- Building a distributed agent platform.
- Supporting every Ollama model uniformly; each agent can declare tested model
  preferences and context limits.
- Implementing long-term memory before short-lived task execution is reliable.

## Design principles

1. **Orchestrator-first**: Prism returns structured evidence and suggestions;
   the calling LLM decides what to do with them.
2. **Narrow specialists**: each agent has a small constitution, clear inputs,
   explicit non-goals, and a bounded output schema.
3. **Local by default**: requests run against a configurable local Ollama host.
4. **Auditable prompts**: all agent prompts and constitutions live in the repo.
5. **Shared core, thin adapters**: CLI and MCP entry points call the same Go
   services instead of duplicating behavior.
6. **Safe capability grants**: tools are opt-in per agent and visible in agent
   metadata.


## Vision guardrails

- Prism is a **delegation layer**, not a replacement orchestrator.
- Prefer **single-agent, single-invocation** calls with explicit skills over
  multi-agent swarms.
- The orchestrator selects the agent **and** which Agent Skills to attach; Prism
  enforces allowlists and loads only that skill set.
- Every agent output must include **confidence and evidence** the orchestrator
  can verify.
- Optimize for **cost predictability, token savings, and debuggability** over
  autonomous breadth.

## Recommended architecture

Build Prism as a Go module with a shared core and two interface adapters.

```text
prism
|-- cmd/prism/                 # Cobra entry point
|-- internal/app/              # application wiring and dependency injection
|-- internal/agent/            # agent specs, registry, prompt assembly
|-- internal/ollama/           # Ollama client and model invocation
|-- internal/session/          # task/session state and transcript handling
|-- internal/result/           # common response schemas and validation
|-- internal/mcp/              # MCP server adapter
|-- internal/cli/              # Cobra command handlers
|-- internal/tooling/          # optional local tools exposed to agents
|-- agents/                    # Markdown + frontmatter agent specs
|-- constitutions/             # initial contracts (migrate into agents/)
|-- skills/                    # Agent Skills directories (SKILL.md)
`-- docs/                      # architecture and planning documents
```

### Core runtime

The core runtime should expose a small service interface used by both CLI and
MCP adapters:

```go
type AgentRunner interface {
    ListAgents(ctx context.Context) ([]AgentSummary, error)
    Run(ctx context.Context, req RunRequest) (RunResult, error)
    GetConstitution(ctx context.Context, agentID string) (Constitution, error)
}
```

The runner is responsible for:

- resolving an agent by ID,
- loading its Markdown+frontmatter spec, constitution, and attached Agent Skills,
- validating requested tools against the agent allowlist,
- assembling the Ollama request,
- enforcing timeout and context-budget limits,
- normalizing the result into a stable response schema, and
- returning enough metadata for the orchestrator to judge usefulness.

### Agent specification (Markdown + frontmatter)

**Decision:** each Prism agent is defined by a single Markdown file with YAML
frontmatter. This is the machine-readable spec and the human-readable contract
in one artifact.

Target layout:

```text
agents/
|-- README.md
|-- researcher.md
|-- planner.md
|-- implementer.md
|-- test-designer.md
`-- reviewer.md

skills/
`-- <skill-name>/SKILL.md    # Agent Skills spec (see skills/README.md)
```

During migration, behavior contracts may still live under `constitutions/` while
agent specs are introduced under `agents/`. The runtime should accept either
until all agents are consolidated.

#### Frontmatter fields (Prism)

Required:

- `id` - stable agent identifier (matches filename stem).
- `name` - display name for orchestrators and CLI output.
- `description` - one-line summary of when to delegate to this agent.
- `model` - default Ollama model tag.
- `context_budget` - maximum prompt/input size for the local model.
- `allowed_skills` - skill `name` values this agent may use (subset of `skills/`).
- `latency_budget_ms` - hard budget for benchmark and runtime warnings.

Recommended:

- `temperature` - conservative default (often low for analysis agents).
- `tools` - explicit allowlist of Prism-local tools (empty in read-only phase).
- `outputs` - short description of the response schema sections expected.
- `constitution_path` - optional path if the Markdown body is not the constitution.

Optional:

- `models` - alternate Ollama tags if the default is unavailable.
- `token_budget` - soft target for assembled prompt size (orchestrator planning).
- `metadata` - arbitrary key-value strings for integrations.

Example `agents/researcher.md`:

```markdown
---
id: researcher
name: Researcher
description: Summarize supplied references and local context for the orchestrator.
model: llama3.1:8b
context_budget: 8192
temperature: 0.2
allowed_skills:
  - repo-skim
  - doc-summary
latency_budget_ms: 45000
tools: []
---

# Researcher constitution

(body: mission, boundaries, output contract, refusal rules)
```

Parsing uses standard YAML frontmatter delimiters (`---`) and a Markdown body.
The Go runtime should validate frontmatter at load time and fail fast on unknown
required fields or skill references.

#### Agent Skills on every invocation

A critical requirement: **each run must attach one or more Agent Skills** so
the local model receives only the procedures relevant to the subtask. This
matches the [Agent Skills frontmatter spec](https://agentskills.io/specification#frontmatter)
and prevents scope creep from loading the entire skill library.

`RunRequest` fields (conceptual):

- `agent_id`
- `task` - user/orchestrator task text or structured input path
- `skill_names` - required list of skill IDs to attach (must be subset of
  `allowed_skills`)
- `format` - `json` or `markdown`

Runtime assembly order:

1. Load agent spec frontmatter and constitution body.
2. Validate `skill_names` against `allowed_skills`.
3. Load each skill's metadata (`name`, `description`) and then the full
   `SKILL.md` body for attached skills only.
4. Build the Ollama prompt with constitution + skills + task input.
5. Return normalized `RunResult` with usage and timing metadata for benchmarks.

The orchestrator (Cursor, Copilot, etc.) is responsible for choosing skills; Prism
is responsible for enforcement and progressive disclosure.

## MCP versus CLI tradeoffs

### Cobra CLI

The CLI is the best first interface for proving the runtime because it is easy
to test, script, and debug without needing an MCP host.

Benefits:

- Simple local installation and invocation.
- Good fit for shell scripts, CI experiments, and manual debugging.
- Easier golden-file testing for prompt assembly and response schemas.
- Clear command surface for inspecting agent metadata.
- Useful even if MCP integration is delayed.

Costs:

- Editor integrations must shell out or use a wrapper.
- Tool discovery is less standardized than MCP.
- Rich interactive sessions require custom conventions.

Initial commands:

```text
prism agent list
prism agent show <agent-id>
prism run <agent-id> --skills repo-skim --input task.md --format json
prism run <agent-id> --stdin --format markdown
prism config doctor
prism mcp serve
```

### MCP server

MCP is the better long-term interface for Cursor, Copilot-like tools, and other
agent clients that support standardized tool discovery.

Benefits:

- Native tool discovery by compatible clients.
- Structured JSON schemas for calls and results.
- Cleaner fit for editor-hosted orchestration.
- Can expose prompts, resources, and tools without teaching each client a
  Prism-specific shell protocol.

Costs:

- Requires an MCP host during integration testing.
- The SDK and protocol evolve faster than basic CLI conventions.
- Streaming, cancellation, and tool metadata need careful compatibility tests.

Initial MCP surface:

- `list_agents`: returns agent IDs, summaries, model hints, and input schemas.
- `run_agent`: invokes one specialist with a bounded task request and required
  `skill_names`.
- `get_constitution`: returns the constitution text for auditability.
- `doctor`: reports Ollama connectivity, model availability, and config state.

### Recommendation

Use a **CLI-first, MCP-ready** implementation:

1. Build the shared `AgentRunner` core.
2. Implement Cobra commands over that core.
3. Add `prism mcp serve` as a thin adapter using the Go MCP SDK.
4. Keep CLI and MCP response schemas identical where possible.

This avoids making MCP protocol details the center of the design while still
supporting the intended editor integration path.

## Agent specialization boundaries

The first local agents should cover tasks that are valuable but bounded. They
should not make repository mutations directly in the first version; instead
they return findings, patches, outlines, or commands for the orchestrator to
review.

| Agent | Primary job | Should avoid |
| --- | --- | --- |
| Researcher | Summarize local docs, fetched references, or pasted context. | Final implementation decisions. |
| Planner | Break a task into steps, risks, and acceptance checks. | Writing code or broad product strategy. |
| Implementer | Produce small, focused code or patch suggestions from supplied context. | Unscoped refactors or direct writes. |
| Test Designer | Propose focused tests, fixtures, and validation commands. | Treating tests as a substitute for review. |
| Reviewer | Find correctness, safety, maintainability, and test gaps. | Style-only feedback unless it blocks clarity. |

The orchestrator should pick one agent at a time and provide only the minimum
context needed for that specialty.

## Prompt and tooling references

Repository layout for prompt assets:

```text
agents/                       # Markdown + frontmatter agent specs (target)
|-- README.md
`-- <agent-id>.md

constitutions/                # initial behavior contracts (migrate into agents/)
|-- README.md
`-- <agent-id>.md

skills/                       # Agent Skills directories (required at run time)
|-- README.md
`-- <skill-name>/SKILL.md
```

Each agent spec body (or linked constitution) should include:

- mission,
- operating boundaries,
- required input assumptions,
- output contract,
- refusal or escalation conditions,
- allowed tools, and
- quality checklist.

Initial external tooling references are summarized in [tooling references](tooling-references.md). Core references:

- **Ollama local API** for model listing and chat/generation requests.
- **Cobra** for command routing, flags, help output, and shell completion.
- **Go MCP SDK** for the MCP server adapter, tool schemas, resources, and
  stdio transport.
- **Go standard library** for config loading, HTTP, JSON, context cancellation,
  and testing unless a dependency clearly pays for itself.

Optional later dependencies:

- A structured logging package once daemon-style MCP operation needs richer
  observability.
- A schema validation package if response contracts become complex.
- A config package if environment variables and a single config file stop being
  sufficient.

## Go implementation approach

### Module setup

Initialize the project as:

```text
module github.com/bryanbarton525/prism
```

Start with a minimal dependency set:

- `github.com/spf13/cobra` for the CLI.
- the selected Go MCP SDK for `prism mcp serve`.

Prefer a small internal Ollama client built on `net/http` for the first pass.
This keeps the runtime contract clear and avoids coupling agent behavior to an
external client abstraction too early.

### Configuration

Configuration should resolve in this order:

1. command flags,
2. environment variables,
3. local config file,
4. defaults.

Suggested environment variables:

- `PRISM_CONFIG`
- `PRISM_OLLAMA_HOST`
- `PRISM_DEFAULT_MODEL`
- `PRISM_AGENT_DIR`

Default Ollama host:

```text
http://127.0.0.1:11434
```

### Result schema

All agents should return a normalized envelope:

```json
{
  "agent_id": "reviewer",
  "model": "llama3.1:8b",
  "status": "ok",
  "summary": "High-level result.",
  "findings": [],
  "artifacts": [],
  "confidence": "medium",
  "usage": {
    "prompt_tokens_estimate": 0,
    "completion_tokens_estimate": 0,
    "duration_ms": 0
  }
}
```

For local models that do not reliably produce strict JSON, the runtime should
store the raw response and mark schema validation errors explicitly instead of
silently repairing the output.

### Testing strategy

- Unit test Markdown + YAML frontmatter parsing for `agents/*.md`.
- Unit test Agent Skills loading and frontmatter validation (`name`,
  `description`, directory name match) per agentskills.io.
- Unit test prompt assembly with golden files, including **skill attachment**
  (only requested skills appear in the prompt).
- Unit test Ollama request construction without requiring a running daemon.
- **Benchmark suite** - executable tests that compare `orchestrator_only` vs
  `prism_delegated` modes and emit token, cost, latency, and pass-rate reports.
  See [success metrics](success-metrics.md).
- Add integration tests behind an opt-in flag for a real local Ollama server.
- Add CLI tests for command parsing, `--skills` flags, and JSON output.
- Add MCP adapter tests around tool registration and request translation.

### Observability

The CLI should support `--verbose` and `--json` output. The MCP server should
log startup state, available agents, Ollama health, request IDs, duration, and
validation failures without logging full user prompts by default.


## Success metrics

Prism is not successful until benchmarks prove **token and cost savings** with
acceptable **pass-rate** and **latency**. See [success metrics](success-metrics.md)
for targets, report format, and CI expectations.

At a minimum, the benchmark suite must demonstrate:

- measurable orchestrator token reduction vs baseline scenarios,
- a cost-vs-time comparison report (orchestrator-only vs Prism-delegated),
- pass-rate on golden task fixtures, and
- compliance with per-agent `latency_budget_ms` values from agent spec frontmatter.

## Milestones

### 1. Repository foundation

- Keep this implementation plan in `docs/`.
- Keep initial constitutions in `constitutions/`.
- Add contribution and development docs when code begins.

### 2. CLI proof of concept

- Initialize the Go module.
- Add Cobra root command.
- Add `agent list`, `agent show`, and `config doctor`.
- Load `agents/*.md` specs (Markdown + frontmatter) and constitutions.
- Validate agent specs and Agent Skills references.
- Add `prism run --skills <name>...` (required skills flag).

### 3. Local model execution

- Add Ollama health check and model listing.
- Implement a single `run` flow for one specialist.
- Add timeout, cancellation, and structured result envelopes.
- Add tests for prompt assembly and request construction.

### 4. MCP adapter

- Add `prism mcp serve`.
- Register `list_agents`, `run_agent`, `get_constitution`, and `doctor`.
- Reuse the same result schema as the CLI.
- Test with a local MCP client configuration.

### 5. Agent hardening

- Tune each initial constitution against real tasks.
- Add per-agent model recommendations.
- Add output validation and retry rules where they improve reliability.
- Document agent and **skill** selection guidance for orchestrators.
- Run the benchmark suite and tune targets in `testdata/benchmarks/thresholds.yaml`.

## Key risks and mitigations

- **Local model quality variance**: keep agent tasks narrow and require
  confidence plus evidence in outputs.
- **Latency**: default to short context, small models, and single-agent calls.
- **Prompt drift**: version constitutions and add golden prompt tests.
- **Unsafe tool use**: require explicit allowlists and keep agents read-only at
  first.
- **MCP compatibility churn**: isolate MCP code in its own adapter package.
- **Schema unreliability**: keep raw output and validation status visible.

## Open decisions

- Which Go MCP SDK should be adopted as the primary dependency after a quick
  spike against current protocol support?
- Which Ollama models should be recommended for each first-party agent?
- Should Prism support streaming output in the first MCP version or defer it
  until non-streaming calls are stable?
- How much local filesystem access should any agent receive after the read-only
  phase?
- How should orchestrators discover available Agent Skills
  (`list_skills` CLI/MCP tool vs static manifest)?
- Should skill validation use `skills-ref` in CI or a Go-native validator?
