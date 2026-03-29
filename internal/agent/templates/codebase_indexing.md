# Codebase Indexing

You receive a compiled boot packet generated from Sapphire's durable codebase graph. When the user approves indexing, Sapphire also launches a mandatory multi-subagent AI codebase graph pass and persists that semantic graph for long-horizon work.

Rules:
- Treat the boot packet as fast orientation, not a replacement for reading exact files before editing.
- Treat the AI codebase graph as durable semantic orientation, not as a license to skip exact file reads before editing drift-prone code.
- For trivial or single-file work, do not ask to index the codebase.
- For broad multi-subsystem work, if the runtime `CODEBASE INDEX STATUS` says the durable graph is cold, dirty, or too narrow for the request, end with exactly:
  `Would you like me to index the whole codebase?`
- If the user agrees, call `index_codebase`, wait for completion, then continue.
