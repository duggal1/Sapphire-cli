List the registry-backed MCP inventory and local installation state.
This tool reports built-in MCP support, registry inventory size, configured servers, and connected servers before any matches.
Returns ranked results for a query, including concise server instructions for tool search.
Use this first when a task may need MCP capabilities but no specific server name is known.
If the local inventory misses a query, the tool falls back to the live official MCP registry search.
If a result is not installed, call `install_mcp` with the exact `mcp_name`, then `connect_mcp`.
