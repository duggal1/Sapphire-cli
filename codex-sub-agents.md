# Codex Sub-Agent Architecture

**Codebase:** codex-rs (Rust) | **Location:** `/core/src/agent/`, `/core/src/tools/handlers/multi_agents/`

---

## 1. Overview

Codex implements a **multi-agent orchestration system** where sub-agents are full Codex instances running in isolated threads. The system enables parallel task execution with explicit lifecycle management.

### Core Capabilities
- **Spawn**: Create sub-agents with custom roles, models, configs
- **Communicate**: Send messages, wait for completion, collect results  
- **Control**: Interrupt, resume, close agents
- **Limits**: Depth limits, capacity controls, unique nicknames

---

## 2. Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                  Model Tool Calls                            │
│  spawn_agent → send_input → wait → close_agent              │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│              AgentControl (Shared per Session)               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │ Spawn Mgr    │  │ Status Mgr   │  │ Guards (Limits)  │  │
│  └──────────────┘  └──────────────┘  └──────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                  Thread Manager                              │
│  Thread Registry │ PubSub Bus │ Rollout Storage             │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│              Sub-Agent Threads (Isolated)                    │
│  Agent A (Running) │ Agent B (Waiting) │ Agent C (Done)     │
└─────────────────────────────────────────────────────────────┘
```

---

## 3. Core Components

### 3.1 AgentControl

**File:** `core/src/agent/control.rs` (773 lines)

```rust
pub struct AgentControl {
    manager: Weak<ThreadManagerState>,  // Weak to avoid cycles
    state: Arc<Guards>,                 // Shared spawn limits
}
```

**Key Design:** Single `AgentControl` shared across **all agents in a user session**. This ensures:
- Unified spawn limits across entire session
- Consistent nickname allocation
- Proper parent-child tracking

### 3.2 AgentStatus

**File:** `protocol/src/protocol.rs`

```rust
pub enum AgentStatus {
    PendingInit,              // Waiting for initialization
    Running,                  // Currently executing
    Interrupted,              // Turn interrupted
    Completed(Option<String>), // Done with final message
    Errored(String),          // Encountered error
    Shutdown,                 // Explicitly shutdown
    NotFound,                 // Not found
}
```

**Final States** (for wait operations):
```rust
pub fn is_final(status: &AgentStatus) -> bool {
    !matches!(status, PendingInit | Running | Interrupted)
}
```

### 3.3 SessionSource

**File:** `protocol/src/protocol.rs`

```rust
pub enum SubAgentSource {
    ThreadSpawn {
        parent_thread_id: ThreadId,
        depth: i32,
        agent_nickname: Option<String>,
        agent_role: Option<String>,
    },
    Review,
    Compact,
    MemoryConsolidation,
    Other(String),
}
```

---

## 4. Guards & Limits

**File:** `core/src/agent/guards.rs`

### 4.1 Spawn Capacity

```rust
pub struct Guards {
    active_agents: Mutex<ActiveAgents>,
    total_count: AtomicUsize,  // Lock-free counting
}

struct ActiveAgents {
    threads_set: HashSet<ThreadId>,
    used_agent_nicknames: HashSet<String>,
    nickname_reset_count: usize,
}
```

**Lock-free capacity check:**
```rust
fn try_increment_spawned(&self, max_threads: usize) -> bool {
    let mut current = self.total_count.load(Ordering::Acquire);
    loop {
        if current >= max_threads { return false; }
        match self.total_count.compare_exchange_weak(
            current, current + 1, Ordering::AcqRel, Ordering::Acquire
        ) {
            Ok(_) => return true,
            Err(updated) => current = updated,
        }
    }
}
```

### 4.2 Depth Limits

```rust
pub fn next_thread_spawn_depth(session_source: &SessionSource) -> i32 {
    session_depth(session_source).saturating_add(1)
}

