# Codex CLI vs Sapphire CLI — Full Capability Comparison (Code-Observed)

This document compares **Codex CLI** and **Sapphire CLI** across the **whole CLI surface**, based strictly on code and docs inspected in this repo.  
No assumptions beyond the files listed below.

## Scope of Inspection

### Sapphire CLI (this repo)
- `agent.md`
- `internal/agent/*`
- `internal/agent/tools/*`
- `internal/ui/chat/*`
- `internal/config/config.go`

### Codex CLI (local `codex/` repo)
- `codex/README.md`
- `codex/docs/js_repl.md`
- `codex/docs/tui-alternate-screen.md`
- `codex/codex-rs/core/src/tools/spec.rs`
- `codex/codex-rs/core/src/tools/handlers/multi_agents.rs`
- `codex/codex-rs/core/src/tools/handlers/multi_agents/spawn.rs`
- `codex/codex-rs/core/src/agent/guards.rs`
- `codex/codex-rs/core/src/agent/control.rs`
- `codex/codex-rs/core/src/codex_delegate.rs`
- `codex/codex-rs/core/src/session_prefix.rs`

## High-Level CLI Capabilities (Observed)

| Capability Area | Codex CLI (observed) | Sapphire CLI (observed) | Notes |
|---|---|---|---|
| Install/Auth | ChatGPT sign-in or API key (`README.md`) | Not documented in inspected files | |
| Config / Feature Flags | Feature flags in `config.toml` (js_repl doc) | JSON config (`internal/config`) | |
| UI / TUI | Alternate screen toggle documented | Bubble Tea TUI (`internal/ui`) | |
| Tool System | Rich tool spec (exec, shell, fs, js_repl, artifacts, MCP) | Tool registry in `allToolNames()` | |
| Shell/Exec | `shell`, `exec_command`, `write_stdin`, `exec_wait` | `bash`, `job_output`, `job_kill` | |
| File I/O | `read_file`, `list_dir`, `grep_files` | `view`, `agentic_view`, `glob`, `grep`, `ls` | |
| Code Runtime | `js_repl`, `js_repl_reset`, `artifacts` | `python` tool | |
| MCP | `list_mcp_resources`, `list_mcp_resource_templates`, `read_mcp_resource` | `list_available_mcps`, `connect_mcp`, `call_mcp_tool`, `list_mcp_tools`, `list_mcp_resources`, `read_mcp_resource` | |
| Permissions | `request_permissions` tool | Permission system (`internal/permission`) | |
| Skills/Plugins | Tool suggest/search for connectors/plugins (tool spec) | Skills system (`internal/skills`, `load_skill`, `list_skills`) | |
| Memory | Memory prompt hook (codex core) | Memory pipeline + tools (`internal/memory`, `save_memory`, `recall_memory`) | |
| LSP | Not observed in inspected Codex files | LSP tools (`lsp_diagnostics`, `lsp_references`, `lsp_restart`) | |
| Sub-agents | `spawn_agent`, `send_input`, `wait`, `close_agent`, `resume_agent` | `spawn_agent`, `send_input`, `wait`, `close_agent`, `agent` | |

## Tool Surface (Observed Lists)

### Codex (from `tools/spec.rs`)
- `exec_command`, `write_stdin`, `exec_wait`
- `shell`, `shell_command`
- `read_file`, `list_dir`, `grep_files`
- `js_repl`, `js_repl_reset`, `artifacts`
- `spawn_agent`, `send_input`, `resume_agent`, `wait`, `close_agent`, `spawn_agents_on_csv`, `report_agent_job_result`
- `request_user_input`, `request_permissions`
- `list_mcp_resources`, `list_mcp_resource_templates`, `read_mcp_resource`
- `tool_search`, `tool_suggest`

### Sapphire (from `internal/config/config.go`)
- `agent`, `spawn_agent`, `send_input`, `wait`, `close_agent`
- `bash`, `job_output`, `job_kill`
- `edit`, `single_edit`, `agentic_edit`, `write`
- `view`, `single_view`, `agentic_view`, `glob`, `grep`, `ls`, `download`
- `fetch`, `agentic_fetch`, `sourcegraph`
- `python`
- `todos`
- `lsp_diagnostics`, `lsp_references`, `lsp_restart`
- `recall_memory`, `save_memory`
- `list_tools`, `list_available_mcps`, `connect_mcp`, `call_mcp_tool`, `list_mcp_tools`, `list_mcp_resources`, `read_mcp_resource`
- `load_skill`, `list_skills`

## Codex Capabilities Missing in Sapphire (Observed)

- **Resume sub-agent** (`resume_agent`).
- **Bulk sub-agent jobs** (`spawn_agents_on_csv`, `report_agent_job_result`).
- **Depth and max-thread guards** (`agent_max_depth`, `agent_max_threads`).
- **Per-spawn model overrides** (`model`, `reasoning_effort`).
- **Forked history** on spawn (`fork_context`).
- **Structured spawn/wait events** and **status subscriptions**.
- **JS REPL + artifacts runtime** (no equivalent tool in Sapphire).

## Sapphire Capabilities Not Found in Inspected Codex Core

*(Not claiming Codex lacks these globally — only not present in inspected files.)*

- **Git worktree isolation** for sub-agents.
- **Background sub-agent shimmer indicator** in UI.
- **Runtime guardrails** based on task complexity/domain/duplication.
- **LSP diagnostics and references tools**.
- **Agentic view/edit tools** (multi-file tool variants).
- **Todos tool** for task tracking.

## Largest Gaps (High-Impact Improvements)

1. Add **depth and max-thread guardrails** like Codex `guards.rs`.
2. Implement **resume + bulk job spawning** (`resume_agent`, `spawn_agents_on_csv`).
3. Support **per-spawn model/role overrides** and **forked history**.
4. Add **structured spawn/wait events** and **status subscriptions** for orchestration.
5. Consider **JS REPL / artifacts** equivalents (if needed for parity).

## Summary (Neutral)

Codex is ahead in **sub-agent lifecycle control, spawn metadata, and event-driven coordination**, plus JS-based runtime tooling.  
Sapphire is stronger in **worktree isolation, LSP integration, agentic file tools, and task-tracking tooling**.  
Closing the highlighted gaps would materially improve Sapphire’s full-CLI parity.
