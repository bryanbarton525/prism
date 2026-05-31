# Prism Launch Promo Copy

Use these after publishing the full article.

## LinkedIn

I built Prism because my AI editor was spending too much time re-reading context it did not need.

Repo rules, runbooks, CI output, Kubernetes dumps, skill docs, prior chat history — all of it kept getting shoved into the premium model.

Prism changes the shape:

- keep Cursor or any MCP host as the orchestrator
- delegate narrow subtasks through MCP
- run local Ollama specialists for context-heavy work
- return compact summaries to the main model

In a live coding benchmark, Prism reduced orchestrator input from `6,191` tokens to `363` tokens (94.1%) while passing the same quality rubric.

The bigger point is not just cost. It is keeping the orchestrator focused.

Prism is local-first, repo-native, and benchmarkable.

Full writeup: https://github.com/bryanbarton525/prism/blob/main/docs/blog-prism-launch.md
Repo: https://github.com/bryanbarton525/prism

## Hacker News

Title:

Show HN: Prism — MCP delegation to local Ollama specialists for AI editors

Text:

I built Prism as a small MCP server + CLI for delegating narrow engineering tasks from Cursor or another MCP host to local Ollama specialists.

The goal is not to replace the AI editor or create another autonomous swarm. The goal is to keep the editor model as the orchestrator while offloading context-heavy subtasks like CI triage, Kubernetes diagnostics, docs lookup, and small implementation work.

Agents, skills, and constitutions are Markdown files in the repo. Prism returns compact summaries back to the orchestrator.

I included benchmark fixtures. In one live coding benchmark, orchestrator input dropped from 6,191 tokens to 363 tokens while passing the same rubric.

Repo: https://github.com/bryanbarton525/prism

## Reddit

Title:

I built an MCP tool that lets Cursor delegate focused tasks to local Ollama agents

Post:

I’ve been experimenting with a pattern where the AI editor stays in charge, but narrow context-heavy work gets delegated to local specialists.

Prism is an MCP server + CLI that lets Cursor or another MCP host call local Ollama-backed agents. The agents are repo-defined Markdown specs with optional skills and constitutions.

The use case is stuff like:

- CI/PR triage
- Kubernetes diagnostics
- Argo debugging
- docs lookup
- small code helpers
- splitting frontend tasks into UI/logic/docs

The point is not “autonomous swarm.” It is reducing how much irrelevant context the premium model has to read.

In a live todo-app coding benchmark, orchestrator input went from 6,191 tokens to 363 tokens with Prism, while both outputs passed the same rubric.

Repo/writeup: https://github.com/bryanbarton525/prism

Feedback welcome, especially on better benchmark scenarios.

## Short X / Mastodon

Built Prism: an MCP delegation layer for AI editors.

Cursor stays the orchestrator. Local Ollama specialists handle focused, context-heavy subtasks.

Live benchmark: 6,191 -> 363 orchestrator input tokens on a coding task (94.1% reduction).

Repo/writeup: https://github.com/bryanbarton525/prism
