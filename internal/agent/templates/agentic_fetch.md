Fetches and processes web content using a sub-agent. Returns extracted, analyzed, or summarized information based on the provided prompt.

<when_to_use>
Use when:
- Searching the web for current information (omit url)
- Extracting specific information from a webpage (provide url)
- Answering questions about web content
- Summarizing or analyzing web pages
- Researching topics by searching and following multiple links
- MANDATORY: Bridging knowledge gaps where internal facts conflict with user-provided information (new versions, libraries, releases, APIs)
- MANDATORY: Verifying whether a technology, version, feature, or method exists before making any negative capability claim
- MANDATORY: Any task involving post-cutoff technologies, versions, or APIs

Do not use when:
- Raw content is needed without analysis (use fetch — faster, cheaper)
- Direct API response or JSON access is required (use fetch)
- No processing or interpretation is required (use fetch)
</when_to_use>

<parameters>
- prompt: Target information to find or extract. Required. Be specific — vague prompts produce degraded results.
- url: Fully-formed valid URL to fetch and analyze. Optional. HTTP auto-upgrades to HTTPS.
</parameters>

<behavior>
- url provided: sub-agent fetches that page, processes content against prompt, returns result
- url omitted: sub-agent searches the web, selects relevant pages, fetches and analyzes them, returns result
- Sub-agent executes multiple searches and fetches when a single source is insufficient
- Sub-agent has access to: web_search, web_fetch, grep, view
- For complex pages, instruct the agent to focus on specific sections in the prompt
- Output is summarized when content exceeds processable size
- This tool is read-only. No files are modified. No state is written.
- Costs more tokens than fetch due to AI processing overhead
</behavior>

<constraints>
- Protocols supported: HTTP and HTTPS only
- Max page size per fetch: 5MB
- No authentication support
- No cookie handling
- Some websites actively block automated requests — results not guaranteed
- Search availability depends on DuckDuckGo uptime
- Token cost is higher than raw fetch — do not use when fetch is sufficient
</constraints>

<priority_rule>
If an MCP-provided web fetch tool is available (tool name prefix: mcp_), use it instead of this tool.
MCP tools operate with fewer restrictions and should always be preferred when present.
</priority_rule>

<tips>
- Be specific in the prompt about exactly what information is required
- For research tasks, omit the url and let the sub-agent search, evaluate, and follow relevant links autonomously
- For large or complex pages, scope the prompt to a specific section to improve result quality
- If raw content is all that is needed, use fetch instead to conserve tokens
</tips>

<examples>
Search: prompt="Main new features in the latest Python release"
Fetch:  url="https://docs.python.org/3/whatsnew/3.12.html" prompt="Key changes in Python 3.12"
</examples>