# Sapphire CLI Features

## Core Agent Capabilities

### Autonomous Task Execution

- **Hierarchical Multi-Agent Delegation**: Main coordinator spawns parallel sub-agents for complex tasks (mapping, risk review)
- **Recursive Agent Invocation**: Agents can invoke other agents via `agent` or `agentic_fetch` tools
- **Session-Based Execution**: Each conversation/task runs in an isolated session with persistent state
- **Task Sessions**: Sub-tasks created via tool calls have dedicated child sessions linked to parent

### Structured Task Management

- **Todos Tool**: Structured task tracking with three states (pending, in_progress, completed)
- **Active Form Tracking**: Present-continuous form for in-progress tasks (e.g., "Running tests")
- **Progress Summaries**: Automatic count of pending, in-progress, and completed tasks
- **Session Persistence**: Todos persist across session boundaries in SQLite database

## Autonomous Skill Loading

### Skill Tools

- **`load_skill`**: Loads domain-specific engineering protocols before technical implementation
- **`list_skills`**: Lists all available skills with names, descriptions, and sources
- **Autonomous Invocation**: Agent auto-triggers skill loading based on task domain recognition
- **Domain-Triggered Loading**: Six domains mapped to specific skills

### Domain-Triggered Loading

| Domain       | Skill       | Trigger                                        |
| ------------ | ----------- | ---------------------------------------------- |
| Frontend/UI  | `frontend`  | React, TypeScript, components, styling, UI/UX  |
| Backend/API  | `backend`   | Server, database, API, business logic          |
| Debugging    | `debug`     | Error investigation, bug fix, failure analysis |
| Architecture | `architect` | System design, structural change, patterns     |
| DevOps       | `devops`    | Deployment, CI/CD, infrastructure, containers  |
| Security     | `security`  | Auth, vulnerabilities, secure coding           |

### Execution Sequence

1. Agent recognizes task domain from user prompt
2. **BEFORE any file read/search/edit** → Agent invokes `load_skill(name: "<domain>")`
3. System returns structured instructions
4. Agent proceeds with implementation

### Exceptions

- Greetings do NOT trigger skill loading
- General questions without technical implementation do NOT trigger skill loading

### Available Skills

- **`architect`**: System design, architectural patterns, structural decisions
- **`backend`**: Go, Node.js, databases, APIs, layered architecture
- **`debug`**: Error investigation, root-cause analysis, evidence-based remediation
- **`devops`**: Deployment, CI/CD, infrastructure-as-code, containers
- **`frontend`**: React, TypeScript, UI/UX, **component** architecture, design systems
- **`security`**: Auth, vulnerabilities, secure coding, threat modeling

### UI Integration

- **Skill Tool Display**: Dedicated UI component for `load_skill` tool calls
- **Render Context**: Shows skill **name**, activation status, and instructions preview
- **Message Tracking**: Loaded skills tracked in message `SkillContext`

### Bundled Skills

- **Location**: `internal/skills/bundled/` (embedded in binary)
- **Format**: `SKILL.md` with YAML frontmatter and markdown instructions
- **Discovery**: Automatic discovery at startup, cached for session lifetime
- **Project Skills**: Additional skills can be loaded from `./skills/` directory

## File Operations

### File Reading (View Tool)

- **Parallel File Reading**: Concurrent reading of multiple files (configurable limit: 50 for main agent, 5 for sub-agents)
- **Line-Numbered Output**: All content displayed with line numbers for precise reference
- **Offset and Limit**: Paginated reading with configurable start line and line count
- **Large File Support**: 25MB maximum file size limit with streaming for large files
- **Image Support**: Base64-encoded image data for supported formats (JPEG, PNG, GIF, WebP)
- **Literal Escape Detection**: Automatic warning for literal `\n` and `\t` sequences that may cause edit failures
- **Skill File Recognition**: Files in skills directories identified with resource type metadata
- **Outside Working Directory Access**: Permission-gated access to files outside project root

### Single File Editing (Edit Tool)

