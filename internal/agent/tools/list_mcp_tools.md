List connected MCP tools, optionally filtered by server or search query.

Use this tool to inspect the live tool surface after an MCP is connected.
If `mcp_name` is omitted, tools are listed for every connected MCP server.
If `query` is provided, tool names and descriptions are searched across the connected MCP tool surface.

Parameters:
- mcp_name: Optional MCP server name. If omitted, tools are listed for all servers.
- query: Optional search query over tool names and descriptions.
- limit: Optional maximum number of tool entries to return.
