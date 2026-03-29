Install an MCP server into local Sapphire configuration by exact name.
Use this immediately after `list_available_mcps` when the server exists in the registry but is not installed yet.
Exact-name install can resolve from the live official MCP registry fallback, not only the local snapshot.
This tool does not connect the server. After installation, call `connect_mcp`.
