# Codex (codex-rs) Sub-Agent System Architecture

## Overview

The Codex sub-agent system enables spawning, coordinating, and managing multiple AI agent threads within a user session. Sub-agents are full Codex agent instances that run as child threads, inheriting configuration from their parent while maintaining isolation. The system supports hierarchical spawning with depth limits, status tracking, inter-agent communication, and automatic completion notification.

---

## File Inventory

### `/protocol/src/protocol.rs`

**Purpose:** Core protocol definitions for sub-agent source tracking and collaboration events.

**Role:** Defines the `SessionSource` and `SubAgentSource` enums that identify how a session was created, plus collaboration event types for UI and API surfaces.

**Key Components:**
- `SessionSource::SubAgent(SubAgentSource)`: Enum variant identifying sub-agent sessions
- `SubAgentSource` enum with variants:
  - `Review`: Guardian review sub-agent
  - `Compact`: Session compaction sub-agent
  - `ThreadSpawn { parent_thread_id, depth, agent_nickname, agent_role }`: User-spawned collaboration agent
  - `MemoryConsolidation`: Memory phase-2 consolidation sub-agent
  - `Other(String)`: Custom sub-agent types (e.g., "guardian")
- `CollabAgentSpawnBeginEvent` / `CollabAgentSpawnEndEvent`: Tool call lifecycle events
- `CollabAgentInteractionBeginEvent` / `CollabAgentInteractionEndEvent`: send_input/resume_agent events
- `CollabWaitingBeginEvent` / `CollabWaitingEndEvent`: wait tool events
- `CollabCloseBeginEvent` / `CollabCloseEndEvent`: close_agent events
- `CollabAgentRef`: Agent reference with thread_id, nickname, role
- `CollabAgentStatusEntry`: Status entry with thread_id, nickname, role, status
- `SessionMeta.agent_nickname` / `SessionMeta.agent_role`: Optional metadata fields

**Important Logic:**
- `SessionSource::get_nickname()`: Extracts nickname from `ThreadSpawn` or returns "Morpheus" for memory consolidation
- `SessionSource::get_agent_role()`: Extracts role from `ThreadSpawn` or returns "memory builder" for memory consolidation
- `SubAgentSource::Display`: Formats as "review", "compact", "memory_consolidation", "thread_spawn_{id}_d{depth}", or custom label

---

### `/core/src/agent/control.rs`

**Purpose:** Central control-plane for multi-agent operations.

**Role:** `AgentControl` provides the API for spawning, messaging, and managing sub-agent threads. Shared across all sessions in a user session to maintain unified guard state.

**Key Components:**
- `AgentControl`: Control handle with `Weak<ThreadManagerState>` and `Arc<Guards>`
- `SpawnAgentOptions`: Options including `fork_parent_spawn_call_id` for context forking
- `AGENT_NAMES`: Built-in nickname list from `agent_names.txt`

**Important Logic:**
- `spawn_agent_with_options()`: Main spawn entry point that:
  - Reserves spawn slot via guards
  - Assigns unique agent nickname from role-specific candidates
  - Creates thread via `ThreadManagerState::spawn_new_thread_with_source()` or `fork_thread_with_source()`
  - Sends initial input and starts completion watcher
- `resume_agent_from_rollout()`: Rehydrates closed agent from persisted rollout file
- `send_input()`: Submits `Op::UserInput` to existing agent
- `interrupt_agent()`: Submits `Op::Interrupt`
- `shutdown_agent()`: Submits `Op::Shutdown` and releases from guards
- `subscribe_status()`: Returns `watch::Receiver<AgentStatus>` for real-time updates
- `maybe_start_completion_watcher()`: Spawns detached task that injects completion notification into parent thread
- `format_environment_context_subagents()`: Builds context line listing active child agents
- `inherited_shell_snapshot_for_source()`: Passes parent's shell snapshot to child for continuity

---

### `/core/src/agent/guards.rs`

**Purpose:** Enforces sub-agent spawning limits and nickname uniqueness.

**Role:** `Guards` tracks active agent count and assigned nicknames per user session.

