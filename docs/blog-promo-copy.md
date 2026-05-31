## LinkedIn

I built Prism because my AI editor was spending way too much time re-reading context it didn't need.

Repo rules, runbooks, CI output, Kubernetes dumps, skill docs, prior chat history—all of it kept getting shoved into the premium model, bloating the context window and dulling the output.

Prism flips the architecture:

- Keep your AI editor as the orchestrator
- Delegate narrow, evidence-heavy subtasks via MCP
- Run specialized local Ollama agents for the heavy lifting
- Return only compact, high-signal summaries to the main model

The results? In a live coding benchmark, Prism dropped orchestrator input from 6,191 tokens to 363 tokens—a 94.1% reduction—while passing the exact same quality rubric.

The biggest win isn't just saving on token costs. It's keeping the orchestrator focused and your workflow sharp. Prism is local-first, repo-native, and benchmarkable.

What is the most context-heavy task currently slowing down your AI editor?

👇 Full writeups and repo below:

Dev.to: https://dev.to/bryan_barton_ff1cfe5d4490/prism-make-your-ai-editor-delegate-ajo
Medium: https://medium.com/@bryanbarton525/prism-make-your-ai-editor-delegate-5f8ab7174cc6
Repo writeup: https://github.com/bryanbarton525/prism/blob/main/docs/blog-prism-launch.md
Repo: https://github.com/bryanbarton525/prism

## Hacker News

Title:

Show HN: Prism — MCP delegation to local Ollama specialists for AI editors

Text:

I built Prism as a small MCP server + CLI for delegating narrow engineering tasks from your AI editor to local Ollama specialists.

The goal is not to replace the editor or create another autonomous swarm. The goal is to keep the orchestrator model in charge while offloading context-heavy subtasks like CI triage, Kubernetes diagnostics, docs lookup, and small implementation work.

Agents, skills, and constitutions are Markdown files in the repo. Prism returns compact summaries back to the orchestrator.

I included benchmark fixtures. In one live coding benchmark, orchestrator input dropped from 6,191 tokens to 363 tokens (94.1%) while passing the same rubric.

Writeups:
Dev.to: https://dev.to/bryan_barton_ff1cfe5d4490/prism-make-your-ai-editor-delegate-ajo
Medium: https://medium.com/@bryanbarton525/prism-make-your-ai-editor-delegate-5f8ab7174cc6

Repo: https://github.com/bryanbarton525/prism

## Reddit

Title:

I built an MCP tool that lets your AI editor delegate focused tasks to local Ollama agents

Post:

I've been experimenting with a pattern where the AI editor stays in charge, but narrow context-heavy work gets delegated to local specialists.

Prism is an MCP server + CLI that any MCP-compatible editor can call to run local Ollama-backed agents. The agents are repo-defined Markdown specs with optional skills and constitutions.

The use case is stuff like:

- CI/PR triage
- Kubernetes diagnostics
- Argo debugging
- docs lookup
- small code helpers
- splitting frontend tasks into UI/logic/docs

The point is not "autonomous swarm." It is reducing how much irrelevant context the premium model has to read.

In a live todo-app coding benchmark, orchestrator input went from 6,191 tokens to 363 tokens with Prism, while both outputs passed the same rubric.

Writeups:
Dev.to: https://dev.to/bryan_barton_ff1cfe5d4490/prism-make-your-ai-editor-delegate-ajo
Medium: https://medium.com/@bryanbarton525/prism-make-your-ai-editor-delegate-5f8ab7174cc6

Repo: https://github.com/bryanbarton525/prism

Feedback welcome, especially on better benchmark scenarios.

## Short X / Mastodon

Built Prism: an MCP delegation layer for AI editors.

Your editor stays the orchestrator. Local Ollama specialists handle focused, context-heavy subtasks.

Live benchmark: 6,191 → 363 orchestrator input tokens on a coding task (94.1% reduction).

Dev.to: https://dev.to/bryan_barton_ff1cfe5d4490/prism-make-your-ai-editor-delegate-ajo
Medium: https://medium.com/@bryanbarton525/prism-make-your-ai-editor-delegate-5f8ab7174cc6
Repo: https://github.com/bryanbarton525/prism