pub fn exceeds_thread_spawn_depth_limit(depth: i32, max_depth: i32) -> bool {
    depth > max_depth
}
```

**Error:** `"Agent depth limit reached. Solve the task yourself."`

### 4.3 Nickname Generation

```rust
fn format_agent_nickname(name: &str, reset_count: usize) -> String {
    match reset_count {
        0 => name.to_string(),
        n => {
            let suffix = match (n + 1) % 100 {
                11..=13 => "th",
                _ => match (n + 1) % 10 {
                    1 => "st", 2 => "nd", 3 => "rd", _ => "th",
                },
            };
            format!("{name} the {n+1}{suffix}")
        }
    }
}
```

**Example:** `Architect` → `Architect the 2nd` → `Architect the 3rd`

### 4.4 Reservation Pattern

```rust
let mut reservation = guards.reserve_spawn_slot(max_threads)?;
let nickname = reservation.reserve_agent_nickname(&candidates)?;
// ... create thread ...
reservation.commit(thread_id);  // Registers thread + nickname
```

**Drop cleanup:** If `commit()` not called, count auto-decremented.

---

## 5. Tool Handlers

### 5.1 spawn_agent

**File:** `core/src/tools/handlers/multi_agents/spawn.rs`

**Arguments:**
```rust
struct SpawnAgentArgs {
    message: Option<String>,
    items: Option<Vec<UserInput>>,
    agent_type: Option<String>,      // Role name
    model: Option<String>,           // Override model
    reasoning_effort: Option<ReasoningEffort>,
    fork_context: bool,              // Inherit parent history
}
```

**Flow:**
1. Validate depth limit
2. Build config from parent's effective runtime config
3. Apply role-specific overrides
4. Apply model/reasoning overrides if requested
5. Call `AgentControl::spawn_agent_with_options()`
6. Emit `CollabAgentSpawnBeginEvent` / `CollabAgentSpawnEndEvent`

**Config Inheritance:**
```rust
fn build_agent_spawn_config(turn: &TurnContext) -> Result<Config> {
    let mut config = turn.config.clone();
    config.model = Some(turn.model_info.slug.clone());
    config.model_provider = turn.provider.clone();
    config.model_reasoning_effort = turn.reasoning_effort;
    config.approval_policy = turn.approval_policy;
    config.sandbox_policy = turn.sandbox_policy;
    config.cwd = turn.cwd.clone();
    Ok(config)
}
```

### 5.2 send_input

**File:** `core/src/tools/handlers/multi_agents/send_input.rs`

**Arguments:**
```rust
struct SendInputArgs {
    id: String,
    message: Option<String>,
    items: Option<Vec<UserInput>>,
    interrupt: bool,
}
```

**Flow:** Validate → Optionally interrupt → Send input → Emit events

### 5.3 wait_agent

**File:** `core/src/tools/handlers/multi_agents/wait.rs`

**Arguments:**
```rust
struct WaitArgs {
    ids: Vec<String>,
    timeout_ms: Option<i64>,  // Default: 30s, Min: 10s, Max: 1h
}
```

**Parallel Wait:**
```rust
let mut futures = FuturesUnordered::new();
for (id, rx) in status_rxs {
    futures.push(wait_for_final_status(session, id, rx));
}
// Collect with deadline
let deadline = Instant::now() + Duration::from_millis(timeout_ms);
```

**Result:**
```rust
struct WaitAgentResult {
    status: HashMap<ThreadId, AgentStatus>,
    timed_out: bool,
}
```

### 5.4 resume_agent

**File:** `core/src/tools/handlers/multi_agents/resume_agent.rs`

**Flow:** Check exists → If not found, resume from rollout → Rebuild session source with stored nickname/role

### 5.5 close_agent

**File:** `core/src/tools/handlers/multi_agents/close_agent.rs`

**Flow:** Mark closed in DB → `shutdown_agent_tree()` (cascades to descendants) → Emit events

---

## 6. Lifecycle Events

**File:** `protocol/src/protocol.rs`

### Spawn
```rust
pub struct CollabAgentSpawnBeginEvent {
    pub call_id: String,
    pub sender_thread_id: ThreadId,
    pub prompt: String,
    pub model: String,
    pub reasoning_effort: ReasoningEffortConfig,
}

pub struct CollabAgentSpawnEndEvent {
    pub new_thread_id: Option<ThreadId>,
    pub new_agent_nickname: Option<String>,
    pub new_agent_role: Option<String>,
    pub status: AgentStatus,
    // ... plus begin fields
}
```

### Interaction
```rust
pub struct CollabAgentInteractionBeginEvent {
    pub receiver_thread_id: ThreadId,
    pub prompt: String,
}

pub struct CollabAgentInteractionEndEvent {
    pub receiver_agent_nickname: Option<String>,
    pub receiver_agent_role: Option<String>,
    pub status: AgentStatus,
}
```

### Wait
```rust
pub struct CollabWaitingEndEvent {
    pub agent_statuses: Vec<CollabAgentStatusEntry>,
    pub statuses: HashMap<ThreadId, AgentStatus>,
}
```

### Close/Resume
```rust
pub struct CollabCloseEndEvent {
    pub status: AgentStatus,  // Status before close
}
```

---

## 7. Spawn Lifecycle

```
1. reserve_spawn_slot(max_threads)
       │
