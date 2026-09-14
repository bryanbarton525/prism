# Graphify query reference

Prism checks the `prism-graphify-mcp-v0.9.61` contract derived from
`Graphify-Labs/graphify` `v0.9.61` before an invocation. Approved tools for
this skill are:

- `query_graph`
- `get_node`
- `get_neighbors`
- `shortest_path`

This skill must verify material graph leads against workspace sources before
reporting conclusions. The upstream `project_path` parameter is intentionally
not available: Prism selects the recorded workspace/index binding instead.

See `GRAPHIFY-RELEASE.json` for the reviewed tag, commit, Python and MCP-extra
requirements, release asset digest, license/notice basis, and explicit
non-goals. Prism does not install Graphify, create Python environments, or
build/refresh indexes.