- **Character-Perfect Matching**: Exact string replacement with whitespace/newline precision
- **Replace All Option**: Global replacement across entire file when multiple matches exist
- **File Creation**: Empty `old_string` creates new file with provided content
- **Content Deletion**: Empty `new_string` removes matched content
- **Pre-Edit Validation**: Requires file to be read before editing (file tracker enforcement)
- **Modification Detection**: Rejects edits if file modified since last read
- **Line Ending Preservation**: Maintains CRLF/LF line endings from original file
- **Diff Statistics**: Reports additions and removals count for each edit
- **Permission Gating**: All write operations require explicit user permission

### Agentic Multi-Edit Tool

- **Parallel File Edits**: Up to 25 files edited concurrently with `errgroup` concurrency
- **Sequential Per-File Edits**: Multiple edits applied sequentially within each file
- **Partial Success Handling**: Failed edits reported individually; successful edits still applied
- **Escape Confusion Auto-Remediation**: Automatic retry with literal escape matching when `\n` or `\t` confusion detected
- **Creation with Chained Edits**: First edit creates file; subsequent edits modify created content
- **Edit Failure Tracking**: Each failed edit includes index, error message, and original edit parameters
- **Metadata Reporting**: Per-file and aggregate statistics for applied and failed edits

## Code Intelligence

### LSP Integration

- **Lazy Server Loading**: Language servers start on-demand based on file types and root markers
- **Multi-Language Support**: Concurrent LSP clients for different languages in same project
- **Auto-Start Detection**: Servers auto-started based on file type matching
- **User Configuration**: Custom LSP commands, arguments, and settings via config
- **Server Restart Tool**: `lsp_restart` tool for manual server restart when stale
- **Graceful Degradation**: Unavailable servers tracked to prevent repeated failed attempts

### Diagnostics Tool

- **Real-Time Compiler Errors**: Retrieves diagnostics from all active LSP clients
- **File and Project Scope**: Can query diagnostics for specific file or entire project
- **Severity Classification**: Errors, warnings, hints, and info-level diagnostics
- **Source Attribution**: Each diagnostic includes LSP server source
- **Code and Tag Metadata**: Diagnostic codes, unnecessary/deprecated tags included
- **Concurrent LSP Notification**: Notifies all relevant LSP clients and waits for updated diagnostics
- **Summary Statistics**: Count of errors and warnings for current file and project

### Semantic References Tool

- **Symbol Search**: Finds all semantic references to functions, variables, types
- **Multi-LSP Coordination**: Searches across all active LSP clients
- **Location Grouping**: References grouped by file with reference counts
- **Precise Positioning**: Line and column numbers for each reference
- **Qualified Symbol Support**: Handles `Class::method`, `obj.method`, `Namespace\Class` patterns
- **Grep Fallback**: Falls back to regex search when LSP references unavailable

## Search and Discovery

### Grep Tool

- **Regex Pattern Matching**: Full regular expression support for content search
- **Literal Text Mode**: Optional escaping of regex special characters
- **File Pattern Filtering**: Glob-based file inclusion (e.g., `*.js`, `*.{ts,tsx}`)
- **Ripgrep Acceleration**: Uses `rg` with JSON output when available
- **Fallback Implementation**: Pure Go search when ripgrep unavailable
- **Ignore File Support**: Respects `.gitignore` and `.crushignore` patterns
- **Result Truncation**: 100-match limit with truncation warning
- **Contextual Output**: Line numbers, character positions, and truncated line text

### Glob Tool

- **Glob Pattern Matching**: Standard glob patterns for file path matching
- **Ripgrep Integration**: Uses `rg --files` for fast glob when available
- **Doublestar Fallback**: Pure Go glob implementation as fallback
- **Hidden File Skipping**: Automatically skips hidden files and directories
- **Result Limiting**: 100-file limit with truncation indicator
- **Path Normalization**: All paths returned as forward-slash separated

### Codebase Search Tool

- **Semantic Code Search**: Embedding-based search for code concepts
- **Symbol Indexing**: Automatic indexing of symbols from LSP diagnostics
- **File Path Search**: Search by file path patterns
- **Documentation Search**: Full-text search over symbol documentation

