# Codex Sub-Agent Architecture

**Location:** `/Users/harshitduggal/workspace/Sapphire-cli/codex/codex-rs`

---

## 1. Overview

Codex implements a **multi-agent orchestration system** that allows spawning, managing, and coordinating sub-agents for parallel task execution. Sub-agents are full Codex instances running in isolated threads with their own conversation history, config, and tool access.

### Key Capabilities

- **Sub-agent Spawning**: Create new agents with custom roles, models, and configs
- **Parallel Execution**: Multiple sub-agents run concurrently with capacity limits
- **Inter-Agent Communication**: Send messages, wait for completion, collect results
- **Lifecycle Management**: Resume, interrupt, and close sub-agents
- **Depth Limits**: Configurable spawn depth to prevent infinite recursion
- **Nickname Assignment**: Auto-generated unique nicknames for agent identification
- **Fork Context**: Sub-agents can inherit parent conversation history

---

## 2. Architecture Components

### 2.1 High-Level Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                    Model Tool Calls                              │
│  spawn_agent → send_input → wait → collect_result → close_agent│
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                  AgentControl (Control Plane)                    │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │ Spawn Mgr   │  │ Status Mgr   │  │ Guards (Limits)      │  │
│  └─────────────┘  └──────────────┘  └──────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                   Thread Manager                                 │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │ Thread Reg  │  │ PubSub Bus   │  │ Rollout Storage      │  │
│  └─────────────┘  └──────────────┘  └──────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Sub-Agent Threads                             │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │ Agent A     │  │ Agent B      │  │ Agent N              │  │
│  │ (Running)   │  │ (Waiting)    │  │ (Completed)          │  │
│  └─────────────┘  └──────────────┘  └──────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

---

## 3. Core Data Structures

### 3.1 AgentStatus Enum

**Location:** `protocol/src/protocol.rs`

```rust
pub enum AgentStatus {
    PendingInit,      // Agent waiting for initialization
    Running,          // Agent currently executing
    Interrupted,      // Turn interrupted, may receive more input
    Completed(Option<String>),  // Done with final message
    Errored(String),  // Encountered error
    Shutdown,         // Explicitly shutdown
    NotFound,         // Agent not found
}
```

**Final States** (for wait operations):
```rust
pub fn is_final(status: &AgentStatus) -> bool {
    !matches!(
        status,
        AgentStatus::PendingInit | AgentStatus::Running | AgentStatus::Interrupted
    )
}
```

### 3.2 SessionSource & SubAgentSource

**Location:** `protocol/src/protocol.rs`

```rust
pub enum SessionSource {
    Cli,
    VSCode,
    Exec,
    Mcp,
    Custom(String),
    SubAgent(SubAgentSource),  // Sub-agent tracking
    Unknown,
}

pub enum SubAgentSource {
    Review,
    Compact,
    ThreadSpawn {
        parent_thread_id: ThreadId,
        depth: i32,
        agent_nickname: Option<String>,
        agent_role: Option<String>,
    },
    MemoryConsolidation,
    Other(String),
}
```

### 3.3 CollabAgentRef

**Location:** `protocol/src/protocol.rs`

```rust
pub struct CollabAgentRef {
    pub thread_id: ThreadId,
    pub agent_nickname: Option<String>,
    pub agent_role: Option<String>,
}
```

---

## 4. AgentControl (Control Plane)

**Location:** `core/src/agent/control.rs`

### 4.1 Structure

```rust
pub struct AgentControl {
    /// Weak handle to global thread registry
    manager: Weak<ThreadManagerState>,
    /// Shared guards for spawn limits
    state: Arc<Guards>,
}
```

**Key Design:**
- `AgentControl` is shared across **all agents in a user session**
- Uses `Weak` reference to avoid cycles: `ThreadManagerState -> CodexThread -> Session -> SessionServices -> ThreadManagerState`
- Guards enforce per-session limits (max threads, nickname uniqueness)

### 4.2 Spawn Lifecycle

```rust
impl AgentControl {
    /// Spawn new agent with initial prompt
    pub async fn spawn_agent(
        &self,
        config: Config,
        items: Vec<UserInput>,
        session_source: Option<SessionSource>,
    ) -> CodexResult<ThreadId>;

    /// Spawn with additional options (e.g., fork context)
    pub async fn spawn_agent_with_options(
        &self,
        config: Config,
        items: Vec<UserInput>,
        session_source: Option<SessionSource>,
        options: SpawnAgentOptions,
    ) -> CodexResult<ThreadId>;
}
```