**Key Components:**
- `Guards`: Contains `Mutex<ActiveAgents>` and `AtomicUsize` total count
- `ActiveAgents`: Tracks `threads_set`, `thread_agent_nicknames`, `used_agent_nicknames`, `nickname_reset_count`
- `SpawnReservation`: RAII guard for spawn slot with optional reserved nickname

**Important Logic:**
- `reserve_spawn_slot(max_threads)`: Atomically increments count if under limit
- `release_spawned_thread(thread_id)`: Decrements count and clears nickname
- `reserve_agent_nickname(names, preferred)`: Selects unique nickname, handles pool exhaustion by resetting and adding ordinal suffix ("the 2nd", "the 3rd")
- `format_agent_nickname(name, reset_count)`: Appends ordinal suffix when pool exhausted
- `session_depth()`: Extracts depth from `SessionSource::SubAgent::ThreadSpawn`
- `next_thread_spawn_depth()`: Returns `depth + 1`
- `exceeds_thread_spawn_depth_limit(depth, max_depth)`: Checks `depth > max_depth`

---

### `/core/src/agent/status.rs`

**Purpose:** Agent status tracking utilities.

**Role:** Converts events to status values and identifies terminal states.

**Key Logic:**
- `agent_status_from_event(EventMsg)`: Maps events to `AgentStatus`:
  - `TurnStarted` → `Running`
  - `TurnComplete` → `Completed(last_agent_message)`
  - `TurnAborted` → `Errored(reason)`
  - `Error` → `Errored(message)`
  - `ShutdownComplete` → `Shutdown`
- `is_final(status)`: Returns true for non-PendingInit/non-Running states

---

### `/core/src/agent/mod.rs`

**Purpose:** Agent module public exports.

**Role:** Re-exports `AgentControl`, `AgentStatus`, and guard utilities.

---

### `/core/src/tools/handlers/multi_agents.rs`

**Purpose:** Collaboration tool handler implementations.

**Role:** Translates model tool calls into `AgentControl` operations. Exports five tool handlers.

**Key Components:**
- `MIN_WAIT_TIMEOUT_MS` (10s), `DEFAULT_WAIT_TIMEOUT_MS` (30s), `MAX_WAIT_TIMEOUT_MS` (1 hour)
- `parse_collab_input(message, items)`: Validates either message or items provided, not both
- `input_preview(items)`: Creates text preview for event logging
- `build_agent_spawn_config()`: Builds child config from parent's effective config
- `build_agent_resume_config()`: Builds config for resumed agent
- `build_agent_shared_config()`: Copies runtime turn state (model, provider, reasoning, approval, sandbox, cwd)
- `apply_spawn_agent_runtime_overrides()`: Applies live turn policy to child config
- `apply_spawn_agent_overrides()`: Disables `SpawnCsv` and `Collab` features at max depth
- `apply_requested_spawn_agent_model_overrides()`: Validates and applies requested model/reasoning

**Tool Handlers:**
- `SpawnAgentHandler`: `spawn_agent` tool
- `ResumeAgentHandler`: `resume_agent` tool
- `SendInputHandler`: `send_input` tool
- `WaitHandler`: `wait` tool
- `CloseAgentHandler`: `close_agent` tool

---

### `/core/src/tools/handlers/multi_agents/spawn.rs`

**Purpose:** `spawn_agent` tool implementation.

**Role:** Handles model requests to spawn new sub-agents.

**Arguments:**
- `message`: Initial task text
- `items`: Structured input items (alternative to message)
- `agent_type`: Role name for config override
- `model`: Optional model override
- `reasoning_effort`: Optional reasoning effort
- `fork_context`: Boolean to copy parent context

**Important Logic:**
- Checks depth limit before spawning
- Sends `CollabAgentSpawnBeginEvent`
- Builds config with role overrides and runtime state
- Calls `agent_control.spawn_agent_with_options()`
- Sends `CollabAgentSpawnEndEvent` with result
- Records telemetry counter `codex.multi_agent.spawn`

**Returns:** `{ agent_id, nickname }`

---

### `/core/src/tools/handlers/multi_agents/wait.rs`

**Purpose:** `wait` tool implementation.

**Role:** Blocks until specified sub-agents reach final status or timeout.

**Arguments:**
- `ids`: List of agent thread IDs
- `timeout_ms`: Optional timeout (clamped to 10s-1h, default 30s)

