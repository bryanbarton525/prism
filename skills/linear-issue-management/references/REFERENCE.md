# Linear MCP reference

Prism treats Linear as an offloaded downstream MCP capability:

- The native `linear` runtime plugin is read-only and reports setup context.
- The generic `mcp` runtime plugin reports configured downstream MCP servers and compact tool inventories.
- The host or CLI should configure an authenticated Linear MCP server under Prism for live Linear reads and writes.
- The Linear agent can use Prism bridge tools during the run when the local model requests them.
- The Linear agent should return proposed Prism `call_mcp_tool` artifacts only when approval, identifiers, or exact tool schemas are missing.

Common operation sequence:

1. `list_mcp_server_tools` through Prism for the `linear` downstream server to confirm exact tool names and schemas.
2. Search teams/projects/issues/labels before creating or editing.
3. Execute approved writes through Prism `call_mcp_tool` against the Linear downstream server.
4. Pass the bounded Linear MCP response back to the specialist if the agent should summarize executed changes.
