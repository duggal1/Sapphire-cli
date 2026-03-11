\*\*\*\*# Sapphire CLI Codebase Reference

## Overview

Sapphire is a terminal-first AI coding assistant built in Go. It provides an interactive TUI for AI-powered code development with features including multi-agent orchestration, persistent memory, LSP integration, and comprehensive file operations.

**Language**: Go 1.26.0  
**Primary Dependencies**: Bubble Tea (TUI), Fantasy (agent framework), SQLite (persistence), PowerNap (LSP client)

---

## Table of Contents

1. [Core Architecture](#core-architecture)
2. [Main Entry Point](#main-entry-point)
3. [Agent System](#agent-system)
4. [Agent Tools](#agent-tools)
5. [Persistent Memory System](#persistent-memory-system)
6. [LSP Integration](#lsp-integration)
7. [Database Layer](#database-layer)
8. [Session Management](#session-management)
9. [Configuration System](#configuration-system)
10. [TUI Architecture](#tui-architecture)
11. [Shell Management](#shell-management)
12. [Permission System](#permission-system)
13. [File Operations](#file-operations)
14. [Search and Discovery](#search-and-discovery)
15. [Skill System](#skill-system)
16. [MCP Integration](#mcp-integration)
17. [Event System](#event-system)
18. [Utility Packages](#utility-packages)

---

## Core Architecture

### Modular Monolith Structure

The codebase follows a modular monolith architecture with distinct internal packages:

```
internal/
├── agent/          # Core agent orchestration
├── app/            # Application lifecycle
├── cmd/            # CLI commands
├── commands/       # User commands
├── config/         # Configuration loading
├── csync/          # Concurrent-safe collections
├── db/             # Database layer
├── diff/           # Diff generation
├── env/            # Environment handling
├── event/          # Event logging
├── filetracker/    # File read tracking
├── format/         # Output formatting
├── fsext/          # Filesystem extensions
├── history/        # File version history
├── home/           # Home directory utilities
├── llm/            # LLM providers
├── log/            # Logging setup
├── lsp/            # Language Server Protocol
├── memory/         # Persistent memory
├── message/        # Message types
├── oauth/          # OAuth authentication
├── permission/     # Permission management
├── projects/       # Project management
├── pubsub/         # Event broker
├── runtimeopt/     # Runtime options
├── session/        # Session management
├── shell/          # Shell execution
├── skills/         # Skill discovery
├── stringext/      # String utilities
├── ui/             # Terminal UI
├── update/         # Update checking
└── version/        # Version info
```

### Key Architectural Patterns

- **Dependency Injection**: Services constructed with explicit dependencies
- **Interface-Based Design**: Services defined via interfaces for testability
- **Publish/Subscribe**: Decoupled communication via `pubsub.Broker`
- **Thread-Safe Collections**: `csync` package for concurrent data structures
- **Context Propagation**: `context.Context` passed through all operations

---

## Main Entry Point

### `main.go`

**Purpose**: Application entry point with optional profiling support.

**Key Logic**:

- Checks `CRUSH_PROFILE` environment variable
- Starts pprof HTTP server on `localhost:6060` if profiling enabled
- Executes root Cobra command via `cmd.Execute()`

**Dependencies**:

- `github.com/charmbracelet/sapphire/internal/cmd`
- `github.com/joho/godotenv/autoload` (auto-loads `.env` files)

---

## Agent System

### `internal/agent/agent.go`

**Purpose**: Core session-based AI agent implementation.

**Key Types**:

- `SessionAgent` (interface): Agent interface for session management
- `sessionAgent` (struct): Concrete implementation
- `SessionAgentCall`: Parameters for agent execution
- `Model`: Wrapper for language model configuration

**Key Methods**:

- `Run(ctx, call)`: Executes agent with session context
- `Summarize(ctx, sessionID, opts)`: Compresses conversation history
- `SetModels(large, small)`: Configures model pair
- `SetTools(tools)`: Registers available tools
- `SetSystemPrompt(prompt)`: Sets system instructions
- `Cancel(sessionID)`: Cancels active request
- `CancelAll()`: Cancels all requests

**Auto-Summarization**:

- Triggered when context window approaches limit
- Large window threshold: 200K tokens with 20K buffer
- Small window ratio: 20% of context window
- Creates narrative summary via specialized agent
- Extracts structured data for persistent memory

**Message Flow**:

1. Fetches session messages from database
2. Builds prompt with history and attachments
3. Injects tiered memory context
4. Streams LLM response with tool calls
5. Handles tool execution and results
6. Updates session usage statistics
7. Triggers summarization if needed

### `internal/agent/coordinator.go`

**Purpose**: Multi-agent orchestration and coordination.

**Key Types**:

- `Coordinator` (interface): Main coordination interface
- `coordinator` (struct): Implementation managing multiple agents

**Key Responsibilities**:

- Creates and manages coder/task agents
- Handles OAuth token refresh on 401 errors
- Resolves API keys from configuration
- Builds agent tools dynamically
- Manages LSP integration
- Coordinates skill loading

**Agent Types**:

- **Coder Agent**: Primary coding assistant with full tool access
- **Task Agent**: Sub-agent for delegated tasks with limited context

**Model Configuration**:

- Merges model options from multiple sources
- Supports OpenAI, Anthropic, Gemini, Azure, OpenRouter, Vercel
- Handles provider-specific options (reasoning_effort, thinking mode)

### `internal/agent/event.go`

**Purpose**: Event emission for agent activities.

**Events Emitted**:

- `PromptSent`: When prompt is sent to model
- `PromptResponded`: When response is received (includes duration)
- `TokensUsed`: Token usage statistics with cost

**Event Fields**:

- Session ID, provider, model
- Reasoning effort, thinking mode
- Input/output/cache tokens
- Request duration

### `internal/agent/errors.go`

**Purpose**: Agent-specific error definitions.

**Errors**:

- `ErrRequestCancelled`: User canceled request
- `ErrSessionBusy`: Session processing another request
- `ErrEmptyPrompt`: Prompt is empty
- `ErrSessionMissing`: Session ID missing

### `internal/agent/loop_detection.go`

**Purpose**: Detects agent tool call loops.

**Algorithm**:

- Examines last 10 steps (window size)
- Computes SHA-256 hash of tool call + result pairs
- Triggers if any signature repeats more than 5 times
- Prevents infinite tool execution loops

**Signature Computation**:

- Pairs tool calls with results by ToolCallID
- Hashes tool name, input, and output
- Returns hex-encoded SHA-256

### `internal/agent/indexer.go`

**Purpose**: Proactive codebase indexing for memory.

**Key Features**:

- Runs every 5 minutes
- Walks working directory for code files
- Extracts symbols via regex (func, type, class, interface)
- Queries LSP for hover documentation
- Stores symbol knowledge in memory database

**Indexed Extensions**:

- `.go`, `.ts`, `.tsx`, `.js`, `.py`

**Skipped Directories**:

- `.git/`, `vendor/`, `node_modules/`

### `internal/agent/prompts.go`

**Purpose**: System prompt template management.

**Templates**:

- `coder.md.tpl`: Primary coding agent instructions
- `task.md.tpl`: Task-focused agent instructions
- `summary.md`: Context compression prompt
- `title.md`: Session title generation

**Template Variables**:

- Working directory
- Provider/model info
- Available tools
- Skill context

### `internal/agent/agent_tool.go`

**Purpose**: Agent-to-agent delegation tool.

**Tool Name**: `agent`

**Parameters**:

- `prompt`: Task for sub-agent

**Execution Flow**:

1. Creates task agent with specialized prompt
2. Creates child session linked to parent
3. Executes sub-agent with isolated context
4. Returns result to parent agent

**Use Cases**:

- Codebase mapping
- Dependency tracing
- Risk review
- Fact auditing

### `internal/agent/skill_tool.go`

**Purpose**: Skill discovery and loading tools.

**Tools**:

- `list_skills`: Lists all available skills
- `load_skill`: Loads specific skill instructions

**Skill Matching**:

- Exact name match (case-insensitive)
- Folder name match
- Fuzzy substring match

**Skill Categories**:

- `frontend`: React, TypeScript, UI/UX
- `backend`: Go, Node.js, APIs, databases
- `debug`: Error investigation, bug fixes
- `architect`: System design, patterns
- `devops`: Deployment, CI/CD, containers
- `security`: Auth, vulnerabilities

**Keyword Mapping**:

- Hardwired keyword patterns for deterministic matching
- Whole-word matching to avoid false positives
- Category aliases for flexible matching

### `internal/agent/agentic_fetch_tool.go`

**Purpose**: Parallel URL fetching with fallback strategies.

**Features**:

- Concurrent URL fetching
- Multiple fallback URLs
- Content type detection
- Error aggregation

---

## Agent Tools

### `internal/agent/tools/tools.go`

**Purpose**: Tool context key definitions.

**Context Keys**:

- `SessionIDContextKey`: Session ID in context
- `MessageIDContextKey`: Message ID in context
- `SupportsImagesContextKey`: Model image support flag
- `ModelNameContextKey`: Model name in context

### `internal/agent/tools/edit.go`

**Purpose**: Single file editing tool.

**Tool Name**: `edit`

**Parameters**:

- `file_path`: Target file path
- `old_string`: Text to replace
- `new_string`: Replacement text
- `replace_all`: Replace all occurrences

**Key Features**:

- Character-perfect matching (whitespace-sensitive)
- File creation with empty `old_string`
- Content deletion with empty `new_string`
- Pre-edit validation (file must be read first)
- Modification detection (rejects if file changed)
- Line ending preservation (CRLF/LF)
- Diff statistics (additions/removals)

**Error Messages**:

- `old_string not found`: Precision violation
- `old_string matches multiple`: Ambiguity violation

**LSP Integration**:

- Notifies LSP clients after edit
- Waits for updated diagnostics
- Appends diagnostics to response

### `internal/agent/tools/multiedit.go`

**Purpose**: Parallel multi-file editing.

**Tool Name**: `agentic_edit`

**Parameters**:

- `file_edits`: Array of file edit operations
- Each file can have multiple sequential edits

**Concurrency**:

- Up to 25 files edited in parallel
- Uses `errgroup` for structured concurrency
- Sequential edits within each file

**Features**:

- Partial success handling
- Escape confusion auto-remediation
- Creation with chained edits
- Per-file edit tracking (applied/failed)

**Validation**:

- Only first edit can have empty `old_string`
- File creation requires empty `old_string` in first edit

### `internal/agent/tools/view.go`

**Purpose**: File reading tool with parallel support.

**Tool Names**: `view`, `agentic_view`

**Parameters**:

- `file_paths`: Array of file paths (parallel)
- `file_path`: Single file path (legacy)
- `offset`: Start line (0-based)
- `limit`: Number of lines (default 2000)

**Key Features**:

- Parallel file reading (configurable concurrency)
- Line-numbered output
- Image support (base64 encoding)
- Literal escape detection (`\n`, `\t`)
- Skill file recognition
- Outside working directory access (permission-gated)

**Limits**:

- Max file size: 25MB
- Default read limit: 2000 lines
- Max line length: 2000 characters

**Image Formats**:

- JPEG, PNG, GIF, WebP

**Line Scanner**:

- 1MB buffer for large lines
- Handles minified files

### `internal/agent/tools/write.go`

**Purpose**: File creation/overwrite tool.

**Tool Name**: `write`

**Parameters**:

- `file_path`: Target file path
- `content`: Full file content

**Key Features**:

- Creates parent directories
- Overwrites existing files
- Pre-write validation (file must be read)
- Diff generation
- LSP notification

**Differences from `edit`**:

- Writes entire file content
- No character-perfect matching
- Simpler but less precise

### `internal/agent/tools/bash.go`

**Purpose**: Shell command execution.

**Tool Name**: `bash`

**Parameters**:

- `command`: Shell command to execute
- `description`: Brief command description
- `working_dir`: Execution directory
- `run_in_background`: Background execution flag

**Key Features**:

- POSIX shell emulation (mvdan.cc/sh/v3)
- Background job management
- Auto-background detection (1-minute threshold)
- Fast failure detection (100ms check)
- Output truncation (30K characters)
- CWD tracking

**Blocked Commands**:

- System: `sudo`, `su`, `doas`
- Package managers: `apt`, `yum`, `pacman`, `brew`, `npm install -g`
- Network: `curl`, `wget`, `ssh`, `scp`
- System modification: `mount`, `fdisk`, `mkfs`

**Argument-Based Blocking**:

- `npm install --global`
- `pip install --user`
- `go test -exec`

**Safe Commands** (no permission prompt):

- `ls`, `cat`, `grep`, `find`
- `git status`, `git diff`
- `pwd`, `echo`, `head`, `tail`

**Background Jobs**:

- Max 120 concurrent jobs
- 8-hour retention
- Ring buffer output (10K lines)
- Job ID tracking

### `internal/agent/tools/todos.go`

**Purpose**: Structured task management.

**Tool Name**: `todos`

**Parameters**:

- `todos`: Array of todo items

**Todo States**:

- `pending`: Not started
- `in_progress`: Currently working
- `completed`: Finished

**Todo Fields**:

- `content`: Task description (imperative form)
- `status`: Current state
- `active_form`: Present-continuous form (e.g., "Running tests")

**Response Metadata**:

- Total count
- Completed count
- Just completed tasks
- Just started task
- Is new list flag

**Persistence**:

- Stored in session database
- Survives context compaction

### `internal/agent/tools/diagnostics.go`

**Purpose**: LSP diagnostic retrieval.

**Tool Name**: `lsp_diagnostics`

**Parameters**:

- `file_path`: Optional specific file

**Features**:

- Real-time compiler errors
- File and project scope
- Severity classification (Error, Warning, Hint, Info)
- Source attribution
- Code and tag metadata

**LSP Coordination**:

- Notifies all relevant LSP clients
- Waits for updated diagnostics (2s timeout)
- Aggregates diagnostics from all clients

**Output Format**:

- File diagnostics section
- Project diagnostics section
- Summary statistics

### `internal/agent/tools/references.go`

**Purpose**: Semantic symbol references.

**Tool Name**: `lsp_references`

**Parameters**:

- `symbol`: Symbol name to search
- `path`: Optional search directory

**Features**:

- Multi-LSP coordination
- Location grouping by file
- Precise positioning (line, column)
- Qualified symbol support (`Class::method`, `obj.method`)

**Symbol Offset Detection**:

- Handles `::` (Rust, C++, PHP)
- Handles `.` (Go, Python, JavaScript)
- Handles `\` (PHP namespaces)

**Fallback**:

- Grep search when LSP unavailable

### `internal/agent/tools/grep.go`

**Purpose**: Regex-based content search.

**Tool Name**: `grep`

**Parameters**:

- `pattern`: Regular expression
- `include`: File pattern (glob)
- `exclude`: Exclusion pattern

**Features**:

- Ripgrep acceleration (when available)
- Pure Go fallback
- Ignore file support (`.gitignore`, `.crushignore`)
- Result truncation (100 matches)
- Contextual output (line numbers, positions)

### `internal/agent/tools/glob.go`

**Purpose**: File path pattern matching.

**Tool Name**: `glob`

**Parameters**:

- `pattern`: Glob pattern

**Features**:

- Ripgrep integration (`rg --files`)
- Doublestar fallback
- Hidden file skipping
- Result limiting (100 files)

### `internal/agent/tools/search.go`

**Purpose**: Embedding-based codebase search.

**Tool Name**: `codebase_search`

**Features**:

- Semantic code search
- Symbol indexing
- File path search
- Documentation search

### `internal/agent/tools/fetch.go`

**Purpose**: URL content retrieval.

**Tool Name**: `fetch`

**Parameters**:

- `url`: Target URL
- `output`: Output format (text, markdown, HTML)
- `timeout`: Request timeout

**Features**:

- HTML to markdown conversion
- Text extraction
- Response size limit (5MB)
- UTF-8 validation
- Permission gating

### `internal/agent/tools/web_search.go`

**Purpose**: Web search via DuckDuckGo.

**Tool Name**: `web_search`

**Parameters**:

- `query`: Search query
- `num_results`: Result count (1-20)

**Features**:

- No API key required
- Randomized headers
- Rate limiting (500-2000ms delay)
- HTML parsing for lite.duckduckgo.com

### `internal/agent/tools/download.go`

**Purpose**: File download tool.

**Tool Name**: `download`

**Parameters**:

- `url`: Download URL
- `path`: Target file path

### `internal/agent/tools/ls.go`

**Purpose**: Directory listing.

**Tool Name**: `ls`

**Parameters**:

- `path`: Directory path
- `depth`: Recursion depth
- `max_items`: Result limit

### `internal/agent/tools/rg.go`

**Purpose**: Ripgrep wrapper.

**Tool Name**: `rg`

**Features**:

- Direct ripgrep invocation
- JSON output parsing
- Performance optimization

### `internal/agent/tools/safe.go`

**Purpose**: Safe command definitions.

**Safe Commands List**:

- Read-only commands
- Git operations
- File inspection

### `internal/agent/tools/sourcegraph.go`

**Purpose**: Sourcegraph integration.

**Tool Name**: `sourcegraph`

**Features**:

- Semantic code search
- Symbol navigation
- Cross-repository search

### `internal/agent/tools/mcp-tools.go`

**Purpose**: MCP tool integration.

**Features**:

- Dynamic tool discovery
- Permission integration
- Schema validation
- Multi-modal support

### `internal/agent/tools/list_mcp_resources.go`

**Purpose**: MCP resource listing.

**Tool Name**: `list_mcp_resources`

### `internal/agent/tools/read_mcp_resource.go`

**Purpose**: MCP resource reading.

**Tool Name**: `read_mcp_resource`

### `internal/agent/tools/memory_query.go`

**Purpose**: Cold memory query tool.

**Tool Name**: `memory_query`

**Features**:

- Searches past session summaries
- Searches codebase knowledge
- Returns formatted markdown

### `internal/agent/tools/job_kill.go`

**Purpose**: Background job termination.

**Tool Name**: `job_kill`

**Parameters**:

- `job_id`: Background job ID

### `internal/agent/tools/job_output.go`

**Purpose**: Background job output retrieval.

**Tool Name**: `job_output`

**Parameters**:

- `job_id`: Background job ID
- `since`: Output offset

### `internal/agent/tools/lsp_restart.go`

**Purpose**: LSP server restart.

**Tool Name**: `lsp_restart`

**Parameters**:

- `language`: Optional language filter

### `internal/agent/tools/multiedit_test.go`

**Purpose**: Multi-edit tool tests.

**Test Cases**:

- Parallel file edits
- Sequential edits per file
- Error handling
- Escape confusion remediation

### `internal/agent/tools/context_test.go`

**Purpose**: Context handling tests.

### `internal/agent/tools/grep_test.go`

**Purpose**: Grep tool tests.

### `internal/agent/tools/job_test.go`

**Purpose**: Background job tests.

### `internal/agent/tools/view_test.go`

**Purpose**: View tool tests.

### `internal/agent/tools/dispatcher.go`

**Purpose**: Tool call dispatcher.

### `internal/agent/tools/fast_dispatcher.go`

**Purpose**: Optimized dispatcher for Go 1.26.

### `internal/agent/tools/dispatcher_benchmark_test.go`

**Purpose**: Dispatcher benchmarks.

### `internal/agent/tools/fast_view.go`

**Purpose**: Optimized view tool.

### `internal/agent/tools/fetch_helpers.go`

**Purpose**: Fetch helper functions.

### `internal/agent/tools/fetch_types.go`

**Purpose**: Fetch type definitions.

---

## Persistent Memory System

### `internal/memory/system.go`

**Purpose**: Top-level memory system coordinator.

**Key Types**:

- `System`: Main entry point for memory operations
- `Config`: Memory configuration

---

**Key Methods**:

- `NewSystem(ctx, sessionID, cfg)`: Initializes memory system
- `Close()`: Stops pipeline and closes store
- `PushToolResult(sessionID, turnIndex, toolName, input, output)`: Queues extraction event
- `ShouldRunCheckpoint()`: Checks if checkpoint needed
- `MarkCheckpointDone()`: Marks checkpoint complete
- `ResetCheckpointState()`: Resets for new cycle
- `RunPreCompactionCheckpoint(ctx, sessionID, lastTurns)`: Sync extraction before compaction
- `BuildContextInjection(ctx)`: Assembles memory context block

**Initialization**:

- Returns `nil` (not error) if API key missing
- Creates Store, Pipeline, Extractor
- Starts background worker

**Context Injection Priority**:

1. Project Constitution (max 2K tokens)
2. Negative Constraints (all, never decayed)
3. Top-K Relevant Records (top 15 by score)
4. Latest Compaction Checkpoint (max 1.5K tokens)

### `internal/memory/store.go`

**Purpose**: SQLite persistent store.

**Key Types**:

- `Store`: Per-session SQLite database

**Schema**:

- `memory_records`: Main memory table
- `memory_fts`: FTS5 virtual table
- `project_constitution`: Project architecture
- `compaction_checkpoints`: Checkpoint storage

**Triggers**:

- Automatic FTS index updates on INSERT/UPDATE/DELETE

**Key Methods**:

- `NewStore(dataDir, sessionID, projectRoot)`: Opens/creates database
- `Close()`: Closes database
- `WriteRecord(ctx, rec)`: Writes with deduplication
- `QueryRecords(ctx, filter, limit)`: Top-K by retrieval score
- `SearchFTS(ctx, query, limit)`: Full-text search
- `GetNegativeConstraints(ctx)`: All constraints
- `GetConstitution(ctx)`: Project constitution
- `UpsertConstitution(ctx, content)`: Updates constitution

**Deduplication**:

- SHA-256 hash on `(sessionID, turnIndex, eventType)`
- Prevents duplicate writes

**Temporal Decay**:

- Formula: `salience = salience * exp(-0.05 * hours)`
- Exceptions: Negative constraints, architectural decisions (zero decay)

**Project Scoping**:

- SHA-256 hash of project root path
- Isolates memory between projects

### `internal/memory/extraction.go`

**Purpose**: Gemini-based structured extraction.

**Key Types**:

- `Extractor`: Extraction model client
- `ExtractionResult`: Structured output

**Memory Record Types**:

1. **ArchitecturalDecision**: Design decisions with rationale
   - Salience: 0.95
   - Fields: decision, rationale, files_affected

2. **FileModified**: Semantic file changes
   - Salience: 0.7
   - Fields: file, change_summary, semantic_change

3. **FailureEncountered**: Errors with resolutions
   - Salience: 0.9
   - Fields: what_failed, root_cause, resolution

4. **NegativeConstraint**: Prohibited actions
   - Salience: 1.0 (zero decay)
   - Fields: constraint, reason

5. **TaskProgress**: Task state tracking
   - Salience: 0.6
   - Fields: completed_steps, current_step, next_steps, blockers

6. **CodebaseDiscovery**: Important findings
   - Salience: 0.5-0.8
   - Fields: discovery, location, importance

**Key Methods**:

- `NewExtractor(apiKey, model, workDir)`: Creates extractor
- `Extract(ctx, rawSource)`: Calls model for structured JSON
- `validateFilePaths(result)`: Hallucination guard
- `ResultToRecords(sessionID, turnIndex, result, rawSource)`: Converts to records

**Extraction Model**:

- Default: `gemini-2.0-flash`
- Temperature: 0.1
- Max output tokens: 4096
- Thinking level: low

**Hallucination Prevention**:

- Validates all file paths against filesystem
- Replaces non-existent paths with `[removed: path not found]`

### `internal/memory/pipeline.go`

**Purpose**: Background extraction worker.

**Key Types**:

- `Pipeline`: Background worker
- `ExtractionEvent`: Queue event

**Configuration**:

- Queue size: 256 (non-blocking, drops if full)
- Batch window: 500ms
- Max batch size: 5 events
- Max retries: 1
- Retry backoff: 2s

**Key Methods**:

- `NewPipeline(store, extractor)`: Creates pipeline
- `Start(ctx)`: Starts background worker
- `Stop()`: Signals drain and exit
- `Push(event)`: Adds to queue (non-blocking)
- `ExtractSync(ctx, sessionID, turnIndex, rawSource)`: Synchronous extraction

**Worker Loop**:

1. Collects batch (wait 500ms or max 5 events)
2. Combines raw sources
3. Calls extractor with retry logic
4. Writes extracted records
5. Updates constitution if architectural decisions

**Constitution Auto-Update**:

- Fetches existing constitution
- Appends new decisions
- Caps at ~2K characters
- Writes back via `UpsertConstitution`

**Checkpoint Building**:

- Fetches last 50 records
- Marshals to JSON
- Writes to `compaction_checkpoints` table

### `internal/memory/tools.go`

**Purpose**: Agent-accessible memory tools.

**Tools**:

- `recall_memory`: Query persistent memory
- `save_memory`: Synchronous high-priority save

**`recall_memory` Parameters**:

- `query`: Plain language search
- `filter`: all, negative_constraints, architectural, failures, progress
- `limit`: Max records (default 5, max 20)

**`save_memory` Parameters**:

- `event_type`: Record type
- `content`: Structured JSON content

### `internal/memory/open_modernc.go`

**Purpose**: SQLite driver (modernc.org/sqlite).

**Platform**: Non-cgo systems.

### `internal/memory/open_ncruces.go`

**Purpose**: SQLite driver (ncruces/go-sqlite3).

**Platform**: CGo-enabled systems.

---

## LSP Integration

### `internal/lsp/client.go`

**Purpose**: LSP client wrapper.

**Key Types**:

- `Client`: High-level LSP client
- `DiagnosticCounts`: Severity counts
- `OpenFileInfo`: Open file metadata

**Key Fields**:

- `client`: PowerNap client
- `fileTypes`: Handled file extensions
- `diagnostics`: Versioned diagnostic map
- `openFiles`: Currently open files
- `diagCountsCache`: Cached counts for UI

**Key Methods**:

- `New(ctx, name, cfg, resolver, cwd, debug)`: Creates client
- `Initialize(ctx, workspaceDir)`: Initializes LSP
- `Close(ctx)`: Graceful shutdown (5s timeout)
- `Kill()`: Force kill
- `Restart()`: Reinitializes client
- `OpenFile(ctx, uri)`: Opens file
- `CloseFile(ctx, uri)`: Closes file
- `NotifyChange(ctx, uri)`: Notifies content change
- `FindReferences(ctx, path, line, char, includeDecl)`: Finds references
- `RequestHover(ctx, path, line, char)`: Gets hover info
- `GetDiagnostics()`: Returns cached diagnostics
- `WaitForDiagnostics(ctx, timeout)`: Waits for publication
- `SetDiagnosticsCallback(callback)`: Sets notification handler

**Server States**:

- `StateUnstarted`: Not initialized
- `StateStarting`: Initializing
- `StateReady`: Ready for requests
- `StateError`: Error state
- `StateStopped`: Stopped
- `StateDisabled`: Disabled by config

**Diagnostic Handling**:

- Thread-safe versioned map
- Cached counts for UI performance
- Callback on diagnostic changes

### `internal/lsp/manager.go`

**Purpose**: LSP client lifecycle management.

**Key Types**:

- `Manager`: Multi-client manager

**Key Methods**:

- `NewManager(cfg)`: Creates manager
- `Start(ctx, path)`: Starts appropriate LSP
- `GetClientFor(filePath)`: Returns handling client
- `Clients()`: Returns all clients
- `SetCallback(cb)`: Sets new client callback
- `TrackConfigured()`: Tracks user-configured LSPs

**Lazy Loading**:

- Starts LSP on first file access
- Based on file type matching
- Root marker detection (go.mod, package.json, etc.)

**Auto-Start Prevention**:

- Generic commands skipped (python, node, etc.)
- Requires explicit user configuration
- Command existence check via `exec.LookPath`

**Configuration Merging**:

- Loads PowerNap defaults
- Merges user LSP config
- Handles disabled servers

### `internal/lsp/handlers.go`

**Purpose**: LSP request/notification handlers.

**Handlers**:

- `HandleApplyEdit`: Workspace edits
- `HandleWorkspaceConfiguration`: Configuration requests
- `HandleRegisterCapability`: Dynamic capability registration
- `HandleDiagnostics`: Diagnostic notifications
- `HandleServerMessage`: Show message notifications

### `internal/lsp/client_test.go`

**Purpose**: LSP client tests.

### `internal/lsp/util/`

**Purpose**: LSP utility functions.

---

## Database Layer

### `internal/db/db.go`

**Purpose**: SQLC-generated database wrapper.

**Key Types**:

- `Queries`: Generated query methods
- `DBTX`: Database transaction interface

**Generated Methods**:

- Session CRUD operations
- Message CRUD operations
- File tracking operations
- Memory operations
- Statistics queries

**Transaction Support**:

- `WithTx(tx)`: Creates transactional queries
- `Close()`: Closes prepared statements

### `internal/db/models.go`

**Purpose**: Database model definitions.

**Models**:

- `Session`: Session record
- `Message`: Message record
- `File`: File history record
- `MemoryRecord`: Memory record
- `CodebaseKnowledge`: Symbol knowledge

### `internal/db/sessions.sql.go`

**Purpose**: Generated session queries.

**Queries**:

- `CreateSession`: Creates new session
- `GetSessionByID`: Fetches session
- `ListSessions`: Lists all sessions
- `UpdateSession`: Updates session
- `DeleteSession`: Soft-deletes session
- `UpdateSessionTitleAndUsage`: Atomic usage update

### `internal/db/messages.sql.go`

**Purpose**: Generated message queries.

**Queries**:

- `CreateMessage`: Creates message
- `GetMessage`: Fetches message
- `ListMessagesBySession`: Lists session messages
- `UpdateMessage`: Updates message
- `DeleteMessage`: Deletes message

### `internal/db/files.sql.go`

**Purpose**: Generated file queries.

**Queries**:

- `CreateFile`: Creates file record
- `GetFile`: Fetches file
- `GetFileByPathAndSession`: Fetches by path/session
- `ListFilesBySession`: Lists session files
- `CreateVersion`: Creates version
- `DeleteFile`: Deletes file

### `internal/db/read_files.sql.go`

**Purpose**: Generated file read tracking queries.

**Queries**:

- `RecordFileRead`: Records read
- `GetFileRead`: Fetches read record
- `ListSessionReadFiles`: Lists read files

### `internal/db/tiered_memory.sql.go`

**Purpose**: Generated memory queries.

**Queries**:

- `UpsertCodebaseKnowledge`: Upserts symbol knowledge
- `GetCodebaseKnowledgeByFilePath`: Fetches by file
- `GetProjectConstitution`: Fetches constitution
- `UpsertProjectConstitution`: Updates constitution
- `CreateStructuredSummary`: Creates summary
- `GetStructuredSummaryBySessionID`: Fetches summary

### `internal/db/stats.sql.go`

**Purpose**: Generated statistics queries.

**Queries**:

- `GetTotalStats`: Aggregate statistics
- `GetUsageByDay`: Daily usage
- `GetUsageByHour`: Hourly usage
- `GetUsageByDayOfWeek`: Day of week usage
- `GetUsageByModel`: Per-model usage
- `GetToolUsage`: Tool usage statistics
- `GetAverageResponseTime`: Average response time
- `GetHourDayHeatmap`: Heatmap data
- `GetRecentActivity`: Recent activity

### `internal/db/codebase.go`

**Purpose**: Codebase knowledge helpers.

### `internal/db/connect.go`

**Purpose**: Database connection setup.

**Features**:

- SQLite WAL mode
- Busy timeout (30s)
- Foreign keys enabled
- Secure delete enabled

### `internal/db/embed.go`

**Purpose**: Embedded migration files.

### `internal/db/querier.go`

**Purpose**: Querier interface.

### `internal/db/migrations/`

**Purpose**: Database migrations (goose).

**Migrations**:

- `20250424200609_initial.sql`: Initial schema
- `20250515105448_add_summary_message_id.sql`: Summary message tracking
- `20250624000000_add_created_at_indexes.sql`: Index optimization
- `20250627000000_add_provider_to_messages.sql`: Provider tracking
- `20250810000000_add_is_summary_message.sql`: Summary flag
- `20250812000000_add_todos_to_sessions.sql`: Todo storage
- `20260127000000_add_read_files_table.sql`: File read tracking
- `20260309000000_add_tiered_memory_tables.sql`: Memory tables

### `internal/db/sql/`

**Purpose**: SQL source files for sqlc.

**Files**:

- `sessions.sql`: Session queries
- `messages.sql`: Message queries
- `files.sql`: File queries
- `read_files.sql`: File read queries
- `stats.sql`: Statistics queries
- `tiered_memory.sql`: Memory queries

---

## Session Management

### `internal/session/session.go`

**Purpose**: Session lifecycle management.

**Key Types**:

- `Session`: Session record
- `Todo`: Task item
- `Service`: Session service interface

**Session Fields**:

- `ID`: UUID session ID
- `ParentSessionID`: Parent session (for sub-agents)
- `Title`: Session title
- `MessageCount`: Message count
- `PromptTokens`: Prompt token count
- `CompletionTokens`: Completion token count
- `SummaryMessageID`: Summary message reference
- `Cost`: Session cost
- `Todos`: Task list
- `CreatedAt`, `UpdatedAt`: Timestamps

**Todo States**:

- `pending`: Not started
- `in_progress`: Currently working
- `completed`: Finished

**Key Methods**:

- `Create(ctx, title)`: Creates session
- `CreateTaskSession(ctx, toolCallID, parentSessionID, title)`: Creates child session
- `CreateTitleSession(ctx, parentSessionID)`: Creates title generation session
- `Get(ctx, id)`: Fetches session
- `List(ctx)`: Lists sessions
- `Save(ctx, session)`: Updates session
- `UpdateTitleAndUsage(ctx, id, title, prompt, completion, cost)`: Atomic update
- `Delete(ctx, id)`: Soft-deletes session

**Agent Tool Sessions**:

- Format: `messageID$$toolCallID`
- `CreateAgentToolSessionID(messageID, toolCallID)`: Creates ID
- `ParseAgentToolSessionID(sessionID)`: Parses components
- `IsAgentToolSession(sessionID)`: Checks format

**Event Publishing**:

- Created, Updated, Deleted events
- PubSub broker integration

---

## Configuration System

### `internal/config/config.go`

**Purpose**: Configuration types and structures.

**Key Types**:

- `Config`: Root configuration
- `ProviderConfig`: AI provider configuration
- `SelectedModel`: Model selection
- `MCPConfig`: MCP server config
- `LSPConfig`: LSP server config
- `Options`: General options
- `Permissions`: Permission settings
- `Agent`: Agent configuration

**Provider Configuration**:

- `ID`: Provider identifier
- `Name`: Display name
- `BaseURL`: API endpoint
- `Type`: Provider type (openai, anthropic, gemini, etc.)
- `APIKey`: API key (with variable expansion)
- `OAuthToken`: OAuth2 token
- `Models`: Available models
- `SystemPromptPrefix`: Custom prefix
- `ExtraHeaders`: Additional headers
- `ExtraBody`: Additional body fields

**Model Configuration**:

- `Model`: Model ID
- `Provider`: Provider ID
- `ReasoningEffort`: OpenAI reasoning effort
- `Think`: Anthropic thinking mode
- `MaxTokens`: Maximum output tokens
- `Temperature`: Sampling temperature
- `TopP`: Nucleus sampling
- `TopK`: Top-K sampling
- `FrequencyPenalty`: Repetition penalty
- `PresencePenalty`: Topic diversity

**MCP Configuration**:

- `Command`: stdio command
- `Args`: Command arguments
- `Env`: Environment variables
- `Type`: stdio, sse, http
- `URL`: HTTP/SSE URL
- `Timeout`: Connection timeout
- `DisabledTools`: Disabled tool list

**LSP Configuration**:

- `Command`: LSP server command
- `Args`: Server arguments
- `Env`: Environment variables
- `FileTypes`: Handled file types
- `RootMarkers`: Project root markers
- `InitOptions`: Initialization options
- `Options`: Server settings
- `Timeout`: Initialization timeout

**Options**:

- `ContextPaths`: Context file paths
- `SkillsPaths`: Skill directories
- `TUI`: TUI options
- `Debug`: Debug logging
- `DebugLSP`: LSP debug logging
- `DisableAutoSummarize`: Disable summarization
- `DataDirectory`: Data storage directory
- `DisabledTools`: Disabled built-in tools
- `DisableProviderAutoUpdate`: Disable auto-update
- `DisableDefaultProviders`: Disable default providers
- `Attribution`: Attribution settings
- `DisableMetrics`: Disable metrics
- `InitializeAs`: Initialization file name
- `AutoLSP`: Auto LSP setup
- `Progress`: Progress bar display

**Permissions**:

- `AllowedTools`: Tools bypassing prompts
- `SkipRequests`: YOLO mode (auto-approve)

**Agents**:

- `ID`: Agent identifier
- `Name`: Display name
- `Model`: Model type (large/small)
- `AllowedTools`: Tool allowlist
- `AllowedMCP`: MCP allowlist
- `ContextPaths`: Agent-specific context paths

### `internal/config/load.go`

**Purpose**: Configuration loading.

**Loading Order**:

1. Global defaults
2. Global config (`~/.config/sapphire/`)
3. Project config (`sapphire.json`)
4. Environment variables (`CRUSH_*`)

**Variable Resolution**:

- Shell substitution (`$VAR`, `$(command)`)
- Recursive resolution
- Environment mapping (`CRUSH_OPENAI_API_KEY` → `OPENAI_API_KEY`)

### `internal/config/provider.go`

**Purpose**: Provider configuration helpers.

### `internal/config/hyper.go`

**Purpose**: Hyper provider configuration.

### `internal/config/copilot.go`

**Purpose**: GitHub Copilot configuration.

### `internal/config/init.go`

**Purpose**: Configuration initialization.

### `internal/config/resolve.go`

**Purpose**: Variable resolution.

**Resolver**:

- Shell variable expansion
- Command substitution
- Environment variable lookup

### `internal/config/catwalk.go`

**Purpose**: Catwalk model integration.

### `internal/config/agent_id_test.go`

**Purpose**: Agent ID tests.

### `internal/config/load_test.go`

**Purpose**: Configuration loading tests.

### `internal/config/load_bench_test.go`

**Purpose**: Configuration loading benchmarks.

### `internal/config/provider_test.go`

**Purpose**: Provider configuration tests.

### `internal/config/resolve_test.go`

**Purpose**: Variable resolution tests.

### `internal/config/lsp_defaults_test.go`

**Purpose**: LSP default tests.

### `internal/config/provider_empty_test.go`

**Purpose**: Empty provider tests.

### `internal/config/recent_models_test.go`

**Purpose**: Recent models tests.

### `internal/config/catwalk_test.go`

**Purpose**: Catwalk tests.

### `internal/config/attribution_migration_test.go`

**Purpose**: Attribution migration tests.

---

## TUI Architecture

### `internal/ui/model/ui.go`

**Purpose**: Main TUI model.

**Key Types**:

- `UI`: Root UI model
- `uiFocusState`: Focus state enum
- `uiState`: UI state enum

**UI Components**:

- `com`: Common utilities
- `session`: Current session
- `dialog`: Dialog overlay
- `status`: Status bar
- `header`: Header component
- `textarea`: Message input
- `attachments`: Attachment list
- `chat`: Chat component
- `completions`: Completions popup
- `sidebarLogo`: Cached logo
- `todoSpinner`: Todo spinner

**State Management**:

- `focus`: Current focus (none, editor, main)
- `state`: UI state (onboarding, initialize, landing, chat)
- `isCanceling`: Cancel confirmation state
- `isCompact`: Compact layout mode
- `detailsOpen`: Details panel (compact mode)
- `pillsExpanded`: Pills expansion state

**Key Methods**:

- `New(com)`: Creates UI model
- `Init()`: Initialization command
- `Update(msg)`: Message handling
- `View()`: Rendering
- `Draw(scr, area)`: Coordinate-based drawing
- `SetSize(width, height)`: Size update

**Keyboard Handling**:

- Key map configuration
- Vim-style navigation
- Custom key bindings

**Mouse Handling**:

- Click detection (single, double, triple)
- Drag handling
- Scroll handling
- Focus-aware filtering

**Layout Modes**:

- Normal mode
- Compact mode (width < 120, height < 30)

**Completions**:

- Triggered by `@` character
- Position tracking
- Query filtering
- Selection handling

**Attachments**:

- Image preview
- Text attachment
- Delete mode
- Keyboard navigation

### `internal/ui/model/chat.go`

**Purpose**: Chat component.

**Key Types**:

- `Chat`: Chat model
- `DelayedClickMsg`: Delayed click action

**Key Fields**:

- `list`: Underlying list component
- `idInxMap`: Message ID to index mapping
- `pausedAnimations`: Paused animation tracking
- `mouseDown`, `mouseDrag`: Mouse state
- `clickCount`, `lastClickTime`: Click tracking
- `follow`: Auto-scroll flag

**Key Methods**:

- `NewChat(com)`: Creates chat
- `SetMessages(msgs)`: Sets messages
- `AppendMessages(msgs)`: Appends messages
- `UpdateNestedToolIDs(containerID)`: Updates nested IDs
- `Animate(msg)`: Animates visible items
- `RestartPausedVisibleAnimations()`: Restarts animations
- `ScrollToBottom()`: Scrolls to bottom
- `ScrollToTop()`: Scrolls to top
- `ScrollBy(lines)`: Relative scroll
- `AtBottom()`: Checks bottom position
- `Follow()`: Returns follow mode

**Animation Optimization**:

- Pauses animations for scrolled-out items
- Restarts when items become visible
- Saves CPU on large conversations

**Mouse Click Detection**:

- Double-click threshold: 400ms
- Click tolerance: 2 characters
- Triple-click support

### `internal/ui/model/header.go`

**Purpose**: Header component.

### `internal/ui/model/status.go`

**Purpose**: Status bar component.

### `internal/ui/model/session.go`

**Purpose**: Session details panel.

### `internal/ui/model/sidebar.go`

**Purpose**: Sidebar component.

### `internal/ui/model/history.go`

**Purpose**: History panel.

### `internal/ui/model/landing.go`

**Purpose**: Landing page.

### `internal/ui/model/onboarding.go`

**Purpose**: Onboarding flow.

### `internal/ui/model/keys.go`

**Purpose**: Key binding definitions.

### `internal/ui/model/filter.go`

**Purpose**: Filter functionality.

### `internal/ui/model/lsp.go`

**Purpose**: LSP status display.

### `internal/ui/model/mcp.go`

**Purpose**: MCP status display.

### `internal/ui/model/clipboard.go`

**Purpose**: Clipboard integration.

### Profiling

- `task profile:cpu`: CPU profiling
- `task profile:heap`: Heap profiling
- `task profile:allocs`: Allocation profiling

### Environment Variables

- `CRUSH_PROFILE`: Enable profiling
- `CRUSH_GLOBAL_DATA`: Global data directory
- `CRUSH_GLOBAL_CONFIG`: Global config directory
- `CRUSH_DISABLE_ANTHROPIC_CACHE`: Disable Anthropic caching
- `CRUSH_DISABLE_PROVIDER_AUTO_UPDATE`: Disable auto-update
- `CRUSH_DISABLE_DEFAULT_PROVIDERS`: Disable default providers
- `CRUSH_DISABLE_METRICS`: Disable metrics
- `TERM_PROGRAM`: Terminal detection
- `WT_SESSION`: Windows Terminal detection

---

## Key Gotchas

### Database

- Never edit `internal/db/*.sql.go` directly
- Edit `.sql` files in `internal/db/sql/`
- Run sqlc generation after changes
- WAL mode with 30s busy timeout
- Foreign keys enforced

### LSP

- Servers lazy-loaded based on file types
- Custom commands configured in `sapphire.json`
- Use `lsp_restart` tool if stale
- Graceful degradation for unavailable servers

### Shell

- Max 120 concurrent background jobs
- Auto-cleanup after 8 hours
- POSIX shell emulation (forward slashes)
- Blocked commands extensive list

### Memory

- Optional feature (requires Gemini API key)
- Returns `nil` if API key missing
- Deduplication by `(sessionID, turnIndex, eventType)`
- Project scoping via SHA-256 hash
- Queue drops if full (256 events)

### Permissions

- YOLO mode auto-approves all
- Session auto-approve per-session
- Persistent grants by tool/action/path
- Allowed tools bypass prompts

### File Operations

- Must read before edit
- Rejects if file modified since read
- Character-perfect matching for edits
- Line ending preservation

### Concurrency

- Use `csync` for thread-safe collections
- Context cancellation respected
- Errgroup for structured concurrency
- Never do IO in `Update`; use `tea.Cmd`

### TUI

- Ultraviolet for coordinate-based drawing
- Cached rendering for performance
- Animation optimization (pause scrolled-out)
- Non-blocking updates via `tea.Cmd`

---

## File Count Summary

- **Total Go files**: ~300
- **Main packages**: 20+
- **Agent tools**: 30+
- **UI components**: 50+
- **Database migrations**: 8
- **SQL files**: 6
- **Test files**: 20+
- **Bundled skills**: 6

---

_Generated from comprehensive codebase analysis._
