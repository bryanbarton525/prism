# The Prism Book

A practical guide to installing Prism, running bounded specialist work, and
extending it. Start with the quick start; follow the later chapters when you
need tool routing, runtime isolation, or contribution guidance.

Prism runs specialist subtasks for an AI editor or MCP host. The host remains
the orchestrator. An offload model runs the specialist; optional embedding and
decision models help find tools within its authorized catalog.

## Contents

| Chapter | What you will learn |
| --- | --- |
| [1. Quick start](quick-start.md) | Build or install Prism, connect a served model, and complete a task |
| [2. Installation and configuration](installation.md) | Host versus runtime scope, model setup, persistent Laya, and upgrades |
| [3. Worked examples](examples.md) | CLI tasks, MCP delegation, explicit tool recommendations, and managed agents |
| [4. Architecture deep dive](architecture.md) | Bundles, task execution, retrieval, choice scoring, and authorization |
| [5. Testing and validation](testing.md) | Deterministic checks, live inference, and complete tool tasks |
| [6. Troubleshooting](troubleshooting.md) | Diagnose endpoint, model, catalog, timeout, and stale-connection problems |
| [7. Contributing](contributing.md) | Develop a change, add coverage, update docs, and prepare a PR |

## Choose a path

- **First installation:** chapters 1–3, then the operator checks in chapter 5.
- **Switching from Kev:** chapter 2's migration section, then chapters 5–6.
- **Understanding the internals:** chapter 4, then chapter 7.
- **Adding an agent or skill:** chapter 3, the authoring guides, and chapter 7.

Commands assume a shell with `prism` on `PATH`; build-from-source commands assume
the repository root. Endpoint URLs and model IDs in templates must match your
own serving deployment. Prism does not provision an Ollama, SGLang, or vLLM
cluster for you.

The detailed references remain [usage](../usage.md), [model runtime](../model-runtime.md),
[tool recommendations](../tool-recommendation.md), [agents](../../agents/README.md),
and [skills](../../skills/README.md).

[Begin: Quick start →](quick-start.md)
