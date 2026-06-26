# Architecture Query Benchmark

The vocabulary-aware query layer is evaluated against 15 architecture questions
covering Prism's principal runtime and control-plane flows.

Current result:

- Useful expected-file hit: 15/15
- Every expected file matched: 15/15
- Directed graph: 1,218 nodes and 2,040 edges
- No inferred `calls` edges

The benchmark covers:

- MCP startup and tool registration
- Model-runtime configuration
- Downstream MCP invocation
- Policy enforcement
- Graph retries
- Bundle verification and installation
- Registry synchronization
- Skill discovery
- Event persistence and export
- Report generation
- Workspace-root resolution
- Runtime plugins
- Route suggestion
- Structured evidence handoff
- Dashboard event access

Run it with:

```bash
python3 graphify-architecture-out/query_graph.py --benchmark
```

Query the graph with audited vocabulary expansion:

```bash
python3 graphify-architecture-out/query_graph.py \
  "Trace a remote MCP tool invocation from handler to client connection"
```

The benchmark is a regression suite for this repository's known architecture
questions. It does not prove that arbitrary natural-language questions will
resolve correctly. New failed paraphrases should become reviewed aliases only
after confirming their target symbols against the source.