## Web and Network

### Web Search Tool

- **DuckDuckGo Integration**: Searches via DuckDuckGo Lite interface
- **No API Key Required**: Works without external API credentials
- **Randomized Headers**: Rotates User-Agent and Accept-Language headers
- **Configurable Result Count**: 1-20 results per search (default 10)
- **Rate Limiting**: 500-2000ms random delay between searches
- **HTML Parsing**: Custom HTML parser for lite.duckduckgo.com results
- **Position Tracking**: Results include position in search results

### Fetch Tool

- **URL Content Retrieval**: Fetches content from HTTP/HTTPS URLs
- **Multiple Output Formats**: Text, markdown, or HTML output
- **HTML to Markdown Conversion**: Automatic conversion using html-to-markdown library
- **Text Extraction**: Clean text extraction from HTML pages
- **Timeout Configuration**: Configurable timeout (max 120 seconds)
- **Response Size Limit**: 5MB maximum response body
- **UTF-8 Validation**: Validates response encoding
- **Permission Gating**: Requires user permission for all fetch operations

### Agentic Fetch Tool

- **Parallel URL Fetching**: Concurrent fetching of multiple URLs
- **Fallback Strategies**: Multiple fallback URLs for resilient fetching
- **Content Type Detection**: Automatic MIME type detection
- **Error Aggregation**: Collects errors from all failed fetches

## MCP (Model Context Protocol) Integration

### MCP Tools

- **Dynamic Tool Discovery**: Automatically discovers tools from connected MCP servers
- **Permission Integration**: MCP tools gated through standard permission system
- **Schema Validation**: Input schema extracted from MCP tool definitions
- **Multi-Modal Support**: Image and media responses from MCP tools
- **Provider Options**: Per-tool provider configuration support

### MCP Resources

- **Resource Listing**: `list_mcp_resources` tool for discovering available resources
- **Resource Reading**: `read_mcp_resource` tool for fetching resource content
- **Prompt Integration**: MCP prompts available as agent tools

## Shell and Process Management

### Bash Tool

- **Command Execution**: Full shell command execution with output capture
- **Background Jobs**: Optional background execution with job ID tracking
- **Auto-Background Detection**: Commands exceeding 1-minute threshold automatically backgrounded
- **Fast Failure Detection**: 100ms check for quick failures (blocked commands, syntax errors)
- **Output Truncation**: 30,000 character limit with line count summary
- **Working Directory**: Configurable execution directory per command
- **Command Blocking**: Extensive blocklist of dangerous commands (sudo, package managers, network tools)
- **Argument-Based Blocking**: Blocks specific argument combinations (e.g., `npm install -g`)
- **Safe Read-Only Commands**: Whitelist of safe commands bypasses permission prompts
- **Exit Code Reporting**: Non-zero exit codes reported with stderr output
- **Interrupt Detection**: Distinguishes between user interrupts and command failures
- **CWD Tracking**: Reports current working directory after command execution

### Background Shell Management

- **Job Limit**: Maximum 120 concurrent background jobs
- **Job Retention**: Completed jobs retained for 8 hours before auto-cleanup
- **Ring Buffer Output**: 10,000-line circular buffers for stdout and stderr
- **Incremental Output**: `GetOutputSince` for fetching output from specific point
- **Job Kill Tool**: `job_kill` terminates running background jobs
- **Job Output Tool**: `job_output` retrieves output from background jobs
- **Automatic Cleanup**: Periodic cleanup of completed jobs

## Permission System

### Permission Requests

- **Tool-Call Integration**: Each permission request linked to specific tool call ID
- **Action Types**: Read, write, execute, fetch actions categorized
- **Path Scoping**: Permissions scoped to specific file paths
- **Parameter Capture**: Full tool parameters included in permission request
- **Session Auto-Approve**: Optional session-level auto-approval mode
- **Persistent Grants**: Option to persist permissions across session
- **Notification System**: Real-time permission notifications via pubsub
- **Allowed Tools List**: Configurable list of tools that bypass permission prompts

