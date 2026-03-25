# Codex-rs Architecture Documentation

**Location:** `/Users/harshitduggal/workspace/Sapphire-cli/codex/codex-rs`

**Total Rust Files:** ~1,404 `.rs` files

**Language:** Rust (Edition 2024)

**Build System:** Cargo workspace with Bazel support

---

## 1. High-Level System Overview

Codex-rs is a production-grade AI agent system that provides:
- Interactive TUI and CLI interfaces for AI-assisted software development
- Multi-agent orchestration with sub-agent spawning and coordination
- Model Context Protocol (MCP) integration for extensible tooling
- Sandboxed command execution with platform-specific security (macOS Seatbelt, Linux Landlock, Windows Restricted Tokens)
- Persistent memory system for long-horizon task management
- Skills system following the Agent Skills open standard
- Real-time conversation capabilities with audio/text input

### Core Architecture Pattern

The system follows a **layered architecture** with clear separation of concerns:

```
┌─────────────────────────────────────────────────────────────┐
│                      UI Layer                                │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │ TUI (Ratatui)│  │ CLI (Clap)   │  │ App Server (JSON)│   │
│  └─────────────┘  └──────────────┘  └──────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                   Application Layer                          │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │ App Server  │  │ Thread Mgr   │  │ Agent Control    │   │
│  └─────────────┘  └──────────────┘  └──────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                      Core Layer                              │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │ Codex       │  │ Model Client │  │ Tool Orchestrator│   │
│  └─────────────┘  └──────────────┘  └──────────────────┘   │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │ MCP Manager │  │ Skills Mgr   │  │ Memory System    │   │
│  └─────────────┘  └──────────────┘  └──────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                   Protocol Layer                             │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │ Protocol    │  │ Config Types │  │ Items/Messages   │   │
│  └─────────────┘  └──────────────┘  └──────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                  Infrastructure Layer                        │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │ Sandboxing  │  │ Exec Engine  │  │ Network Proxy    │   │
│  │ MCP Client  │  │ State DB     │  │ Auth/Login       │   │
│  └─────────────┘  └──────────────┘  └──────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

---

## 2. Workspace Structure (Cargo Crates)

The codebase is organized as a **Cargo workspace** with 70+ crates. Key crates:

### Core Business Logic

| Crate | Path | Description |
|-------|------|-------------|
| `codex-core` | `core/` | Main business logic, agent orchestration, tool execution |
| `codex-protocol` | `protocol/` | Protocol definitions, message types, config types |
| `codex-config` | `config/` | Configuration loading, layering, validation |
| `codex-state` | `state/` | SQLite-backed state management for rollout metadata |

### User Interfaces

| Crate | Path | Description |
|-------|------|-------------|
| `codex-tui` | `tui/` | Terminal UI using Ratatui (Bubble Tea pattern) |
| `codex-cli` | `cli/` | CLI entry points, sandbox commands |
| `codex-app-server` | `app-server/` | JSON-RPC server for IDE integration |
| `codex-app-server-protocol` | `app-server-protocol/` | App server protocol definitions (v1/v2) |

### Model & API Integration

| Crate | Path | Description |
|-------|------|-------------|
| `codex-api` | `codex-api/` | Responses API client, WebSocket handling |
| `codex-client` | `codex-client/` | High-level model client abstraction |
| `codex-backend-client` | `backend-client/` | Backend API client |
| `codex-rmcp-client` | `rmcp-client/` | Model Context Protocol client |

### Execution & Sandboxing

| Crate | Path | Description |
|-------|------|-------------|
| `codex-exec` | `exec/` | Command execution engine |
| `codex-execpolicy` | `execpolicy/` | Execution policy evaluation |
| `codex-linux-sandbox` | `linux-sandbox/` | Linux sandboxing (Landlock, bubblewrap) |
| `codex-windows-sandbox` | `windows-sandbox-rs/` | Windows sandboxing (Restricted Tokens) |
| `codex-network-proxy` | `network-proxy/` | Network policy enforcement, MITM proxy |

### Skills & Plugins

| Crate | Path | Description |
|-------|------|-------------|
| `codex-skills` | `skills/` | Skills system (Agent Skills standard) |
| `codex-plugins` | `plugins/` | Plugin management |
| `codex-connectors` | `connectors/` | External service connectors |

### Utilities

| Crate | Path | Description |
|-------|------|-------------|
| `codex-utils-*` | `utils/*/` | Various utility crates (absolute-path, cache, git, etc.) |
| `codex-otel` | `otel/` | OpenTelemetry integration |
| `codex-login` | `login/` | Authentication management |
| `codex-keyring-store` | `keyring-store/` | Secure credential storage |

---

## 3. Core Module Structure (codex-core)

The `codex-core` crate is the heart of the system. Key modules:

### `/Users/harshitduggal/workspace/Sapphire-cli/codex/codex-rs/core/src/`

```
core/src/
├── lib.rs                    # Root library, module declarations
├── codex.rs                  # Main Codex agent (7,335 lines)
├── codex_thread.rs           # Thread (conversation) abstraction
├── thread_manager.rs         # Thread lifecycle management
├── client.rs                 # Model API client (Responses API, WebSocket)
├── client_common.rs          # Shared client types (Prompt, ResponseEvent)
│
├── agent/                    # Multi-agent orchestration
│   ├── mod.rs
│   ├── control.rs            # AgentControl: spawn/message agents
│   ├── guards.rs             # Spawn depth limits, guardrails
│   ├── role.rs               # Agent role configuration
│   └── status.rs             # Agent status tracking
│
├── tools/                    # Tool system
│   ├── mod.rs
│   ├── spec.rs               # Tool specifications (3,064 lines)
│   ├── registry.rs           # Tool registry
│   ├── router.rs             # Tool routing
│   ├── orchestrator.rs       # Approval + sandbox + retry orchestration
│   ├── sandboxing.rs         # Tool sandboxing
│   ├── network_approval.rs   # Network access approval
│   ├── parallel.rs           # Parallel tool execution
│   ├── context.rs            # Tool invocation context
│   ├── handlers/             # Tool implementations
│   │   ├── mod.rs
│   │   ├── shell.rs          # Shell command execution
│   │   ├── apply_patch.rs    # Code patch application
│   │   ├── read_file.rs      # File reading
│   │   ├── list_dir.rs       # Directory listing
│   │   ├── grep_files.rs     # File search
│   │   ├── mcp.rs            # MCP tool calls
│   │   ├── multi_agents.rs   # Sub-agent operations
│   │   ├── plan.rs           # Plan tool
│   │   ├── tool_search.rs    # Tool search
│   │   ├── tool_suggest.rs   # Tool suggestions
│   │   ├── request_permissions.rs
│   │   ├── request_user_input.rs
│   │   ├── artifacts.rs
│   │   ├── js_repl.rs
│   │   ├── unified_exec.rs
│   │   └── dynamic.rs
│   ├── runtimes/             # Tool runtime implementations
│   │   ├── mod.rs
│   │   ├── shell/            # Shell runtime
│   │   ├── apply_patch.rs
│   │   └── unified_exec.rs
│   └── code_mode/            # Code mode tools
│       ├── mod.rs
│       ├── protocol.rs
│       ├── worker.rs
│       └── service.rs
│
├── mcp/                      # Model Context Protocol
│   ├── mod.rs
│   ├── auth.rs               # MCP authentication
│   └── skill_dependencies.rs
│
├── mcp_connection_manager.rs # MCP server connection management (1,711 lines)
│
├── config/                   # Configuration system
│   ├── mod.rs                # Config struct (3,010 lines)
│   ├── types.rs              # Config types (SandboxMode, etc.)
│   ├── edit.rs               # Config editing
│   ├── permissions.rs        # Permission profiles
│   ├── profile.rs            # Config profiles
│   ├── agent_roles.rs        # Agent role configuration
│   ├── managed_features.rs   # Feature flags
│   └── network_proxy_spec.rs # Network proxy configuration
│
├── memories/                 # Persistent memory system
│   ├── mod.rs
│   ├── phase1.rs             # Startup extraction
│   ├── phase2.rs             # Consolidation
│   ├── control.rs
│   ├── storage.rs
│   ├── prompts.rs
│   └── usage.rs
│
├── skills/                   # Skills system
│   ├── mod.rs
│   ├── manager.rs            # SkillsManager
│   ├── loader.rs             # Skill loading
│   ├── model.rs              # Skill data structures
│   ├── render.rs             # Skill rendering
│   ├── system.rs             # System skills
│   └── injection.rs          # Skill injection
│
├── plugins/                  # Plugin system
│   ├── mod.rs
│   └── manager.rs
│
├── sandboxing/               # Sandboxing abstraction
│   ├── mod.rs                # CommandSpec, ExecRequest
│   ├── macos_permissions.rs
│   └── ...
│
├── exec.rs                   # Command execution (1,103 lines)
├── exec_policy.rs            # Execution policy
├── exec_env.rs               # Execution environment
│
├── compact.rs                # Context compaction
├── compact_remote.rs         # Remote compaction
│
├── models_manager/           # Model management
│   ├── manager.rs
│   ├── collaboration_mode_presets.rs
│   └── model_presets.rs
│
├── rollout/                  # Session rollout management
│   ├── mod.rs
│   ├── list.rs
│   ├── session_index.rs
│   └── policy.rs
│
├── state_db.rs               # SQLite state database
├── state/                    # State management
│
├── guardian/                 # Guardian approval system
│   ├── mod.rs
│   └── ...
│
├── hooks/                    # Hook system
│   └── hook_runtime.rs
│
├── instructions/             # User/project instructions
│   └── user_instructions.rs
│
├── context_manager.rs        # Context window management
├── message_history.rs        # Message history tracking
├── mentions.rs               # Skill/app mention parsing
├── git_info.rs               # Git repository information
├── shell_snapshot.rs         # Shell state snapshots
├── file_watcher.rs           # File system watcher
│
├── auth.rs                   # Authentication (re-exports codex-login)
├── login/                    # Login module
│
├── network_proxy_loader.rs   # Network proxy loading
├── network_policy_decision.rs # Network policy decisions
│
├── tools/                    # (see above)
├── agent/                    # (see above)
│
├── unified_exec/             # Unified execution engine
│   └── ...
│
├── tasks/                    # Background tasks
│   ├── mod.rs
│   ├── review.rs
│   ├── compact.rs
│   ├── undo.rs
│   └── user_shell.rs
│
├── web_search.rs             # Web search integration
├── apply_patch.rs            # Patch application logic
├── landlock.rs               # Linux Landlock sandboxing
├── seatbelt.rs               # macOS Seatbelt sandboxing
├── windows_sandbox.rs        # Windows sandboxing
│
├── error.rs                  # Error types
├── util.rs                   # Utilities
├── flags.rs                  # Feature flags
├── env.rs                    # Environment detection
│
└── tests/                    # Integration tests
    ├── suite/                # Test suites (194 files)
    │   ├── exec.rs
    │   ├── tools.rs
    │   ├── approvals.rs
    │   ├── mcp_client.rs
    │   ├── skills.rs
    │   ├── memories.rs
    │   └── ...
    └── common/               # Test utilities
```

---

## 4. Key Data Structures & Types

### Protocol Types (`/Users/harshitduggal/workspace/Sapphire-cli/codex/codex-rs/protocol/src/`)

#### Submission Queue / Event Queue Pattern

```rust
// Submission Queue Entry - requests from user
pub struct Submission {
    pub id: String,
    pub op: Op,
    pub trace: Option<W3cTraceContext>,
}

// Operations users can perform
pub enum Op {
    Interrupt,
    UserInput { items: Vec<UserInput>, ... },
    UserTurn { items: Vec<UserInput>, cwd: PathBuf, approval_policy: AskForApproval, ... },
    OverrideTurnContext { cwd: Option<PathBuf>, approval_policy: Option<AskForApproval>, ... },
    ExecApproval { id: String, turn_id: Option<String>, decision: ReviewDecision },
    PatchApproval { id: String, decision: ReviewDecision },
    ResolveElicitation { server_name: String, request_id: RequestId, ... },
    ListMcpTools,
    RefreshMcpServers { config: McpServerRefreshConfig },
    ListSkills { cwds: Vec<PathBuf>, force_reload: bool },
    Compact,
    Shutdown,
    // ... many more
}

// Event Queue Entry - responses from agent
pub enum Event {
    id: String,
    msg: EventMsg,
}

pub enum EventMsg {
    SessionConfigured(SessionConfiguredEvent),
    UserMessage(UserMessageEvent),
    AgentMessage(AgentMessageEvent),
    AgentReasoning(AgentReasoningEvent),
    ExecCommandStarted(ExecCommandStartedEvent),
    ExecCommandOutputDelta(ExecCommandOutputDeltaEvent),
    ExecCommandCompleted(ExecCommandCompletedEvent),
    ExecApprovalRequest(ExecApprovalRequestEvent),
    ApplyPatchApprovalRequest(ApplyPatchApprovalRequestEvent),
    McpToolCall(McpToolCallEvent),
    McpListToolsResponse(McpListToolsResponseEvent),
    TurnComplete(TurnCompleteEvent),
    Error(ErrorEvent),
    // ... many more
}
```

#### Configuration Types

```rust
// Sandbox modes
pub enum SandboxMode {
    ReadOnly,
    WorkspaceWrite,
    DangerFullAccess,
}

// Command approval policies
pub enum AskForApproval {
    UnlessTrusted,
    OnRequest,  // default
    Granular(GranularApprovalConfig),
    Never,
}

pub struct GranularApprovalConfig {
    pub sandbox_approval: bool,
    pub rules: bool,
    pub skill_approval: bool,
    pub request_permissions: bool,
    pub mcp_elicitations: bool,
}

// Sandbox policies
pub enum SandboxPolicy {
    DangerFullAccess,
    ReadOnly {
        access: ReadOnlyAccess,
        network_access: bool,
    },
    ExternalSandbox { network_access: NetworkAccess },
    WorkspaceWrite {
        writable_roots: Vec<AbsolutePathBuf>,
        read_only_access: ReadOnlyAccess,
        network_access: bool,
        exclude_tmpdir_env_var: bool,
        exclude_slash_tmp: bool,
    },
}
```

#### Turn Items (Conversation History)

```rust
pub enum TurnItem {
    UserMessage(UserMessageItem),
    HookPrompt(HookPromptItem),
    AgentMessage(AgentMessageItem),
    Plan(PlanItem),
    Reasoning(ReasoningItem),
    WebSearch(WebSearchItem),
    ImageGeneration(ImageGenerationItem),
    ContextCompaction(ContextCompactionItem),
}

pub struct UserMessageItem {
    pub id: String,
    pub content: Vec<UserInput>,
}

pub struct AgentMessageItem {
    pub id: String,
    pub content: Vec<AgentMessageContent>,
    pub phase: Option<MessagePhase>,
    pub memory_citation: Option<MemoryCitation>,
}
```

#### Tool System

```rust
// Tool handler trait
#[async_trait]
pub trait ToolHandler: Send + Sync {
    type Output: ToolOutput + 'static;

    fn kind(&self) -> ToolKind;
    async fn is_mutating(&self, invocation: &ToolInvocation) -> bool;
    async fn handle(&self, invocation: ToolInvocation) -> Result<Self::Output, FunctionCallError>;
}

// Tool kinds
pub enum ToolKind {
    Function,
    Mcp,
}

// Tool invocation context
pub struct ToolInvocation {
    pub call_id: String,
    pub tool_name: String,
    pub payload: ToolPayload,
    // ...
}

pub enum ToolPayload {
    Function { name: String, arguments: String },
    Mcp { server_name: String, tool_name: String, arguments: String },
    ToolSearch { query: String },
    // ...
}
```

#### Multi-Agent Types

```rust
// Agent control for spawning/managing sub-agents
pub struct AgentControl {
    manager: Weak<ThreadManagerState>,
    state: Arc<Guards>,
}

impl AgentControl {
    pub async fn spawn_agent(&self, config: Config, items: Vec<UserInput>, ...) -> CodexResult<ThreadId>;
    pub async fn send_input(&self, agent_id: ThreadId, items: Vec<UserInput>) -> CodexResult<()>;
    pub async fn wait_for_agents(&self, agent_ids: Vec<ThreadId>, timeout_ms: i64) -> ...;
    pub async fn collect_result(&self, agent_id: ThreadId) -> CodexResult<AgentResult>;
    pub async fn close_agent(&self, agent_id: ThreadId) -> CodexResult<()>;
}

// Agent status
pub enum AgentStatus {
    PendingInit,
    Running,
    Shutdown,
    NotFound,
    Completed { output: String },
    Errored { error: String },
}

// Sub-agent source tracking
pub enum SessionSource {
    Primary,
    SubAgent(SubAgentSource),
}

pub enum SubAgentSource {
    ThreadSpawn {
        parent_thread_id: ThreadId,
        depth: i32,
        agent_nickname: Option<String>,
        agent_role: Option<String>,
    },
}
```

#### Memory System Types

```rust
// Phase 1: Startup extraction
pub struct RawMemory {
    pub content: String,
    pub source_thread_id: ThreadId,
    pub timestamp: DateTime<Utc>,
}

// Phase 2: Consolidation
pub struct ConsolidatedMemory {
    pub summary: String,
    pub citations: Vec<MemoryCitation>,
}

pub struct MemoryCitation {
    pub thread_id: ThreadId,
    pub turn_id: String,
    pub description: String,
}
```

#### MCP Types

```rust
// MCP Tool definition
pub struct Tool {
    pub name: String,
    pub title: Option<String>,
    pub description: Option<String>,
    pub input_schema: serde_json::Value,
    pub output_schema: Option<serde_json::Value>,
    pub annotations: Option<serde_json::Value>,
}

// MCP Resource
pub struct Resource {
    pub uri: String,
    pub name: String,
    pub description: Option<String>,
    pub mime_type: Option<String>,
    // ...
}

// MCP Connection Manager
pub struct McpConnectionManager {
    servers: HashMap<String, Arc<RmcpClient>>,
    tool_cache: HashMap<String, Vec<Tool>>,
    // ...
}
```

---

## 5. Architecture Patterns

### 5.1 Submission Queue / Event Queue (SQ/EQ) Pattern

The core communication pattern between UI and agent:

```rust
// User submits operations
ui -> Submission { op: Op::UserTurn { ... } } -> Codex

// Agent processes and emits events
Codex -> Event { msg: EventMsg::AgentMessage(...) } -> ui
Codex -> Event { msg: EventMsg::ExecApprovalRequest(...) } -> ui
```

**Location:** `/Users/harshitduggal/workspace/Sapphire-cli/codex/codex-rs/protocol/src/protocol.rs`

### 5.2 Thread Manager Pattern

Manages multiple concurrent conversation threads:

```rust
pub struct ThreadManager {
    state: Arc<ThreadManagerState>,
}

pub struct ThreadManagerState {
    threads: RwLock<HashMap<ThreadId, Arc<CodexThread>>>,
    file_watcher: Arc<FileWatcher>,
    skills_manager: Arc<SkillsManager>,
    mcp_manager: Arc<McpManager>,
    models_manager: Arc<ModelsManager>,
    plugins_manager: Arc<PluginsManager>,
    auth_manager: Option<Arc<AuthManager>>,
    // ...
}
```

**Location:** `/Users/harshitduggal/workspace/Sapphire-cli/codex/codex-rs/core/src/thread_manager.rs`

### 5.3 Tool Orchestrator Pattern

Central orchestration for tool execution with approval + sandbox + retry:

```rust
pub struct ToolOrchestrator {
    sandbox: SandboxManager,
}

impl ToolOrchestrator {
    pub async fn run<Rq, Out, T>(
        &self,
        tool: &mut T,
        request: &Rq,
        tool_ctx: &ToolCtx,
    ) -> Result<OrchestratorRunResult<Out>, ToolError>
    where
        T: ToolRuntime<Rq, Out>,
    {
        // 1. Check if approval needed
        // 2. Select sandbox based on policy
        // 3. Attempt execution
        // 4. Retry with escalated sandbox on denial
        // 5. Handle network approval (immediate or deferred)
    }
}
```

**Location:** `/Users/harshitduggal/workspace/Sapphire-cli/codex/codex-rs/core/src/tools/orchestrator.rs`

### 5.4 Multi-Agent Lifecycle Pattern

Explicit sub-agent lifecycle:

```
spawn_agent → resume_agent → send_input → wait → collect_result → close_agent
```

**Key operations:**
- `spawn_agent`: Creates sub-agent with optional git worktree isolation
- `wait`: Blocks until specified sub-agents complete
- `collect_result`: Retrieves output and diff from sub-agent
- `close_agent`: Releases resources and cleans up

**Location:** `/Users/harshitduggal/workspace/Sapphire-cli/codex/codex-rs/core/src/tools/handlers/multi_agents.rs`

### 5.5 Memory Pipeline Pattern

Two-phase memory extraction and consolidation:

```
Phase 1 (Startup Extraction):
  - Select eligible rollouts
  - Extract raw memories using gpt-5.1-codex-mini
  - Persist to raw_memories.md
  - Enqueue consolidation jobs

Phase 2 (Consolidation):
  - Claim global consolidation lock
  - Materialize consolidation inputs
  - Dispatch consolidation agent (gpt-5.3-codex)
  - Update memory_summary.md
```

**Location:** `/Users/harshitduggal/workspace/Sapphire-cli/codex/codex-rs/core/src/memories/`

### 5.6 MCP Client Pattern

Connection management for Model Context Protocol servers:

```rust
pub struct McpConnectionManager {
    // One client per configured server
    clients: HashMap<String, Arc<RmcpClient>>,
    // Cached tool lists
    tool_cache: HashMap<String, Vec<Tool>>,
    // Auth status tracking
    auth_status: HashMap<String, McpAuthStatus>,
}

impl McpConnectionManager {
    pub async fn initialize_servers(&self, config: &McpServerConfig) -> Result<()>;
    pub async fn list_tools(&self) -> HashMap<String, Tool>;
    pub async fn call_tool(&self, server: &str, tool: &str, args: &Value) -> Result<CallToolResult>;
}
```

**Location:** `/Users/harshitduggal/workspace/Sapphire-cli/codex/codex-rs/core/src/mcp_connection_manager.rs`

### 5.7 Sandboxing Abstraction Pattern

Platform-agnostic sandboxing interface:

```rust
pub struct CommandSpec {
    pub program: String,
    pub args: Vec<String>,
    pub cwd: PathBuf,
    pub env: HashMap<String, String>,
    pub sandbox_permissions: SandboxPermissions,
}

pub struct ExecRequest {
    pub command: Vec<String>,
    pub cwd: PathBuf,
    pub env: HashMap<String, String>,
    pub network: Option<NetworkProxy>,
    pub sandbox: SandboxType,  // None, Seatbelt, Landlock, WindowsSandbox
    pub sandbox_policy: SandboxPolicy,
    pub file_system_sandbox_policy: FileSystemSandboxPolicy,
    pub network_sandbox_policy: NetworkSandboxPolicy,
}

// Platform-specific implementations:
// - macOS: Seatbelt (sandbox-exec)
// - Linux: Landlock + bubblewrap
// - Windows: Restricted Token sandbox
```

**Location:** `/Users/harshitduggal/workspace/Sapphire-cli/codex/codex-rs/core/src/sandboxing/mod.rs`

### 5.8 Config Layering Pattern

Multi-layer configuration with override precedence:

```
1. Default values
2. System config (~/.codex/config.toml)
3. Workspace config (.codex/config.toml)
4. Profile overrides
5. CLI overrides
6. Runtime overrides
```

**Location:** `/Users/harshitduggal/workspace/Sapphire-cli/codex/codex-rs/core/src/config/mod.rs`

---

## 6. Component Interactions

### 6.1 User Input Flow

```
User Input (TUI/CLI/App Server)
    │
    ▼
ThreadManager::create_thread() or ThreadManager::get_thread()
    │
    ▼
CodexThread::submit(Op::UserTurn { items, cwd, approval_policy, ... })
    │
    ▼
Codex::submit_with_trace(op, trace)
    │
    ▼
Session::handle_user_turn(items, turn_config)
    │
    ├──► ContextManager::build_prompt()  # Build context
    │       ├── SkillsManager::skills_for_config()
    │       ├── PluginsManager::plugins_for_config()
    │       ├── McpManager::list_tools()
    │       └── collect_user_messages()
    │
    ├──► ModelClient::stream()  # Call model API
    │       ├── WebSocket connection (prewarm)
    │       ├── SSE fallback
    │       └── Response parsing
    │
    └──► ToolOrchestrator::run()  # Execute tools
            ├── Approval check (Guardian)
            ├── Sandbox selection
            ├── Execution
            └── Network approval (if needed)
    │
    ▼
Event emission (EventMsg::*)
    │
    ▼
TUI/App Server renders response
```

### 6.2 Sub-Agent Spawn Flow

```
Model calls spawn_agent tool
    │
    ▼
SpawnAgentHandler::handle(invocation)
    │
    ▼
AgentControl::spawn_agent(config, items, session_source)
    │
    ├──► Guards::reserve_spawn_slot(max_threads)
    ├──► Guards::reserve_agent_nickname(candidates)
    ├──► ThreadManager::create_thread()
    │       ├── New Codex instance with inherited config
    │       ├── Apply role-specific overrides
    │       └── Submit initial prompt
    │
    └──► Return ThreadId
    │
    ▼
Parent agent waits via wait_for_agents()
    │
    ▼
Collect result via collect_result()
    │
    ▼
Close agent via close_agent()
```

### 6.3 MCP Tool Call Flow

```
Model calls MCP tool (e.g., "mcp__github__search")
    │
    ▼
McpToolHandler::handle(invocation)
    │
    ▼
McpConnectionManager::call_tool(server_name, tool_name, args)
    │
    ├──► Get RmcpClient for server
    ├──► Call tool via MCP protocol
    │       ├── Stdio transport
    │       ├── Streamable HTTP
    │       └── WebSocket
    │
    ├──► Handle elicitation (if needed)
    │       └── Emit McpElicitationRequest event
    │
    └──► Return CallToolResult
    │
    ▼
Format output for model
```

### 6.4 Memory Startup Flow

```
Codex startup
    │
    ▼
MemoriesManager::start_memories_startup_task()
    │
    ├──► Phase 1: Extract raw memories
    │       ├── Scan eligible rollouts (last N sessions)
    │       ├── For each rollout:
    │       │   ├── Claim job in state DB
    │       │   ├── Build extraction prompt
    │       │   ├── Call gpt-5.1-codex-mini
    │       │   ├── Parse raw memories
    │       │   └── Persist to raw_memories.md
    │       └── Enqueue Phase 2 consolidation jobs
    │
    └──► Phase 2: Consolidate memories
            ├── Claim consolidation lock
            ├── Load raw memories
            ├── Build consolidation prompt
            ├── Call gpt-5.3-codex
            ├── Update memory_summary.md
            └── Update citations
```

---

## 7. Key Interfaces & Traits

### ToolHandler Trait

```rust
#[async_trait]
pub trait ToolHandler: Send + Sync {
    type Output: ToolOutput + 'static;

    fn kind(&self) -> ToolKind;
    fn matches_kind(&self, payload: &ToolPayload) -> bool;
    async fn is_mutating(&self, invocation: &ToolInvocation) -> bool;
    async fn handle(&self, invocation: ToolInvocation) -> Result<Self::Output, FunctionCallError>;
}
```

**Implementations:** All tool handlers in `core/src/tools/handlers/`

### ToolOutput Trait

```rust
pub trait ToolOutput: Serialize {
    fn to_response_item(&self, call_id: &str, payload: &ToolPayload) -> ResponseInputItem;
    fn code_mode_result(&self, payload: &ToolPayload) -> serde_json::Value;
}
```

### ToolRuntime Trait

```rust
#[async_trait]
pub trait ToolRuntime<Rq, Out>: Send + Sync {
    async fn run(
        &self,
        request: &Rq,
        attempt: &SandboxAttempt<'_>,
        tool_ctx: &ToolCtx,
    ) -> Result<Out, ToolError>;

    fn network_approval_spec(&self, request: &Rq, tool_ctx: &ToolCtx) -> Option<NetworkApprovalSpec>;
}
```

---

## 8. File Counts by Module

Based on analysis of 1,404 Rust files:

| Module | File Count | Description |
|--------|-----------|-------------|
| `core/tests/suite/` | 194 | Integration test suites |
| `core/src/tools/` | 78 | Tool implementations |
| `tui/src/bottom_pane/` | 63 | TUI bottom pane components |
| `tui/src/` (lib.rs files) | 54 | TUI library modules |
| `core/tests/common/` | 25 | Test utilities |
| `core/src/plugins/` | 18 | Plugin system |
| `core/src/config/` | 17 | Configuration |
| `core/src/bin/` | 17 | Binary entry points |
| `core/src/rollout/` | 13 | Rollout management |
| `core/src/models/` | 13 | Model types |
| `core/src/models_manager/` | 9 | Model management |
| `core/src/agent/` | 8 | Multi-agent orchestration |
| `protocol/src/` | 7 | Protocol definitions |

---

## 9. Dependencies (Key External Crates)

From `Cargo.toml` workspace dependencies:

### Async & Concurrency
- `tokio` - Async runtime
- `async-channel` - Async channels
- `async-trait` - Async trait support
- `futures` - Future utilities

### Serialization
- `serde` + `serde_json` - Serialization
- `toml` + `toml_edit` - TOML parsing/editing
- `ts-rs` - TypeScript type generation
- `schemars` - JSON Schema generation

### CLI & TUI
- `clap` - CLI parsing
- `ratatui` - TUI framework
- `crossterm` - Terminal manipulation

### Networking
- `reqwest` - HTTP client
- `tokio-tungstenite` - WebSocket client
- `rmcp` - Model Context Protocol

### Database
- `sqlx` - SQLite with async support

### Logging & Telemetry
- `tracing` + `tracing-subscriber` - Structured logging
- `opentelemetry` + `opentelemetry-otlp` - Telemetry

### Error Handling
- `thiserror` - Error trait macros
- `anyhow` - Application errors

### Testing
- `insta` - Snapshot testing
- `tokio-test` - Async testing
- `assert_cmd` - Command testing

---

## 10. Build & Development

### Build Commands

```bash
# Build all workspace members
cargo build --workspace

# Build specific crate
cargo build -p codex-core
cargo build -p codex-tui

# Run tests
cargo test --workspace

# Run specific test suite
cargo test -p codex-core --test suite

# Lint
cargo clippy --workspace

# Format
cargo fmt --all
```

### Feature Flags

Key features in `codex-core`:
- `voice-input` - Voice input support
- `default` - Standard feature set

### Configuration Files

- `Cargo.toml` - Workspace configuration
- `Cargo.lock` - Dependency lock file
- `rust-toolchain.toml` - Rust version (nightly)
- `clippy.toml` - Clippy configuration
- `rustfmt.toml` - Formatting configuration
- `deny.toml` - cargo-deny configuration

---

## 11. Testing Architecture

### Test Organization

```
core/tests/
├── suite/              # Integration test suites (194 files)
│   ├── mod.rs          # Test module registry
│   ├── exec.rs         # Execution tests
│   ├── tools.rs        # Tool tests
│   ├── approvals.rs    # Approval workflow tests
│   ├── mcp_client.rs   # MCP client tests
│   ├── skills.rs       # Skills system tests
│   ├── memories.rs     # Memory system tests
│   ├── multi_agents.rs # Multi-agent tests
│   ├── sandboxing.rs   # Sandboxing tests
│   └── ...
│
└── common/             # Test utilities (25 files)
    ├── lib.rs
    ├── test_codex.rs   # Codex test harness
    ├── test_codex_exec.rs
    ├── responses.rs    # Response mocking
    ├── process.rs      # Process utilities
    └── ...
```

### Test Patterns

1. **Test Codex Harness**: Full Codex instance for integration tests
2. **Response Mocking**: Mock API responses for deterministic tests
3. **Snapshot Testing**: `insta` for output comparison
4. **Sandbox Testing**: Platform-specific sandbox testing

---

## 12. Key File Paths Summary

| Component | File Path | Lines |
|-----------|-----------|-------|
| Main Codex Agent | `core/src/codex.rs` | 7,335 |
| TUI App | `tui/src/app.rs` | 7,993 |
| TUI ChatWidget | `tui/src/chatwidget.rs` | 9,497 |
| Tool Spec | `core/src/tools/spec.rs` | 3,064 |
| Config | `core/src/config/mod.rs` | 3,010 |
| MCP Connection Manager | `core/src/mcp_connection_manager.rs` | 1,711 |
| Protocol | `protocol/src/protocol.rs` | 4,576 |
| App Server Protocol v2 | `app-server-protocol/src/protocol/v2.rs` | 7,983 |
| Client | `core/src/client.rs` | 1,824 |
| Thread Manager | `core/src/thread_manager.rs` | 844 |
| Exec | `core/src/exec.rs` | 1,103 |
| Agent Control | `core/src/agent/control.rs` | 773 |
| Multi-Agent Handlers | `core/src/tools/handlers/multi_agents.rs` | 418 |

---

## 13. Quick Reference: Module Responsibilities

| Module | Responsibility |
|--------|---------------|
| `codex.rs` | Main agent loop, turn processing, event handling |
| `codex_thread.rs` | Thread abstraction, submission interface |
| `thread_manager.rs` | Thread lifecycle, resource management |
| `client.rs` | Model API communication (Responses API, WebSocket) |
| `tools/*` | Tool definitions, handlers, orchestration |
| `agent/*` | Multi-agent spawning, coordination, guards |
| `mcp*` | MCP server connections, tool calls |
| `config/*` | Configuration loading, validation, editing |
| `memories/*` | Memory extraction, consolidation |
| `skills/*` | Skills loading, rendering, injection |
| `sandboxing/*` | Sandbox abstraction, platform wrappers |
| `exec.rs` | Command execution, output capture |
| `guardian/*` | Guardian approval system |
| `rollout/*` | Session rollout management |
| `state_db.rs` | SQLite state persistence |
| `compact.rs` | Context compaction |
| `models_manager/*` | Model selection, presets |
| `plugins/*` | Plugin management |
| `hooks/*` | Hook system |

---

**Document Generated:** Based on analysis of 1,404 Rust source files in `/Users/harshitduggal/workspace/Sapphire-cli/codex/codex-rs`

**Source of Truth:** All information derived directly from source code analysis.
