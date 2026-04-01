# ForgeCode — Comprehensive Codebase Analysis

## Overview

ForgeCode is an **AI-enhanced terminal development environment** built in Rust. It acts as a **coding agent** that integrates AI capabilities (LLM providers like OpenAI, Anthropic, Google, etc.) with a terminal-based development workflow. The project follows a **clean architecture** pattern with clear separation between domain, application, infrastructure, and presentation layers.

**Install**: `curl -fsSL https://forgecode.dev/cli | sh`
**Repository**: https://github.com/antinomyhq/forgecode.git

---

## Architecture: Clean Architecture Layers

```
crates/forge_main/    ← Presentation/CLI layer (UI, commands, rendering)
crates/forge_api/     ← Facade/API layer (public interface for main)
crates/forge_app/     ← Application layer (orchestration, use cases)
crates/forge_domain/  ← Domain layer (core types, business rules, no external deps)
crates/forge_services/← Services layer (domain logic implementations)
crates/forge_infra/   ← Infrastructure layer (external IO, LLM clients, file system)
```

### Supporting Crates

| Crate | Path | Description |
|-------|------|-------------|
| **forge_config** | `crates/forge_config/` | Configuration management (forge.yaml, environment) |
| **forge_display** | `crates/forge_display/` | Terminal rendering, markdown formatting, colored output |
| **forge_fs** | `crates/forge_fs/` | File system abstraction with content type detection |
| **forge_walker** | `crates/forge_walker/` | Recursive directory traversal with gitignore support |
| **forge_repo** | `crates/forge_repo/` | Git repository interface (uses gRPC/protobuf for workspace sync) |
| **forge_stream** | `crates/forge_stream/` | Async stream abstractions (MpscStream for chat responses) |
| **forge_template** | `crates/forge_template/` | Handlebars template engine for system prompts |
| **forge_select** | `crates/forge_select/` | Interactive fzf-based selection widgets (model/provider picker) |
| **forge_spinner** | `crates/forge_spinner/` | Terminal spinner/progress bar management |
| **forge_snaps** | `crates/forge_snaps/` | Snapshot testing utilities |
| **forge_test_kit** | `crates/forge_test_kit/` | Test fixtures and kit |
| **forge_tool_macros** | `crates/forge_tool_macros/` | Procedural macros for tool definitions |
| **forge_json_repair** | `crates/forge_json_repair/` | JSON parsing recovery for malformed LLM output |
| **forge_markdown_stream** | `crates/forge_markdown_stream/` | Streaming markdown renderer |
| **forge_tracker** | `crates/forge_tracker/` | Telemetry/tracing with PostHog integration |
| **forge_ci** | `crates/forge_ci/` | CI tooling and workflow tests |
| **forge_embed** | `crates/forge_embed/` | Embedded assets (templates, prompts, system info) |

---

## Core Domain Types (`forge_domain/src/`)

### Agents & Conversations

| File | Summary |
|------|---------|
| `agent.rs` | `AgentId` — Newtype wrapper over `String` for type-safe agent identifiers. Built-in agents: `forge` (default coding), `sage` (research), `muse` (planning). Implements `Default`, `FromStr`, `Display`, `Serialize`, `Deserialize`. |
| `agent_definition.rs` | `Agent` struct defines: `id`, `title`, `model` (ModelId), `provider` (optional ProviderId), `system_prompt`, `tools` (tool names), `tool_supported`, `max_requests_per_turn`, `max_tool_failure_per_turn`, `compact` (compaction config), `reasoning` (reasoning config), `path` (for custom agents from disk). Derives `Setters`, `Clone`, `Debug`. |
| `conversation.rs` | `Conversation` — central data structure with `id: ConversationId`, `context: Option<Context>`, `metadata: ConversationMetadata`, `metrics: SessionMetrics`. Methods: `generate()` creates new conversation, `new(id)` creates with given ID, `accumulated_usage()` returns total token/cost usage, `accumulated_cost()` calculates cost from usage, `to_html()` renders conversation as HTML, `to_html_with_related()` includes nested agent conversations. |
| `context.rs` | `Context` — ordered vector of `Arc<dyn ContextMessage>` with token count tracking. Methods: `append_message()` adds assistant response with tool calls/results, `token_count()` returns total tokens, `total_messages()`, `user_message_count()`, `assistant_message_count()`, `tool_call_count()`. Supports reasoning model detection via `is_reasoning_supported()`. |

### Messages & Tool Calls

| File | Summary |
|------|---------|
| `message.rs` | Core message types: `ContextMessage` (enum: `Text` for user/assistant messages, `ToolCall` for tool invocations, `ToolResult` for tool outputs), `ChatCompletionMessageFull` (full LLM response with content, tool_calls, reasoning, usage, finish_reason). `ToolCallFull` contains tool name and input. `ToolResult` contains output text and error status. `ToolOutput` wraps output content with `text()` and `from_json()` constructors. |
| `model.rs` | `ModelId` — newtype for model identifiers (e.g., `claude-sonnet-4`). `Model` struct with `id`, `name` (display name), `context_length` (max tokens), `tools_supported` (bool), `input_modalities` (Text/Image). `ProviderModels` groups models by provider. |
| `provider.rs` | `ProviderId` — typed provider identifier. `Provider<Url>` generic over URL type with fields: `id`, `url`, `api_key` (optional), `provider_type` (Llm/ContextEngine), `auth_methods`. Distinguishes `AnyProvider` (enum: `Url`/`Template`) from `Provider<Url>` (configured with actual URL). |

