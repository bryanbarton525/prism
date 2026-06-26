# Prism Architecture Graph

This is a curated, directed Graphify view of Prism's production architecture.
It complements the exhaustive graph in `graphify-out/`.

Included:

- Production Go code from `cmd/prism`, `internal`, and `pkg`
- Runtime and installation scripts
- Agent specifications and constitutions
- Skill contracts and references
- Core product, architecture, runtime, and usage documentation

Excluded:

- Tests and test fixtures
- `testdata/`
- Generated schemas and images
- Benchmark scenario data and response fixtures
- Blog and marketing material

Cleanup applied:

- Removed inferred `calls` edges, which commonly joined unrelated helpers with
  identical names across packages
- Removed self-loop edges
- Preserved edge direction for caller-to-callee and source-to-target traversal

The graph remains an index, not authoritative proof. Verify inferred semantic
relationships against their cited source files.

## Vocabulary-aware queries

`query_graph.py` expands natural-language questions using only vocabulary and
node anchors present in this graph. Expansion is printed before results so the
mapping remains auditable.

```bash
python3 graphify-architecture-out/query_graph.py \
  "How does run_agent reach a downstream MCP server?"
```

The reviewed mappings live in:

- `query_aliases.json` for graph-vocabulary terms
- `query_anchors.json` for exact graph node IDs

## Regression benchmark

```bash
python3 graphify-architecture-out/query_graph.py --benchmark
```

See `QUERY_BENCHMARK.md` and `query_benchmark_results.json` for current results.