**Spawn Process:**
1. Reserve spawn slot via `Guards::reserve_spawn_slot()`
2. Reserve unique agent nickname
3. Create new thread via `ThreadManagerState::spawn_new_thread()`
4. Commit reservation
5. Send initial input via `send_input()`
6. Start completion watcher (for parent notification)

### 4.3 Communication Operations

```rust
impl AgentControl {
    /// Send user input to existing agent
    pub async fn send_input(
        &self,
        agent_id: ThreadId,
        items: Vec<UserInput>,
    ) -> CodexResult<String>;

    /// Interrupt agent's current turn
    pub async fn interrupt_agent(&self, agent_id: ThreadId) -> CodexResult<String>;

    /// Subscribe to status changes
    pub async fn subscribe_status(
        &self,
        agent_id: ThreadId,
    ) -> CodexResult<watch::Receiver<AgentStatus>>;

    /// Get current status
    pub async fn get_status(&self, agent_id: ThreadId) -> AgentStatus;

    /// Close agent and descendants
    pub async fn close_agent(&self, agent_id: ThreadId) -> CodexResult<String>;

    /// Resume agent from rollout
    pub async fn resume_agent_from_rollout(
        &self,
        config: Config,
        thread_id: ThreadId,
        session_source: SessionSource,
    ) -> CodexResult<ThreadId>;
}
```

---

## 5. Guards & Limits

**Location:** `core/src/agent/guards.rs`

### 5.1 Spawn Limits

```rust
pub struct Guards {
    active_agents: Mutex<ActiveAgents>,
    total_count: AtomicUsize,
}

struct ActiveAgents {
    threads_set: HashSet<ThreadId>,
    thread_agent_nicknames: HashMap<ThreadId, String>,
    used_agent_nicknames: HashSet<String>,
    nickname_reset_count: usize,
}
```

### 5.2 Reservation Pattern

```rust
pub struct SpawnReservation {
    state: Arc<Guards>,
    active: bool,
    reserved_agent_nickname: Option<String>,
}

impl Guards {
    pub fn reserve_spawn_slot(
        &self,
        max_threads: Option<usize>,
    ) -> Result<SpawnReservation>;
}

impl SpawnReservation {
    pub fn reserve_agent_nickname(&mut self, names: &[&str]) -> Result<String>;
    pub fn commit(self, thread_id: ThreadId);
}
```

**Usage:**
```rust
let mut reservation = guards.reserve_spawn_slot(config.agent_max_threads)?;
let nickname = reservation.reserve_agent_nickname(&candidate_names)?;
// ... create thread ...
reservation.commit(new_thread_id);
```

### 5.3 Depth Limits

```rust
pub fn next_thread_spawn_depth(session_source: &SessionSource) -> i32 {
    session_depth(session_source).saturating_add(1)
}

pub fn exceeds_thread_spawn_depth_limit(depth: i32, max_depth: i32) -> bool {
    depth > max_depth
}
```

**Default:** `agent_max_depth` typically 5-10 levels

### 5.4 Nickname Generation

```rust
fn format_agent_nickname(name: &str, nickname_reset_count: usize) -> String {
    match nickname_reset_count {
        0 => name.to_string(),
        n => {
            let suffix = match (n + 1) % 100 {
                11..=13 => "th",
                _ => match (n + 1) % 10 {
                    1 => "st",
                    2 => "nd",
                    3 => "rd",
                    _ => "th",
                },
            };
            format!("{name} the {n+1}{suffix}")
        }
    }
}
```

**Example:** `Architect`, `Architect the 2nd`, `Architect the 3rd`...

---

## 6. Tool Handlers

**Location:** `core/src/tools/handlers/multi_agents/`

### 6.1 spawn_agent

**File:** `spawn.rs`

**Arguments:**
```rust
struct SpawnAgentArgs {
    message: Option<String>,
    items: Option<Vec<UserInput>>,
    agent_type: Option<String>,       // Role name
    model: Option<String>,            // Override model
    reasoning_effort: Option<ReasoningEffort>,
    fork_context: bool,               // Inherit parent history
}
```

