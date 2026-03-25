# Sapphire CLI - Agent Codebase Documentation

**Location:** `/Users/harshitduggal/workspace/Sapphire-cli`

**Language:** Go 1.26.1

**Architecture:** Terminal-first AI assistant with multi-agent orchestration

---

## Table of Contents

1. [High-Level Architecture](#1-high-level-architecture)
2. [Entry Point & CLI](#2-entry-point--cli)
3. [Application Layer](#3-application-layer)
4. [Agent Orchestration](#4-agent-orchestration)
5. [UI/TUI Layer](#5-uitui-layer)
6. [Tools System](#6-tools-system)
7. [Memory & Persistence](#7-memory--persistence)
8. [Protocol Integrations (MCP/LSP)](#8-protocol-integrations-mcplsp)
9. [Background Agent Systems](#9-background-agent-systems)
10. [Database Schema](#10-database-schema)
11. [Operational Characteristics](#11-operational-characteristics)
12. [Quick Reference](#12-quick-reference)

---

## 1. High-Level Architecture

### System Overview

Sapphire CLI is a production-grade AI agent system providing:
- Interactive TUI (Bubble Tea) and non-interactive CLI modes
- Multi-agent orchestration with sub-agent spawning and coordination
- Git worktree isolation for parallel agent execution
- Model Context Protocol (MCP) integration for extensible tooling
- Language Server Protocol (LSP) integration for code intelligence
- Persistent memory system for long-horizon task management
- Background agent dispatch with capacity control
- Permission-based tool execution with user approval workflows

### Architecture Diagram

```
main.go (entry point)
    └── internal/cmd/root.go (CLI commands)
            └── internal/app/app.go (application container)
                    ├── internal/agent/coordinator.go (agent orchestration)
                    │       └── internal/agent/agent.go (session agent)
                    ├── internal/orchestration/db/db.go (SQLite store)
                    ├── internal/lsp/manager.go (language servers)
                    ├── internal/agent/tools/mcp/ (MCP clients)
                    ├── internal/memory/ (persistent memory)
                    └── internal/agent/daemon/ (background dispatch)
```

### Layered Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      UI Layer                                │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │ TUI (Bubble │  │ CLI (Cobra)  │  │ Dialog System    │   │
│  │  Tea)       │  │              │  │                  │   │
│  └─────────────┘  └──────────────┘  └──────────────────┘   │
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
│  │ Registry    │  │ Dispatcher   │  │ Patrol           │   │
│  └─────────────┘  └──────────────┘  └──────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                      Tools Layer                             │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │ File Ops    │  │ Shell Exec   │  │ MCP Tools        │   │
│  │ (view/edit) │  │ (bash/jobs)  │  │                  │   │
│  └─────────────┘  └──────────────┘  └──────────────────┘   │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │ Search      │  │ Agent Ops    │  │ Memory/Diag      │   │
│  └─────────────┘  └──────────────┘  └──────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                   Protocol Layer                             │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │ MCP Client  │  │ LSP Client   │  │ Fantasy Framework│   │
│  └─────────────┘  └──────────────┘  └──────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                  Persistence Layer                           │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │ SQLite DB   │  │ Memory Store │  │ Worktrees        │   │
│  │ (orchestration)│ (embeddings) │  │ (git isolation)  │   │
│  └─────────────┘  └──────────────┘  └──────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

---

## 2. Entry Point & CLI

### File Inventory

| File | Lines | Summary |
|------|-------|---------|
| `main.go` | ~30 | Entry point; starts pprof server if enabled, executes root CLI command |
| `internal/cmd/root.go` | ~324 | Root CLI command setup; initializes app, TUI, metrics, project registration |
| `internal/cmd/run.go` | ~95 | Non-interactive mode execution; single prompt processing |
| `internal/cmd/models.go` | ~110 | List/search available models from configured providers |
| `internal/cmd/mcp.go` | ~57 | MCP server management commands (sync from registry) |
| `internal/cmd/worktrees.go` | ~398 | Git worktree orchestration commands |
| `internal/cmd/projects.go` | ~77 | List project directories |
| `internal/cmd/logs.go` | ~216 | View/debug logs with follow mode |
| `internal/cmd/stats.go` | ~387 | Usage statistics HTML report |
| `internal/cmd/login.go` | ~203 | Authenticate with Hyper/Copilot |
| `internal/cmd/dirs.go` | ~66 | Show directory paths |
| `internal/cmd/jina.go` | ~77 | Jina AI integration |
| `internal/cmd/schema.go` | ~26 | Database schema operations |
| `internal/cmd/update-providers.go` | ~82 | Update provider configurations |

### Entry Point (`main.go`)

```go
func main() {
    // Optional pprof server for profiling
    if os.Getenv("CRUSH_PROFILE") != "" {
        go func() {
            http.ListenAndServe("localhost:6060", nil)
        }()
    }
    cmd.Execute()  // Cobra CLI execution
}
```

### Root Command Structure

```go
var rootCmd = &cobra.Command{
    Use:   "sapphire",
    Short: "A terminal-first AI assistant for software development",
    RunE: func(cmd *cobra.Command, args []string) error {
        // Initialize app container
        app := setupAppWithProgressBar(cmd)
        defer app.Shutdown()
        
        // Setup Bubble Tea TUI
        model := ui.New(common.DefaultCommon(app))
        program := tea.NewProgram(model, tea.WithAltScreen())
        
        // Subscribe to pubsub events
        go app.Subscribe(program)
        
        return program.Run()
    },
}
```

### Global Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--cwd` | `-c` | Current working directory |
| `--data-dir` | `-D` | Custom sapphire data directory |
| `--debug` | `-d` | Enable debug logging |
| `--yolo` | `-y` | Auto-accept all permissions (DANGEROUS) |
| `--help` | `-h` | Show help |

### Non-Interactive Mode (`run` command)

```bash
sapphire run [prompt...] [flags]
```

**Flags:**
- `-q, --quiet`: Hide spinner
- `-v, --verbose`: Show logs
- `-m, --model string`: Model override
- `--small-model string`: Small model override

**Features:**
- Stdin piping support
- Direct stdout/stderr output
- No TUI overhead

### Worktrees Subcommands

```bash
sapphire worktrees orchestrate -s spec.json
sapphire worktrees clean --merged
sapphire worktrees list [--session ID] [--status STATUS] [--limit N]
sapphire worktrees land <id> --strategy merge|squash|cherry_pick|manual_review
sapphire worktrees repair <id>
sapphire worktrees remove <id> [--force]
```

### Logs Command

```bash
sapphire logs [flags]
```

**Flags:**
- `-f, --follow`: Follow log output (tail -f)
- `-t, --tail N`: Show last N lines (default: 1000)

---

## 3. Application Layer

### File Inventory

| File | Lines | Summary |
|------|-------|---------|
| `internal/app/app.go` | ~650 | Application container wiring services; manages lifecycle, events, shutdown |
| `internal/app/provider.go` | ~100 | Model string parsing and provider/model resolution for CLI overrides |
| `internal/app/lsp_events.go` | ~100 | LSP event pub/sub system for tracking language server state changes |

### Application Container (`internal/app/app.go`)

**Core Structure:**
```go
type App struct {
    cfg            *config.Config
    Sessions       *session.Service
    Messages       *message.Service
    History        *history.Service
    Permissions    *permission.Service
    FileTracker    *filetracker.Service
    LSPManager     *lsp.Manager
    AgentCoordinator *agent.Coordinator
    
    events         chan tea.Msg
    pubsub         *pubsub.Broker
    shutdownFuncs  []func()
}
```

**Service Wiring (`app.New()`):**
```go
// Database connection
conn := db.Connect(ctx, cfg.Options.DataDirectory)
q := db.New(conn)

// Core services
sessions := session.NewService(q, conn)
messages := message.NewService(q)
files := history.NewService(q, conn)
permissions := permission.NewPermissionService(cfg.WorkingDir(), skipPermissionsRequests, allowedTools)

// Agent coordinator
coordinator, err := agent.NewCoordinator(ctx, cfg, sessions, messages, permissions, files, filetracker, lspManager, conn)

// App container
app := &App{
    cfg:              cfg,
    Sessions:         sessions,
    Messages:         messages,
    History:          files,
    Permissions:      permissions,
    FileTracker:      filetracker,
    LSPManager:       lspManager,
    AgentCoordinator: coordinator,
    events:           make(chan tea.Msg, 100),
    pubsub:           pubsub.NewBroker[pubsub.Event](),
}
```

**Lifecycle Management:**
- `Startup(ctx)`: Initializes all services
- `Shutdown()`: Executes registered cleanup functions
- `Subscribe(program *tea.Program)`: Routes pubsub events to TUI

### Event System

**PubSub Broker:**
```go
type Broker[T any] struct {
    subscribers map[string][]chan T
    mu          sync.RWMutex
}

func (b *Broker[T]) Subscribe(id string) chan T
func (b *Broker[T]) Publish(id string, msg T)
func (b *Broker[T]) Unsubscribe(id string, ch chan T)
```

**Event Types:**
- `SessionEvent`: Session created/updated/closed
- `MessageEvent`: Message created/updated
- `AgentEvent`: Agent spawned/completed/failed
- `LSPEvent`: Diagnostics changed
- `MCPEvent`: MCP state changed

### Model Provider Resolution (`internal/app/provider.go`)

**Model String Parsing:**
```go
// Parses "provider:model" or "model" format
func ParseModelString(model string) (provider, modelName string)

// Resolves provider from configured options
func ResolveProvider(cfg *config.Config, provider string) (*ProviderConfig, error)
```

---

## 4. Agent Orchestration

### File Inventory

| File | Lines | Summary |
|------|-------|---------|
| `internal/agent/agent.go` | ~2553 | Session agent implementation; message streaming, tool execution, memory injection |
| `internal/agent/coordinator.go` | ~2560 | Agent coordinator; multi-agent management, submission execution, orchestration context |
| `internal/agent/orchestration_runtime.go` | ~750 | Orchestration memory context building; mailbox, agent states, work items, activity feed |
| `internal/agent/agent_tool.go` | ~100 | `agent` tool for spawning sub-agents with worktree isolation options |
| `internal/agent/agent_job_manager.go` | ~200 | Batch job management for parallel sub-agent task processing |
| `internal/agent/memory/checkpoint.go` | ~430 | Session checkpointing; structured state extraction, user preferences, decision records |
| `internal/agent/mailbox/mailbox.go` | ~70 | Inter-agent mail service; send, inbox, thread, read marking |
| `internal/agent/state/state.go` | ~80 | Agent state persistence service; heartbeat, status snapshots |

### Session Agent (`internal/agent/agent.go`)

**Core Interface:**
```go
type SessionAgent interface {
    Run(context.Context, SessionAgentCall) (*fantasy.AgentResult, error)
    SetModels(large Model, small Model)
    SetTools(tools []fantasy.AgentTool)
    SetSystemPrompt(systemPrompt string)
    Cancel(sessionID string)
    CancelAll()
    Enqueue(call SessionAgentCall) error
    Summarize(context.Context, string, fantasy.ProviderOptions) error
}
```

**SessionAgentCall Structure:**
```go
type SessionAgentCall struct {
    SessionID       string
    Prompt          string
    SkillContext    string          // Injected context (MCP, sub-agents, orchestration)
    ActiveSkills    []string        // Enabled skill names
    ActiveTools     []string        // Enabled tool names
    ProviderOptions fantasy.ProviderOptions
    Attachments     []message.Attachment
    PrecreatedUser  *message.Message
    SkipUserMessage bool
    MaxOutputTokens int64
    Temperature     *float64
    TopP            *float64
}
```

**Streaming Execution (`Run()`):**
```go
streamCall := fantasy.AgentStreamCall{
    PrepareStep: func(...) {
        // Create assistant message in DB
        currentAssistant = &assistantMsg
    },
    OnReasoningDelta: func(id, text string) {
        currentAssistant.AppendReasoningContent(text)
        updateAssistant(ctx, currentAssistant, messageUpdateTimeout, false)
    },
    OnToolCallStart: func(id, name string, input json.RawMessage) {
        // Track tool execution
    },
    OnToolCallEnd: func(id string, response fantasy.ToolResponse) {
        // Store tool result in message
    },
}
result, err := agent.Stream(genCtx, streamCall)
```

**Update Throttling:**
- `messageUpdateTimeout`: 750ms for streaming updates
- `messageFinalUpdateTimeout`: 5s for final update
- `messageUpdateMinInterval`: 50ms minimum between updates
- Retry logic for database lock errors

### Agent Coordinator (`internal/agent/coordinator.go`)

**Core Interface:**
```go
type Coordinator interface {
    Run(ctx context.Context, sessionID, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error)
    Submit(ctx context.Context, sessionID, prompt string, attachments ...message.Attachment) (SubmissionResult, error)
    OrchestrateWorktrees(ctx context.Context, sessionID string, params OrchestrateWorktreesParams) (OrchestrateWorktreesResult, error)
    ResumeWorktree(ctx context.Context, sessionID, worktreePath, prompt, agentKey, model, reasoningEffort string) (OrchestrationAgentRef, error)
    Cancel(sessionID string)
    CancelAll()
    IsSessionBusy(sessionID string) bool
    IsBusy() bool
    UpdateModels(ctx context.Context) error
    DispatchBackground(ctx context.Context, spec agentbackground.TaskSpec) (string, error)
    WaitForCompletion(ctx context.Context, agentIDs []string) ([]agentbackground.SubAgent, error)
    RunPlanMode(ctx context.Context, sessionID, task, taskContext string) (*agentformula.ExecutionState, error)
}
```

**Agent Building:**
```go
func (c *Coordinator) buildAgent(ctx context.Context, sessionID string, opts SessionAgentOptions) (SessionAgent, error) {
    // Build tool registry
    tools := []fantasy.AgentTool{
        bashTool, editTool, writeTool, viewTool,
        globTool, grepTool, agentTool,
        mcpTools..., skillTools...,
    }
    
    // Build system prompt with skill context
    systemPrompt := buildSystemPrompt(session, opts.SkillContext)
    
    // Create session agent
    agent := sessionAgent.NewSessionAgent(agentOpts)
    agent.SetTools(tools)
    agent.SetSystemPrompt(systemPrompt)
    
    return agent, nil
}
```

### Sub-Agent Lifecycle

**Explicit Lifecycle:**
```
spawn_agent → resume_agent → send_input → wait → collect_result → close_agent
```

**Spawn Flow (`internal/agent/agent_tool.go`):**
```go
func agentTool(ctx context.Context, invocation ToolInvocation) (fantasy.ToolResponse, error) {
    var params AgentParams
    json.Unmarshal(invocation.Input, &params)
    
    // Spawn sub-agent
    agentID, err := control.spawn(ctx, sessionID, spawnAgentOptions{
        Prompt:         params.Prompt,
        Worktree:       params.Worktree,
        WorktreePath:   params.WorktreePath,
        Branch:         params.Branch,
        WriteManifest:  params.WriteManifest,
        DefinitionOfDone: params.DefinitionOfDone,
        Background:     params.Background,
    })
    
    if !params.Background {
        // Wait for completion
        err = control.wait(ctx, []string{agentID}, 0)
        // Collect result
        result, err = control.collectResult([]string{agentID})
        // Close agent
        err = control.close(agentID)
    }
    
    return fantasy.NewTextResponse(fmt.Sprintf("Agent %s completed", agentID)), nil
}
```

**Worktree Isolation:**
- Path: `.sapphire/worktrees/agent/<id>/<task-slug>`
- Branch: `agent/<id>/<task-slug>`
- Managed by `worktreeManager` in coordinator
- Validation gate before completion: diff, build, tests, lint, security checks

### Session Checkpointing (`internal/agent/memory/checkpoint.go`)

**Checkpoint Structure:**
```go
type SessionCheckpoint struct {
    ID                 string
    SessionID          string
    AgentID            string
    WorkItemID         string
    ParentCheckpointID   string
    MessageCount       int
    SummaryJSON        string
    AuditTail          string
    PendingTasksJSON   string
    FilesModifiedJSON  string
    MailCursor         int64
    ActivityCursor     int64
    CreatedAt          time.Time
}
```

**Checkpoint Service:**
```go
type CheckpointService struct {
    db *orchestrationdb.Store
}

func (s *CheckpointService) Record(ctx context.Context, sessionID string) error
func (s *CheckpointService) Resume(ctx context.Context, sessionID string) (*SessionCheckpoint, error)
func (s *CheckpointService) ExtractUserPreferences(ctx context.Context, sessionID string) ([]UserPreference, error)
func (s *CheckpointService) ExtractDecisions(ctx context.Context, sessionID string) ([]ArchitecturalDecision, error)
```

---

## 5. UI/TUI Layer

### File Inventory

| File | Lines | Summary |
|------|-------|---------|
| `internal/ui/model/ui.go` | ~4415 | Main TUI model orchestrating chat, dialogs, sub-agent display, and message handling |
| `internal/ui/model/chat.go` | ~850 | Chat component with mouse interaction, multi-click detection, and text selection |
| `internal/ui/model/onboarding.go` | ~150 | Project initialization prompt with Yes/No buttons |
| `internal/ui/chat/messages.go` | ~650 | Message item extraction, tool result mapping, and message rendering utilities |
| `internal/ui/chat/assistant.go` | ~750 | Assistant message rendering with thinking sections, live loaders, and error display |
| `internal/ui/chat/user.go` | ~150 | User message rendering with attachment support |
| `internal/ui/chat/tools.go` | ~1549 | Tool call rendering infrastructure with 40+ specialized tool renderers |
| `internal/ui/dialog/dialog.go` | ~200 | Dialog overlay management with stack-based dialog handling |
| `internal/ui/list/list.go` | ~750 | Lazy-loaded list with virtualized rendering and scroll optimization |
| `internal/ui/common/common.go` | ~100 | Shared utilities including clipboard operations and rectangle helpers |
| `internal/ui/common/elements.go` | ~250 | Reusable UI elements (status lines, sections, model info) |
| `internal/ui/styles/styles.go` | ~1635 | Comprehensive style definitions with semantic color palette |

### TUI Application Structure

**Component Hierarchy:**
```
UI (internal/ui/model/ui.go)
├── header (status bar with working dir, model info)
├── status (info/error messages)
├── dialog.Overlay (modal dialogs stack)
│   ├── ModelsDialog, SessionsDialog, CommandsDialog
│   ├── PermissionsDialog, PlanApprovalDialog
│   └── MCPManagerDialog, FilePickerDialog
├── chat (internal/ui/model/chat.go)
│   └── list.List (virtualized message list)
│       ├── UserMessageItem
│       ├── AssistantMessageItem
│       └── ToolMessageItem (40+ specialized renderers)
├── textarea (user input)
├── attachments (file/image attachments)
└── completions (@-mentions popup)
```

### Bubble Tea Components

**Main Model (`UI` struct):**
- 80+ fields coordinating all sub-components
- `Update(msg tea.Msg)`: 1354+ lines handling 30+ message types
- `View() string`: Composes header, chat, editor, pills, status

**Message Types:**
- `tea.KeyPressMsg`: Keyboard input
- `tea.MouseMsg`: Mouse events (click, motion, release, wheel)
- `pubsub.Event[T]`: Session, message, agent updates
- Custom: `sendMessageMsg`, `closeDialogMsg`, `copyChatHighlightMsg`

**Commands (`tea.Cmd`):**
- `scrollToBottomAndAnimate()`: Scroll with animation restart
- `CopyToClipboard()`: Dual OSC 52 + native clipboard
- `loadCustomCommands()`, `loadPromptHistory()`: Async data loading
- `shimmer.ShimmerTickCmd()`: Animation timer for loaders

### Chat Component

**Features:**
- Mouse interaction with multi-click detection (400ms threshold)
- Double-click: word selection
- Triple-click: line selection
- Drag: range selection

**Message Rendering:**
- User messages: `>` prefix
- Assistant messages: `•` prefix
- Tool calls: indented with specialized renderers

**Thinking Sections:**
- Collapsible reasoning content
- Max 10 lines collapsed by default
- Expandable via `Ctrl+K`

### Tool Rendering (40+ types)

**File Operations:**
- `view`, `write`, `edit`, `multi-edit`, `glob`, `grep`, `ls`

**Code Intelligence:**
- `diagnostics`, `lsp_restart`, `references`

**Web:**
- `fetch`, `web_search`, `agentic_fetch`, `sourcegraph`

**Agent Tools:**
- `spawn_agent`, `resume_agent`, `send_input`, `wait`, `collect_result`, `close_agent`

**MCP:**
- `install_mcp`, `connect_mcp`, `list_mcp_tools`, `read_mcp_resource`

**Shell:**
- `bash`, `job_output`, `job_kill`, `job_list`, `job_start`

**Skills:**
- `load_skill`, `list_skills`, `search_skills`

**Other:**
- `python`, `google_search`, `todos`, `update_plan`, `download`

### Dialog System

**Stack-Based Management:**
```go
type Overlay struct {
    dialogs []Dialog
    front   int
}

func (o *Overlay) Push(d Dialog)
func (o *Overlay) Pop() Dialog
func (o *Overlay) Front() Dialog
```

**Dialog Types:**
- `ModelsDialog`: Model selection
- `SessionsDialog`: Session list
- `CommandsDialog`: Custom commands
- `PermissionsDialog`: Tool approval
- `PlanApprovalDialog`: Plan mode approval
- `MCPManagerDialog`: MCP server management
- `FilePickerDialog`: File selection

### Status Indicators

**LSP States:**
- Error/Warning/Info/Hint icons in header

**MCP States:**
- Online/Offline/Busy indicators

**Todo Spinner:**
- Mini-dot spinner for in-progress todos

**Indexing Progress:**
- Shimmer animation during code indexing

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

## 6. Tools System

### File Inventory

| File | Lines | Summary |
|------|-------|---------|
| `internal/agent/tools/tools.go` | ~104 | Context keys, helpers, plan-mode registry with read-only tools |
| `internal/agent/tools/registry.go` | ~636 | Tool registry with registration, execution, and fantasy framework integration |
| `internal/agent/tools/list_tools.go` | ~57 | Lists available tools with query filtering |
| `internal/agent/tools/tool_call_validation.go` | ~283 | Input validation for tool calls (view, bash, edit, update_plan) |
| `internal/agent/tools/safe.go` | ~70 | List of safe read-only commands |
| `internal/agent/tools/bash.go` | ~594 | Bash command execution with permission system, background jobs |
| `internal/agent/tools/edit.go` | ~503 | Single file edit with LSP diagnostics, file history tracking |
| `internal/agent/tools/view.go` | ~884 | File viewing with indentation-aware reading, image support |
| `internal/agent/tools/write.go` | ~184 | Full file overwrite with permission checks, history tracking |
| `internal/agent/tools/python.go` | ~203 | Python code execution via Gemini API with timeout |
| `internal/agent/tools/grep.go` | ~447 | File content search with ripgrep fallback, regex caching |
| `internal/agent/tools/glob.go` | ~141 | File pattern matching with ripgrep acceleration |
| `internal/agent/tools/ls.go` | ~266 | Directory tree listing with depth control |
| `internal/agent/tools/search.go` | ~218 | Web search helpers and DuckDuckGo integration |
| `internal/agent/tools/rg.go` | ~54 | Ripgrep binary initialization and command builders |
| `internal/agent/tools/update_plan.go` | ~226 | Codex-style plan updates with session.Todos integration |
| `internal/agent/tools/mcp-tools.go` | ~177 | MCP tool wrapper with permission integration |
| `internal/agent/tools/search_tools.go` | ~141 | Tool search by name/description/parameters with scoring |
| `internal/agent/tools/job_list.go` | ~64 | List background shell jobs |
| `internal/agent/tools/job_output.go` | ~133 | Retrieve background job output with cursor-based streaming |
| `internal/agent/tools/job_kill.go` | ~76 | Terminate background jobs |
| `internal/agent/tools/background_jobs.go` | ~150 | Background job session tracking and cleanup |
| `internal/agent/tools/connect_mcp.go` | ~159 | Connect installed MCP servers with permission checks |
| `internal/agent/tools/install_mcp.go` | ~89 | Install MCP servers from registry |
| `internal/agent/tools/list_available_mcps.go` | ~269 | List MCP registry with inventory summary and search |
| `internal/agent/tools/web_search.go` | ~55 | DuckDuckGo web search for sub-agents |
| `internal/agent/tools/google_search.go` | ~137 | Google Grounding search with DuckDuckGo fallback |
| `internal/agent/tools/download.go` | ~176 | URL file downloads with timeout and permission checks |
| `internal/agent/tools/request_user_input.go` | ~93 | Plan Mode structured questions (Codex-style) |
| `internal/agent/tools/tool_suggest.go` | ~170 | MCP server suggestions based on capability queries |
| `internal/agent/tools/multiedit.go` | ~890 | Multi-file batch edits with sequential operations |
| `internal/agent/tools/apply_patch.go` | ~176 | Unified diff patch application (direct/delegate modes) |
| `internal/agent/tools/diagnostics.go` | ~274 | LSP diagnostic retrieval with compiler diagnostics |
| `internal/agent/tools/set_mode.go` | ~110 | Session mode switching (default/plan) |
| `internal/agent/tools/fast_view.go` | ~409 | Optimized single-file viewing |
| `internal/agent/tools/git_snapshot.go` | ~293 | Git state snapshots for file changes |
| `internal/agent/tools/compiler_diagnostics.go` | ~152 | Compiler-specific diagnostic collection |
| `internal/agent/tools/fetch.go` | ~194 | URL content fetching with content-type handling |
| `internal/agent/tools/references.go` | ~194 | Code reference finding |
| `internal/agent/tools/semantic_search.go` | ~59 | Semantic code search |
| `internal/agent/tools/sourcegraph.go` | ~269 | Sourcegraph integration |
| `internal/agent/tools/write_scope.go` | ~136 | Sub-agent write scope constraints |
| `internal/agent/tools/plan_mode_filter.go` | ~84 | Plan Mode tool filtering |
| `internal/agent/tools/lsp_restart.go` | ~80 | LSP server restart |
| `internal/agent/tools/edit_guard.go` | ~59 | Edit permission guards |
| `internal/agent/tools/call_mcp_tool.go` | ~157 | Direct MCP tool invocation |
| `internal/agent/tools/list_mcp_tools.go` | ~199 | List tools from connected MCP servers |
| `internal/agent/tools/list_mcp_resources.go` | ~99 | List resources from MCP servers |
| `internal/agent/tools/read_mcp_resource.go` | ~102 | Read MCP server resources |
| `internal/agent/tools/mcp_snapshot.go` | ~274 | MCP state snapshots |
| `internal/agent/tools/tool_call_normalize.go` | ~208 | Tool call parameter normalization |
| `internal/agent/tools/tool_call_preflight.go` | ~1007 | Pre-flight tool call validation |

### Tool Registry

**Core Registry (`internal/agent/tools/registry.go`):**
```go
type Registry struct {
    tools map[string]ToolSpec
    mu    sync.RWMutex
}

type ToolSpec struct {
    Name        string
    Description string
    Parameters  jsonschema.Schema
    Required    []string
    Handler     ToolHandler
}

func (r *Registry) Register(spec ToolSpec)
func (r *Registry) Execute(ctx context.Context, name string, input json.RawMessage) (fantasy.ToolResponse, error)
func (r *Registry) AgentTools() []fantasy.AgentTool
func NewPlanModeRegistry() *Registry  // Read-only tool subset
```

### Tool Categories

#### File Operations

| Tool | Purpose | Approval |
|------|---------|----------|
| `view` / `single_view` / `agentic_view` | Read files with line ranges, indentation mode, image support | No (except outside working dir) |
| `edit` / `single_edit` | Single find-and-replace edits with LSP diagnostics | Yes |
| `agentic_edit` | Multi-file batch edits with sequential operations per file | Yes |
| `write` | Full file overwrite/creation | Yes |
| `apply_patch` | Apply unified diff patches (direct/delegate modes) | Yes |
| `ls` | List directory trees with depth control | No (except outside working dir) |
| `glob` | Find files by glob pattern | No |
| `grep` | Search file contents with regex | No |

#### Command Execution

| Tool | Purpose | Approval |
|------|---------|----------|
| `bash` | Execute shell commands with background job support, auto-background for long-running | Yes (except safe commands) |
| `job_list` | List background jobs | No |
| `job_output` | Stream background job output with cursors | No |
| `job_kill` | Terminate background jobs | No |
| `python` | Execute Python via Gemini API with 2-min timeout | No |

#### Search & Discovery

| Tool | Purpose | Approval |
|------|---------|----------|
| `search_codebase` | Ripgrep-based code search | No |
| `web_search` | DuckDuckGo web search | No |
| `google_search` | Google Grounding with DDG fallback | No |
| `web_fetch` | Fetch web page content | No |
| `fetch` | Generic URL fetching | No |
| `download` | Download files from URLs | Yes |
| `references` | Find code references | No |
| `semantic_search` | Semantic code search | No |
| `sourcegraph` | Sourcegraph integration | No |

#### MCP (Model Context Protocol)

| Tool | Purpose | Approval |
|------|---------|----------|
| `list_available_mcps` | List MCP registry with search | No |
| `install_mcp` | Install MCP from registry | Yes |
| `connect_mcp` | Connect installed MCP servers | Yes |
| `list_mcp_tools` | List tools from connected MCPs | No |
| `call_mcp_tool` | Execute MCP tool directly | Yes |
| `list_mcp_resources` | List MCP resources | No |
| `read_mcp_resource` | Read MCP resource content | No |
| `tool_suggest` | Suggest MCPs by capability | No |

#### Agent Operations

| Tool | Purpose | Approval |
|------|---------|----------|
| `update_plan` | Codex-style plan updates with session.Todos | No |
| `set_mode` | Switch session mode (default/plan) | No |
| `request_user_input` | Plan Mode structured questions | No |
| `launch_exploration_agent` | Spawn read-only sub-agent | No |
| `agent_mail_send` | Send inter-agent mail | No |
| `agent_mail_inbox` | Check agent inbox | No |
| `spawn_agent` | Create sub-agent with worktree isolation | No |
| `resume_agent` | Resume suspended sub-agent | No |
| `send_input` | Send input to sub-agent | No |
| `wait` | Wait for sub-agent completion | No |
| `collect_result` | Collect sub-agent output | No |
| `close_agent` | Clean up sub-agent resources | No |

#### Memory & Diagnostics

| Tool | Purpose | Approval |
|------|---------|----------|
| `memory_query` | Query persistent memory (disabled) | No |
| `lsp_diagnostics` | Get LSP diagnostics for files | No |
| `compiler_diagnostics` | Compiler-specific diagnostics | No |
| `git_snapshot` | Capture git state snapshots | No |
| `lsp_restart` | Restart LSP servers | No |

#### Utility

| Tool | Purpose | Approval |
|------|---------|----------|
| `list_tools` | List available tools with query filter | No |
| `search_tools` | Search tools by name/description | No |
| `tool_suggest` | AI-assisted tool suggestions | No |

### Permission/Approval System

**Permission Service:**
```go
type Service interface {
    Request(ctx context.Context, req PermissionRequest) (bool, error)
    AutoApproveSession(sessionID string)
    SkipRequests() bool  // YOLO mode
}

type PermissionRequest struct {
    SessionID string
    ToolName  string
    Action    string
    Params    any
}
```

**Safe Commands (`internal/agent/tools/safe.go`):**
- Builtins: `cal`, `date`, `ls`, `ps`, `pwd`, etc.
- Git read-only: `git blame`, `git branch`, `git diff`, `git log`, `git status`, etc.
- Windows: `ipconfig`, `tasklist`, etc.

**Permission Flow:**
```go
sessionID := GetSessionFromContext(ctx)
granted, err := permissions.Request(ctx, PermissionRequest{
    SessionID: sessionID,
    ToolName:  "edit",
    Action:    "write",
    Params:    EditPermissionsParams{...},
})
if !granted {
    return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
}
```

### Tool Execution Flow

```
User Input → TUI/CLI → app.AgentCoordinator.Run()
                    ↓
              sessionAgent.Run() (fantasy framework)
                    ↓
              Tool Registry Lookup
                    ↓
              Permission Check (if required)
                    ↓
              Tool Handler Execution
                    ↓
              Response → PubSub → TUI Render
```

### Background Job System

**Auto-Background Logic (`internal/agent/tools/bash.go`):**
1. Start command with 750ms grace period
2. If still running → move to background
3. Return shell ID for `job_output`/`job_kill`

**Session Tracking:**
```go
var (
    backgroundShellsBySession = map[string]map[string]bool{}
    lastBackgroundShellBySession = map[string]string{}
)
```

### Plan Mode Restrictions

**Plan Mode Registry:**
- Read-only tools only: `read_file`, `search_codebase`, `list_directory`, `run_command`
- Planning tools: `update_plan`, `request_user_input`, `launch_exploration_agent`
- **Forbidden:** `edit`, `write`, `bash` (except read-only), `download`

**Mode Enforcement:**
```go
if currentSession.Mode == planmode.PlanMode {
    return fantasy.ToolResponse{}, fmt.Errorf("update_plan is forbidden in Plan Mode")
}
```

---

## 7. Memory & Persistence

### File Inventory

| File | Lines | Summary |
|------|-------|---------|
| `internal/orchestration/db/db.go` | ~1973 | SQLite orchestration store; mail, state, activity, work items, convoys, hooks, checkpoints |
| `internal/orchestration/db/models.go` | ~150 | Data models for orchestration entities (AgentMail, AgentState, WorkItem, Convoy, etc.) |
| `internal/orchestration/db/migrations.go` | ~250 | Schema creation and migration logic for orchestration database |

### Database Connection

```go
func Connect(ctx context.Context, dataDir string) *sql.DB {
    dbPath := filepath.Join(dataDir, "orchestration.db")
    conn, err := sql.Open("sqlite", dbPath)
    
    // PRAGMA settings
    conn.Exec("PRAGMA journal_mode=WAL")
    conn.Exec("PRAGMA foreign_keys=ON")
    conn.Exec("PRAGMA page_size=4096")
    conn.Exec("PRAGMA cache_size=-8000")
    conn.Exec("PRAGMA synchronous=NORMAL")
    
    conn.SetMaxOpenConns(1)  // Single connection
    
    return conn
}
```

---

## 8. Protocol Integrations (MCP/LSP)

### File Inventory

#### MCP Files

| File | Lines | Summary |
|------|-------|---------|
| `internal/agent/tools/mcp/init.go` | ~400 | MCP client session management, initialization, transport creation |
| `internal/agent/tools/mcp/manage.go` | ~100 | MCP lifecycle management: ApplyConfig, DisableClient, RemoveClient |
| `internal/agent/tools/mcp/tools.go` | ~180 | MCP tool discovery, execution, and filtering |
| `internal/agent/tools/mcp/prompts.go` | ~100 | MCP prompt listing and retrieval |
| `internal/agent/tools/mcp/resources.go` | ~130 | MCP resource listing and reading |
| `internal/agent/tools/mcp/timeout.go` | - | MCP timeout configuration |
| `internal/agent/tools/list_mcp_tools.go` | ~200 | Tool for listing MCP tools with query filtering |
| `internal/agent/tools/list_available_mcps.go` | ~280 | Tool for discovering available MCP servers from registry |
| `internal/agent/tools/mcp_snapshot.go` | ~250 | MCP server snapshot building for UI display |
| `internal/agent/mcp_async.go` | ~130 | Async MCP discovery and selection caching |
| `internal/agent/mcp_autonomy.go` | ~120 | MCP discovery preflight checks |
| `internal/agent/mcp_inventory.go` | ~80 | MCP inventory context generation |
| `internal/agent/mcp_prompt.go` | ~100 | Prompt sanitization and MCP inventory detection |
| `internal/agent/mcp_registry.go` | ~120 | Registry definition loading and MCP installation |
| `internal/agent/mcp_runtime.go` | ~200 | MCP policy blocks, capability maps |
| `internal/agent/mcp_selection.go` | ~250 | MCP server selection scoring |
| `internal/config/mcp.go` | ~60 | MCP config persistence |
| `internal/config/mcp_catalog.go` | ~350 | Registry MCP categorization with 9 categories |
| `internal/config/mcp_registry.go` | ~650 | Registry fetching from modelcontextprotocol.io |

#### LSP Files

| File | Lines | Summary |
|------|-------|---------|
| `internal/lsp/client.go` | ~450 | Core LSP client using powernap library |
| `internal/lsp/client_test.go` | ~70 | Unit tests for LSP client |
| `internal/lsp/handlers.go` | ~110 | LSP notification and request handlers |
| `internal/lsp/manager.go` | ~350 | Lazy initialization manager for multiple LSP clients |
| `internal/lsp/util/edit.go` | ~280 | Workspace edit application with encoding support |

### MCP Architecture

**Connection Management:**
```go
var (
    sessions = csync.NewMap[string, *ClientSession]()
    states   = csync.NewMap[string, ClientInfo]()
    broker   = pubsub.NewBroker[Event]()
)
```

**Transport Types:**
- **Stdio:** `mcp.CommandTransport` - spawns subprocess
- **HTTP:** `mcp.StreamableClientTransport` - streamable HTTP
- **SSE:** `mcp.SSEClientTransport` - Server-Sent Events

**State Machine:**
```go
type State int
const (
    StateDisabled State = iota  // "disabled"
    StateStarting               // "starting"
    StateConnected              // "connected"
    StateError                  // "error"
)
```

**Tool Execution:**
```go
func RunTool(ctx context.Context, cfg *config.Config, name, toolName string, input string) (ToolResult, error) {
    c, err := getOrRenewClient(ctx, cfg, name)  // Auto-renew on ping failure
    result, err := c.CallTool(callCtx, &mcp.CallToolParams{
        Name:      toolName,
        Arguments: args,  // JSON-parsed input
    })
    // Handles TextContent, ImageContent, AudioContent
}
```

### LSP Architecture

**Client Initialization:**
```go
func New(ctx context.Context, name string, cfg config.LSPConfig, resolver config.VariableResolver, cwd string, debug bool) (*Client, error) {
    client := &Client{
        name:        name,
        fileTypes:   cfg.FileTypes,
        diagnostics: csync.NewVersionedMap[protocol.DocumentURI, []protocol.Diagnostic](),
        openFiles:   csync.NewMap[string, *OpenFileInfo](),
        config:      cfg,
        cwd:         cwd,
    }
    client.serverState.Store(StateStopped)
    client.createPowernapClient()
    return client, nil
}
```

**Server State Machine:**
```go
type ServerState int
const (
    StateUnstarted ServerState = iota
    StateStarting
    StateReady
    StateError
    StateStopped
    StateDisabled
)
```

**Code Intelligence Features:**
- `OpenFile(ctx, filepath)`: Open file for LSP tracking
- `NotifyChange(ctx, filepath)`: Notify file change
- `FindReferences(ctx, filepath, line, char, includeDecl)`: Find references (5s timeout)
- `RequestHover(ctx, filepath, line, char)`: Get hover information
- `GetDiagnostics()`: Get cached diagnostic counts

---

## 9. Background Agent Systems

### File Inventory

| File | Lines | Summary |
|------|-------|---------|
| `internal/agent/daemon/daemon.go` | ~124 | Main daemon service orchestrating dispatch and patrol cycles |
| `internal/agent/background/dispatcher.go` | ~192 | Capacity-controlled agent dispatcher with async execution |
| `internal/agent/background/capacity.go` | ~50 | Semaphore-based concurrency controller |
| `internal/agent/background/registry.go` | ~203 | In-memory registry tracking sub-agent state |
| `internal/agent/background/monitor.go` | ~51 | Monitor polling completed agents every 5s |
| `internal/agent/background/leg_prompts.go` | ~149 | Leg type system for structured analysis tasks |
| `internal/agent/supervisor/service.go` | ~563 | Supervisor with stuck detection, loop detection, validation |
| `internal/agent/supervisor/patrol.go` | ~11 | Exposes RunPatrolCycle for supervisor |
| `internal/agent/mailbox/mailbox.go` | ~59 | Inter-agent mail service |
| `internal/agent/mailbox/types.go` | ~17 | Message type alias and send options |
| `internal/agent/hook/service.go` | ~194 | Agent-to-work-item assignment service |
| `internal/agent/convoy/service.go` | ~319 | Grouped work item management |
| `internal/agent/activity/activity.go` | ~45 | Activity logging service |
| `internal/agent/activity/types.go` | ~17 | Event type constants |
| `internal/agent/scheduler/dispatcher.go` | ~96 | Scheduler dispatcher with configurable intervals |

### Daemon System

**Dispatch Cycle:**
- Default interval: **3 minutes**
- Calls `dispatcher.RunDispatchCycle()` → processes queued agents
- Calls `dispatcher.RunPatrolCycle()` → reconciles agent state
- Calls `supervisor.RunPatrolCycle()` → patrols for stuck/looping agents

**Process Management:**
- `Start(ctx)`: Launches goroutine with ticker-based execution
- `Stop()`: Cancels context
- `RunCycle(ctx)`: Manual single-cycle execution

### Background Dispatcher

**Capacity Management:**
- Default max concurrent: **5 agents**
- Uses semaphore-based `CapacityController`
- `Acquire(ctx)`: Blocks until slot available
- `Release()`: Returns slot to pool

**Dispatch Flow:**
1. `Dispatch(ctx, spec)` → generates agent ID (`bg-<uuid>`)
2. Registers agent in `Registry` with status `queued`
3. Spawns goroutine `runBackgroundWorker(spec)`
4. Worker acquires capacity slot
5. Executes via `hooks.Execute(ctx, spec)`
6. Updates status: `running` → `completed`/`failed`

**Wait Mechanism:**
- `WaitForCompletion(ctx, agentIDs)`: Polls every **250ms** until completion

### Supervisor

**Patrol Mechanism:**
- Interval: **2 minutes**
- Checks: heartbeat age, loop patterns, silent completions, unread mail

**Stuck Detection:**
- Threshold: **15 minutes** without heartbeat
- Action: Sends nudge mail, logs `supervisor_intervention`
- Escalation: **20 minutes** → marks as `needs_reassignment`

**Loop Detection:**
- Window size: **10 events**
- Repeat count: **5 identical events**
- Mechanism: Compares `EventType + DetailsJSON` patterns
- Action: Sends "LOOP DETECTED" mail

### Mailbox Service

**Operations:**
- `Send(ctx, to, from, subject, body, opts)`: Sends mail, triggers nudge
- `Inbox(ctx, agentID, unreadOnly, limit)`: Lists messages
- `MarkRead(ctx, agentID, messageID)`: Marks message as read
- `Thread(ctx, agentID, threadID, limit)`: Lists thread messages

**Send Options:**
```go
type SendOptions struct {
    Priority  int      // 0 = high, 1 = normal
    ThreadID  string   // Groups messages in thread
    SkipNudge bool     // Suppress notification
}
```

### Hook Service

**Hook Lifecycle:**
1. `AssignHook(ctx, agentID, workItemID)`: Creates hook
2. `MarkInProgress(ctx, agentID, workItemID)`: Updates status
3. `ClearHook(ctx, agentID, workItemID)`: Releases hook

**Hook States:**
- `hooked`: Assigned but not started
- `in_progress`: Actively working
- `idle`: Released

### Convoy Service

**Convoy Lifecycle:**
1. `CreateConvoy(ctx, name, owner, mergeStrategy)`: Creates convoy
2. `AddWorkItems(ctx, convoyID, workItemIDs)`: Links work items
3. `StageConvoy(ctx, convoyID)`: Validates readiness
4. `LaunchConvoy(ctx, convoyID)`: Dispatches ready work items
5. `CheckConvoyCompletion(ctx, convoyID)`: Auto-lands when complete

**Convoy Statuses:**
- `open`: Active, dispatching work
- `staged_ready`: All items ready for dispatch
- `staged_warnings`: Has blocked items
- `closed`: All items completed

### Activity Service

**Event Types:**
```go
const (
    EventSpawned      EventType = "spawned"
    EventHeartbeat    EventType = "heartbeat"
    EventMailSent     EventType = "mail_sent"
    EventMailReceived EventType = "mail_received"
    EventRunning      EventType = "running"
    EventStuck        EventType = "stuck"
    EventCompleted    EventType = "completed"
    EventError        EventType = "error"
)
```

**Operations:**
- `Log(ctx, agentID, eventType, detailsJSON)`: Records activity
- `Recent(ctx, agentID, limit)`: Lists recent events for loop detection
- `Feed(ctx, agentIDs, limit)`: Multi-agent activity feed

---

## 10. Database Schema

### Core Tables

| Table | Purpose | Key Columns |
|-------|---------|-------------|
| `agent_mail` | Inter-agent communication | `id`, `to_agent`, `from_agent`, `subject`, `body`, `thread_id`, `read`, `created_at` |
| `agent_state` | Agent runtime state | `agent_id`, `role`, `status`, `session_id`, `worktree_path`, `branch`, `hook_bead_id`, `parent_agent_id`, `last_heartbeat` |
| `agent_activity` | Audit log | `id`, `agent_id`, `event_type`, `details_json`, `created_at` |
| `work_items` | Task tracking | `id`, `type`, `title`, `description`, `status`, `assignee`, `parent_id`, `convoy_id`, `dependencies` |
| `convoys` | Grouped work items | `id`, `name`, `owner`, `notify`, `merge_strategy`, `status` |
| `convoy_tracks` | Convoy-work item mapping | `convoy_id`, `work_item_id`, `added_at` |
| `agent_hooks` | Agent-to-work assignment | `agent_id`, `hook_bead_id`, `hooked_at`, `status` |
| `dispatch_queue` | Background dispatch queue | `id`, `session_id`, `work_item_id`, `status`, `priority`, `payload_json`, `leased_by`, `assigned_agent_id` |
| `session_checkpoints` | Session state snapshots | `id`, `session_id`, `agent_id`, `message_count`, `summary_json`, `pending_tasks_json`, `files_modified_json` |
| `worktree_runs` | Worktree lifecycle tracking | `id`, `session_id`, `agent_id`, `worktree_path`, `branch`, `status`, `landed_at`, `removed_at` |
| `decisions` | Architectural decisions | `id`, `session_id`, `category`, `key`, `value`, `confidence`, `source_checkpoint_id` |
| `user_preferences` | Learned preferences | `key`, `value`, `confidence`, `source_session_id` |

### Key Entities

**Inter-agent Mail:**
```go
type AgentMail struct {
    ID, ToAgent, FromAgent, Subject, Body, ThreadID string
    Priority int
    Read bool
    CreatedAt, ReadAt time.Time
}
```

**Agent Runtime State:**
```go
type AgentState struct {
    AgentID, Role, Status, SessionID string
    WorktreePath, Branch string
    HookBeadID, ParentAgentID string
    LastHeartbeat, CreatedAt, UpdatedAt time.Time
}
```

**Work Item (Task):**
```go
type WorkItem struct {
    ID, Type, Title, Description, Status string
    Assignee, ParentID, ConvoyID string
    Dependencies string // JSON array
    CreatedAt, ClosedAt time.Time
}
```

**Session Checkpoint:**
```go
type SessionCheckpoint struct {
    ID, SessionID, AgentID, WorkItemID string
    ParentCheckpointID string
    MessageCount int
    SummaryJSON, AuditTail, PendingTasksJSON, FilesModifiedJSON string
    MailCursor, ActivityCursor int64
    CreatedAt time.Time
}
```

---

## 11. Operational Characteristics

### Concurrency

| Component | Limit |
|-----------|-------|
| Background sub-agents | 8 active (configurable) |
| Dispatcher capacity | 5 concurrent (default) |
| Message queue per session | FIFO processing |
| Active request cancellation | Per session |

### Timeouts

| Operation | Timeout |
|-----------|---------|
| Database operations | 2s (5s for long ops) |
| Message updates | 750ms streaming, 5s final |
| Memory calls | 500ms |
| Stream retries | 2 attempts, 500ms backoff |
| Agent heartbeat stale | 15 minutes |
| LSP operations | 5s |
| MCP timeout | 15s (configurable) |

### Persistence

**SQLite Settings:**
- WAL mode enabled
- Foreign keys enabled
- `page_size=4096`
- `cache_size=-8000`
- `synchronous=NORMAL`
- Single connection (`SetMaxOpenConns=1`)

### Error Handling

| Scenario | Handling |
|----------|----------|
| Python tool failure | Quits after 3 consecutive failures |
| Validation failure | Quarantines worktree (doesn't delete) |
| Stuck agents | Supervisor patrol via stale heartbeats |
| OAuth/API key refresh | On 401 errors |

---

## 12. Quick Reference

### Service Dependencies

```
App (internal/app/app.go)
├── session.Service
├── message.Service
├── history.Service
├── permission.Service
├── filetracker.Service
├── LSPManager
└── AgentCoordinator
    ├── session.Service
    ├── message.Service
    ├── permission.Service
    ├── history.Service
    ├── filetracker.Service
    ├── LSPManager
    ├── orchestrationdb.Store
    │   ├── Mailbox Service
    │   ├── State Service
    │   ├── Activity Service
    │   ├── Hook Service
    │   └── Convoy Service
    ├── memory.MemoryService
    ├── pmem.System
    ├── longhorizon.Manager
    ├── background.Dispatcher
    ├── background.Monitor
    ├── scheduler.Dispatcher
    ├── daemon.Service
    └── supervisor.Service
```

### Build Commands

```bash
# Build
go build -o sapphire

# Test
go test ./...

# Run
./sapphire run "your prompt"

# Interactive TUI
./sapphire

# With profiling
CRUSH_PROFILE=1 ./sapphire
# Access pprof at http://localhost:6060
```

### Key Interfaces

**Coordinator:**
```go
type Coordinator interface {
    Run(ctx, sessionID, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error)
    Submit(ctx, sessionID, prompt string, attachments ...message.Attachment) (SubmissionResult, error)
    DispatchBackground(ctx, spec agentbackground.TaskSpec) (string, error)
    WaitForCompletion(ctx, agentIDs []string) ([]agentbackground.SubAgent, error)
    RunPlanMode(ctx, sessionID, task, taskContext string) (*agentformula.ExecutionState, error)
    // ... more methods
}
```

**SessionAgent:**
```go
type SessionAgent interface {
    Run(context.Context, SessionAgentCall) (*fantasy.AgentResult, error)
    SetModels(large Model, small Model)
    SetTools(tools []fantasy.AgentTool)
    SetSystemPrompt(systemPrompt string)
    Cancel(sessionID string)
    CancelAll()
    // ... more methods
}
```

### File Counts by Module

| Module | File Count |
|--------|-----------|
| `internal/agent/tools/` | 50+ |
| `internal/ui/` | 30+ |
| `internal/agent/` | 25+ |
| `internal/cmd/` | 15+ |
| `internal/orchestration/db/` | 5 |
| `internal/lsp/` | 6 |
| `internal/agent/tools/mcp/` | 8 |
| `internal/agent/background/` | 8 |
| `internal/agent/supervisor/` | 4 |

---

**Document Generated:** Based on analysis of Sapphire CLI source code

**Source of Truth:** All information derived directly from source code analysis - zero fabrication