### Tools

| File | Summary |
|------|---------|
| `tools/*.rs` | All tool definitions implementing `Tool` trait: `read_file` (reads file content with line range support, binary detection), `write_file` (creates/writes files with undo tracking), `replace_in_file` (SEARCH/REPLACE blocks with fuzzy matching), `search_files` (regex search with file pattern filtering), `execute_command` (shell execution with timeout, stdout cap, restricted mode), `list_files` (recursive/non-recursive directory listing), `read_image` (base64 image encoding for vision models), `glob_tool` (glob pattern matching), `grep` (ripgrep-like search), `find_file` (find files by name), `semantic_search` (vector similarity search on indexed workspace), `undo_write` (reverts last write_file), `compaction` (forces context compaction). Each tool has `name()`, `description()`, `parameters()` (JSON Schema), `requires_stdout()` markers. |
| `tool_order.rs` | `ToolOrder` enum controlling tool sorting priority in prompts. Tools with higher priority appear first in tool list. |
| `tool_call_context.rs` | `ToolCallContext` passed to tool invocations containing conversation metrics, optional sender channel for progress updates. |

### Providers & Authentication

| File | Summary |
|------|---------|
| `provider.rs` | `AuthMethod` enum: `ApiKey` (simple key), `OAuthDevice` (device code flow), `OAuthCode` (authorization code), `GoogleAdc` (Google application default credentials), `CodexDevice` (OpenAI Codex device flow). |
| `auth/*.rs` | `AuthContextRequest` (enum: `ApiKey`, `DeviceCode`, `Code`), `AuthContextResponse`, `ApiKeyRequest` (required params, existing params), `DeviceCodeRequest` (verification URI, user code), `CodeRequest` (authorization URL). Credential storage and refresh token management. |

### Configuration

| File | Summary |
|------|---------|
| `env.rs` | `Environment` — global runtime config: `cwd`, `credentials_path`, `max_walker_depth`, `max_conversations`, `max_line_length`, `retry_config`, `http_config`, `updates`, `auto_dump` (auto-dump conversation on completion), `auto_open_dump` (open dump in browser), `log_path`. Built via `EnvironmentBuilder`. |
| `retry_config.rs` | `RetryConfig` — `initial_backoff_ms`, `max_attempts`, `backoff_factor`, `retry_status_codes` (Vec<u16>), `suppress_retry_errors` (bool). Used by `retry_with_config()` in app layer. |
| `http_config.rs` | `HttpConfig` — connection timeout, read timeout, pool idle timeout, max idle per host, max redirects, hickory DNS toggle, TLS backend, min/max TLS versions, adaptive window, keep-alive settings, accept invalid certs (dangerous), root cert paths. |
| `temperature.rs` | `Temperature` newtype wrapping `f64` with validation (0.0-2.0 range). Used in generation config. |
| `max_tokens.rs` | `MaxTokens` newtype for limiting LLM output length. |
| `top_k.rs` / `top_p.rs` | `TopK` and `TopP` sampling parameters for LLM generation. |
| `reasoning.rs` | `Reasoning` struct with `enabled` flag and possibly reasoning-specific config. Controls whether reasoning tokens are generated. |
| `mcp.rs` | `McpConfig` with `mcp_servers: IndexMap<ServerName, McpServerConfig>`. `McpServerConfig` enum: `Stdio` (command+args+env), `Http` (url+headers). `ServerName` newtype. |
| `mcp_servers.rs` | MCP server management: `McpServer` type, server type detection (`server_type()`), `is_disabled()` check. |
| `command.rs` | `Command` struct: `name`, `description`, `prompt` (Handlebars template or raw text). User-defined slash commands. |
| `skill.rs` | `Skill` struct: `name`, `description`, `path` (optional, for custom skills). Loaded from disk or embedded. |
| `hook.rs` | `Hook` — event-driven lifecycle manager with `on_start`, `on_request`, `on_response`, `on_toolcall_start`, `on_toolcall_end`, `on_end` handlers. Each handler receives `LifecycleEvent` and mutable conversation reference. Handlers can be chained with `.and()`. |
| `update.rs` | `Update` struct: `auto_update` (bool), `channel` (stable/beta). Controls self-update behavior. |
| `merge.rs` | `Merge` trait for combining configuration layers (user config + local config). Derives from `merge` crate. |
| `validation.rs` | Input validation helpers for configuration values. |
| `point.rs` | `Point` struct for cursor position in editors, used by some tools. |
| `line_numbers.rs` | Line number formatting utilities for file read display. |
| `xml.rs` | XML parsing helpers for tool output processing. |
| `suggestion.rs` | `Suggestion` type for shell command suggestions. |
| `suggest_config.rs` | `SuggestConfig` with `provider` and `model` for command suggestion feature. |
| `commit_config.rs` | `CommitConfig` with `provider` and `model` for AI commit message generation. |
| `session_metrics.rs` | `SessionMetrics` tracks token usage, cost, timing per conversation session. |
| `snapshot.rs` | `Snapshot` type for capturing conversation state at a point in time. |
| `template.rs` | `Template` type for system prompt template references. |
| `shell.rs` | Shell-related domain types and constants. |
| `image.rs` | `Image` struct for image data with base64 encoding support. |
| `attachment.rs` | `Attachment` type for file attachments in messages. |
| `chat_request.rs` | `ChatRequest` with `event: Event` and `conversation_id: ConversationId`. |
| `chat_response.rs` | `ChatResponse` enum: `TaskMessage` (tool input/output/markdown), `ToolCallStart`, `ToolCallEnd`, `RetryAttempt`, `Interrupt`, `TaskReasoning`, `TaskComplete`. Used for streaming responses. |
| `data_gen.rs` | `DataGenerationParameters` for batch JSONL processing through LLM with schema-constrained tools. |
| `event.rs` | `Event` with `text`, `additional_context`, parsed from user input. Converts to `UserCommand`. |
| `error.rs` | Domain errors: `Error` enum with variants for various failure modes. `TitleFormat` for formatted error display. |
| `file.rs` | `File` domain type with path, content, hash. |
| `file_operation.rs` | `FileOperation` enum: `Read`, `Write`, `Replace`, tracking for change detection. |
| `fuzzy_search.rs` | Fuzzy string matching utilities for tool input validation. |
| `group_by_key.rs` | Generic group-by utility for collections. |
| `max_tokens.rs` | Max token limit enforcement. |
| `message_pattern.rs` | Pattern matching utilities for message types. |
| `migration.rs` | Credential migration types from env vars to file-based storage. |
| `node.rs` | `Node` with `NodeData` enum: `FileChunk` (path + line range), `File` (full file metadata), `FileRef` (file reference), `Note` (arbitrary text), `Task` (task description). Used by semantic search. |
| `policies/*.rs` | Content policies: rate limiting, content filtering, usage policies. |
| `repo.rs` | Repository-related domain types for git operations. |
| `result_stream_ext.rs` | Extension trait for `ResultStream` with `into_full_streaming()` method. |
| `top_k.rs` | Top-k parameter for semantic search. |
| `top_p.rs` | Top-p (nucleus sampling) parameter. |