**Result:**
```rust
struct SpawnAgentResult {
    agent_id: String,
    nickname: Option<String>,
}
```

**Flow:**
1. Parse arguments and validate depth limit
2. Build config from parent's effective config
3. Apply role-specific overrides
4. Apply model/reasoning overrides if requested
5. Call `AgentControl::spawn_agent_with_options()`
6. Emit `CollabAgentSpawnBeginEvent` / `CollabAgentSpawnEndEvent`

**Config Inheritance:**
```rust
fn build_agent_spawn_config(
    base_instructions: &BaseInstructions,
    turn: &TurnContext,
) -> Result<Config> {
    let mut config = turn.config.clone();
    config.model = Some(turn.model_info.slug.clone());
    config.model_provider = turn.provider.clone();
    config.model_reasoning_effort = turn.reasoning_effort;
    config.approval_policy = turn.approval_policy;
    config.sandbox_policy = turn.sandbox_policy;
    config.cwd = turn.cwd.clone();
    // ... runtime overrides
    Ok(config)
}
```

### 6.2 send_input

**File:** `send_input.rs`

**Arguments:**
```rust
struct SendInputArgs {
    id: String,
    message: Option<String>,
    items: Option<Vec<UserInput>>,
    interrupt: bool,  // Interrupt current turn
}
```

**Flow:**
1. Validate agent exists
2. Optionally interrupt agent
3. Emit `CollabAgentInteractionBeginEvent`
4. Call `AgentControl::send_input()`
5. Emit `CollabAgentInteractionEndEvent`

### 6.3 wait_agent

**File:** `wait.rs`

**Arguments:**
```rust
struct WaitArgs {
    ids: Vec<String>,
    timeout_ms: Option<i64>,  // Default: 30s, Min: 10s, Max: 1h
}
```

**Result:**
```rust
struct WaitAgentResult {
    status: HashMap<ThreadId, AgentStatus>,
    timed_out: bool,
}
```

**Flow:**
1. Subscribe to status channels for all agents
2. Check for already-final statuses
3. Wait via `watch::Receiver::changed()` with timeout
4. Emit `CollabWaitingBeginEvent` / `CollabWaitingEndEvent`

**Parallel Wait:**
```rust
let mut futures = FuturesUnordered::new();
for (id, rx) in status_rxs {
    futures.push(wait_for_final_status(session, id, rx));
}
// Collect results with deadline
```

### 6.4 resume_agent

**File:** `resume_agent.rs`

**Arguments:**
```rust
struct ResumeAgentArgs {
    id: String,
}
```

**Flow:**
1. Check if agent exists
2. If not found, attempt resume from rollout
3. Rebuild session source with stored nickname/role
4. Call `AgentControl::resume_agent_from_rollout()`
5. Emit `CollabResumeBeginEvent` / `CollabResumeEndEvent`

### 6.5 close_agent

**File:** `close_agent.rs`

**Arguments:**
```rust
struct CloseAgentArgs {
    id: String,
}
```

**Flow:**
1. Subscribe to current status
2. Mark as closed in persisted spawn-edge state
3. Call `shutdown_agent_tree()` (cascades to descendants)
4. Emit `CollabCloseBeginEvent` / `CollabCloseEndEvent`

---

## 7. Lifecycle Events

**Location:** `protocol/src/protocol.rs`

### 7.1 Spawn Events

```rust
pub struct CollabAgentSpawnBeginEvent {
    pub call_id: String,
    pub sender_thread_id: ThreadId,
    pub prompt: String,
    pub model: String,
    pub reasoning_effort: ReasoningEffortConfig,
}

pub struct CollabAgentSpawnEndEvent {
    pub call_id: String,
    pub sender_thread_id: ThreadId,
    pub new_thread_id: Option<ThreadId>,
    pub new_agent_nickname: Option<String>,
    pub new_agent_role: Option<String>,
    pub prompt: String,
    pub model: String,
    pub reasoning_effort: ReasoningEffortConfig,
    pub status: AgentStatus,
}
```

### 7.2 Interaction Events

