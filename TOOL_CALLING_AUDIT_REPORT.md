# Sapphire CLI Tool-Calling Audit Report

## Scope

This pass audited the runtime-relevant Sapphire tool pipeline, not unrelated UI/static files.

Primary active paths inspected:

- `internal/agent/agent.go`
- `internal/agent/runtime_control.go`
- `internal/agent/collab_tools.go`
- `internal/agent/subagent_manager.go`
- `internal/agent/subagent_events_helpers.go`
- `internal/agent/agent_tool.go`
- `internal/agent/agentic_fetch_tool.go`
- `internal/agent/agent_job_tools.go`
- `internal/agent/tools/tool_call_preflight.go`
- `internal/agent/tools/tool_call_validation.go`
- `internal/agent/tools/tool_call_normalize.go`
- `internal/agent/tools/bash.go`
- `internal/agent/tools/background_jobs.go`
- `internal/agent/tools/view.go`
- `internal/agent/tools/edit.go`
- `internal/agent/tools/multiedit.go`
- `internal/agent/tools/fetch.go`
- `internal/agent/tools/fetch_types.go`
- `internal/agent/tools/download.go`
- `internal/agent/tools/grep.go`
- `internal/agent/tools/call_mcp_tool.go`
- `internal/agent/tools/connect_mcp.go`
- `internal/agent/tools/list_mcp_tools.go`
- `internal/agent/tools/list_mcp_resources.go`
- `internal/agent/tools/read_mcp_resource.go`
- `internal/agent/tools/memory_query.go`
- `internal/memory/tools.go`
- `internal/ui/model/ui.go`
- `internal/ui/chat/agent.go`
- `internal/ui/chat/file.go`
- `internal/ui/chat/fetch.go`
- `internal/ui/chat/bash.go`
- `internal/ui/chat/mcp.go`
- `codex/codex-rs/core/src/tools/orchestrator.rs`
- `codex/codex-rs/core/src/agent/control.rs`
- `codex/codex-rs/core/src/tools/handlers/multi_agents.rs`

## Shared Root Causes Found

### 1. Preflight canonicalization was corrupting valid tool calls

This was the biggest remaining cross-tool defect.

Sapphire was "repairing" some tool calls into parameter names the runtime structs do not use:

- `fetch`: canonicalized to `output`, but runtime expects `format`
- `download`: canonicalized to `path`, but runtime expects `file_path`
- `web_search`: canonicalized to `num_results`, but runtime expects `max_results`
- `agentic_fetch`: canonicalized to `query`, but runtime expects `prompt`

That means the repair layer could succeed locally while still leaving fantasy validation or the runtime tool with the wrong payload shape.

Fixed in:

- `internal/agent/tools/tool_call_preflight.go`

### 2. Sapphire carried extra dispatcher and interception code that was no longer on the production path

After tracing all production references, the live runtime path is:

1. fantasy validates the model-emitted tool call
2. Sapphire repairs invalid calls through `RepairToolCall` in `internal/agent/agent.go`
3. `tools.PrepareToolCall(...)` normalizes/repairs the payload
4. fantasy re-validates required fields
5. the typed tool `Run(...)` executes
6. `OnToolCall` / `OnToolResult` in `internal/agent/agent.go` persist and publish updates
7. `internal/ui/model/ui.go` merges those updates into the chat tree

`internal/agent/tools/dispatcher.go`, `internal/agent/tools/fast_dispatcher.go`, `internal/agent/tools/tool_intercept.go`, and `internal/agent/tools/dispatcher_benchmark_test.go` were not referenced anywhere on that production path.

Removed:

- `internal/agent/tools/dispatcher.go`
- `internal/agent/tools/fast_dispatcher.go`
- `internal/agent/tools/tool_intercept.go`
- `internal/agent/tools/dispatcher_benchmark_test.go`

This is the closest Sapphire equivalent to simplifying toward Codex's single orchestrated execution path.

### 3. Validators were stricter than the actual tool runtime

Sapphire had validator behavior that rejected payloads the runtime tool intentionally supports:

- `edit` runtime supports create/delete shapes, but validator required both `old_string` and `new_string`
- `agentic_edit` runtime supports create-first operations, but validator required `old_string`
- `bash` runtime can execute with only `command`, but validator required `description`

This is a design bug, not a model bug.

Fixed in:

- `internal/agent/tools/tool_call_validation.go`

### 4. Typed tool boundaries were still brittle when preflight was bypassed or incomplete

Several tools still assumed upstream repair already normalized aliases and decoded nested JSON:

- fetch/web-search/agentic-fetch/download
- MCP tool invocation

That left direct runtime decoding vulnerable even when the model intent was fine.

Fixed by adding alias-aware `UnmarshalJSON` support in:

- `internal/agent/tools/fetch_types.go`
- `internal/agent/tools/download.go`
- `internal/agent/tools/call_mcp_tool.go`

### 5. Sub-agent completion semantics were previously deadlocking and polling-heavy

Already fixed in the prior pass, but it is part of the same root-cause family:

- sub-agent completion/failure events were published while holding the runner mutex
- wait semantics were polling-based instead of lifecycle-event-based

Fixed in:

- `internal/agent/subagent_manager.go`
- `internal/agent/subagent_events_helpers.go`

### 6. Todo payload normalization was still vulnerable to string payloads

Already fixed in the prior pass:

- string `tasks` payloads now coerce into task arrays instead of failing during typed decode

Fixed in:

- `internal/agent/tools/tool_call_preflight.go`
- `internal/agent/tools/todos.go`

## What Was Changed

### Rewritten or simplified

- Removed dormant dispatcher/interceptor implementations entirely
- Corrected preflight canonical keys for fetch/download/web-search/agentic-fetch
- Added JSON-object coercion for stringified MCP args and job/result payloads
- Reduced the live validator to custom cross-field checks only
- Added tolerant runtime decoders for fetch/download/MCP tools
- Replaced sub-agent wait polling with lifecycle-event waiting
- Simplified child-session UI merging to a single container lookup instead of repeated no-op scans

### Intentionally removed or not extended

- No new fallback middleware layer was added
- No prompt-only patching was used to hide runtime problems
- No "prettier" error surface work was done
- No Crush-style broad fork-specific orchestration layer was copied over for advanced Sapphire tools

## What Was Borrowed From Codex

Main ideas borrowed from `codex-rs`:

- tool boundary should be tolerant, with correctness enforced in one runtime path instead of many shallow guards
- approval/execution/finalization should be centralized and state-aware
- multi-agent completion should be event-driven, not inferred from polling
- child-agent lifecycle should have explicit completion signaling

Applied in Sapphire as:

- alias-tolerant runtime decoding instead of assuming preflight succeeded
- event-driven sub-agent waiting/finalization
- fewer contradictory validation layers

## What Was Not Borrowed From Crush

I did not use Crush as the primary model for this pass.

Reason:

- Sapphire’s remaining failures were concentrated in orchestration, tool normalization, MCP invocation, and sub-agent completion
- Codex is the stronger reference for those systems
- importing Crush-style patterns here would risk adding another compatibility layer instead of removing corruption points

Crush remained relevant for the earlier todo comparison, but not as the main reference for the broader tool runtime.

## Proof / Regression Coverage

Added or extended tests:

- `internal/agent/tools/tool_call_preflight_test.go`
  - fetch alias normalization
  - download alias normalization
  - agentic-fetch alias normalization
  - web-search alias normalization
  - stringified MCP argument decoding
- `internal/agent/tools/tool_call_validation_test.go`
  - edit create/delete validation parity
  - agentic-edit create-operation parity
- `internal/agent/collab_tools_test.go`
  - spawn/send_input alias acceptance
  - event-driven sub-agent wait behavior
- `internal/agent/tools/todos_test.go`
  - string task payload coercion

Executed successfully:

```bash
go test ./internal/agent/tools ./internal/agent/... ./internal/memory/...
```

## Brutally Honest Residual Risk

- I inspected the TUI render/finalization path and did not find an active duplicate-render bug in the current main-session or child-session merge logic. `updateSessionMessage(...)` only appends tool items when the ID is new, and `handleChildSessionMessage(...)` updates nested tools by `ToolCallID` instead of appending blindly.
- The remaining live normalization surface is concentrated in `internal/agent/tools/tool_call_preflight.go`. That file is still large because it carries cross-tool alias and shape repair for the fantasy repair hook. It remains on the active path because fantasy only re-validates required fields, not alias/cross-field semantics.
- Sapphire still has a dedicated repair layer that Codex largely avoids by keeping a tighter orchestrator boundary. The dead branches are gone, but the repair module is still the largest active-path risk surface.

## Bottom Line

The remaining failures were still mostly Sapphire-originated:

- dead parallel-dispatcher branches that obscured the real runtime path
- incorrect repair canonicalization
- validator/runtime contract drift
- brittle typed decoders

This pass fixed those at the shared orchestration boundary instead of treating them as isolated tool mistakes.