**Important Logic:**
- Sends `CollabWaitingBeginEvent`
- Subscribes to status watch channels for each agent
- If any agent already final, returns immediately
- Otherwise uses `FuturesUnordered` with `timeout_at` deadline
- After first completion, drains remaining futures without blocking
- Sends `CollabWaitingEndEvent` with status snapshots
- Returns `{ status: HashMap<ThreadId, AgentStatus>, timed_out: bool }`

**Wait Implementation:**
```rust
async fn wait_for_final_status(session, thread_id, mut status_rx):
  loop:
    if status_rx.changed().await.is_err():
      status = agent_control.get_status(thread_id)
      return is_final(status) ? Some((thread_id, status)) : None
    status = status_rx.borrow().clone()
    if is_final(status):
      return Some((thread_id, status))
```

---

### `/core/src/tools/handlers/multi_agents/resume_agent.rs`

**Purpose:** `resume_agent` tool implementation.

**Role:** Reattaches to closed sub-agent sessions.

**Arguments:**
- `id`: Agent thread ID

**Important Logic:**
- Checks depth limit
- Sends `CollabResumeBeginEvent`
- If agent not found, calls `try_resume_closed_agent()`:
  - Builds resume config with `build_agent_resume_config()`
  - Calls `agent_control.resume_agent_from_rollout()`
  - Rehydrates nickname/role from SQLite metadata
- Sends `CollabResumeEndEvent` with final status
- Records telemetry counter `codex.multi_agent.resume`

**Returns:** `{ status: AgentStatus }`

---

### `/core/src/tools/handlers/multi_agents/send_input.rs`

**Purpose:** `send_input` tool implementation.

**Role:** Sends follow-up tasks to running sub-agents.

**Arguments:**
- `id`: Agent thread ID
- `message`: Follow-up text
- `items`: Structured input items
- `interrupt`: Boolean to interrupt current task first

**Important Logic:**
- If `interrupt=true`, calls `agent_control.interrupt_agent()`
- Sends `CollabAgentInteractionBeginEvent`
- Calls `agent_control.send_input()`
- Sends `CollabAgentInteractionEndEvent` with status

**Returns:** `{ submission_id }`

---

### `/core/src/tools/handlers/multi_agents/close_agent.rs`

**Purpose:** `close_agent` tool implementation.

**Role:** Terminates and cleans up sub-agents.

**Arguments:**
- `id`: Agent thread ID

**Important Logic:**
- Subscribes to status channel
- Sends `CollabCloseBeginEvent`
- If not already `Shutdown`, calls `agent_control.shutdown_agent()`
- Sends `CollabCloseEndEvent`

**Returns:** `{ status: AgentStatus }`

---

### `/core/src/guardian.rs`

**Purpose:** Guardian approval review sub-agent.

**Role:** Runs a locked-down sub-agent to assess `on-request` approval risks automatically.

**Key Components:**
- `GUARDIAN_SUBAGENT_NAME = "guardian"`
- `GUARDIAN_PREFERRED_MODEL = "gpt-5.4"`
- `GUARDIAN_REVIEW_TIMEOUT = 90 seconds`
- `GUARDIAN_APPROVAL_RISK_THRESHOLD = 80` (fail closed at or above)
- `GuardianAssessment`: `{ risk_level, risk_score, rationale, evidence }`

**Important Logic:**
- `routes_approval_to_guardian(turn)`: Returns true if `OnRequest` + `Feature::GuardianApproval` enabled
- `is_guardian_subagent_source(source)`: Checks if source is `SubAgent::Other("guardian")`
- `run_guardian_review()`: Main entry point that:
  - Builds compact transcript (user messages + bounded recent context)
  - Formats planned action JSON
  - Runs guardian sub-agent with JSON schema constraint
  - Fails closed on timeout/error
  - Emits warning event with verdict summary
- `run_guardian_subagent()`: Spawns guardian with:
  - `approval_policy = Never`
  - Read-only sandbox
  - Disabled nonessential features
  - Source = `SubAgentSource::Other("guardian")`
  - Inherits parent's network approval allowlist
- Guardian does NOT inherit exec-policy rules from parent

---

### `/core/src/memories/phase2.rs`

**Purpose:** Memory consolidation sub-agent (phase 2).

