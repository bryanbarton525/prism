# Prism control plane architecture

Prism is now structured as a local-first AI offload control plane. The editor or MCP host remains the orchestrator; Prism owns governed specialist execution underneath it.

## Implemented control-plane layers

- Interface layer: Cobra CLI and MCP stdio tools.
- Agent runner: agent specs, constitutions, skill allowlists, prompt assembly, Ollama calls, normalized results.
- Runtime plugins: native read-only Kubernetes evidence collection.
- Policy: optional YAML governance that can deny or require approval before plugin/model execution.
- Events: optional local SQLite run history with metadata only by default.
- Router: deterministic rule-based route suggestions.
- Bundles: local registry-source commands plus signed registry manifest verification/install.
- Graphs: bounded DAG validation, graph-level policy prechecks, sequential execution through the shared runner, and graph aggregate events.
- Dashboard and reports: local event-store summaries for runs, graph executions, policy denials, plugins, bundles, and validation failures.

## Default safety posture

Existing `prism run` and `prism mcp serve` behavior remains compatible when no policy or event store is configured. Once `--policy-file` is set, Prism fails closed for denied or approval-required decisions. Once `--event-store` is set, runs emit metadata-only events to SQLite.

Prism does not store raw prompts, raw logs, or raw evidence in the event store by default. Runtime evidence may still appear in returned run artifacts because the caller requested a live diagnostic result.

## Proof path

The first full product proof is the Kubernetes incident offload path:

1. `prism route suggest` recommends `kubectl` with Kubernetes skills.
2. Policy allows only read-only Kubernetes plugin usage.
3. The Kubernetes plugin emits bounded text evidence plus a typed evidence pack artifact.
4. The local specialist returns a compact result envelope.
5. Optional event storage records run metadata.
6. The signed `k8s-core-triage` registry fixture proves verified bundle distribution.
7. The dashboard and reports summarize usage and estimated savings.
