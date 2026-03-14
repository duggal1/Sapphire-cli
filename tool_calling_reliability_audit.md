# Strict Tool-Calling Reliability Audit and Middleware Validation

## Scope (What I Actually Read)
I did **not** read the entire Codex repository or the entire Sapphire codebase. Below is the exact scope I reviewed. Anything outside this list is **unknown** and explicitly **not audited**.

### Codex (inspected)
- `codex/codex-rs/app-server/src/codex_message_processor.rs` (dynamic tool validation)
- `codex/codex-rs/core/src/tools/spec.rs` (tool schema parsing + sanitization)
- `codex/codex-rs/app-server/src/dynamic_tools.rs` (dynamic tool response decoding)

### Sapphire (inspected)
- `internal/agent/prompts.go`
- `internal/agent/templates/coder.md.tpl`
- `internal/agent/templates/templates/coder.md.tpl` (separate template file; usage unclear)
- `internal/agent/coordinator.go` (tool registry + interceptor injection)
- `internal/agent/tools/tool_intercept.go`
- `internal/agent/tools/tool_call_preflight.go`
- `internal/agent/tools/tool_call_normalize.go`
- `internal/agent/tools/tool_call_validation.go`
- `internal/agent/tools/view.go`
- `internal/agent/tools/fast_view.go`
- `internal/agent/tools/edit.go`
- `internal/agent/tools/multiedit.go`
- `internal/agent/tools/write.go`
- `internal/agent/tools/todos.go`
- `internal/agent/tools/bash.go`

If you need a **complete** audit, this scope is insufficient and must be expanded.

---

## Step 1 — Strict Schema Consistency Audit (Prompt vs Runtime)

### Observed model-facing tool instructions
Two separate coder templates exist:
- `internal/agent/templates/coder.md.tpl`
- `internal/agent/templates/templates/coder.md.tpl`

`internal/agent/prompts.go` references `internal/agent/templates/coder.md.tpl`. The other template appears **unused** in the code paths I read. This can create **drift risk** if it is used elsewhere or assumed to be authoritative.

### Runtime tool schemas
Runtime schemas are derived from Go structs and tool wrappers (`fantasy.NewAgentTool` / `fantasy.NewParallelAgentTool`). Observed parameter schemas include:
- `view`, `single_view`, `agentic_view`: `ViewParams` (`file_path`, `file_paths`, aliases, `offset`, `limit`)
- `edit`, `single_edit`: `EditParams` (`file_path`, `old_string`, `new_string`, `replace_all`)
- `agentic_edit`: `MultiEditParams` (supports `file_edits` and legacy shapes)
- `write`: `WriteParams` (`file_path`, `content`)
- `todos`: `TodosParams` (supports `action`, `tasks`, `task`, etc.)
- `bash`: `BashParams` (`description`, `command`, `working_dir`, `run_in_background`)

### Mismatch findings (prompt vs runtime)
1. **Absolute paths requirement**
   - Prompt: “Always use absolute paths for file operations.”
   - Runtime: `filepathext.SmartJoin` accepts relative paths and resolves them against working dir.
   - Status: **Mismatch (prompt stricter than runtime)**. Not necessarily harmful, but inconsistent.

2. **Agentic view file-count guidance**
   - Prompt: “agentic_view (max 10 files per call).”
   - Runtime: `FastViewTool` accepts arbitrarily many paths; only `maxConcurrent` (250) is enforced, not a count limit.
   - Status: **Mismatch (prompt limit not enforced)**.

3. **Template duplication risk**
   - Two different coder templates exist. Only one is wired by `prompts.go`.
   - Status: **Potential mismatch source** if the unused template is treated as canonical.

### No hard evidence of parameter-name mismatch between prompt and runtime
The prompt does not include a structured schema block, so parameter-name mismatches **cannot be directly verified** at the prompt-template level. Instead, the runtime schema is enforced by Go structs + validation logic (see middleware below). This means the **real source of mismatches** is likely between **model behavior** and **runtime expectations**, not between prompt schemas and runtime schemas.

---

## Step 2 — Validate the Middleware Architecture (Model → Tool → Validation → Execution)

### What exists in Sapphire
A deterministic preflight layer **does exist** and is injected in `internal/agent/coordinator.go`:

```
Model Output
   ↓
Tool Request (fantasy.ToolCall)
   ↓
InterceptToolCall (wrapper)
   ↓
PrepareToolCall
   - Normalize tool name
   - Parse JSON (partial/repaired)
   - Repair + normalize params
   - Validate required fields & tool-specific constraints
   - Enforce read-before-edit via filetracker
   ↓
Tool Execution
```

### Evidence
- `internal/agent/coordinator.go` injects `tools.InterceptToolCall(...)` for all tools except `google_search`.
- `internal/agent/tools/tool_intercept.go` calls `PrepareToolCall` before execution.
- `PrepareToolCall` performs normalization, repair, and validation before dispatching to tool logic.