## Persistent Memory System

### Memory Architecture

- **Background Extraction Pipeline**: Non-blocking extraction with 256-event queue
- **Per-Session SQLite Storage**: Isolated databases per project and session
- **FTS5 Full-Text Search**: Full-text search over memory content with automatic triggers
- **Project Scoping**: Records scoped to project via SHA-256 hash of project root
- **Deduplication**: SHA-256 hash prevents duplicate records for same event

### Memory Record Types

- **Architectural Decisions**: Design decisions with rationale and affected files (salience: 0.95)
- **Negative Constraints**: Prohibited actions with reasons (salience: 1.0, zero decay)
- **Failure Modes**: Errors with root causes and resolutions (salience: 0.9)
- **File Modifications**: Semantic change summaries (salience: 0.7)
- **Task Progress**: Completed, current, and next steps with blockers (salience: 0.6)
- **Codebase Discoveries**: Important findings with importance levels (salience: 0.5-0.8)

### Context Injection

- **Project Constitution**: Auto-generated architecture documentation (max 2K tokens)
- **Constraint Injection**: All negative constraints always included
- **Top-K Retrieval**: Top 15 records by retrieval score with temporal decay
- **Checkpoint Retrieval**: Latest compaction checkpoint (max 1.5K tokens)

### Temporal Decay

- **Exponential Decay**: `salience = salience * exp(-0.05 * hours)`
- **Zero-Decay Exceptions**: Negative constraints and architectural decisions never decay
- **Re-Scoring on Query**: Decay applied at query time for fresh scoring

### Agent Memory Tools

- **Recall Memory**: Query persistent memory with filters (all, negative_constraints, architectural, failures, progress)
- **Save Memory**: Synchronous high-priority save at maximum salience
- **Memory Query**: Cold memory search across session summaries and codebase knowledge

### Constitution Auto-Update

- **Automatic Architecture Tracking**: New architectural decisions appended to constitution
- **Size Capping**: Constitution capped at ~2K characters
- **Upsert Logic**: Creates or updates constitution as needed

### Pre-Compaction Checkpoint

- **Synchronous Extraction**: On-demand extraction before context compaction
- **Checkpoint Storage**: Last 50 records saved as compaction checkpoint
- **Retrieval on Demand**: Latest checkpoint included in context injection

## Skill System

### Skill Discovery

- **File-Based Skills**: Skills defined in markdown files with structured metadata
- **Automatic Discovery**: Scans configured skill directories
- **Embedding-Based Retrieval**: 768-dimensional Gemini embeddings for semantic search
- **Cosine Similarity**: Default threshold 0.45 for skill relevance
- **Context Injection**: Relevant skill instructions injected into agent context

### Skill Embedding Service

- **Concurrent Embedding**: Batch embedding of all discovered skills
- **Query Embedding**: On-demand embedding of user prompts
- **Normalized Vectors**: L2-normalized vectors for cosine similarity
- **Caching**: Embedded skills cached for session lifetime

## Session Management

### Session Lifecycle

- **Session Creation**: UUID-based session IDs with titles
- **Parent-Child Relationships**: Task sessions linked to parent sessions
- **Token Tracking**: Prompt and completion token counts per session
- **Cost Tracking**: Per-session cost calculation
- **Message Count**: Running count of messages per session
- **Soft Deletion**: Sessions marked deleted rather than hard-deleted

### Session Persistence

- **SQLite Storage**: All sessions persisted to SQLite database
- **PubSub Integration**: Session events published to event broker
- **Event Types**: Created, updated, deleted events
- **Subscriber Support**: Components can subscribe to session events

### File Tracking

- **Read Recording**: All file reads recorded with timestamps
- **Last-Read Validation**: Edit operations require prior read
- **Modification Detection**: Rejects edits if file modified after read
- **Session-Scoped**: File reads tracked per session

## TUI Features

### Chat Interface