**Role:** Runs background sub-agent to consolidate raw memories into refined artifacts.

**Important Logic:**
- `run()`: Main entry point that:
  - Claims job from state DB
  - Queries stage-1 outputs
  - Syncs filesystem artifacts
  - Spawns consolidation sub-agent with `SessionSource::SubAgent(MemoryConsolidation)`
  - Starts detached handler to monitor completion
- Agent config:
  - `approval_policy = Never`
  - `Feature::SpawnCsv` and `Feature::Collab` disabled
  - Sandbox: `WorkspaceWrite` limited to codex_home
  - Model: configured `consolidation_model` or default
- Handler loops until final status, sends heartbeats to maintain job lease
- Auto-closes agent on completion

---

### `/core/src/memories/prompts.rs`

**Purpose:** Consolidation sub-agent prompt builder.

**Role:** Generates the task prompt for memory consolidation agents.

---

### `/core/src/memories/start.rs`

**Purpose:** Memory system startup checks.

**Role:** Prevents recursive delegation from internal sub-agents.

**Important Logic:**
- `should_skip_memory_phase2(source)`: Returns true if source is `SubAgent(_)`

---

### `/core/src/session_prefix.rs`

**Purpose:** Session context formatting helpers.

**Role:** Formats sub-agent notifications and context lines for model visibility.

**Important Logic:**
- `format_subagent_notification_message(agent_id, status)`: Wraps JSON payload in `<subagent_notification>` tags
- `format_subagent_context_line(agent_id, agent_nickname)`: Formats as "- {agent_id}: {nickname}" or "- {agent_id}"

---

### `/core/src/contextual_user_message.rs`

**Purpose:** Contextual message fragment definitions.

**Role:** Defines `SUBAGENT_NOTIFICATION_FRAGMENT` for model-visible notifications.

**Key Constants:**
- `SUBAGENT_NOTIFICATION_OPEN_TAG = "<subagent_notification>"`
- `SUBAGENT_NOTIFICATION_CLOSE_TAG = "</subagent_notification>"`
- `SUBAGENT_NOTIFICATION_FRAGMENT`: Fragment definition

---

### `/state/src/model/thread_metadata.rs`

**Purpose:** Thread metadata schema.

**Role:** Defines database schema for thread listing, including sub-agent fields.

**Key Fields:**
- `ThreadMetadata.agent_nickname`: Optional nickname
- `ThreadMetadata.agent_role`: Optional role
- `ThreadMetadata.source`: Stringified `SessionSource`
- `ThreadMetadataBuilder`: Builder pattern for constructing metadata

---

### `/tui/src/app.rs`

**Purpose:** TUI application state and event handling.

**Role:** Manages sub-agent navigation, picker UI, and thread switching.

**Key Components:**
- `agent_navigation`: Cached picker metadata
- `select_agent_thread()`: Switches active thread to sub-agent
- `upsert_agent_picker_thread()`: Updates cached nickname/role/closed state
- `mark_agent_picker_thread_closed()`: Marks thread closed without removing

**UI Elements:**
- Subagent picker view with title "Subagents"
- Shows thread ID as description, nickname as name
- Supports filtering by closed status

---

### `/tui/src/chatwidget.rs`

**Purpose:** Chat widget UI component.

**Role:** Handles multi-agent enablement dialog and token tracking.

**Key Constants:**
- `MULTI_AGENT_ENABLE_TITLE = "Enable subagents?"`
- `MULTI_AGENT_ENABLE_NOTICE = "Subagents will be enabled in the next session."`

**Important Logic:**
- Shows selection dialog when collab feature disabled
- Options: "Enable for future sessions" or "Keep subagents disabled"

---

### `/codex-api/src/requests/headers.rs`

**Purpose:** HTTP header builder for API requests.

**Role:** Adds sub-agent identification headers to upstream requests.

**Important Logic:**
- `subagent_header(source)`: Returns header value based on sub-agent type:
  - `Review` → "review"
  - `Compact` → "compact"
  - `MemoryConsolidation` → "memory_consolidation"
  - `ThreadSpawn` → "collab_spawn"
  - `Other(label)` → label
- Header name: `x-openai-subagent`

---

### `/app-server/src/filters.rs`