### Workspace & Semantic Search

| File | Summary |
|------|---------|
| `workspace.rs` | `Workspace` with `workspace_id`, `working_dir`, `created_at`, `last_updated`. `WorkspaceInfo` for display. `WorkspaceId` newtype with generation and parsing. |
| `node.rs` | `NodeData` variants for different indexable content types used by semantic search. |
| `file.rs` / `file_operation.rs` | File tracking structures for detecting modifications between conversation turns. |

### Transformers & Context

| File | Summary |
|------|---------|
| `transformer/*.rs` | Context transformation pipeline: `SortTools` (orders tools by priority), `TransformToolCalls` (converts tool calls to text format for non-tool-supporting models), `ImageHandling` (processes image content), `DropReasoningDetails` (removes reasoning when model doesn't support it), `ReasoningNormalizer` (strives reasoning signatures on model change), `SetModel` (updates model in context). Each implements `Transformer` trait with `transform(Context) -> Context`. |
| `system_context.rs` | `SystemContext` builder for assembling full system prompt with agent definition, tools, file tree, custom rules, partial templates. |

---

## Application Layer (`forge_app/src/`)

### Core Orchestrator

| File | Summary |
|------|---------|
| `orch.rs` | **`Orchestrator<S>`** — the execution engine. Generic over `S: AgentService`. Fields: `services`, `sender` (ArcSender for streaming), `conversation`, `environment`, `tool_definitions`, `models`, `agent`, `error_tracker` (ToolErrorTracker), `hook`. Key method `run()` — main loop: 1) Fires `LifecycleEvent::Start`, 2) Builds `Context` from conversation, 3) Applies transformers (sort tools, handle images, normalize reasoning), 4) Calls `services.chat_agent()` for LLM completion, 5) Streams response via `into_full_streaming()`, 6) Fires `LifecycleEvent::Request/Response`, 7) Executes tool calls sequentially via `execute_tool_calls()`, 8) Fires `LifecycleEvent::ToolcallStart/ToolcallEnd` per tool, 9) Appends results to context, 10) Updates conversation via `services.update()`, 11) Checks failure limits and request limits, 12) Fires `LifecycleEvent::End`, 13) Sends `TaskComplete`. Loop suspends on `should_yield` (task complete, tool requests follow-up, error limit reached, request limit reached). |
| `app.rs` | **`ForgeApp<S>`** — facade constructor for chat operations. `chat()` method: resolves agent/provider/model, refreshes credentials, builds system prompt via `SystemPrompt`, inserts user prompt via `UserPromptGenerator`, detects changed files via `ChangedFiles`, creates `Orchestrator` with hooks (`TracingHandler`, `TitleGenerationHandler`, `CompactionHandler`, `DoomLoopDetector`), spawns `MpscStream` with orchestrator execution, always saves conversation after dispatch. Also provides `compact_conversation()`, `list_tools()`, `get_models()`, `get_all_provider_models()`. |

### Executors & Services

