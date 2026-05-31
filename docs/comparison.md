# How Prism compares

There is no widely adopted product that matches **Prism’s exact combination** today. Several tools overlap in pieces; the full pattern — **editor orchestrator → MCP → local Ollama specialists with per-run skills, constitutions, and measured token savings** — is still fairly niche.

## What makes Prism different

Prism is a **delegation layer**, not a replacement orchestrator or autonomous swarm:

- The **primary LLM stays in the editor** — it picks the agent, attaches skills, and judges results.
- **Narrow specialists** run on **local Ollama**, invoked via **MCP** (`run_agent`, not pasted prompt bloat).
- **Progressive disclosure** — only attached skills and constitution per call; the orchestrator does not load the full skill library.
- **Structured, compact results** — JSON envelope with summary, findings, artifacts, and confidence.
- **Executable proof** — benchmark suite and monthly projections in `results.yaml` / `scale-profiles.yaml`.

## Closest alternatives

**[Claude Code subagents](https://code.claude.com/docs/en/sub-agents)** — closest *architecture*: isolated specialist context, parent gets a summary, MCP can be scoped to a subagent only. Difference: subagents run **Anthropic models** (Haiku/Sonnet/Opus), not local Ollama — you save context and can use cheaper cloud tiers, but not offline/local compute at $0.

**[Cursor Skills + MCP](https://cursor.com/docs/skills)** — same *host*, different economics: Skills (same [Agent Skills](https://agentskills.io/) standard Prism uses) and MCP tools live in the orchestrator session. Cursor also offers cloud agents for long autonomous work — the opposite of offloading to local specialists. No built-in “run this skill on Ollama and return a compact envelope.”

**CrewAI, LangGraph, AutoGen, OpenAI Agents SDK** — same *delegation idea*, different *orchestrator*: you build the orchestrator app; the editor is not the brain. MCP and local models are optional. Dify, n8n, and Flowise are workflow-first variants of the same pattern.

**Ollama + MCP orchestrators** (community projects) — inverted model: Ollama **is** the orchestrator and MCP servers are tools, not editor-side specialists.

**Continue, Aider, OpenHands, SWE-agent** — coding assistants or autonomous agents with local/cloud models, but not repo-native agent specs, skill allowlists, MCP `run_agent`, or benchmarked orchestrator token reduction.

## Honest landscape summary

| Capability | Prism | Claude subagents | Cursor Skills+MCP | CrewAI/LangGraph |
|------------|-------|------------------|-------------------|------------------|
| Orchestrator stays in editor | ✓ | ✓ | ✓ | ✗ (you build it) |
| Specialist isolation | ✓ | ✓ | partial | ✓ |
| Local Ollama specialists | ✓ | ✗ | ✗ | optional |
| MCP integration | ✓ | ✓ | ✓ | optional |
| Agent Skills standard | ✓ | ✓ | ✓ | varies |
| Token/cost benchmark suite | ✓ | ✗ | ✗ | ✗ |
| Ops specialists (gh/kubectl/argo) | ✓ | DIY | DIY | DIY |

**Bottom line:** nothing mainstream is “Prism under another name.” The closest mental model is **Claude Code subagents + MCP scoping**, but with **Ollama instead of Haiku** and **repo-defined ops/codegen specialists** plus **measured orchestrator savings**. Cursor and Claude are converging on Skills + MCP + subagents; Prism’s bet is combining those primitives with **local execution** and **provable token reduction**.

## Related

- [Usage guide](usage.md)
- [Benchmark at scale](benchmark-scale.md)
- [Implementation plan](implementation-plan.md)