**Purpose:** Thread listing filter logic.

**Role:** Computes source filters for thread list queries.

**Key Components:**
- `ThreadSourceKind`: Enum with `SubAgent`, `SubAgentReview`, `SubAgentCompact`, `SubAgentThreadSpawn`, `SubAgentOther`

**Important Logic:**
- `compute_source_filters()`: Returns (allowed_sources, post_filter_kinds)
  - Sub-agent variants require post-filtering (not expressible in SQL)
- `source_kind_matches()`: Matches concrete `SessionSource` against filter kinds
- Distinguishes between sub-agent variants (review vs compact vs thread_spawn vs other)

---

### `/app-server-protocol/src/protocol/v2.rs`

**Purpose:** App-server protocol v2 definitions.

**Role:** Mirrors protocol definitions for API surface.

**Key Fields:**
- `ThreadMetadata.agent_nickname`
- `ThreadMetadata.agent_role`

---

## Architecture Overview

### Sub-Agent Creation Flow

```
Model calls spawn_agent tool
         ↓
SpawnAgentHandler.handle()
         ↓
Check depth limit: next_thread_spawn_depth(parent_source) <= agent_max_depth
         ↓
Send CollabAgentSpawnBeginEvent
         ↓
build_agent_spawn_config(parent_turn)
  - Clone parent's effective config
  - Apply runtime overrides (model, provider, reasoning, approval, sandbox, cwd)
  - Apply role-specific config overrides
  - Disable collab/spawn_csv at max depth
         ↓
Guards.reserve_spawn_slot(agent_max_threads)
  - Atomically increment count
  - Reserve unique nickname from role candidates
         ↓
ThreadManagerState.spawn_new_thread_with_source(config, agent_control, SessionSource)
  - If fork_context=true: fork_thread_with_source() copies parent rollout
  - Creates new CodexThread with session_source = SubAgent(ThreadSpawn {...})
         ↓
Guards.register_spawned_thread(thread_id, nickname)
         ↓
AgentControl.send_input(thread_id, initial_items)
         ↓
maybe_start_completion_watcher(child_thread_id, session_source)
  - Spawns detached task that:
    1. Subscribes to child status channel
    2. Waits until is_final(status)
    3. Injects <subagent_notification> message into parent thread
         ↓
Send CollabAgentSpawnEndEvent
         ↓
Return { agent_id, nickname }
```

### Sub-Agent Lifecycle

```
PendingInit → Running ↔ Running (multiple turns) → Completed/Errored/Shutdown
                                              ↓
                              Completion watcher injects notification
                                              ↓
                              Parent receives <subagent_notification>
                                              ↓
                              User/model can call resume_agent or close_agent
```

### Depth Tracking

Depth is encoded in `SessionSource::SubAgent::ThreadSpawn { depth, ... }`:

```
CLI session (depth=0)
  └─ spawn_agent → ThreadSpawn { depth=1 }
       └─ spawn_agent → ThreadSpawn { depth=2 }
            └─ spawn_agent → ThreadSpawn { depth=3 }
                 ...
                 └─ At depth >= agent_max_depth: spawn rejected
```

### Nickname Assignment

```
1. Resolve role config for nickname candidates
2. Filter out already-used nicknames in session
3. If pool exhausted:
   - Clear used set
   - Increment nickname_reset_count
   - Append ordinal suffix ("the 2nd", "the 3rd", etc.)
4. Reserve selected nickname in Guards
5. Store in SessionSource.ThreadSpawn.agent_nickname
6. Release on shutdown
```

### Status Subscription Pattern

```rust
// Subscribe to status updates
let mut status_rx = agent_control.subscribe_status(thread_id).await?;

// Get initial status
let mut status = status_rx.borrow().clone();

// Wait for changes
while !is_final(&status) {
    if status_rx.changed().await.is_err() {
        // Channel closed, poll final status
        status = agent_control.get_status(thread_id).await;
        break;
    }
    status = status_rx.borrow().clone();
}
// status is now final (Completed, Errored, Shutdown, NotFound)
```

### Completion Notification Flow