```rust
pub struct CollabAgentInteractionBeginEvent {
    pub call_id: String,
    pub sender_thread_id: ThreadId,
    pub receiver_thread_id: ThreadId,
    pub prompt: String,
}

pub struct CollabAgentInteractionEndEvent {
    pub call_id: String,
    pub sender_thread_id: ThreadId,
    pub receiver_thread_id: ThreadId,
    pub receiver_agent_nickname: Option<String>,
    pub receiver_agent_role: Option<String>,
    pub prompt: String,
    pub status: AgentStatus,
}
```

### 7.3 Wait Events

```rust
pub struct CollabWaitingBeginEvent {
    pub sender_thread_id: ThreadId,
    pub receiver_thread_ids: Vec<ThreadId>,
    pub receiver_agents: Vec<CollabAgentRef>,
    pub call_id: String,
}

pub struct CollabWaitingEndEvent {
    pub sender_thread_id: ThreadId,
    pub call_id: String,
    pub agent_statuses: Vec<CollabAgentStatusEntry>,
    pub statuses: HashMap<ThreadId, AgentStatus>,
}
```

### 7.4 Close/Resume Events

```rust
pub struct CollabCloseBeginEvent {
    pub call_id: String,
    pub sender_thread_id: ThreadId,
    pub receiver_thread_id: ThreadId,
}

pub struct CollabCloseEndEvent {
    pub call_id: String,
    pub sender_thread_id: ThreadId,
    pub receiver_thread_id: ThreadId,
    pub receiver_agent_nickname: Option<String>,
    pub receiver_agent_role: Option<String>,
    pub status: AgentStatus,
}

pub struct CollabResumeBeginEvent { /* similar */ }
pub struct CollabResumeEndEvent { /* similar */ }
```

---

## 8. Parallel Execution

### 8.1 Concurrent Spawning

```rust
// Spawn multiple agents in parallel
let agent_a = session.services.agent_control.spawn_agent(config_a, items_a, source_a);
let agent_b = session.services.agent_control.spawn_agent(config_b, items_b, source_b);
let (id_a, id_b) = tokio::join!(agent_a, agent_b);
```

### 8.2 Parallel Waiting

```rust
let mut futures = FuturesUnordered::new();
for (id, rx) in status_receivers {
    futures.push(wait_for_final_status(session, id, rx));
}

// Collect all results with timeout
let deadline = Instant::now() + Duration::from_millis(timeout_ms);
loop {
    match timeout_at(deadline, futures.next()).await {
        Ok(Some(Some(result))) => results.push(result),
        Ok(Some(None)) => continue,
        Ok(None) | Err(_) => break,
    }
}
```

### 8.3 Capacity Control

```rust
impl Guards {
    fn try_increment_spawned(&self, max_threads: usize) -> bool {
        let mut current = self.total_count.load(Ordering::Acquire);
        loop {
            if current >= max_threads {
                return false;
            }
            match self.total_count.compare_exchange_weak(
                current,
                current + 1,
                Ordering::AcqRel,
                Ordering::Acquire,
            ) {
                Ok(_) => return true,
                Err(updated) => current = updated,
            }
        }
    }
}
```

**Lock-free:** Uses atomic CAS for thread-safe counting

---

## 9. Inter-Agent Communication

### 9.1 Message Passing

```
Parent Agent                          Sub-Agent
     │                                    │
     │── spawn_agent ────────────────────>│
     │                                    │
     │<── CollabAgentSpawnEnd ────────────│
     │                                    │
     │── send_input ─────────────────────>│
     │                                    │
     │<── CollabAgentInteractionEnd ──────│
     │                                    │
     │── wait_agent ─────────────────────>│
     │                                    │
     │<── CollabWaitingEnd (Completed) ───│
     │                                    │
     │── close_agent ────────────────────>│
```

### 9.2 Status Subscription

```rust
pub async fn subscribe_status(
    &self,
    agent_id: ThreadId,
) -> CodexResult<watch::Receiver<AgentStatus>> {
    let state = self.upgrade()?;
    let thread = state.get_thread(agent_id).await?;
    Ok(thread.subscribe_status())
}
```

**Usage:**
```rust
let mut status_rx = agent_control.subscribe_status(agent_id).await?;
while !is_final(status_rx.borrow().deref()) {
    status_rx.changed().await?;
}
```

### 9.3 Completion Watcher