| File | Summary |
|------|---------|
| `tool_executor.rs` | Executes tool calls by delegating to service layer's `call()` method. Handles tool input parsing, calls service, wraps result in `ToolResult`. |
| `mcp_executor.rs` | Executes MCP tool calls via MCP client. Connects to MCP servers, calls tools, returns results. |
| `agent_executor.rs` | Agent-specific execution logic, handles agent lifecycle and configuration. |
| `agent_provider_resolver.rs` | `AgentProviderResolver` — resolves which provider to use for an agent. If agent has explicit provider, uses it; otherwise falls back to default provider. |
| `retry.rs` | `retry_with_config()` — generic retry function using exponential backoff. Takes `RetryConfig`, operation function, and optional error callback for logging. Uses `backon` crate. |
| `compact.rs` | `Compactor` — reduces context size by summarizing. Uses `CompactConfig` (model, provider, prompt). Can summarize partial context. Returns `CompactionResult` with before/after token and message counts. |
| `command_generator.rs` | `CommandGenerator` — generates shell commands from natural language descriptions via LLM. |
| `title_generator.rs` | `TitleGenerator` — generates conversation titles from first messages via LLM. |
| `template_engine.rs` | `TemplateEngine` — Handlebars template rendering with embedded template loading. |
| `system_prompt.rs` | `SystemPrompt` — builds complete system prompt by combining: agent system prompt, custom instructions, tool definitions, model list, file tree, working directory info, partial templates. |
| `user_prompt.rs` | `UserPromptGenerator` — processes user input, handles additional context, formats user message. |
| `changed_files.rs` | `ChangedFiles` — detects files modified externally (not by Forge) between conversation turns, adds notification to context. |
| `data_gen.rs` | `DataGenerator` — batch processes JSONL files through LLM with schema-constrained tools for structured data extraction. |
| `tool_registry.rs` | `ToolRegistry` — central registry of all available tools (system tools + MCP tools). `list()` returns all tool definitions. `list_mcp()` returns MCP tool overview. |
| `tool_resolver.rs` | `ToolResolver` — resolves which tools an agent can use based on agent's tool configuration (include/exclude lists). |
| `walker.rs` | `Walker` — recursive file tree walker respecting gitignore, hidden files, depth limits. Returns file list for context gathering. |
| `workspace_status.rs` | `WorkspaceStatus` — calculates sync status for each file in workspace (InSync, Modified, New, Deleted, Failed). |
| `file_tracking.rs` | `FileTracker` — tracks file reads and writes within a turn for change detection. |
| `transformers/*.rs` | Message transformation pipeline applied before LLM calls. Order matters: `SortTools` → `TransformToolCalls` → `ImageHandling` → `ReasoningNormalizer`. |
| `hooks/*.rs` | Lifecycle hook implementations: `TracingHandler` (logs all events), `TitleGenerationHandler` (generates title on End event), `CompactionHandler` (auto-compacts when context gets large), `DoomLoopDetector` (detects repeated identical tool calls within request cycle). |
| `search_dedup.rs` | `SearchDedup` — removes duplicate search results based on file path similarity. |
| `init_conversation_metrics.rs` | `InitConversationMetrics` — initializes session metrics when conversation starts. |
| `apply_tunable_parameters.rs` | `ApplyTunableParameters` — applies temperature, max_tokens, top_k, top_p to context from agent config. |
| `set_conversation_id.rs` | `SetConversationId` — ensures conversation ID is set in context. |

---

## Infrastructure Layer (`forge_infra/src/`)

| File | Summary |
|------|---------|
| `llm_provider.rs` | LLM client implementations per provider type. Supports OpenAI-compatible API, Anthropic API, Google Vertex AI, AWS Bedrock. Handles streaming SSE responses, tool call parsing, retry logic. |
| `http_client.rs` | Shared HTTP client with configurable TLS (ring/rustls), timeouts (connect/read/pool), keep-alive, adaptive HTTP/2 window, root certs. Built on `reqwest` with rustls backend. |
| `mcp_client.rs` | MCP client using `rmcp` crate (v0.10). Supports Stdio (child process), SSE (reqwest-based), and Streamable HTTP transports for connecting to MCP servers. |
| `file_service.rs` | File IO operations: read, write, exists, metadata. Uses `forge_fs` abstraction. |
| `shell_service.rs` | Shell command execution via `tokio::process::Command` with timeout, stdout/stderr capture, line length limits. |


## Services Layer (`forge_services/src/`)

| File | Summary |
|------|---------|
| `services.rs` | `Services` trait combining all service traits |
| `agent.rs` | `AgentRegistry` — load built-in and custom agents from disk |
| `conversation.rs` | `ConversationService` — CRUD for conversations |
| `provider_auth.rs` | `ProviderAuthService` — OAuth flows, API key storage, credential refresh |
| `model.rs` | Model listing and validation |
| `custom_instructions.rs` | Load custom rules from forge.yaml |
| `file_discovery.rs` | `FileDiscoveryService` — list files respecting gitignore |
| `workspace.rs` | Workspace sync, query, status operations |
| `mcp.rs` | MCP server management (add/remove/list/reload) |
| `commit.rs` | AI-powered git commit message generation |
| `suggest.rs` | AI-powered shell command suggestion |

---

## Presentation Layer (`forge_main/src/`)