### Middleware behavior (deterministic)
- **Tool name normalization**: aliases like `read_file` → `view`, `list_files` → `ls`.
- **Input parsing**: `schema.ParsePartialJSON` allows partial/invalid JSON and repairs it.
- **Parameter normalization**: key aliasing (`file_path` ← `path`, `file`, etc.).
- **Shape repair**: auto-switch `view` ↔ `agentic_view`, `edit` ↔ `agentic_edit`.
- **Read-before-edit enforcement**: if unread, tool call is converted to view (see below).
- **Hard validation**: required fields enforced via `validateToolCallInput`.

**Conclusion**: A deterministic validation layer **does exist** and is in use.

---

## Step 3 — Strict Parameter Validation for Every Tool

### Implemented validations
- **Required fields**: enforced via `validateRequiredFields` + tool-specific checks.
- **Todos validation**: structural validation (e.g., `task` must be object) and action inference.
- **Edit validation**: error cases for missing/invalid JSON.
- **Write validation**: explicit checks for `file_path` and `content`.
- **Bash validation**: requires `command`, with fallback repair logic to infer from `description`.
- **MCP tool validation**: validates `mcp_name`, `tool_name`, etc.

### Gaps / unknowns
- The validation layer depends on `tool.Info().Required` for generic required fields. I did not inspect the `fantasy` library to confirm how required fields are derived for every tool. So the **full required-field matrix is not verified**.
- If `GetSessionFromContext(ctx)` returns empty, **read-before-edit** enforcement does not trigger (see below).

---

## Step 4 — Enforce Correct Tool Execution Order (Read → Edit)

### What exists
- `repairEditCall` checks for unread file paths via `filetracker` and converts edit calls to `view`/`agentic_view` when unread.
- `write` tool checks last read time and rejects if file changed since last read.

### Gap
- `unreadFilePaths` returns `nil` if `sessionID` is empty. In that case:
  - edits are **not** auto-converted to view,
  - read-before-edit enforcement does **not** happen.

This is a **real gap** if any path allows tool calls with missing session IDs.

---

## Step 5 — Enterprise-Level Reliability Requirements

### What is already strong
- Tool name aliases + schema repair reduce model mismatch.
- Preflight normalization for multiple tools is deterministic.
- Edit-before-read is enforced (when session ID + filetracker are present).
- Dynamic tool input schema validation exists in Codex (see below).

### What is not fully guaranteed
- There is no explicit evidence that **every tool** has full structured validation beyond required fields and select tool-specific logic.
- The prompt itself does not carry a structured schema for tools, so **schema consistency depends on runtime tool registration** rather than template content.

---

## How Codex Handles Tool Schemas (Observed)

### Dynamic tool schema validation
From Codex:
- `validate_dynamic_tools` rejects invalid tool names and unsupported input schemas.
- `parse_tool_input_schema` sanitizes JSON schema before parsing.
- `sanitize_json_schema` coerces missing `type`, inserts missing `properties`, and normalizes invalid forms.

This design makes Codex **more tolerant** of poorly-formed schemas while still enforcing a shape it can execute.

### Dynamic tool response decoding
- Responses are decoded into strict types (`DynamicToolCallResponse`).
- Invalid responses return a **fallback error result**, not a crash.

**Net effect**: Codex applies deterministic schema sanitization and strict validation at the tool boundary.

---

## Verdict: Are the issues resolved?

**No.** Based on this limited audit, there are **clear gaps** and **unverified areas**.

### Confirmed gaps
- Read-before-edit enforcement can be bypassed if `sessionID` is missing.
- Prompt constraints (absolute paths, file-count limits) are not enforced at runtime.
- Two distinct coder templates exist; only one is wired. This is a drift risk.

### Unknowns (not audited)
- Whether any tools outside this list have parameter mismatches.
- Whether MCP tool schemas or custom tools are aligned with prompt expectations.
- Whether provider-level tool schema generation (`fantasy`) ever diverges from runtime expectations.

---

## Key Room for Improvement (Concrete, Based on What I Saw)

1. **Eliminate template ambiguity**
   - Ensure only one coder template exists or explicitly document usage.

2. **Make read-before-edit enforcement unconditional**
   - If `sessionID` is empty, treat as **unread** and convert edits to view anyway.

3. **Enforce prompt constraints at runtime**
   - If prompt mandates absolute paths or file count limits, enforce them in `PrepareToolCall`.

4. **Add full schema cross-check tests**
   - For each tool: compare prompt-facing schema (if any) vs runtime struct schema.
   - This is currently **not implemented** in any file I read.

---

## Final Notes
This is a **partial audit**. I did not read the full Codex repository or Sapphire CLI. If you want a complete, strict, end-to-end validation, we need to expand the scope to:
- All tool definitions in Sapphire (including MCP, subagent, memory, and skill tools).
- The `fantasy` tool schema generation pipeline.
- Any additional prompt templates or tool registries.
- Full Codex tool boundary handling beyond the dynamic tool path.