```rust
fn maybe_start_completion_watcher(
    &self,
    child_thread_id: ThreadId,
    session_source: Option<SessionSource>,
) {
    let Some(SessionSource::SubAgent(SubAgentSource::ThreadSpawn {
        parent_thread_id, ..
    })) = session_source
    else {
        return;
    };

    let control = self.clone();
    tokio::spawn(async move {
        // Wait for final status
        let status = wait_for_final_status(&control, child_thread_id).await;
        // Parent could be notified via PubSub or mailbox
    });
}
```

---

## 10. Fork Context

### 10.1 Forked Spawn

```rust
pub struct SpawnAgentOptions {
    pub fork_parent_spawn_call_id: Option<String>,
}
```

**When `fork_context: true`:**
1. Materialize parent rollout to JSONL
2. Copy parent's rollout items
3. Add synthetic `FunctionCallOutput` explaining fork
4. Create new thread with `InitialHistory::Forked(items)`

**Fork Message:**
```rust
const FORKED_SPAWN_AGENT_OUTPUT_MESSAGE: &str =
    "You are the newly spawned agent. The prior conversation history was forked \
     from your parent agent. Treat the next user message as your new task, and \
     use the forked history only as background context.";
```

---

## 11. Resume from Rollout

### 11.1 Resume Process

```rust
pub async fn resume_agent_from_rollout(
    &self,
    config: Config,
    thread_id: ThreadId,
    session_source: SessionSource,
) -> CodexResult<ThreadId> {
    // Resume single agent
    let resumed_thread_id = self
        .resume_single_agent_from_rollout(config.clone(), thread_id, session_source)
        .await?;

    // Recursively resume children
    let mut resume_queue = VecDeque::from([(thread_id, root_depth)]);
    while let Some((parent_id, parent_depth)) = resume_queue.pop_front() {
        let child_ids = state_db
            .list_thread_spawn_children_with_status(parent_id, Open)
            .await?;
        for child_id in child_ids {
            // Resume each child
            resume_queue.push_back((child_id, parent_depth + 1));
        }
    }
    Ok(resumed_thread_id)
}
```

### 11.2 Nickname/Role Restoration

```rust
let (resumed_agent_nickname, resumed_agent_role) =
    if let Some(state_db_ctx) = state_db::get_state_db(&config).await {
        match state_db_ctx.get_thread(thread_id).await {
            Ok(Some(metadata)) => (metadata.agent_nickname, metadata.agent_role),
            Ok(None) | Err(_) => (None, None),
        }
    } else {
        (None, None)
    };
```

---

## 12. TUI Integration

**Location:** `tui/src/multi_agents.rs`

### 12.1 Agent Picker

```rust
pub struct AgentPickerThreadEntry {
    pub agent_nickname: Option<String>,
    pub agent_role: Option<String>,
    pub is_closed: bool,
}
```

**Rendering:**
```rust
pub fn format_agent_picker_item_name(
    agent_nickname: Option<&str>,
    agent_role: Option<&str>,
    is_primary: bool,
) -> String {
    if is_primary {
        return "Main [default]".to_string();
    }
    match (agent_nickname, agent_role) {
        (Some(nick), Some(role)) => format!("{nick} [{role}]"),
        (Some(nick), None) => nick.to_string(),
        (None, Some(role)) => format!("[{role}]"),
        (None, None) => "Agent".to_string(),
    }
}
```

### 12.2 Status Display

```rust
fn status_summary_spans(status: &AgentStatus) -> Vec<Span<'static>> {
    match status {
        AgentStatus::PendingInit => vec![Span::from("Pending init").cyan()],
        AgentStatus::Running => vec![Span::from("Running").cyan().bold()],
        AgentStatus::Interrupted => vec![Span::from("Interrupted").yellow()],
        AgentStatus::Completed(msg) => {
            // Show preview of final message
        }
        AgentStatus::Errored(err) => {
            // Show error preview
        }
        AgentStatus::Shutdown => vec![Span::from("Shutdown")],
        AgentStatus::NotFound => vec![Span::from("Not found").red()],
    }
}
```

### 12.3 Keyboard Navigation

```rust
pub fn previous_agent_shortcut() -> KeyBinding {
    alt(KeyCode::Left)
}

pub fn next_agent_shortcut() -> KeyBinding {
    alt(KeyCode::Right)
}
```

---

## 13. Key File Locations