| File | Summary |
|------|---------|
| `main.rs` | Entry point: parses CLI, sets up panic handler, initializes tokio, runs UI |
| `cli.rs` | **`Cli`** — comprehensive clap-based CLI with subcommands: agent, list, mcp, config, conversation, workspace, commit, provider, etc. |
| `ui.rs` | **`UI<A, F>`** — main interactive loop. Handles commands, renders output, manages provider/model selection, conversation management |
| `input.rs` | `Console` — readline with reedline, slash-command completion |
| `prompt.rs` | `ForgePrompt` — builds prompt with usage stats, model info |
| `stream_renderer.rs` | `StreamingWriter` — renders streaming LLM output with tool call display |
| `state.rs` | `UIState` — current cwd, conversation id |
| `model.rs` | `ForgeCommandManager` — slash-command parsing and registration |
| `conversation_selector.rs` | Interactive conversation picker |
| `porcelain.rs` | Machine-readable output formatting (for scripting) |
| `info.rs` | `Info` struct — key-value display builder |
| `display_constants.rs` | Shared display markers and headers |
| `editor.rs` | External editor integration |
| `banner.rs` | ASCII art startup banner |
| `update.rs` | Self-update handling |
| `tools_display.rs` | Tool listing formatter |
| `title_display.rs` | Title/status line formatting |
| `sync_display.rs` | Workspace sync progress display |
| `tracker.rs` | Telemetry integration |
| `vscode.rs` | VS Code extension installation |
| `zsh/` | ZSH plugin/theme generation |
| `sandbox.rs` | Git worktree sandboxing |

---

## API Facade (`forge_api/src/`)

| File | Summary |
|------|---------|
| `api.rs` | `API` trait — complete facade used by UI layer |
| `forge_api.rs` | `ForgeAPI` implementation of all trait methods: chat, conversation CRUD, model/provider management, MCP, workspace sync, commit, etc. |

---

## Key Capabilities

### 1. Multi-Agent System
- **Forge** — default coding agent
- **Sage** (`:sage`) — research/investigation agent
- **Muse** (`:muse`) — planning/strategy agent
- Custom agents loaded from `.forge.toml` files
- Each agent has its own model, tools, system prompt, and retry config

### 2. Multi-Provider LLM Support
- **Supported**: OpenAI, Anthropic, Google Vertex AI (ADC), OpenRouter, Requesty, xAI, z.ai, Cerebras, IO Intelligence, Groq, Amazon Bedrock, OpenAI-compatible endpoints, ForgeCode Services
- **Auth methods**: API Key, OAuth Device Flow, OAuth Authorization Code, Google Application Default Credentials, OpenAI Codex Device Flow
- **Interactive login**: `forge provider login` with credential migration from env vars

### 3. Tool System
- **File ops**: `read_file` (with line ranges), `write_file`, `replace_in_file` (SEARCH/REPLACE blocks), `undo_write`
- **Search**: `search_files` (regex), `grep`, `find_file`, `glob`, `semantic_search` (vector DB)
- **Shell**: `execute_command` with timeout, stdout limits, restricted mode
- **Image**: `read_image` for vision models
- **MCP**: External tool servers via Model Context Protocol (Stdio/SSE/Streamable HTTP)
- **Compaction**: `compact` to reduce context window usage
- Tool failure limits prevent infinite retry loops

### 4. Conversation Management
- Full conversation history with JSON/HTML export (`/dump`)
- Compaction to reduce token usage (`/compact`)
- Clone, rename, delete, retry conversations
- Auto-save after every turn
- Related conversations tracking (via agent tool calls)
- Auto-dump on task completion (configurable)

### 5. Workspace & Semantic Search
- **Sync**: Index directory with file chunking, embedding generation
- **Query**: Semantic search with use-case intent, top-k filtering, prefix/suffix matching
- **Status**: Track which files are modified/added/deleted vs index
- gRPC-based workspace server for remote indexing

### 6. CLI Architecture
- Rich subcommand system: `forge agent`, `forge config`, `forge conversation`, `forge mcp`, `forge workspace`, `forge provider`, `forge commit`, `forge suggest`, `forge list`, `forge data`
- **Porcelain mode**: Machine-readable output for scripting (`--porcelain`)
- Sandboxed mode (`--sandbox`) via git worktrees
- JSONL batch data processing (`forge data`)
- Piped stdin support: `cat file | forge -p "explain this"`

### 7. Shell Integration
- ZSH plugin with command transformation: `# forge list` → actual command
- Right-prompt with model name, token count, cost
- Theme with Forge-specific formatting
- Keyboard shortcuts (`forge zsh keyboard`)
- Doctor diagnostics (`forge zsh doctor`)

### 8. Hooks & Lifecycle Events
- **Lifecycle events**: Start, Request, Response, ToolcallStart, ToolcallEnd, End
- **Built-in handlers**: TracingHandler (logging), TitleGenerationHandler, CompactionHandler (auto-compact long conversations), DoomLoopDetector (detects repeated tool failures)
- Custom hooks via agent configuration

### 9. Retry & Error Management
- Exponential backoff with configurable retry count, backoff factor, status codes
- Tool error tracker with per-tool failure limits
- JSON repair for malformed LLM responses
- Panic hook with user-friendly error display

### 10. MCP (Model Context Protocol)
- Stdio, SSE, and Streamable HTTP transports
- Local and user-scoped configuration (`.mcp.json`)
- Commands: `forge mcp add/list/remove/import/show/reload`
- Failure tracking and display per server

---

## Key Templates (`templates/`)

| Template | Purpose |
|----------|---------|
| `forge-partial-system-info.md` | System context (OS, shell, cwd) for prompts |
| `forge-partial-tool-use-example.md` | Tool usage examples for LLM |
| `forge-partial-tool-error-reflection.md` | Error reflection prompt for retries |
| `forge-tool-retry-message.md` | Retry message template when tool fails |
| `forge-commit-message-prompt.md` | Git commit message generation prompt |
| `forge-command-generator-prompt.md` | Shell command generation prompt |
| `forge-custom-agent-template.md` | Template for custom agent definitions |
| `forge-doom-loop-reminder.md` | Reminder when agent is stuck in retry loop |
| `forge-partial-summary-frame.md` | Summary frame for context compaction |
| `forge-system-prompt-title-generation.md` | Prompt for generating conversation titles |

