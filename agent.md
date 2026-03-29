# Sapphire CLI - Agent Codebase Documentation

**Location:** `/Users/harshitduggal/workspace/Sapphire-cli`

**Language:** Go 1.26.1

**Architecture:** Terminal-first AI assistant with multi-agent orchestration, graph-based code indexing, and durable memory

**Knowledge Cutoff:** Mid-2025 (agent personality constraint)

---

## Table of Contents

1. [High-Level Architecture](#1-high-level-architecture)
2. [Graph-Based Code Indexing](#2-graph-based-code-indexing)
3. [Sub-Agent Orchestration System](#3-sub-agent-orchestration-system)
4. [Agent Runtime & Execution](#4-agent-runtime--execution)
5. [Tools System](#5-tools-system)
6. [Collaboration Modes](#6-collaboration-modes)
7. [MCP Integration](#7-mcp-integration)
8. [LSP Integration](#8-lsp-integration)
9. [Persistent Memory System](#9-persistent-memory-system)
10. [UI/TUI Layer](#10-uitui-layer)
11. [Background Services](#11-background-services)
12. [Database Schema](#12-database-schema)
13. [Configuration](#13-configuration)
14. [Quick Reference](#14-quick-reference)

---

## 1. High-Level Architecture

### System Overview

Sapphire CLI is a production-grade AI agent system providing:

- **Interactive TUI (Bubble Tea)** and non-interactive CLI modes
- **Multi-agent orchestration** with explicit lifecycle sub-agent spawning and coordination
- **Graph-based code indexing** with AST parsing for Go (symbols, call graphs, import graphs)
- **Git worktree isolation** for parallel agent execution with validation gates
- **Model Context Protocol (MCP)** integration with registry-backed server discovery
- **Language Server Protocol (LSP)** integration for semantic code intelligence
- **Persistent memory system** with SQLite storage, Gemini embeddings, and progressive context injection
- **Background agent dispatch** with capacity control and autonomous supervision
- **Permission-based tool execution** with user approval workflows
- **Seven collaboration modes** (default, plan, architect, debug, security, review, orchestrator)

### Core Innovation: Graph-Based Code Understanding

Sapphire uses **AST-based symbol graphs** as a replacement for embeddings for structural code understanding:

- **Symbols extracted**: Functions, methods, types, consts, vars with signatures and documentation
- **Edge types**: `imports`, `calls`, `defines`, `test_covers`
- **Storage**: SQLite with tables for files, symbols, and edges
- **Context injection**: Task-specific graph slices built by traversing from seed files/symbols
- **Boot packets**: JSON-serialized graph slices injected into agent context as system messages

### Architecture Diagram

```
main.go (entry point)
    └── internal/cmd/root.go (CLI commands)
            └── internal/app/app.go (application container)
                    ├── internal/agent/coordinator.go (agent orchestration)
                    │       ├── internal/agent/agent.go (session agent)
                    │       ├── internal/agent/memory/compiler.go (graph boot packets)
                    │       └── internal/agent/memory/indexer.go (AST-based indexing)
                    ├── internal/agent/subagent_manager.go (sub-agent lifecycle)
                    ├── internal/orchestration/db/db.go (SQLite orchestration store)
                    ├── internal/lsp/manager.go (language servers via powernap)
                    ├── internal/agent/tools/mcp/ (MCP clients via go-sdk/mcp)
                    ├── internal/memory/ (persistent memory with Gemini embeddings)
                    └── internal/agent/supervisor/ (autonomous health monitoring)
```

### Layered Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      UI Layer                                │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │ TUI (Bubble │  │ CLI (Cobra)  │  │ Dialog System    │   │
│  │  Tea)       │  │              │  │ (MCP Manager,    │   │
│  └─────────────┘  └──────────────┘  │  File Picker)    │   │
│                                      └──────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                   Application Layer                          │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │ App         │  │ Session Mgr  │  │ Message Service  │   │
│  │ Container   │  │              │  │                  │   │
│  └─────────────┘  └──────────────┘  └──────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                      Agent Layer                             │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │ Coordinator │  │ SessionAgent │  │ Tool Orchestrator│   │
│  └─────────────┘  └──────────────┘  └──────────────────┘   │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │ Sub-Agent   │  │ Background   │  │ Supervisor       │   │
│  │ Manager     │  │ Dispatcher   │  │ Patrol           │   │
│  └─────────────┘  └──────────────┘  └──────────────────┘   │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │ Memory      │  │ Long-Horizon │  │ Mailbox Service  │   │
│  │ Compiler    │  │ Manager      │  │                  │   │
│  └─────────────┘  └──────────────┘  └──────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                      Tools Layer                             │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │ File Ops    │  │ Shell Exec   │  │ MCP Tools        │   │
│  │ (view/edit/ │  │ (bash/jobs/  │  │ (66 tools total) │   │
│  │  agentic)   │  │  fast bg)    │  │                  │   │
│  └─────────────┘  └──────────────┘  └──────────────────┘   │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │ Search      │  │ Agent Ops    │  │ Memory & Diag    │   │
│  │ (glob/grep/ │  │ (spawn/wait/ │  │ (index_codebase, │   │
│  │  semantic)  │  │  collect)    │  │  lsp_*)          │   │
│  └─────────────┘  └──────────────┘  └──────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                   Protocol Layer                             │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │ MCP Client  │  │ LSP Client   │  │ Fantasy Framework│   │
│  │ (go-sdk/mcp)│  │ (powernap)   │  │ (charm.land)     │   │
│  └─────────────┘  └──────────────┘  └──────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                  Persistence Layer                           │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │ SQLite DB   │  │ Memory Store │  │ Worktrees        │   │
│  │ (orchestra- │  │ (embeddings  │  │ (git isolation   │   │
│  │  tion +     │  │  + FTS5)     │  │  + validation)   │   │
│  │  graph)     │  │              │  │                  │   │
│  └─────────────┘  └──────────────┘  └──────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

---

## 2. Graph-Based Code Indexing

### Overview

Sapphire builds an **AST-based symbol graph** for Go code that serves as a structural alternative to embeddings. This graph is compiled into **Boot Packets** and injected into agent context.

### Core Files

| File | Lines | Purpose |
|------|-------|---------|
| `internal/agent/memory/indexer.go` | ~1376 | AST parsing, symbol extraction, edge discovery |
| `internal/agent/memory/compiler.go` | ~1472 | Boot packet compilation, graph slice building |
| `internal/agent/memory/runtime.go` | ~581 | Persistence, resume points, boot packet storage |
| `internal/db/migrations/20260325000000_add_durable_memory_graph.sql` | - | SQLite schema for symbols/edges |
| `internal/agent/index_codebase_tool.go` | ~40 | `index_codebase` tool implementation |

### Graph Structure

#### Symbols (Nodes)

```go
type indexedRepoSymbol struct {
    StableKey   string  // "path::kind::receiver::name"
    Name        string
    Kind        string  // "function", "method", "type", "const", "var"
    Signature   string  // Full function/type signature
    Doc         string  // Documentation (max 240 chars)
    StartLine   int
    EndLine     int
    Exported    bool
    Status      string  // "active" or "deprecated"
    Fingerprint string  // SHA256 hash for change detection
}
```

#### Edges (Relationships)

```go
type storedRepoEdge struct {
    Type        string             // "imports", "calls", "defines", "test_covers"
    FromFile    string
    FromSymbol  string
    ToFile      string
    ToSymbol    string
    ToSymbolKey string
    Metadata    map[string]any     // {"evidence": "...", "import": "..."}
}
```

**Edge Types:**

| Type | Description | From | To |
|------|-------------|------|-----|
| `imports` | Package import relationships | File | File (import path) |
| `defines` | Symbol definition in file | File | Symbol |
| `calls` | Function/method calls | Symbol | Symbol |
| `test_covers` | Test function coverage | Test symbol | Tested symbol |

### Indexing Flow

```
┌─────────────────────────────────────────────────────────────┐
│  WarmCodebase() Call (triggered by index_codebase tool)     │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│  captureRepoSnapshot() - Git metadata                       │
│  - repo_root, branch, head_commit, dirty, changed_files     │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│  scanRepoFileCandidates() - File discovery                  │
│  - Walk directory tree, filter by extension/size            │
│  - Allowed: .go, .md, .sql, .json, .yaml, .toml, etc.       │
│  - Ignored: .git, node_modules, vendor, build, etc.         │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│  loadExistingScope() - Check database for existing index    │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│  Diff Detection - Compare modtime, size, language           │
│  - Identify changed_paths, removed_paths                    │
│  - Incremental: only re-index changed files                 │
└────────────────────┬────────────────────────────────────────┘
                     │
          ┌──────────┴──────────┐
          │  No changes?        │
          └──────────┬──────────┘
                    │
      ┌─────────────┴─────────────┐
      │ YES                       │ NO
      ▼                           ▼
┌─────────────┐         ┌─────────────────────────────────────┐
│ Early       │         │ Begin Transaction                   │
│ return      │         │ - Upsert scope (epoch++)            │
│ cached      │         │ - Delete removed files              │
│ scope       │         │ - Parse changed files (parallel)    │
└─────────────┘         └──────────────────┬──────────────────┘
                                           │
                                           ▼
┌──────────────────────────────────────────────────────────────┐
│  extractGoFacts() per file (AST PARSING)                     │
│  1. Parse Go source with go/parser                          │
│  2. Extract imports → create "imports" edges                │
│  3. Extract FuncDecl → create symbols + "defines" edges     │
│  4. Extract TypeSpec → create type symbols                  │
│  5. Extract ValueSpec → create const/var symbols            │
│  6. discoverCallsForSymbol() → create "calls" edges         │
│  7. Handle TestXxx → create "test_covers" edges             │
└──────────────────────────────────────────────────────────────┘
                                           │
                                           ▼
┌──────────────────────────────────────────────────────────────┐
│  Insert into SQLite                                          │
│  - memory_repo_files (file metadata)                         │
│  - memory_repo_symbols (symbol table)                        │
│  - memory_repo_edges (graph relationships)                   │
│  - memory_provenance (tracking)                              │
└──────────────────────────────────────────────────────────────┘
                                           │
                                           ▼
┌──────────────────────────────────────────────────────────────┐
│  Commit Transaction                                          │
│  - Insert memory_index_epochs                                │
│  - Report progress via callback                              │
└──────────────────────────────────────────────────────────────┘
```

### Boot Packet Compilation

**BootPacket Structure:**

```go
type BootPacket struct {
    Version          string             // "memory.boot.v1"
    GeneratedAt      string             // RFC3339 timestamp
    TaskClass        string             // "debug", "performance", "architecture", "feature"
    RepoSnapshot     BootRepoSnapshot   // Git state, index epoch
    RuntimeState     BootRuntimeState   // Current task, plan, blockers
    Handoff          BootHandoffState   // Previous checkpoint state
    RelevantPolicies []string           // From AGENTS.md, constitution
    GraphSlice       BootGraphSlice     // THE CODEBASE GRAPH
    RequiredReads    []BootRequiredRead // Files agent should read
    WorkingSet       BootWorkingSet     // Task prompt, seeds
    ArtifactPath     string             // .sapphire/state/memory/boot_packets/*.json
}
```

**GraphSlice Structure:**

```go
type BootGraphSlice struct {
    Files   []BootGraphFile   // Max 12 files
    Symbols []BootGraphSymbol // Max 40 symbols
    Edges   []BootGraphEdge   // Max 64 edges
}
```

**Task-Specific Edge Traversal:**

```go
allowedEdge := func(edgeType string) bool {
    switch taskClass {
    case "debug":
        return "calls" || "imports" || "test_covers"
    case "performance":
        return "calls" || "imports" || "config_controls"
    case "architecture":
        return "imports" || "implements" || "test_covers"
    default:
        return "calls" || "imports" || "test_covers" || "implements"
    }
}
```

### Context Injection

**RenderPromptInjection()** compiles and injects the boot packet:

```go
func (c *Compiler) RenderPromptInjection(ctx context.Context, req CompileRequest) string {
    packet, err := c.Compile(ctx, req)
    raw, _ := json.Marshal(packet)
    return "## COMPILED BOOT PACKET\n```json\n" + string(raw) + "\n```"
}
```

Injected as **system message** at top of conversation history.

### Progress Reporting

**WarmProgress** structure for UI updates:

```go
type WarmProgress struct {
    Workspace       string        // Working directory
    Phase           string        // "discovering", "parsing", "persisting", "ready"
    Message         string        // Human-readable status
    Active          bool
    Finished        bool
    FilesDiscovered int
    FilesProcessed  int
    FilesIndexed    int
    Percent         float64       // 0.0 to 1.0
    StartedAt       time.Time
    UpdatedAt       time.Time
    Error           string
}
```

**UI Display:**
- Progress bar with block characters (`█`)
- Shimmer animation during active indexing
- Status indicators (green=ready, amber=stopped, red=failed)
- Metrics: files, chunks, elapsed time

---

## 3. Sub-Agent Orchestration System

### Overview

Sapphire implements a **mature sub-agent system** with explicit lifecycle management, worktree isolation, and agent-to-agent communication.

### Core Files

| File | Lines | Purpose |
|------|-------|---------|
| `internal/agent/subagent_manager.go` | ~1723 | Sub-agent lifecycle management |
| `internal/agent/subagent_control_plane.go` | ~150 | Control plane API wrapper |
| `internal/agent/subagent_coordination.go` | ~300 | Assignment building, prompts |
| `internal/agent/subagent_guardrails.go` | - | Spawn limits, depth validation |
| `internal/agent/subagent_worktree.go` | - | Git worktree isolation |
| `internal/agent/subagent_task_context.go` | ~100 | TASK.md injection |
| `internal/agent/collab_tools.go` | ~600 | Collaboration tool implementations |
| `internal/agent/background_subagents.go` | ~400 | Autonomous background spawning |
| `internal/agent/agent_job_manager.go` | ~200 | CSV-driven batch jobs |

### Sub-Agent Lifecycle States

**Active States:**
- `queued` - Task submitted, awaiting execution
- `starting` - Runtime booting up
- `ready` - Runtime ready, awaiting task
- `waiting_on_mail` - Processing coordination mail
- `retrying` - Attempting retry after failure
- `running` - Actively executing task
- `degraded` - Running but heartbeat stale (30s+)

**Terminal States:**
- `blocked` - Agent reported a blocker
- `timed_out` - Turn exceeded timeout (5 min default)
- `stuck` - Heartbeat stuck (2+ min, 3+ misses)
- `completed` - Task completed successfully
- `error` - Task failed with error
- `closed` - Agent manually terminated

### Collaboration Tools

#### spawn_agent

**Purpose:** Spawn a sub-agent with its own terminal

**Parameters:**
```go
type SpawnAgentParams struct {
    Message          string   // Initial task/prompt
    Items            []string // Structured input items
    Title            string   // Session title
    Isolation        string   // "worktree" or "default"
    Worktree         *bool    // Run in git worktree
    WorktreePath     string   // Custom worktree path
    Branch           string   // Branch name
    WriteManifest    []string // Allowed write paths
    DefinitionOfDone string   // Completion criteria
    Agent            string   // "coder" or "task"
    Model            string   // provider:model
    ReasoningEffort  string   // low/medium/high
    ForkContext      *bool    // Copy parent context
}
```

**Returns:** `agent_id`, `submission_id`, `status`, `work_dir`

#### resume_agent

**Purpose:** Resume a previously spawned sub-agent

**Parameters:** `id`, `message`, `items`

#### send_input

**Purpose:** Send follow-up task to running sub-agent

**Parameters:** `id`, `message`, `items`, `interrupt`

#### wait

**Purpose:** Block until sub-agents reach terminal status

**Parameters:** `ids`, `timeout_ms` (default 60000)

**Returns:** Array of agent status entries

#### collect_result

**Purpose:** Collect final results after wait completes

**Parameters:** `ids`

**Returns:** Array of results with validation status

#### close_agent

**Purpose:** Terminate sub-agent and clean up resources

**Parameters:** `id`

### Configuration Limits

| Constant | Default | Description |
|----------|---------|-------------|
| `AgentMaxDepth` | 2 | Maximum nested sub-agent depth |
| `AgentMaxThreads` | 6 | Maximum concurrent sub-agents |
| `subAgentTurnTimeout` | 5 minutes | Turn execution timeout |
| `subAgentHeartbeatInterval` | 5 seconds | Heartbeat tick |
| `subAgentHeartbeatDegradedAge` | 30 seconds | Degraded threshold |
| `subAgentHeartbeatStuckAge` | 2 minutes | Stuck threshold |

### Worktree Isolation

**Path Pattern:**
```
<repo-root>/.sapphire/worktrees/agent/<assignment-id>/<task-slug>/
```

**Branch Pattern:**
```
agent/<assignment-id>/<task-slug>
```

**Validation Gates:**
1. Git diff check
2. Build verification (auto-detect: go build, npm build, cargo build)
3. Test verification (go test, npm test, cargo test)
4. Lint verification (golangci-lint, npm lint, cargo clippy)
5. Security verification (gosec, npm audit)

**Quarantine:** Failed validation with changes → moved to `.sapphire/worktrees/quarantine/`

### Agent-to-Agent Mailbox

**Tools:**
- `agent_mail_send` - Send durable mail (To, Subject, Body, Priority)
- `agent_mail_inbox` - Check received mail (Limit, ThreadID, LeaseTTL)
- `agent_mail_ack` - Acknowledge delivery (ID, IDs)

**Delivery Semantics:**
- Lease-based with TTL
- Ack-required
- Thread-aware (defaults to assignment ID)
- Priority queue

### CSV-Driven Agent Jobs

**Tools:**
- `spawn_agents_on_csv` - Spawn agents for CSV rows
- `report_agent_job_result` - Report item completion

**Configuration:**
- Default concurrency: 16
- Max concurrency: 64
- Capped by `AgentMaxThreads`

**Worker Prompt:**
```
You are processing one item for a batch agent job.
Job ID: {id}
Item ID: {item_id}
Task instruction: {instruction}
Input row: {row_json}
Expected result schema: {schema}

You MUST call `report_agent_job_result` exactly once.
```

### Supervisor Service

**Autonomous Health Monitoring:**

**Patrol Cycle:** Every 2 minutes

**Verdicts:**
- `healthy` - Agent progressing normally
- `slow` - Progressing slowly (status request triggered)
- `stale` - Heartbeat stale (recovery nudge or reassignment)
- `looping` - Repeated activity pattern detected
- `blocked` - Blocked on dependency
- `orphaned` - Runner disappeared
- `crashed` - Runtime failure
- `completed` - Task completed, awaiting validation

**Thresholds:**
- Startup failure: 45 seconds
- Slow: 5 minutes
- Degraded: 10 minutes
- Stuck: 15 minutes
- Stuck escalation: 20 minutes

**Loop Detection:** 10-window, 5-repeat pattern

---

## 4. Agent Runtime & Execution

### Core Files

| File | Lines | Purpose |
|------|-------|---------|
| `internal/agent/agent.go` | ~2824 | Session agent implementation |
| `internal/agent/coordinator.go` | ~2735 | Agent coordinator, tool building |
| `internal/agent/orchestration_runtime.go` | ~838 | Context building |
| `internal/agent/longhorizon/manager.go` | - | Long-horizon task management |

### SessionAgent.Run() Flow

```
┌─────────────────────────────────────────────────────────────┐
│  1. Context Setup                                           │
│     - Inject session_id, session_mode, runtime_control      │
│     - Set working_dir, write_scope                          │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│  2. Memory Injection (injectTieredMemory)                   │
│     - Check context utilization stage (0-50%+)              │
│     - Fetch: Constitution, Long-Horizon, Historical         │
│     - Compile: Codebase boot packet if stage >= 50%         │
│     - Inject: Codebase index status                         │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│  3. System Prompt Assembly (PrepareStep)                    │
│     - Base system prompt (from template)                    │
│     - Provider prefix                                       │
│     - Active skill context (<active_skill_context>)         │
│     - Python capabilities (Gemini only)                     │
│     - Tiered memory as system messages                      │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│  4. Tool Filtering (buildAgentTools)                        │
│     - Build all available tools (66+)                       │
│     - Filter by agent.AllowedTools whitelist                │
│     - Add MCP tools based on agent.AllowedMCP               │
│     - Coder: Full toolset                                   │
│     - Task: Read-only (no edit/write/agent)                 │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│  5. Stream Execution                                        │
│     - fantasy.Agent.Stream() with callbacks                 │
│     - OnToolCall: Track execution, record to memory         │
│     - OnToolResult: Process, persist, compact               │
│     - Retry logic (max 2 retries, 500ms backoff)            │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│  6. Post-Processing                                         │
│     - Check for context compaction (75% threshold)          │
│     - Trigger summarization if needed                       │
│     - Process queued messages                               │
│     - Validate structured blocks (plan/architect modes)     │
└─────────────────────────────────────────────────────────────┘
```

### Memory Injection Stages

**Context Load Stage Determination:**

```go
func determineContextLoadStage(tokens int64, contextWindow int, postCompaction bool) ContextLoadStage {
    if postCompaction {
        return ContextLoadStage50
    }
    percentUsed := int((tokens * 100) / int64(contextWindow))
    switch {
    case percentUsed >= 50: return ContextLoadStage50
    case percentUsed >= 40: return ContextLoadStage40
    case percentUsed >= 30: return ContextLoadStage30
    case percentUsed >= 20: return ContextLoadStage20
    case percentUsed >= 10: return ContextLoadStage10
    default: return ContextLoadStageCold
    }
}
```

**Injection by Stage:**

| Stage | Utilization | Injected Components |
|-------|-------------|---------------------|
| Cold | 0-10% | No memory injection |
| 10% | 10-20% | Session Snapshot, Active Workstreams |
| 20% | 20-30% | + Constitution, Recent Decisions |
| 30% | 30-40% | + Negative Constraints, Failures |
| 40% | 40-50% | + AI Codebase Graph, Critical Files |
| 50% | 50%+ | + Supporting Files, Full Boot Packet |

### Long-Horizon Task Management

**Artifacts:**
- `frozen_spec.md` - Task specification (immutable)
- `milestones.json` - Milestone plan
- `runbook.md` - Operating procedures
- `audit.log` - Decision audit trail

**Injection:**
```xml
<long_horizon_runbook>
...
</long_horizon_runbook>

<long_horizon_frozen_spec path="...">
...
</long_horizon_frozen_spec>

<long_horizon_milestones path="...">
...
</long_horizon_milestones>

<long_horizon_audit path="...">
...
</long_horizon_audit>
```

### Checkpoint System

**Triggers:**
- Every 50 messages
- Every 30 minutes
- Force flag for explicit checkpoints

**Checkpoint Data:**
```go
type SessionCheckpoint struct {
    SessionID          string
    AgentID            string
    WorkItemID         string
    ParentCheckpointID string
    MessageCount       int
    SummaryJSON        string
    AuditTail          string
    PendingTasksJSON   string
    FilesModifiedJSON  string
    MailCursor         int64
    ActivityCursor     int64
}
```

**Pruning:**
- Keep last 24 checkpoints
- Keep up to 8 high-value older checkpoints (error status or 200-message intervals)

---

## 5. Tools System

### Overview

Sapphire provides **66+ tools** across multiple categories with permission-gated execution.

### Tool Categories

#### File Operations
- `view` - View file content (with indentation analysis)
- `single_view` - View exactly 1 file
- `agentic_view` - View 2-30 files in batch
- `edit` - Single find/replace edit
- `single_edit` - Edit exactly 1 file
- `agentic_edit` - Edit 2-25 files in batch (Go 1.26 optimized)
- `write` - Create/overwrite entire file
- `apply_patch` - Apply unified diff patch
- `ls` - List directory contents
- `glob` - Pattern-based file matching
- `grep` - Content search with ripgrep

#### Shell & Jobs
- `bash` - Execute bash commands (with permission)
- `job_output` - View background job output
- `job_kill` - Terminate background job
- `job_list` - List active jobs
- `job_start` - Start background job
- `download` - Download file from URL

#### Search & Web
- `fetch` - Fetch URL content
- `agentic_fetch` - Fetch multiple URLs
- `web_search` - Web search
- `web_fetch` - Fetch with extraction
- `google_search` - Google Grounding search with DuckDuckGo fallback
- `sourcegraph` - Sourcegraph code search

#### Agent Operations
- `agent` - Spawn sub-agent (legacy)
- `spawn_agent` - Spawn sub-agent with lifecycle
- `resume_agent` - Resume paused sub-agent
- `send_input` - Send follow-up to sub-agent
- `wait` - Wait for sub-agents
- `collect_result` - Collect sub-agent results
- `close_agent` - Close sub-agent
- `spawn_agents_on_csv` - Batch spawn from CSV
- `report_agent_job_result` - Report job item result
- `orchestrate_worktrees` - Multi-worktree orchestration

#### Memory & Context
- `view_memory` - View persistent memory
- `refresh_memory` - Refresh memory context
- `recall_memory` - Search memory
- `save_memory` - Save to memory
- `index_codebase` - Build durable codebase graph (AST-based)
- `tool_suggest` - Suggest relevant tools

#### LSP & Diagnostics
- `lsp_diagnostics` - Get LSP/compiler diagnostics
- `lsp_references` - Find symbol references
- `lsp_restart` - Restart LSP servers

#### MCP Tools
- `list_available_mcps` - Search MCP registry
- `install_mcp` - Install MCP from registry
- `connect_mcp` - Connect installed MCP
- `call_mcp_tool` - Execute MCP tool
- `list_mcp_tools` - List MCP server tools
- `list_mcp_resources` - List MCP resources
- `read_mcp_resource` - Read MCP resource by URI

#### Skills
- `load_skill` - Load skill into context
- `list_skills` - List loaded skills
- `search_skills` - Search skill registry
- `install_skill` - Install skill from marketplace

#### Planning & Coordination
- `update_plan` - Update todo plan
- `request_user_input` - Ask user for input
- `set_mode` - Switch collaboration mode
- `agent_mail_send` - Send mail to agents
- `agent_mail_inbox` - Check agent mailbox
- `agent_mail_ack` - Acknowledge mail delivery
- `agent_directory` - List agent directory

#### Utility
- `list_tools` - List available tools
- `search_tools` - Search tools by description
- `list_available_mcps` - List MCP servers
- `python` - Python code execution (Gemini models only)

### Agent Tool Whitelists

#### AgentCoder (Full Access)
All 66+ tools including:
- Edit tools: `edit`, `single_edit`, `agentic_edit`
- Write tools: `write`, `apply_patch`
- Agent tools: `spawn_agent`, `resume_agent`, etc.
- `index_codebase` ✓

#### AgentTask (Read-Only)
~57 tools, **excluding**:
- `agent`, `spawn_agent`, `resume_agent`, `send_input`, `wait`, `collect_result`, `close_agent`
- `edit`, `single_edit`, `agentic_edit`
- `apply_patch`, `write`

**Includes:** `index_codebase` ✓

### Fast View (Go 1.26 Optimizations)

**Features:**
- Green Tea GC for reduced GC overhead
- 30% faster small string allocations
- errgroup bounded parallelism
- Pre-allocated buffers
- Can inspect up to 250 files in parallel (`agentic_view`)
- Image support with base64 encoding

---

## 6. Collaboration Modes

### Overview

Sapphire supports **7 collaboration modes** that can be switched via `set_mode` tool.

### Available Modes

| Mode | Purpose | Tool Restrictions |
|------|---------|-------------------|
| `default` | Normal coding with full tool access | None |
| `plan` | Read-only planning mode | No edit/write tools |
| `architect` | Structural design and interface reasoning | Focus on architecture |
| `debug` | Root-cause-first debugging | Debug-focused tools |
| `security` | Security analysis and exploitability review | Security-focused |
| `review` | Rigorous diff and behavior review | Read-only with analysis |
| `orchestrator` | Multi-agent execution topology planning | Agent tools enabled |

### Mode Switching

**Tool:** `set_mode`

**Parameters:**
```go
type SetModeParams struct {
    Mode string `json:"mode"`  // One of: default, plan, architect, debug, security, review, orchestrator
}
```

### Mode-Specific Guidance

Each mode provides:
- Mode-specific system prompt
- Tool availability restrictions
- Different reasoning patterns
- Specialized output formats

---

## 7. MCP Integration

### Overview

Sapphire provides comprehensive **Model Context Protocol (MCP)** integration with registry-backed server discovery, async selection, and intent-based recommendation.

### Core Files

| File | Purpose |
|------|---------|
| `internal/agent/tools/mcp/init.go` | Client session management |
| `internal/agent/tools/mcp/tools.go` | Tool execution |
| `internal/agent/tools/mcp/prompts.go` | Prompt management |
| `internal/agent/tools/mcp/resources.go` | Resource management |
| `internal/agent/mcp_registry.go` | Registry integration |
| `internal/agent/mcp_selection.go` | Intent-based selection |
| `internal/agent/mcp_async.go` | Async discovery |
| `internal/agent/mcp_inventory.go` | Inventory context |

### Transport Types

| Type | Description |
|------|-------------|
| `stdio` | Command-based transport |
| `http` | Streamable HTTP transport |
| `sse` | Server-Sent Events transport |

### MCP Categories (Registry Coverage)

- **Cloud Infrastructure** - AWS, GCP, Azure, Kubernetes
- **Authentication** - OAuth, JWT, Auth0
- **Payments** - Stripe, PayPal, Square
- **Databases** - PostgreSQL, MySQL, MongoDB, Redis
- **AI/Vector Search** - Pinecone, Weaviate, Qdrant
- **Development Infra** - GitHub, GitLab, CI/CD
- **Testing/Debugging** - Testing tools, debuggers
- **Design** - Figma, design systems
- **Productivity** - Notion, Slack, email

### MCP Tools

| Tool | Purpose |
|------|---------|
| `list_available_mcps` | Search MCP registry with query/limit |
| `install_mcp` | Install MCP from registry |
| `connect_mcp` | Connect installed MCP server |
| `call_mcp_tool` | Execute MCP tool with arguments |
| `list_mcp_tools` | List tools from connected MCPs |
| `list_mcp_resources` | List resources from specific MCP |
| `read_mcp_resource` | Read resource content by URI |

### Intent-Based Selection

**Ranking Factors:**
- Category keyword matching
- Tag matching
- Token overlap with prompt
- Historical usage

**Async Discovery:**
- Background preflight discovery
- Cached per session/prompt
- Non-blocking selection

### Connection States

| State | Description |
|-------|-------------|
| `disabled` | MCP disabled |
| `starting` | Connection initializing |
| `connected` | Ready for use |
| `error` | Connection failed |

### Pub/Sub Events

- `EventStateChanged` - Connection state changed
- `EventToolsListChanged` - Tool list updated
- `EventPromptsListChanged` - Prompts updated
- `EventResourcesListChanged` - Resources updated

---

## 8. LSP Integration

### Overview

LSP integration uses **github.com/charmbracelet/x/powernap** for lazy-initialized language servers with diagnostic management.

### Core Files

| File | Purpose |
|------|---------|
| `internal/lsp/manager.go` | Lazy server initialization |
| `internal/lsp/client.go` | LSP client wrapper |
| `internal/lsp/handlers.go` | LSP request handlers |

### Lazy Initialization

- Servers start on-demand based on file types
- Auto-detects installed servers via `exec.LookPath()`
- Root marker detection (`go.mod`, `Cargo.toml`, etc.)
- Parallel server initialization

### Server States

| State | Description |
|-------|-------------|
| `unstarted` | Not yet initialized |
| `starting` | Initialization in progress |
| `ready` | Server ready |
| `error` | Initialization failed |
| `stopped` | Gracefully stopped |
| `disabled` | Disabled by config |

### LSP Tools

| Tool | Purpose |
|------|---------|
| `lsp_diagnostics` | Get diagnostics for file/project |
| `lsp_references` | Find symbol references semantically |
| `lsp_restart` | Restart LSP servers |

### Diagnostic Integration

**Compiler Diagnostics:**
- **Go**: `go test -run=^$` (type check)
- **TypeScript**: `tsc --noEmit`
- **Python**: `python -m py_compile`
- **Rust**: `cargo check -q`

**Features:**
- Real-time diagnostic collection
- Severity-based filtering (Error/Warning/Info/Hint)
- Diagnostic counts in header
- File-specific and project-wide views

### LSP Features

- **Find References**: Semantic symbol reference finding
- **Hover Information**: Type signatures, documentation
- **File Management**: Open/close/notify changes
- **Diagnostics**: Cached with versioning
- **Handlers**: workspace/applyEdit, workspace/configuration, client/registerCapability

---

## 9. Persistent Memory System

### Overview

**Separate from graph indexing**, the persistent memory system provides SQLite-backed structured memory with Gemini embeddings and progressive context injection.

### Core Files

| File | Purpose |
|------|---------|
| `internal/memory/system.go` | Main coordinator |
| `internal/memory/store.go` | SQLite store with FTS5 |
| `internal/memory/embedding.go` | Gemini embeddings |
| `internal/memory/pipeline.go` | Background extraction |
| `internal/memory/memory_md.go` | MEMORY.md management |

### Database Schema

**Tables:**
- `memory_records` - Main memory with FTS5 full-text search
- `memory_embeddings` - Semantic vectors (Gemini, 768 dimensions)
- `project_constitution` - Immutable core decisions
- `compaction_checkpoints` - Session compaction state
- `memory_dead_letter` - Failed extractions

### Key Features

- **Project-scoped**: One DB per project (SHA256 hash of root)
- **Session-separated**: Multiple sessions per project
- **FTS5 search**: Automatic full-text search triggers
- **Hybrid retrieval**: FTS + semantic embeddings
- **Temporal decay**: Salience decays over time (except constraints)
- **Deduplication**: `dedup_hash` prevents duplicates

### Background Extraction Pipeline

**Worker Loop:**
1. **Batching**: Collect events for 500ms or up to 5 events
2. **Retry**: 3 retries with exponential backoff (1s-10s)
3. **Fallback**: Switch to secondary extractor after failures
4. **Emergency**: Write high-salience record if failure rate > 30%

**Queue:**
- Size: 1024 events
- Backpressure at 80% capacity
- Stats tracking with rolling failure rate

### MEMORY.md Management

**Refresh Triggers:**
1. First session (no prior refresh)
2. Every 100 turns
3. Major achievement detected + state change
4. Major change + 20+ turns since last refresh
5. Force flag (after codebase changes)

**Structure:**
```markdown
# Sapphire Memory Handbook

## Session Snapshot
## Active Workstreams
## Repo Constitution
## Stable Decisions
## Failures and Guardrails
## Architecture Overview
## AI Codebase Graph
## Critical Files
## Supporting Files
## Provenance
```

### Context Injection by Stage

| Stage | Components | Token Range |
|-------|------------|-------------|
| 10% | Session Snapshot, Active Workstreams | 180-600 |
| 20% | + Constitution, Decisions | 320-900 |
| 30% | + Failures/Guardrails | 500-1400 |
| 40% | + AI Codebase Graph, Critical Files | 700-2000 |
| 50% | + Supporting Files, Full content | Full budget |

### User Preferences Extraction

**Pattern-Based:**
- `i prefer ...` → Preference record
- `my name is ...` → User name
- `always use ...` → Constraint
- `use postgres/sqlite/mysql` → Database decision

**Confidence Levels:**
- `confirmed` - Single clear statement
- `conflicted` - Multiple contradictory statements

---

## 10. UI/TUI Layer

### Overview

Bubble Tea-based TUI with component architecture, shimmer animations, and real-time progress updates.

### Core Files

| File | Purpose |
|------|---------|
| `internal/ui/model/ui.go` | Main UI model (~4415 lines) |
| `internal/ui/model/chat.go` | Chat component with mouse interaction |
| `internal/ui/chat/indexing.go` | Indexing progress display |
| `internal/ui/chat/subagent_tools.go` | Sub-agent tool rendering |
| `internal/ui/shimmer/indexing.go` | Shimmer animation |

### Indexing Progress Display

**IndexingMessageItem:**
- Progress bar with block characters (`█`)
- Color-coded: filled (#A8A29E), remaining (#44403C)
- Shimmer animation during active indexing
- Status messages by phase

**Phases:**
- `discovering` → "Discovering files"
- `preparing` → "Preparing chunks"
- `embedding` → "Embedding chunks"
- `upserting` → "Writing vectors"
- `ready` → Uses progress.Message field

**Status Colors:**
- Active: Shimmer animation
- Canceled: Amber (#F59E0B)
- Failed: Red (#FB7185)
- Complete: White (#E7E5E4)

### Sub-Agent Tool Rendering

**Tool Message Items:**
- `SpawnAgentToolMessageItem`
- `ResumeAgentToolMessageItem`
- `SendInputToolMessageItem`
- `WaitAgentsToolMessageItem`
- `CollectResultToolMessageItem`
- `CloseAgentToolMessageItem`

**Status Icons:**
- `completed` → ✓ (green #9FE3C1)
- `blocked/error/stuck` → ✕ (red #E48AA7)
- `closed` → ○ (gray #B8AFBF)
- `running/starting/ready` → ● spinner (purple #B9A3E8)

**Display Format:**
```
● agent-abc123  worktree/feature-branch  Running  [2m 15s]
✓ agent-def456  worktree/bug-fix  Completed
```

### Todo/Pills Display

**Features:**
- Mini-dot spinner for in-progress todos
- Progress counter: X/Y completed/total
- Current task name (truncated to 40 chars)
- Queue pill with gradient triangles

### Dialog System

**Stack-Based Management:**
- `ModelsDialog` - Model selection
- `SessionsDialog` - Session list
- `CommandsDialog` - Custom commands
- `PermissionsDialog` - Tool approval
- `PlanApprovalDialog` - Plan mode approval
- `MCPManagerDialog` - MCP server management
- `FilePickerDialog` - File selection

### Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `Ctrl+Enter` | Send message |
| `Shift+Enter` | Newline in input |
| `Ctrl+D` | Clear input |
| `Ctrl+K` | Expand selected thinking section |
| `Ctrl+L` | Clear chat |
| `Ctrl+H` | Show history |
| `Ctrl+J/K` | Navigate messages |
| `Esc` | Close dialog |
| `Tab` | Cycle dialog focus |
| `Enter` | Confirm dialog |

---

## 11. Background Services

### Supervisor Service

**File:** `internal/agent/supervisor/service.go`

**Autonomous Patrol:**
- Cycle: Every 2 minutes
- Verdicts: healthy, slow, stale, looping, blocked, orphaned, crashed, completed
- Actions: Recovery nudge, status request, reassignment, escalation, loop break

### Activity Service

**File:** `internal/agent/activity/activity.go`

**Features:**
- Event logging per agent
- Activity feed retrieval
- Recent activity queries
- Used by supervisor for loop detection

### Mailbox Service

**File:** `internal/agent/mailbox/mailbox.go`

**Features:**
- Send/receive messages between agents
- Inbox management (read/unread)
- Message leasing with TTL
- Thread support
- Priority handling
- Nudge mechanism for unread mail

### Background Stop System

**File:** `internal/agent/background_stop.go`

**Cleanup Actions:**
- Stops orchestration services
- Closes all active sub-agents
- Stops background tasks registry
- Blocks dispatches and work items
- Dead-letters mail
- Kills background shells
- Cancels codebase indexing

**Summary Metrics:**
- Closed sub-agents count
- Stopped background tasks count
- Stopped dispatches count
- Blocked work items count
- Dead-lettered mail count
- Killed background shells count

### Orchestration Recovery

**File:** `internal/agent/orchestration_recovery.go`

**Startup Recovery:**
- Expired mail lease requeuing
- Dispatch lease reclamation
- Running dispatch recovery
- Stale mail delivery healing
- Supervisor tracker rehydration
- Resume prompts for interrupted sub-agents

---

## 12. Database Schema

### Orchestration DB (SQLite)

**Tables:**
```sql
-- Agent state tracking
agent_state (
    agent_id, session_id, parent_agent_id,
    role, status, worktree_path,
    hook_bead_id, last_heartbeat
)

-- Agent communication
agent_mail (
    id, thread_id, to_agent, from_agent,
    subject, body, priority, read, leased_until
)

-- Activity audit
agent_activity (
    id, agent_id, event_type, details, created_at
)

-- Work items
work_items (
    id, type, title, description,
    status, assignee, parent_id, dependencies
)

-- Session checkpoints
session_checkpoints (
    id, session_id, agent_id, work_item_id,
    parent_checkpoint_id, message_count,
    summary_json, audit_tail,
    pending_tasks_json, files_modified_json,
    mail_cursor, activity_cursor
)
```

### Memory DB (SQLite)

**Tables:**
```sql
-- Main memory records with FTS5
memory_records (
    id, session_id, project_scope,
    event_type, timestamp, turn_index, salience,
    content_json, is_negative_constraint,
    is_architectural_decision, is_failure_mode,
    dedup_hash UNIQUE
)

-- Semantic embeddings
memory_embeddings (
    record_id, vector BLOB, dimensions,
    session_id, created_at
)

-- Project constitution
project_constitution (
    project_scope PRIMARY KEY,
    content, created_at
)

-- Compaction checkpoints
compaction_checkpoints (
    id, session_id, message_count,
    summary_json, created_at
)

-- Dead-letter queue
memory_dead_letter (
    id, event_json, error, retries, created_at
)
```

### Graph DB (SQLite)

**Tables:**
```sql
-- Repository scopes
memory_repo_scopes (
    id, repo_root, scope_path, branch,
    head_commit, dirty, changed_files_json,
    latest_epoch, last_indexed_at
)

-- Indexed files
memory_repo_files (
    id, scope_id, path, language, role, status,
    content_hash, mod_time_unix, size_bytes,
    symbol_count, imports_json, facts_json
)

-- Symbols (graph nodes)
memory_repo_symbols (
    id, scope_id, file_id, stable_key,
    name, kind, signature, doc,
    start_line, end_line, exported, status, fingerprint
)

-- Edges (graph relationships)
memory_repo_edges (
    id, scope_id, from_file_path, from_symbol_key,
    edge_type, to_file_path, to_symbol_name, to_symbol_key,
    metadata_json
)

-- Index epochs
memory_index_epochs (
    id, scope_id, epoch, head_commit,
    changed_files_json, removed_files_json,
    file_count, status
)
```

---

## 13. Configuration

### Agent Configuration

**File:** `internal/config/config.go`

**Constants:**
```go
const (
    defaultAgentMaxDepth   = 2      // Maximum nested sub-agent depth
    defaultAgentMaxThreads = 6      // Maximum concurrent sub-agents
)
```

**Agent Profiles:**
```go
AgentCoder: {
    ID:           "coder",
    Name:         "Coder",
    Model:        SelectedModelTypeLarge,
    AllowedTools: allTools,  // 66+ tools
}

AgentTask: {
    ID:           "task",
    Name:         "Task",
    Model:        SelectedModelTypeLarge,
    AllowedTools: taskAllowedTools,  // ~57 tools (read-only)
}
```

### Codebase Indexing Configuration

**File:** `internal/codeindex/types.go`

```go
const (
    DefaultEmbeddingModel      = "jina-code-embeddings-1.5b"
    DefaultEmbeddingDimensions = 1024
)
```

**Note:** Jina/Qdrant embedding system is **disabled**. Graph-based indexing uses AST parsing instead.

### MCP Configuration

```go
type MCPConfig struct {
    Type           MCPType  // stdio, http, sse
    Command        string
    Args           []string
    URL            string
    Env            map[string]string
    Headers        map[string]string
    Timeout        int
    Disabled       bool
    DisabledTools  []string
}
```

### LSP Configuration

```go
type LSPConfig struct {
    Command      string
    Args         []string
    Env          map[string]string
    FileTypes    []string
    RootMarkers  []string
    InitOptions  map[string]any
    Options      map[string]any
    Timeout      int
    Disabled     bool
}
```

---

## 14. Quick Reference

### Key File Paths

| Component | Path |
|-----------|------|
| **Graph Indexing** | |
| Indexer | `internal/agent/memory/indexer.go` |
| Compiler | `internal/agent/memory/compiler.go` |
| Schema | `internal/db/migrations/20260325000000_add_durable_memory_graph.sql` |
| **Sub-Agents** | |
| Manager | `internal/agent/subagent_manager.go` |
| Control Plane | `internal/agent/subagent_control_plane.go` |
| Collaboration | `internal/agent/collab_tools.go` |
| **Agent Runtime** | |
| Coordinator | `internal/agent/coordinator.go` |
| Session Agent | `internal/agent/agent.go` |
| Orchestration | `internal/agent/orchestration_runtime.go` |
| **Memory** | |
| Persistent Memory | `internal/memory/system.go` |
| Checkpoints | `internal/agent/memory/checkpoint.go` |
| **MCP/LSP** | |
| MCP Init | `internal/agent/tools/mcp/init.go` |
| LSP Manager | `internal/lsp/manager.go` |

### Tool Names (Key Tools)

| Category | Tools |
|----------|-------|
| **Graph/Indexing** | `index_codebase` |
| **Sub-Agents** | `spawn_agent`, `resume_agent`, `send_input`, `wait`, `collect_result`, `close_agent` |
| **File Ops** | `view`, `single_view`, `agentic_view`, `edit`, `single_edit`, `agentic_edit`, `write` |
| **MCP** | `call_mcp_tool`, `list_mcp_tools`, `connect_mcp`, `install_mcp` |
| **LSP** | `lsp_diagnostics`, `lsp_references`, `lsp_restart` |
| **Memory** | `save_memory`, `recall_memory`, `view_memory` |
| **Modes** | `set_mode` |

### Important Constants

| Name | Value | Purpose |
|------|-------|---------|
| `AgentMaxDepth` | 2 | Max sub-agent nesting |
| `AgentMaxThreads` | 6 | Max concurrent sub-agents |
| `subAgentTurnTimeout` | 5m | Turn execution timeout |
| `defaultGraphFileLimit` | 12 | Max files in graph slice |
| `defaultGraphSymbolLimit` | 40 | Max symbols in graph slice |
| `defaultGraphEdgeLimit` | 64 | Max edges in graph slice |
| `maxRetainedBootPackets` | 48 | Boot packet retention |
| `maxRetainedCheckpoints` | 24 | Checkpoint retention |

### Commands

```bash
# Start TUI
sapphire

# Non-interactive mode
sapphire run "your prompt"

# Index codebase
sapphire run "index the codebase"

# List models
sapphire models

# View logs
sapphire logs -f

# Worktree orchestration
sapphire worktrees orchestrate -s spec.json

# MCP management
sapphire mcp sync
```

---

## Appendix: Recent Additions (Last 30 Days)

### New Tools
- `fast_view` - Go 1.26 optimized parallel file reading
- `google_search` - Google Grounding with DuckDuckGo fallback
- `install_skill` - Install skills from marketplace
- `set_mode` - Switch collaboration modes

### New Services
- **Agent Supervisor** - Autonomous health monitoring
- **Agent Activity** - Activity logging and feed
- **Background Stop** - Comprehensive cleanup system

### New Features
- **7 Collaboration Modes** - default, plan, architect, debug, security, review, orchestrator
- **MCP Registry** - 30+ curated MCP servers across 9 categories
- **MCP Async Selection** - Intent-based server recommendation
- **Memory Compiler** - Durable boot packet caching
- **Codebase Semantic Survey** - Multi-agent indexing strategy

### New Templates
- **Personality Templates** - autonomous.md, SOUL.md
- **Orchestration Templates** - 10 modular prompt files
- **Codebase Indexing Template** - Usage rules and guidelines

---

**Last Updated:** 2026-03-29
**Version:** 2.0 (Complete rewrite with graph indexing and sub-agent orchestration)
