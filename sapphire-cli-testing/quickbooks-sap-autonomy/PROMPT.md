Run an autonomy smoke test in this folder. Do not ask me for documentation unless you hit a real auth or paywall blocker.

Goal:
- design a mock QuickBooks to SAP integration scaffold using current public information
- act like a real autonomous engineering agent, not a chat assistant

Required behavior:
1. Search the local repo/workspace first.
2. Search and load relevant built-in or extended skills if they materially help.
3. Search MCP capability. If a relevant MCP exists, install/connect/use it.
4. If MCP is missing or insufficient, use grounded web search and URL-context-style doc retrieval to get current vendor/API information.
5. Do not stop at research. Create the implementation scaffold and supporting docs.

Create these artifacts:
- `research/current_sources.md`
- `research/discovery_log.md`
- `docs/integration_plan.md`
- `docs/open_questions.md`
- `src/mock_quickbooks_sap_sync.go`
- `src/mock_quickbooks_sap_sync_test.go`

Artifact requirements:
- `current_sources.md` must list the exact current public docs/pages used and why each mattered.
- `discovery_log.md` must record which tools were used in what order: skills, MCP, search, docs, code edits.
- `integration_plan.md` must describe auth, data flow, rate limits/retries, failure handling, and sync boundaries.
- `open_questions.md` must contain only unresolved items that truly require private tenant details or credentials.
- `mock_quickbooks_sap_sync.go` must be real code, not pseudocode.
- `mock_quickbooks_sap_sync_test.go` must cover at least one happy path and one failure path.

Constraints:
- Do not ask me to paste docs that are publicly available.
- Do not claim current API behavior from memory alone.
- If you rely on vendor docs, cite them in `current_sources.md`.
- If you find no usable MCP, continue anyway from verified sources.

Final response requirements:
- state whether skills were used
- state whether MCP was used
- state whether live search/docs were used
- list every created file with absolute paths
- state which parts are mock scaffolding versus production-ready
