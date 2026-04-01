Locate the exact code file, symbol, or text region needed for the task without reading the whole repository.
Return only the highest-value ranked navigation candidates.

Use this tool when:
- the code location is unknown and you need the repo locator first
- the repo is so large that broad reading would waste context before the task even starts
- you need the shortest path to the exact file, symbol, or code region first
- the user names a feature, integration, subsystem, or product term, but not the file path
- context is stale or incomplete and you need a fast repo locator before `agentic_view`

How to use it:
- give one focused query first, such as `zapier`, `zapier webhook`, `billing portal`, or `OAuth callback`
- if needed, refine only 1-2 more times with better nouns, subsystem names, or path hints
- stop once it returns a small set of strong candidate files or symbols, then switch to `agentic_view` by default
- use `single_view` only if the resulting read is explicitly narrow and trivial
- treat it as a locator shortcut, not as a substitute for reading the located code
- prefer it before older generic browsing loops when the code location is still unknown
- if you have multiple independent unknowns, issue multiple `tool_search` calls in parallel rather than serializing unrelated locator work

Search strategy:
- prefer durable index matches first when the codebase graph exists
- use filename/path matches next to find likely target files fast
- use bounded literal text fallback only if the higher-signal stages are insufficient

Do not use this tool for:
- tool inventory or MCP discovery; use `search_tools` or `tool_suggest`
- broad repository reading; use `agentic_view`
- exact text search when you already know the pattern; use `rg`
- exact filename/path search when you already know the filename shape; use `rg_files`
- indefinite search loops; after a small number of focused queries, read the best candidates

Parameters:
- `query`: natural-language, symbol-name, or filename query
- `path` / `paths`: optional roots to restrict search
- `include`: optional file glob for text fallback
- `limit`: ranked result cap; defaults to 8 and is capped at 20