```
Child agent completes (TurnComplete event)
         ↓
Completion watcher detects is_final(status)
         ↓
format_subagent_notification_message(agent_id, status)
  → "<subagent_notification>{\"agent_id\":\"...\",\"status\":\"...\"}</subagent_notification>"
         ↓
parent_thread.inject_user_message_without_turn(notification_text)
         ↓
Notification appears in parent transcript as user message
         ↓
Model sees notification and can act on it
```

### Guardian Review Flow

```
Agent requests on-request approval (shell/exec/patch/network/MCP)
         ↓
routes_approval_to_guardian(turn) checks:
  - approval_policy == OnRequest
  - Feature::GuardianApproval enabled
         ↓
build_guardian_prompt_items()
  - Compact transcript (user messages + bounded recent context)
  - Format planned action JSON
         ↓
run_guardian_subagent()
  - Spawn with approval_policy=Never, read-only sandbox
  - Source = SubAgentSource::Other("guardian")
  - 90-second timeout
  - JSON schema constraint for structured output
         ↓
Parse GuardianAssessment { risk_level, risk_score, rationale, evidence }
         ↓
if risk_score < 80: Approved
else: Denied
         ↓
Emit WarningEvent with verdict summary
```

### Memory Consolidation Flow

```
State DB detects dirty memories (watermark drift)
         ↓
Claim global phase-2 job
         ↓
Query stage-1 outputs (raw memories)
         ↓
Sync filesystem artifacts
         ↓
Spawn consolidation sub-agent
  - Source = SubAgentSource::MemoryConsolidation
  - approval_policy = Never
  - Sandbox = WorkspaceWrite(codex_home)
  - Features::Collab/SpawnCsv disabled
         ↓
Detached handler monitors status
  - Sends heartbeats to maintain job lease
  - On completion: mark job succeeded
  - Auto-close agent
```

### Wait Tool Timeout Handling

```
wait(ids=[A, B, C], timeout_ms=30000)
         ↓
Subscribe to status channels for A, B, C
         ↓
If any already final: return immediately
         ↓
Create FuturesUnordered with wait_for_final_status for each
         ↓
Set deadline = now + 30s
         ↓
loop:
  timeout_at(deadline, futures.next())
    - Ok(Some(result)): First agent finished, drain remaining without blocking
    - Err(timeout): Return with timed_out=true
         ↓
Return { status: {A: Completed, B: Running, ...}, timed_out: false }
```

---

## File Relationships

```
protocol/src/protocol.rs
    └── SessionSource, SubAgentSource, Collab*Event types

core/src/agent/
    ├── mod.rs (exports)
    ├── control.rs (AgentControl - main API)
    ├── guards.rs (spawn limits, nickname uniqueness)
    └── status.rs (status tracking utilities)

core/src/tools/handlers/multi_agents.rs
    ├── spawn.rs (spawn_agent tool)
    ├── wait.rs (wait tool)
    ├── resume_agent.rs (resume_agent tool)
    ├── send_input.rs (send_input tool)
    └── close_agent.rs (close_agent tool)

core/src/guardian.rs (guardian review sub-agent)
core/src/memories/phase2.rs (memory consolidation sub-agent)
core/src/session_prefix.rs (notification formatting)
core/src/contextual_user_message.rs (notification tags)

state/src/model/thread_metadata.rs (DB schema)
tui/src/app.rs (UI navigation)
tui/src/chatwidget.rs (enablement dialog)

codex-api/src/requests/headers.rs (HTTP headers)
app-server/src/filters.rs (thread listing filters)
app-server-protocol/src/protocol/v2.rs (API protocol)
```

---

## Unclear / Unverified Aspects

1. **`agent_names.txt`**: The built-in nickname list content is not visible (referenced but not read).

2. **Role config resolution**: `resolve_role_config()` in `core/src/agent/role.rs` is referenced but the file was not read; unclear how role-specific nickname candidates and config overrides are defined.

3. **ThreadManagerState implementation**: The `spawn_new_thread_with_source()`, `fork_thread_with_source()`, and `resume_thread_from_rollout_with_source()` methods are called but their implementation is in a separate module not read.

4. **CodexThread structure**: The internal structure of agent threads and how `agent_status()` derives status is not fully traced.

5. **SQLite state persistence**: How `agent_nickname` and `agent_role` are persisted to and retrieved from the state database is referenced but the state_db module was not read.
