# Runtime extensions alongside an immutable release bundle

Prism needs to import useful skills and agents without making release-defined specialists mutable or restoring the retired independently distributed bundle model. User-reviewed design decisions keep the release bundle immutable and introduce separately tracked runtime extensions with unique identities; Claude Code and Codex definitions are translated deterministically, and imported agents require an explicitly selected local or self-hosted offload target.

User-managed skill bindings and MCP access selections apply to user-added agents. A bundled agent can be copied into a separately named user-owned agent, preserving its starting configuration and constitution. This gives users a customization path while keeping bundled behavior predictable. Runtime extensions and editor-host installation remain distinct lifecycles; configuration changes require a new CLI process or an MCP server restart.

Graphify repository investigation is a first-party release-bundle capability, not a required user extension. Bundle its specialist, query instructions, constitution, and host delegation integration. Keep the upstream execution dependency and workspace knowledge graphs outside the immutable content bundle, with explicit setup and workspace binding. This preserves release-defined behavior without embedding repository data or silently running graph construction or inference during installation.

These boundaries are accepted design intent, not a claim that implementation is complete. See [plan.md](../../plan.md) for implementation and verification work.