- **Assistant Messages**: Formatted AI responses with thinking bubbles
- **Thinking Process Display**: Collapsible thinking boxes (max 10 lines collapsed)
- **Double-Click Copy**: Click-to-copy assistant message content
- **Highlighting**: Syntax highlighting for code blocks
- **Truncation Indicators**: Expandable truncated messages
- **Cached Rendering**: Avoids redundant Lipgloss rendering

### Message Items

- **Highlightable Messages**: Search term highlighting in messages
- **Focusable Items**: Keyboard navigation between messages
- **Cached Rendering**: Performance optimization for message rendering
- **Animation Support**: Loading animations for in-progress messages

### UI Components

- **Coordinate-Based Drawing**: Ultraviolet-based rendering engine
- **Composite Model Pattern**: Root UI model delegates to sub-components
- **Non-Blocking Updates**: All I/O via `tea.Cmd`, never in `Update`
- **Event Subscription**: Reactive updates via pubsub events

### Styles and Themes

- **Orange-Dominant Gradient**: Zest → Mustard → Salmon gradient theme
- **Yellow Mode Toggle**: Alternative Lime → Green gradient
- **Dark Background**: Near-black background for reduced eye strain
- **Thinking Bubble Styling**: Internal padding for thinking process display
- **Edit Color Coding**: Pink color for agentic_edit operations

### Animations

- **Gradient Shimmer**: Loading state animations
- **Scrambled Rune Display**: Animated loading indicators
- **Color Cycling**: Rotating gradient colors for animations

## Configuration System

### Multi-Tier Configuration

- **Global Defaults**: Built-in default configuration
- **Global Config**: `~/.config/sapphire/` user configuration
- **Project Config**: `sapphire.json` project-specific configuration
- **Environment Variables**: `CRUSH_*` environment variable overrides

### Variable Resolution

- **Shell Substitution**: `$VAR` and `$(command)` expansion
- **Recursive Resolution**: Nested variable resolution
- **Environment Mapping**: `CRUSH_*` mapped to standard names

### Provider Configuration

- **Multi-Provider Support**: OpenAI, Anthropic, Gemini, Azure, VertexAI, OpenRouter, Vercel
- **Custom Base URLs**: Provider endpoint configuration
- **API Key Management**: Per-provider API key configuration
- **OAuth Integration**: OAuth2 token support for compatible providers
- **Model Configuration**: Per-model temperature, top_p, max_tokens settings
- **Provider Options**: Provider-specific configuration parameters

### Model Selection

- **Large/Small Models**: Separate large and small model configurations
- **Reasoning Effort**: OpenAI reasoning effort levels (low, medium, high)
- **Thinking Mode**: Anthropic thinking mode toggle
- **Temperature Control**: Per-model temperature settings
- **Penalty Settings**: Frequency and presence penalties

### LSP Configuration

- **Custom Commands**: Override default LSP server commands
- **Server Arguments**: Custom arguments for LSP servers
- **Environment Variables**: Per-server environment configuration
- **File Type Mapping**: Custom file type associations
- **Root Markers**: Project root detection patterns
- **Init Options**: Language server initialization options
- **Server Settings**: Per-server configuration settings

### Agent Configuration

- **Coder Agent**: Primary coding agent configuration
- **Task Agent**: Task-focused agent configuration
- **Tool Limits**: Per-tool parallelism and rate limits
- **Permission Defaults**: Default permission behavior

## Testing and Development

### Test Framework

- **Testify Assertions**: `testify/require` for test assertions
- **Parallel Tests**: `t.Parallel()` and `t.SetEnv()` support
- **VCR Recording**: Charm VCR for recording/replaying API interactions
- **Mock Providers**: `config.UseMockProviders` for isolated testing

### Development Tools

- **Profiling**: CPU, heap, and allocation profiling via pprof
- **Schema Generation**: JSON schema generation for configuration
- **Linting**: golangci-lint with custom rules
- **Formatting**: gofumpt for consistent formatting
- **Modernize**: Automated code simplification suggestions

### Golden Files

- **Update Flag**: `-update` flag for regenerating golden files
- **Test Comparison**: Automatic comparison against expected output

## Database and Storage

