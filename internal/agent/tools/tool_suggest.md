Suggest MCP servers when the required capability is not available in the current tool list.

Use this tool when:
- `search_tools` found no relevant tools, and
- the request clearly maps to a known MCP server capability.

Parameters:
- query: The capability you need (e.g., "payments", "database", "auth", "observability").
- limit (optional): Max suggestions to return.

The response lists suggested MCP servers and tells you whether to use `install_mcp` or `connect_mcp`.
