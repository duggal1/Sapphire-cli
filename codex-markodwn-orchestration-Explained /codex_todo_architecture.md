# Codex To-Do List (update_plan) Architecture

## Core Files

### `/Users/harshitduggal/Desktop/sapphire-cli/codex/codex-rs/protocol/src/plan_tool.rs`
**Purpose:** Type definitions for the `update_plan` tool (TODO/checklist).

**Key Types:**
- `StepStatus` enum: `Pending`, `InProgress`, `Completed`
- `PlanItemArg`: `{ step: String, status: StepStatus }`
- `UpdatePlanArgs`: `{ explanation: Option<String>, plan: Vec<PlanItemArg> }`

---

### `/Users/harshitduggal/Desktop/sapphire-cli/codex/codex-rs/core/src/tools/handlers/plan.rs`
**Purpose:** Tool handler implementation for `update_plan`.

**Key Components:**
- `PLAN_TOOL` (LazyLock<ToolSpec>): Tool specification with JSON schema
- `PlanHandler`: Implements `ToolHandler` trait
- `handle_update_plan()`: Processes plan updates, sends `EventMsg::PlanUpdate` to session

**Tool Schema:**
```json
{
  "name": "update_plan",
  "description": "Updates the task plan. Provide an optional explanation and a list of plan items...",
  "parameters": {
    "type": "object",
    "properties": {
      "explanation": {"type": "string"},
      "plan": {
        "type": "array",
        "items": {
          "type": "object",
          "properties": {
            "step": {"type": "string"},
            "status": {"type": "string", "enum": ["pending", "in_progress", "completed"]}
          },
          "required": ["step", "status"]
        }
      }
    },
    "required": ["plan"]
  }
}
```

---

### `/Users/harshitduggal/Desktop/sapphire-cli/codex/codex-rs/protocol/src/prompts/base_instructions/default.md`
**Purpose:** System prompt instructing the model on when/how to use `update_plan`.

**Key Prompt Sections (lines 54-75, 267-275):**

> "You have access to an `update_plan` tool which tracks steps and progress and renders them to the user... A good plan should break the task into meaningful, logically ordered steps that are easy to verify as you go."

> "Do not repeat the full contents of the plan after an `update_plan` call — the harness already displays it."

> "The user has asked you to use the plan tool (aka 'TODOs')"

> "## `update_plan` - A tool named `update_plan` is available to you. You can use it to keep an up‑to‑date, step‑by‑step plan for the task. To create a new plan, call `update_plan` with a short list of 1‑sentence steps (no more than 5-7 words each) with a `status` for each step (`pending`, `in_progress`, or `completed`)."

---

### `/Users/harshitduggal/Desktop/sapphire-cli/codex/codex-rs/core/src/tools/spec.rs` (line 2592)
**Purpose:** Tool registration in the global registry.

```rust
builder.register_handler("update_plan", plan_handler);
```

---

## Architecture Flow

```
Model decides to create/update TODO list
         │
         ▼
Model calls `update_plan` tool with JSON args
         │
         ▼
`PlanHandler.handle()` receives ToolInvocation
         │
         ▼
`handle_update_plan()` parses arguments via `parse_update_plan_arguments()`
         │
         ▼
Validates not in Plan mode (ModeKind::Plan)
         │
         ▼
`session.send_event(turn_context, EventMsg::PlanUpdate(args))`
         │
         ▼
UI renders plan as checklist/TODO list
         │
         ▼
Returns "Plan updated" to model
```

---

## Key Constraints

1. **Plan mode exclusion:** `update_plan` is disabled when `ModeKind::Plan` is active
2. **Single in_progress:** At most one step can be `in_progress` at a time
3. **Step length:** Prompts recommend 5-7 words per step
4. **Status values:** Only `pending`, `in_progress`, `completed` are valid

---

## Event Protocol

`EventMsg::PlanUpdate(UpdatePlanArgs)` is sent to the session, which clients render as a TODO checklist UI component.