### SQLite Schema

- **WAL Mode**: Write-ahead logging for concurrent access
- **Busy Timeout**: 30-second busy timeout for contention handling
- **Foreign Keys**: Referential integrity enforcement
- **Secure Delete**: Secure deletion of sensitive data
- **Triggers**: Automatic trigger-based index updates

### SQLC Integration

- **Type-Safe Queries**: Compile-time query validation
- **Generated Code**: Auto-generated Go code from SQL
- **Transaction Support**: Transaction-wrapped query execution

### Goose Migrations

- **Version Control**: Database version tracking
- **Rollback Support**: Migration rollback capabilities
- **Automatic Migration**: Auto-migration on database connection

## Event System

### PubSub Broker

- **Generic Events**: Type-safe generic event system
- **Subscriber Support**: Components subscribe to event types
- **Event Types**: Created, updated, deleted events
- **Thread-Safe**: Concurrent-safe event publishing

### Event Types

- **Session Events**: Session creation, update, deletion
- **Message Events**: Message creation and updates
- **Permission Events**: Permission grant and denial notifications
- **LSP Events**: LSP client lifecycle events

## Security Features

### Command Blocking

- **System Command Blocklist**: sudo, su, doas blocked
- **Package Manager Blocklist**: apt, yum, dnf, pacman, brew, etc.
- **Network Tool Blocklist**: curl, wget, ssh, scp blocked
- **Argument-Based Blocking**: Specific argument combinations blocked
- **Go Test Exec Blocking**: `go test -exec` blocked for arbitrary command prevention

### Path Security

- **Working Directory Validation**: Files outside working directory require permission
- **Symlink Resolution**: Symlinks resolved to prevent traversal attacks
- **Skills Path Whitelisting**: Skills directories accessible without prompts

### Permission Gating

- **Read Permissions**: Files outside working directory require permission
- **Write Permissions**: All file modifications require permission
- **Execute Permissions**: All command executions require permission
- **Fetch Permissions**: All URL fetches require permission

## Performance Optimizations

### Concurrency Patterns

- **Parallel File Operations**: Concurrent file reading and editing
- **Errgroup Coordination**: Structured concurrency with error handling
- **Worker Pools**: Background workers for extraction and indexing
- **Channel-Based Queues**: Non-blocking event queues

### Caching Strategies

- **Regex Cache**: Thread-safe compiled regex caching
- **Message Rendering Cache**: Avoids redundant Lipgloss rendering
- **Skill Embedding Cache**: Cached skill embeddings for session lifetime
- **LSP Client Cache**: Cached LSP clients for file types

### Database Optimizations

- **FTS5 Virtual Tables**: Full-text search optimization
- **Deduplication Hashes**: Prevents duplicate record storage
- **Batched Writes**: Batched database writes for efficiency

### File System Optimizations

- **Fastwalk Integration**: Optimized directory traversal
- **Ignore Pattern Caching**: O(1) lookup of ignore patterns
- **Smart Path Joining**: Intelligent path resolution

## Integration Points

### OAuth Providers

- **GitHub OAuth**: GitHub authentication for Copilot integration
- **Hyper OAuth**: Hyper authentication for AI provider access
- **Token Management**: Automatic token refresh and storage

### AI Providers

- **OpenAI**: GPT-4, GPT-4o, o1, o3 models
- **Anthropic**: Claude 3, Claude 3.5, Claude 4 models with caching
- **Google**: Gemini 2.0, Gemini 2.5, Gemini 3 models
- **OpenRouter**: Multi-provider routing
- **Azure**: Azure OpenAI Service
- **VertexAI**: Google Cloud Vertex AI
- **Bedrock**: AWS Bedrock models
- **Vercel**: Vercel AI Gateway

### LSP Servers

- **gopls**: Go language server
- **typescript-language-server**: TypeScript/JavaScript
- **pyright**: Python language server
- **rust-analyzer**: Rust language server
- **clangd**: C/C++ language server
- **User-Configured**: Custom LSP server configuration

---

_Generated from Sapphire CLI codebase analysis._