| Component | File Path | Description |
|-----------|-----------|-------------|
| AgentControl | `core/src/agent/control.rs` | Main control plane (773 lines) |
| Guards | `core/src/agent/guards.rs` | Spawn limits, nicknames |
| Status | `core/src/agent/status.rs` | Status derivation from events |
| Spawn Handler | `core/src/tools/handlers/multi_agents/spawn.rs` | spawn_agent tool |
| Wait Handler | `core/src/tools/handlers/multi_agents/wait.rs` | wait_agent tool |
| Send Input | `core/src/tools/handlers/multi_agents/send_input.rs` | send_input tool |
| Resume | `core/src/tools/handlers/multi_agents/resume_agent.rs` | resume_agent tool |
| Close | `core/src/tools/handlers/multi_agents/close_agent.rs` | close_agent tool |
| Multi-Agents | `core/src/tools/handlers/multi_agents.rs` | Shared utilities |
| Protocol Types | `protocol/src/protocol.rs` | Events, AgentStatus, SessionSource |
| TUI Multi-Agents | `tui/src/multi_agents.rs` | TUI rendering & navigation |

---

## 14. Design Principles

### 14.1 Shared Control Plane

- Single `AgentControl` per user session
- All sub-agents share the same guards
- Prevents resource exhaustion across entire session

### 14.2 Explicit Lifecycle

```
spawn_agent → resume_agent → send_input → wait → collect_result → close_agent
```

**No implicit cleanup:** Parent must explicitly close sub-agents

### 14.3 Config Inheritance

Sub-agents inherit from parent's **effective runtime config**:
- Model & provider
- Reasoning effort
- Approval policy
- Sandbox policy
- CWD

Role-specific overrides applied after inheritance.

### 14.4 Depth-Limited Recursion

```rust
if exceeds_thread_spawn_depth_limit(child_depth, max_depth) {
    return Err("Agent depth limit reached. Solve the task yourself.");
}
```

**Prevents:** Infinite spawn loops, stack overflow

### 14.5 Unique Nicknames

- Nicknames reserved atomically via `Guards`
- Pool reset when exhausted (`Architect the 2nd`, etc.)
- Stored in SQLite for resume

---

## 15. Error Handling

### 15.1 Common Errors

```rust
pub enum CodexErr {
    ThreadNotFound(ThreadId),
    InternalAgentDied,
    AgentLimitReached { max_threads: usize },
    Fatal(String),
    // ...
}
```

### 15.2 Tool Error Mapping

```rust
fn collab_agent_error(agent_id: ThreadId, err: CodexErr) -> FunctionCallError {
    match err {
        CodexErr::ThreadNotFound(id) => {
            FunctionCallError::RespondToModel(format!("agent with id {id} not found"))
        }
        CodexErr::InternalAgentDied => {
            FunctionCallError::RespondToModel(format!("agent with id {agent_id} is closed"))
        }
        CodexErr::UnsupportedOperation(_) => {
            FunctionCallError::RespondToModel("collab manager unavailable")
        }
        err => FunctionCallError::RespondToModel(format!("collab tool failed: {err}"))
    }
}
```

---

## 16. Testing

### 16.1 Guard Tests

**Location:** `core/src/agent/guards_tests.rs`

```rust
#[test]
fn reserve_spawn_slot_respects_max_threads() {
    let guards = Arc::new(Guards::default());
    let mut reservations = Vec::new();

    for _ in 0..3 {
        let res = guards.reserve_spawn_slot(Some(3)).unwrap();
        reservations.push(res);
    }

    // 4th should fail
    assert!(guards.reserve_spawn_slot(Some(3)).is_err());
}
```

### 16.2 Integration Tests

**Location:** `core/src/codex_tests_guardian.rs`

```rust
#[tokio::test]
async fn multi_agent_spawn_and_wait() {
    let session = create_test_session();
    let agent_a = session.services.agent_control
        .spawn_agent(config_a, items_a, source_a).await?;
    let agent_b = session.services.agent_control
        .spawn_agent(config_b, items_b, source_b).await?;

    let result = session.services.agent_control
        .wait_agent(vec![agent_a, agent_b], Some(30000)).await?;

    assert!(!result.timed_out);
}
```

---

**Document Generated:** Based on analysis of codex-rs codebase

**Source of Truth:** All information derived directly from source code analysis.
