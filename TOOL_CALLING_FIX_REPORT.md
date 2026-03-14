# Sapphire CLI Tool-Calling Fix Report

## Exact failure cases

```text
ERROR   Todo tool required

Use the todos tool to create and update the task list before proceeding.
```

```text
ERROR   error searching files: context deadline exceeded
```

```text
ERROR   missing required parameter: new_string
```

```text
ERROR   missing required parameter: event_type
```

```text
ERROR   missing required parameter: message
```

## Root causes found

1. Sapphire had a post-generation todo gate in `internal/agent/agent.go` that could delete a completed assistant turn and replay the whole request. Crush does not do this.
2. Sapphire wrapped tools with `InterceptToolCall` in `internal/agent/coordinator.go`, which ran `PrepareToolCall(...)` again before execution and converted middleware failures into text tool results. That made internal validation failures look like model failures and encouraged self-retry loops.
3. `PrepareToolCall(...)` in `internal/agent/tools/tool_call_preflight.go` was too aggressive. It could rewrite tool intent, including converting unread `edit` calls into `view` calls before dispatch.
4. Sapphire added global agent watchdog timeouts in `internal/agent/agent.go`:
   - hard request timeout: 2 minutes
   - idle watchdog: 45 seconds

   Crush does not have those guards. They were a strong candidate for stuck sub-agents, silent cancellation, and incomplete finalization on large tasks.
5. Sapphire’s structured-summary ingestion path in `internal/agent/agent.go` extracted session history back into SQL-backed structured summaries and reinjected them on later turns. That path was fragile and was removed.
6. The grep timeout baseline was too small for large repos: `internal/config/config.go` defaults grep to 5 seconds, and `internal/agent/tools/grep.go` used that directly.

## Divergence from Crush CLI

- Crush todo architecture is advisory, prompt-level, and tool-driven.
- Sapphire added enforcement after generation, plus replay (`ErrTodoToolRequired`) in `internal/agent/coordinator.go` and `internal/agent/agent.go`.
- Crush does not globally wrap every tool with a second repair/validation layer before execution.
- Crush does not install the Sapphire watchdog timeout pair that can cancel long turns mid-flight.

## What changed

### Removed or simplified

- Removed the todo replay loop from `internal/agent/coordinator.go`.
- Removed the post-generation todo enforcement block from `internal/agent/agent.go`.
- Removed the global tool interceptor installation from `internal/agent/coordinator.go`.
- Removed the structured-summary extraction, validation, persistence, and warm-memory reinjection path from `internal/agent/agent.go`.
- Removed the hard 2-minute generation timeout and 45-second idle watchdog from `internal/agent/agent.go`.
- Removed the preflight behavior that silently rewrote unread `edit` calls into `view`.

### Kept, but narrowed

- `PrepareToolCall(...)` is still available through the repair path, but it is no longer forced in front of every tool execution.
- Minimal alias repair was added for the concrete observed failures:
  - `edit`: normalize aliases into `file_path`, `old_string`, `new_string`
  - `save_memory`: normalize aliases into `event_type`, `content`
  - `recall_memory`: normalize `query`, `filter`, `limit`

### Search behavior

- Increased the minimum grep execution timeout in `internal/agent/tools/grep.go` from an effective 5-second floor to 15 seconds.

### Memory behavior

- Disabled session-history summaries in `internal/agent/tools/memory_query.go`.
- Kept useful persistent memory:
  - project constitution
  - codebase knowledge
  - persistent memory pipeline (`internal/memory/...`)

## Why the old validation/preflight logic failed

- It was in the wrong place.
- It mutated calls before real execution.
- It turned internal middleware failures into ordinary tool text results.
- It rewrote tool intent instead of letting the real tool reject or execute.
- It ran outside reliable state ownership, so the model saw a distorted version of what actually failed.

The clearest example was `edit`:

- intended tool: `edit`
- Sapphire preflight could rewrite it to `view`
- the model then received a different tool result than the one it requested
- the turn state drifted

That is orchestration corruption, not model failure.

## Duplicate rendering / retrigger result

The strongest replay path removed was the todo enforcement path:

- assistant turn generates
- Sapphire decides todos were not touched
- assistant message can be deleted
- request is retried
- final output appears to repeat

I did not find a single TUI-only duplicate-render smoking gun stronger than that replay path. The fix was to stop replaying the request after generation.

## Spawn-agent / stuck execution result

The two largest runtime risks removed for spawned agents were:

- hard 2-minute request timeout
- 45-second idle watchdog

Those guards were especially dangerous on large repos or slow tools, because long-running work can be healthy while still producing no stream activity for a while.

## Todo enforcement result

- Todo usage is no longer enforced by replaying or deleting finished turns.
- Sapphire is now closer to Crush here: advisory prompting, not architectural self-sabotage.

## Session-history SQL/DB ingestion result

Removed:

- structured summary extraction from conversation history
- structured summary validation/retry
- structured summary persistence into SQL
- structured summary reinjection into later prompts

Preserved:

- project constitution reads
- codebase knowledge
- persistent memory pipeline in `internal/memory`

## Proof

Targeted verification passed:

```text
go test ./internal/agent/tools ./internal/agent/...
go test ./internal/memory/...
```

Added regression coverage in `internal/agent/tools/tool_call_preflight_test.go` for:

- edit alias repair to prevent `missing required parameter: new_string`
- save-memory alias repair to prevent `missing required parameter: event_type`
- prevention of unread-edit rewrite into `view`

## Brutally honest remaining risk

- `spawn_agent` / sub-agent lifecycle now has fewer false cancellations, but I did not rewrite the entire sub-agent control plane in this pass.
- `tool_intercept.go` still exists in the tree, but it is no longer installed in `buildTools`.
- There may still be provider- or SDK-level retry edge cases outside the removed replay paths.
- The grep change reduces timeout failures; it does not yet implement partial-result streaming or adaptive narrowing.