---

## Shell Plugin (`shell-plugin/`)

- **forge.plugin.zsh** — Main plugin: command completion, `#` prefix transformation
- **forge.theme.zsh** — Terminal theme integration
- **forge.setup.zsh** — Setup helpers
- **doctor.zsh** — Diagnostics
- **keyboard.zsh** — Keyboard shortcut help
- **lib/** — Modular plugin components (bindings, completion, config, dispatcher, helpers, highlight, actions)

---


## Supporting Crate Details

### `forge_config` — Configuration Management
| File | Summary |
|------|---------|
| `lib.rs` | Configuration loading pipeline: merges user config (~/.forge.toml) + local config (.forge.toml) + CLI overrides. Uses `merge` crate for layering. |
| `config.rs` | `ForgeConfig` struct: parsed forge.yaml with model, provider, custom_rules, commands, max_walker_depth, temperature, max_requests_per_turn, max_tool_failure_per_turn. |

### `forge_display` — Terminal Rendering
| File | Summary |
|------|---------|
| `markdown.rs` | `MarkdownFormat` — renders markdown via `termimad` with syntax highlighting via `syntect`. Supports code block language detection and coloring. |
| `table.rs` | Table formatting for info display with column alignment. |
| `colors.rs` | Color constants and theme helpers using `colored` crate. |

### `forge_fs` — File System Abstraction
| File | Summary |
|------|---------|
| `forge_fs.rs` | `ForgeFS` — file operations: `read`, `read_utf8`, `read_range` (byte ranges for large files), `write`, `exists`, `metadata`. Binary content detection via `infer` crate. Content type detection for images, text, binary. |
| `line_range.rs` | `LineRange` for reading specific line ranges from files (e.g., lines 10-50). |
| `search.rs` | File search utilities using `grep-searcher` and `grep-regex`. Supports file pattern filtering. |

### `forge_walker` — Directory Traversal
| File | Summary |
|------|---------|
| `walker.rs` | `Walker` — recursive directory traversal respecting `.gitignore`, `.forgeignore`, hidden files, max depth limits. Returns sorted file list with relative paths. Uses `ignore` crate. |

### `forge_repo` — Git Operations
| File | Summary |
|------|---------|
| `lib.rs` | Git repository abstraction: diff generation, status checking, commit operations. Uses `gix` crate for git operations. ProtoBuf definitions for workspace sync protocol. |
| `diff.rs` | Git diff generation with size limits for commit command. |

### `forge_stream` — Async Stream Abstractions
| File | Summary |
|------|---------|
| `mpsc_stream.rs` | `MpscStream` — spawns a task producing items sent via tokio mpsc channel. Used by chat to stream responses without blocking. |
| `result_stream_ext.rs` | Extension trait for Result streams with error handling utilities. |

### `forge_template` — Template Engine
| File | Summary |
|------|---------|
| `template.rs` | `TemplateEngine` — Handlebars template rendering with embedded template loading from `include_dir!`. Used for system prompts, tool error messages, retry messages. |

### `forge_select` — Interactive Selection
| File | Summary |
|------|---------|
| `widget.rs` | `ForgeWidget::select()` and `ForgeWidget::confirm()` — fzf-based interactive selection with starting cursor, header lines, help messages. Used for model/provider/agent/conversation selection. |

### `forge_spinner` — Terminal Spinners
| File | Summary |
|------|---------|
| `spinner.rs` | `SpinnerManager` — animated spinner for in-progress operations. Supports write-while-spinning, start/stop/reset, message updates. |
| `progress.rs` | `ProgressBarManager` — progress bar for workspace sync with percentage tracking. |

### `forge_tracker` — Telemetry
| File | Summary |
|------|---------|
| `tracker.rs` | `Tracker` — PostHog-based telemetry with events: login, model set, tool call, prompt, error, crash. Respects `FORGE_TRACKER=false` env var. |
| `payload.rs` | `ToolCallPayload` — structured tool call tracking with name, input, output, error, duration. |

### `forge_json_repair` — JSON Recovery
| File | Summary |
|------|---------|
| `lib.rs` | Repairs malformed JSON from LLM responses: fixes missing commas, unclosed brackets, trailing commas, unquoted strings. Critical for parsing tool call inputs. |

### `forge_markdown_stream` — Streaming Markdown
| File | Summary |
|------|---------|
| `stream.rs` | Processes and renders markdown incrementally as text chunks arrive from LLM streaming. |

### `forge_ci` — CI & Testing
| File | Summary |
|------|---------|
| `lib.rs` | CI workflow definitions and GitHub Actions integration. |
| `tests/` | Integration tests for forge workflows. |

### `forge_embed` — Embedded Assets
| File | Summary |
|------|---------|
| `lib.rs` | Embeds templates, prompts, system info, built-in agent definitions using `include_dir!`. Zero-IO access to static assets at runtime. |

---

## Data Flow Architecture

```
User Input (CLI/Prompt)
    ↓
ForgeAPI.chat(ChatRequest)
    ↓
ForgeApp.chat() ─────────────────────────────────┐
    ↓                                              │
1. Resolve Agent/Provider/Model                    │
2. Refresh Credentials                             │
3. Build System Prompt (files, tools, rules)      │
4. Insert User Prompt                              │
5. Detect Changed Files                            │
6. Apply Tunable Parameters                        │
7. Create Orchestrator with Hooks                  │
8. Spawn MpscStream                                │
    ↓                                              │
Orchestrator.run() ────────────────────────────────┤
    ↓                                              │
Loop:                                              │
    ├─ Apply Transformers → Context                │
    ├─ LLM Completion → ChatCompletionMessageFull  │
    ├─ Stream Delta to UI                          │
    ├─ Execute Tool Calls (sequential)             │
    ├─ Fire Lifecycle Hooks                        │
    ├─ Update Context                              │
    ├─ Check Limits (errors, requests)             │
    └─ Save Conversation                           │
```

---

## Lifecyle Event Flow

```
Start → Request → Response → (ToolcallStart → ToolcallEnd)* → (Request → Response)* → End
  │       │        │             │         │              │                │
  │       │        │             │         │              │                └─ TitleGeneration
  │       │        │             │         │              └─ CompactionCheck
  │       │        │             │         └─ Tool result recorded
  │       │        │             └─ Tool input recorded
  │       │        └─ DoomLoopDetection
  │       └─ Request count tracked
  └─ Tracing logs
```

---

## Build & Configuration

- **Rust edition 2024**, MSRV 1.92
- **Key deps**: tokio, anyhow, thiserror, tracing, clap, serde, reqwest (rustls), handlebars, reedline, fzf-wrapped, rmcp (MCP), posthog-rs, aws-sdk-bedrock, google-cloud-auth, arboard (clipboard), open (browser)
- **Profiles**: release with LTO=true, codegen-units=1, opt-level=3, strip=true
- **Nix support** via `flake.nix` (latest dev branch)
- **Cross-compilation** via `Cross.toml`
- **Diesel ORM** for database migrations (workspace sync)
- **Protobuf** via `forge_repo/proto/` for workspace gRPC protocol
- **Testing**: `insta` for snapshot tests, `pretty_assertions` for diff output, `mockito` for HTTP mocking

---

## Codebase Statistics

- **~30 crates** in workspace
- **forge_main/src/ui.rs** — largest file (~3,000 lines), handles all UI/command logic
- **forge_main/src/cli.rs** — comprehensive CLI definitions with 100+ tests
- **forge_app/src/orch.rs** — core orchestrator loop (~300 lines)
- **forge_app/src/app.rs** — chat entry point (~200 lines)
- Clean test patterns: fixtures/expected/actual with `pretty_assertions`
- All tests in same file as source per AGENTS.md

---

## Plans Directory

The `plans/` directory contains architecture decision records (ADRs) and implementation plans:

| Plan | Description |
|------|-------------|
| `2025-04-02-system-context-rendering-*.md` | System context rendering design (3 versions) |
| `2025-04-06-retry-config-migration.md` | Retry configuration migration plan |
| `2025-04-11-tool-call-context-implementation.md` | ToolCallContext design for tool execution |
| `2025-04-16-model-selection-command.md` | Model selection UI implementation |
| `2025-04-26/27-large-file-read-range-support-v*.md` | Large file read support with line ranges (3 versions) |
| `2025-06-07-tool-service-migration-v1.md` | Tool service migration plan |
| `2025-09-*` | Various: shell env variables, agent loader, dump auto-open, slash commands, history file, conversation ID |
| `agent-context-compaction-*.md` | Context compaction design documents (4 versions over 2 days) |




------------------------------------------------------------------------------------------------------------------------



# Agent Guidelines

This document contains guidelines and best practices for AI agents working with this codebase.

## Error Management

- Use `anyhow::Result` for error handling in services and repositories.
- Create domain errors using `thiserror`.
- Never implement `From` for converting domain errors, manually convert them

## Writing Tests

- All tests should be written in three discrete steps:

  ```rust,ignore
  use pretty_assertions::assert_eq; // Always use pretty assertions

  fn test_foo() {
      let setup = ...; // Instantiate a fixture or setup for the test
      let actual = ...; // Execute the fixture to create an output
      let expected = ...; // Define a hand written expected result
      assert_eq!(actual, expected); // Assert that the actual result matches the expected result
  }
  ```

- Use `pretty_assertions` for better error messages.

- Use fixtures to create test data.

- Use `assert_eq!` for equality checks.

- Use `assert!(...)` for boolean checks.

- Use unwraps in test functions and anyhow::Result in fixtures.

- Keep the boilerplate to a minimum.

- Use words like `fixture`, `actual` and `expected` in test functions.

- Fixtures should be generic and reusable.

- Test should always be written in the same file as the source code.

- Use `new`, Default and derive_setters::Setters to create `actual`, `expected` and specially `fixtures`. For example:

  **Good:**

  ```rust,ignore
  User::default().age(12).is_happy(true).name("John")
  User::new("Job").age(12).is_happy()
  User::test() // Special test constructor
  ```

  **Bad:**

  ```rust,ignore
  User {name: "John".to_string(), is_happy: true, age: 12}
  User::with_name("Job") // Bad name, should stick to User::new() or User::test()
  ```

- Use `unwrap()` unless the error information is useful. Use `expect` instead of `panic!` when error message is useful. For example:

  **Good:**

  ```rust,ignore
  users.first().expect("List should not be empty")
  ```

  **Bad:**

  ```rust,ignore
  if let Some(user) = users.first() {
      // ...
  } else {
      panic!("List should not be empty")
  }
  ```

- Prefer using `assert_eq` on full objects instead of asserting each field:

  **Good:**

  ```rust,ignore
  assert_eq!(actual, expected);
  ```

  **Bad:**

  ```rust,ignore
  assert_eq!(actual.a, expected.a);
  assert_eq!(actual.b, expected.b);
  ```

## Verification

Always verify changes by running tests and linting the codebase

1. Run crate specific tests to ensure they pass.

   ```
   cargo insta test --accept
   ```

2. **Build Guidelines**:
   - **NEVER** run `cargo build --release` unless absolutely necessary (e.g., performance testing, creating binaries for distribution)
   - For verification, use `cargo check` (fastest), `cargo insta test`, or `cargo build` (debug mode)
   - Release builds take significantly longer and are rarely needed for development verification

## Writing Domain Types

- Use `derive_setters` to derive setters and use the `strip_option` and the `into` attributes on the struct types.

## Documentation

- **Always** write Rust docs (`///`) for all public methods, functions, structs, enums, and traits.
- Document parameters with `# Arguments` and errors with `# Errors` sections when applicable.
- **Do not include code examples** - docs are for LLMs, not humans. Focus on clear, concise functionality descriptions.

## Refactoring

- If asked to fix failing tests, always confirm whether to update the implementation or the tests.

## Git Operations

- Safely assume git is pre-installed
- Safely assume github cli (gh) is pre-installed
- Always use `Co-Authored-By: ForgeCode <noreply@forgecode.dev>` for git commits and Github comments

## Service Implementation Guidelines

Services should follow clean architecture principles and maintain clear separation of concerns:

### Core Principles

- **No service-to-service dependencies**: Services should never depend on other services directly
- **Infrastructure dependency**: Services should depend only on infrastructure abstractions when needed
- **Single type parameter**: Services should take at most one generic type parameter for infrastructure
- **No trait objects**: Avoid `Box<dyn ...>` - use concrete types and generics instead
- **Constructor pattern**: Implement `new()` without type bounds - apply bounds only on methods that need them
- **Compose dependencies**: Use the `+` operator to combine multiple infrastructure traits into a single bound
- **Arc<T> for infrastructure**: Store infrastructure as `Arc<T>` for cheap cloning and shared ownership
- **Tuple struct pattern**: For simple services with single dependency, use tuple structs `struct Service<T>(Arc<T>)`

### Examples

#### Simple Service (No Infrastructure)

```rust,ignore
pub struct UserValidationService;

impl UserValidationService {
    pub fn new() -> Self { ... }

    pub fn validate_email(&self, email: &str) -> Result<()> {
        // Validation logic here
        ...
    }

    pub fn validate_age(&self, age: u32) -> Result<()> {
        // Age validation logic here
        ...
    }
}
```

#### Service with Infrastructure Dependency

```rust,ignore
// Infrastructure trait (defined in infrastructure layer)
pub trait UserRepository {
    fn find_by_email(&self, email: &str) -> Result<Option<User>>;
    fn save(&self, user: &User) -> Result<()>;
}

// Service with single generic parameter using Arc
pub struct UserService<R> {
    repository: Arc<R>,
}

impl<R> UserService<R> {
    // Constructor without type bounds, takes Arc<R>
    pub fn new(repository: Arc<R>) -> Self { ... }
}

impl<R: UserRepository> UserService<R> {
    // Business logic methods have type bounds where needed
    pub fn create_user(&self, email: &str, name: &str) -> Result<User> { ... }
    pub fn find_user(&self, email: &str) -> Result<Option<User>> { ... }
}
```

#### Tuple Struct Pattern for Simple Services

```rust,ignore
// Infrastructure traits
pub trait FileReader {
    async fn read_file(&self, path: &Path) -> Result<String>;
}

pub trait Environment {
    fn max_file_size(&self) -> u64;
}

// Tuple struct for simple single dependency service
pub struct FileService<F>(Arc<F>);

impl<F> FileService<F> {
    // Constructor without bounds
    pub fn new(infra: Arc<F>) -> Self { ... }
}

impl<F: FileReader + Environment> FileService<F> {
    // Business logic methods with composed trait bounds
    pub async fn read_with_validation(&self, path: &Path) -> Result<String> { ... }
}
```

### Anti-patterns to Avoid

```rust,ignore
// BAD: Service depending on another service
pub struct BadUserService<R, E> {
    repository: R,
    email_service: E, // Don't do this!
}

// BAD: Using trait objects
pub struct BadUserService {
    repository: Box<dyn UserRepository>, // Avoid Box<dyn>
}

// BAD: Multiple infrastructure dependencies with separate type parameters
pub struct BadUserService<R, C, L> {
    repository: R,
    cache: C,
    logger: L, // Too many generic parameters - hard to use and test
}

impl<R: UserRepository, C: Cache, L: Logger> BadUserService<R, C, L> {
    // BAD: Constructor with type bounds makes it hard to use
    pub fn new(repository: R, cache: C, logger: L) -> Self { ... }
}

// BAD: Usage becomes cumbersome
let service = BadUserService::<PostgresRepo, RedisCache, FileLogger>::new(...);
```
---------------------------------------------------------------------------------------------