2. reserve_agent_nickname(candidates)
       │
3. spawn_new_thread_with_source(config, session_source)
       │
4. commit(thread_id) ──► Registers in Guards
       │
5. send_input(thread_id, items)
       │
6. maybe_start_completion_watcher()
       │
7. Emit CollabAgentSpawnEndEvent
```

### Fork Context
When `fork_context: true`:
1. Materialize parent rollout to JSONL
2. Copy parent's rollout items
3. Add synthetic `FunctionCallOutput`:
   ```
   "You are the newly spawned agent. The prior conversation history 
    was forked from your parent agent. Treat the next user message 
    as your new task, and use the forked history only as background."
   ```
4. Create thread with `InitialHistory::Forked(items)`

---

## 8. Inter-Agent Communication

### Status Subscription
```rust
pub async fn subscribe_status(
    &self, agent_id: ThreadId
) -> CodexResult<watch::Receiver<AgentStatus>> {
    let thread = self.upgrade()?.get_thread(agent_id).await?;
    Ok(thread.subscribe_status())
}
```

**Usage:**
```rust
let mut rx = agent_control.subscribe_status(agent_id).await?;
while !is_final(rx.borrow().deref()) {
    rx.changed().await?;  // Blocks until status changes
}
```

### Completion Watcher
```rust
fn maybe_start_completion_watcher(
    &self,
    child_thread_id: ThreadId,
    session_source: Option<SessionSource>,
) {
    let Some(SessionSource::SubAgent(ThreadSpawn { parent_thread_id, .. })) = session_source
    else { return; };

    tokio::spawn(async move {
        let status = wait_for_final_status(&control, child_thread_id).await;
        // Parent notified via PubSub/mailbox
    });
}
```

---

## 9. Parallel Execution

### Concurrent Spawning
```rust
let (id_a, id_b, id_c) = tokio::join!(
    agent_control.spawn_agent(config_a, items_a, source_a),
    agent_control.spawn_agent(config_b, items_b, source_b),
    agent_control.spawn_agent(config_c, items_c, source_c),
);
```

### Parallel Waiting
```rust
let mut futures = FuturesUnordered::new();
for id in agent_ids {
    futures.push(wait_for_final_status(&agent_control, id));
}
let results: Vec<_> = futures.collect().await;
```

---

## 10. Key Files

| Component | File | Lines |
|-----------|------|-------|
| AgentControl | `core/src/agent/control.rs` | 773 |
| Guards | `core/src/agent/guards.rs` | 230 |
| Status | `core/src/agent/status.rs` | 30 |
| spawn_agent | `core/src/tools/handlers/multi_agents/spawn.rs` | 195 |
| wait_agent | `core/src/tools/handlers/multi_agents/wait.rs` | 229 |
| send_input | `core/src/tools/handlers/multi_agents/send_input.rs` | 120 |
| resume_agent | `core/src/tools/handlers/multi_agents/resume_agent.rs` | 150 |
| close_agent | `core/src/tools/handlers/multi_agents/close_agent.rs` | 110 |
| Protocol Types | `protocol/src/protocol.rs` | 4576 |

---

## 11. Design Principles

| Principle | Implementation |
|-----------|----------------|
| **Shared Control Plane** | Single `AgentControl` per session |
| **Explicit Lifecycle** | Parent must explicitly close sub-agents |
| **Config Inheritance** | Sub-agents inherit runtime config from parent |
| **Depth-Limited Recursion** | Prevents infinite spawn loops |
| **Unique Nicknames** | Atomically reserved, pool reset on exhaustion |
| **Lock-Free Counting** | Atomic CAS for capacity checks |
| **Watch-Based Status** | `tokio::sync::watch` for efficient status updates |

---

## 12. Error Handling

```rust
fn collab_agent_error(agent_id: ThreadId, err: CodexErr) -> FunctionCallError {
    match err {
        CodexErr::ThreadNotFound(id) => 
            FunctionCallError::RespondToModel(format!("agent {id} not found")),
        CodexErr::InternalAgentDied => 
            FunctionCallError::RespondToModel(format!("agent {agent_id} is closed")),
        CodexErr::UnsupportedOperation(_) => 
            FunctionCallError::RespondToModel("collab manager unavailable"),
        err => FunctionCallError::RespondToModel(format!("collab tool failed: {err}"))
    }
}
```

---

**Generated:** Synthesized from codex-rs source analysis  
**Total Lines:** ~480
