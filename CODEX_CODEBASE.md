# Codex Codebase Documentation

**Repository:** `codex/` (OpenAI Codex CLI)  
**Primary Implementation:** Rust (`codex-rs/`)  
**Legacy Implementation:** TypeScript/JavaScript (`codex-cli/`)  
**Total Rust Source Files:** ~1,184 `.rs` files  
**Total Lines of Code (Rust):** ~1363 files including tests, configs, docs

---

## Table of Contents

1. [Overview](#overview)
2. [Repository Structure](#repository-structure)
3. [Core Architecture](#core-architecture)
4. [Workspace Crates (codex-rs)](#workspace-crates-codex-rs)
5. [Key Modules Deep Dive](#key-modules-deep-dive)
6. [Protocol & API](#protocol--api)
7. [Security Model](#security-model)
8. [Configuration System](#configuration-system)
9. [Testing Infrastructure](#testing-infrastructure)
10. [Build System](#build-system)

---

## Overview

Codex is an AI-powered coding agent from OpenAI that runs locally. It provides:
- Interactive CLI for chat-driven development
- Code execution in sandboxed environments
- File manipulation with approval workflows
- Multi-modal input (text + images)
- MCP (Model Context Protocol) server support

**Key Design Principles:**
- Sandbox-first security model
- Approval-based execution (configurable autonomy levels)
- JSON-RPC 2.0 protocol for client-server communication
- Rust workspace with modular crate architecture
- Bazel + Cargo dual build system

---

## Repository Structure

```
codex/
├── codex-rs/                          # Rust implementation (primary)
│   ├── core/                          # Core agent logic, tools, execution
│   ├── tui/                           # Terminal UI (ratatui-based)
│   ├── app-server/                    # JSON-RPC server for IDE integrations
│   ├── app-server-protocol/           # Protocol definitions (v1, v2)
│   ├── protocol/                      # Shared protocol types
│   ├── config/                        # Configuration loading & validation
│   ├── tools/                         # Tool implementations (shell, file, etc.)
│   ├── mcp-server/                    # MCP protocol server
│   ├── rmcp-client/                   # MCP client implementation
│   ├── network-proxy/                 # HTTP/SOCKS5 proxy for network policy
│   ├── linux-sandbox/                 # Linux sandboxing (landlock, bubblewrap)
│   ├── windows-sandbox-rs/            # Windows sandbox implementation
│   ├── execpolicy/                    # Execution policy parser & checker
│   ├── execpolicy-legacy/             # Legacy exec policy (deprecated)
│   ├── state/                         # SQLite state database
│   ├── otel/                          # OpenTelemetry integration
│   ├── login/                         # OAuth/device code authentication
│   ├── chatgpt/                       # ChatGPT API client
│   ├── codex-client/                  # High-level Codex API client
│   ├── codex-api/                     # REST API endpoints
│   ├── backend-client/                # Backend API client
│   ├── connectors/                    # External service connectors
│   ├── skills/                        # Skill system (reusable workflows)
│   ├── hooks/                         # Event hooks system
│   ├── artifacts/                     # Artifact runtime management
│   ├── package-manager/               # Package management utilities
│   ├── file-search/                   # Fuzzy file search (ripgrep wrapper)
│   ├── shell-command/                 # Shell command parsing & safety
│   ├── shell-escalation/              # Privilege escalation (sudo)
│   ├── apply-patch/                   # Unified diff patch application
│   ├── cli/                           # CLI entrypoint
│   ├── exec/                          # Execution engine
│   ├── feedback/                      # Feedback submission
│   ├── debug-client/                  # Debugging utilities
│   ├── lmstudio/                      # LM Studio integration
│   ├── ollama/                        # Ollama integration
│   ├── keyring-store/                 # Secure credential storage
│   ├── secrets/                       # Secret sanitization
│   ├── process-hardening/             # Process security hardening
│   ├── responses-api-proxy/           # OpenAI Responses API proxy
│   ├── stdio-to-uds/                  # Stdio to Unix domain socket bridge
│   ├── test-macros/                   # Test helper macros
│   ├── cloud-tasks/                   # Cloud task management
│   ├── cloud-tasks-client/            # Cloud tasks API client
│   ├── cloud-requirements/            # Cloud requirement enforcement
│   ├── app-server-client/             # App server client library
│   ├── app-server-test-client/        # Test client for app-server
│   ├── codex-backend-openapi-models/  # OpenAPI models for backend
│   ├── codex-experimental-api-macros/ # Experimental API macros
│   └── utils/                         # Utility crates
│       ├── absolute-path/             # Absolute path handling
│       ├── ansi-escape/               # ANSI escape sequence parsing
│       ├── approval-presets/          # Approval mode presets
│       ├── async-utils/               # Async utilities
│       ├── cache/                     # Caching utilities
│       ├── cargo-bin/                 # Cargo binary location helpers
│       ├── cli/                       # CLI utilities
│       ├── elapsed/                   # Elapsed time formatting
│       ├── fuzzy-match/               # Fuzzy matching
│       ├── git/                       # Git operations
│       ├── home-dir/                  # Home directory resolution
│       ├── image/                     # Image handling
│       ├── json-to-toml/              # JSON/TOML conversion
│       ├── oss/                       # OSS (Open Source Software) utilities
│       ├── pty/                       # PTY (pseudo-terminal) handling
│       ├── readiness/                 # Readiness checks
│       ├── rustls-provider/           # rustls TLS provider
│       ├── sandbox-summary/           # Sandbox configuration summary
│       ├── sleep-inhibitor/           # Sleep inhibition (macOS/Linux/Windows)
│       ├── stream-parser/             # Stream text parsing
│       └── string/                    # String utilities
├── codex-cli/                         # Legacy TypeScript implementation
│   ├── bin/                           # CLI entrypoint (codex.js)
│   ├── scripts/                       # Build & install scripts
│   └── package.json
├── docs/                              # Documentation
│   ├── contributing.md
│   ├── install.md
│   ├── open-source-fund.md
│   └── ...
├── patches/                           # Git patches for dependencies
├── scripts/                           # Repository scripts
├── sdk/                               # SDK documentation/examples
├── shell-tool-mcp/                    # Shell tool MCP server
├── third_party/                       # Third-party dependencies
└── .github/                           # GitHub Actions workflows
```

---

## Core Architecture

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Client Layer                              │
├─────────────────┬─────────────────┬─────────────────────────────┤
│   codex-tui     │   VS Code Ext   │   MCP Clients / Custom      │
│   (ratatui)     │   (WebSocket)   │   (JSON-RPC over stdio/ws)  │
└────────┬────────┴────────┬────────┴──────────────┬──────────────┘
         │                 │                        │
         └─────────────────┼────────────────────────┘
                           │
                  ┌────────▼────────┐
                  │  codex-app-server │
                  │  (JSON-RPC 2.0)   │
                  └────────┬────────┘
                           │
         ┌─────────────────┼─────────────────┐
         │                 │                 │
┌────────▼────────┐ ┌─────▼──────┐ ┌────────▼────────┐
│  codex-core     │ │codex-state │ │  codex-otel     │
│  (Agent Logic)  │ │ (SQLite DB)│ │ (Telemetry)     │
└────────┬────────┘ └────────────┘ └─────────────────┘
         │
    ┌────┴────┬─────────────┬──────────────┬─────────────┐
    │         │             │              │             │
┌───▼──┐ ┌───▼────┐ ┌──────▼──────┐ ┌─────▼────┐ ┌────▼─────┐
│Tools │ │Sandbox │ │ Exec Policy │ │ Network  │ │  Skills  │
│Registry│ │Loader │ │  Checker   │ │  Proxy   │ │  System  │
└───────┘ └────────┘ └─────────────┘ └──────────┘ └──────────┘
```

### Data Flow

1. **User Input** → TUI/App Server Client → JSON-RPC request
2. **App Server** → Routes to Message Processor → Thread State
3. **Codex Core** → Processes turn → Selects tools → Executes with sandbox
4. **Tool Execution** → Shell/File/MCP → Returns output
5. **Response Streaming** → SSE/JSON-RPC notifications → Client UI
6. **State Persistence** → SQLite (state_db) + Rollout files

---

## Workspace Crates (codex-rs)

### Core Crates

#### `codex-core`
**Path:** `codex-rs/core/`  
**Purpose:** Central agent logic, tool orchestration, conversation management

**Key Modules:**
- `codex/` - Main Codex agent loop, turn processing
- `codex_thread/` - Thread (conversation) state management
- `tools/` - Tool registry and handlers
  - `handlers/` - Individual tool implementations
    - `shell.rs` - Shell command execution
    - `apply_patch.rs` - File patch application
    - `read_file.rs` - File reading
    - `list_dir.rs` - Directory listing
    - `grep_files.rs` - Code search
    - `mcp.rs` - MCP tool calls
    - `multi_agents/` - Sub-agent spawning
    - `request_user_input.rs` - Interactive prompts
    - `request_permissions.rs` - Permission requests
    - `plan.rs` - Planning tool
    - `artifacts.rs` - Artifact generation
    - `js_repl.rs` - JavaScript REPL
    - `tool_suggest.rs` - Tool suggestions
    - `tool_search.rs` - Tool discovery
    - `view_image.rs` - Image viewing
    - `web_search.rs` - Web search
  - `runtimes/` - Execution runtimes
    - `shell.rs` - Shell runtime
    - `apply_patch.rs` - Patch runtime
    - `unified_exec.rs` - Unified execution engine
  - `code_mode/` - Code mode execution
  - `registry.rs` - Tool registration
  - `router.rs` - Tool routing logic
  - `orchestrator.rs` - Tool orchestration
  - `parallel.rs` - Parallel tool execution
  - `sandboxing.rs` - Sandbox enforcement
- `config/` - Configuration types and loading
- `auth/` - Authentication management
- `mcp/` - MCP connection management
- `mcp_connection_manager.rs` - MCP server connections
- `skills/` - Skill system
  - `loader.rs` - Skill loading
  - `manager.rs` - Skill management
  - `invocation.rs` - Skill invocation
  - `injection.rs` - Skill injection
  - `render.rs` - Skill rendering
  - `system.rs` - System skills
  - `remote.rs` - Remote skills
- `exec/` - Execution engine
- `exec_policy.rs` - Execution policy enforcement
- `shell.rs` - Shell abstraction
- `shell_snapshot.rs` - Shell state snapshots
- `seatbelt.rs` - macOS sandboxing (sandbox-exec)
- `landlock.rs` - Linux Landlock LSM
- `windows_sandbox.rs` - Windows Sandbox
- `network_proxy_loader.rs` - Network proxy management
- `network_policy_decision.rs` - Network policy decisions
- `guardian.rs` - Safety checks
- `memories/` - Long-term memory system
- `plugins/` - Plugin system
- `models_manager.rs` - Model management
- `rollout.rs` - Session persistence
- `thread_manager.rs` - Thread lifecycle
- `state_db.rs` - SQLite database
- `token_data.rs` - Token tracking
- `truncate.rs` - Text truncation
- `turn_diff_tracker.rs` - Turn change tracking
- `turn_metadata.rs` - Turn metadata
- `turn_timing.rs` - Turn timing
- `unified_exec/` - Unified execution
  - `process_manager.rs` - Process management
  - `async_watcher.rs` - Async output watching
  - `head_tail_buffer.rs` - Output buffering
  - `process.rs` - Process abstraction
- `agent/` - Agent behavior
- `codex_delegate.rs` - Delegate pattern
- `context_manager.rs` - Context management
- `event_mapping.rs` - Event type mapping
- `function_tool.rs` - Function tooling
- `review_format.rs` - Review formatting
- `safety/` - Safety checks
- `tasks/` - Task management
  - `compact.rs` - Context compaction
  - `review.rs` - Review tasks
  - `undo.rs` - Undo operations
  - `regular.rs` - Regular tasks
  - `ghost_snapshot.rs` - Ghost snapshots
- `state/` - State management
  - `session.rs` - Session state
  - `turn.rs` - Turn state
  - `service.rs` - State service
- `user_shell_command.rs` - User shell commands
- `web_search.rs` - Web search integration
- `git_info.rs` - Git information
- `instructions.rs` - Custom instructions
- `mention_syntax.rs` - Mention parsing
- `mentions.rs` - Mention handling
- `message_history.rs` - Message history
- `model_provider_info.rs` - Model provider config
- `path_utils.rs` - Path utilities
- `personality_migration.rs` - Personality migration
- `sandbox_tags.rs` - Sandbox tagging
- `session_prefix.rs` - Session prefixes
- `shell_detect.rs` - Shell detection
- `spawn.rs` - Process spawning
- `terminal.rs` - Terminal abstraction
- `text_encoding.rs` - Text encoding
- `util.rs` - Utilities

**Dependencies:** 80+ workspace crates, anyhow, tokio, serde, reqwest, tracing

---

#### `codex-tui`
**Path:** `codex-rs/tui/`  
**Purpose:** Terminal user interface using ratatui

**Key Modules:**
- `app.rs` - Main application state
- `chatwidget.rs` - Chat widget
  - `agent.rs` - Agent display
  - `interrupts.rs` - Interrupt handling
  - `realtime.rs` - Realtime conversation
  - `session_header.rs` - Session header
  - `skills.rs` - Skills display
- `bottom_pane/` - Bottom pane components
  - `chat_composer.rs` - Input composer
  - `approval_overlay.rs` - Approval dialogs
  - `command_popup.rs` - Command suggestions
  - `file_search_popup.rs` - File search
  - `skill_popup.rs` - Skill selection
  - `slash_commands.rs` - Slash command handling
  - `textarea.rs` - Text input widget
  - `unified_exec_footer.rs` - Execution footer
  - `request_user_input/` - User input prompts
  - `mcp_server_elicitation.rs` - MCP server setup
  - `multi_select_picker.rs` - Multi-select UI
  - `pending_thread_approvals.rs` - Pending approvals
- `streaming/` - Streaming output
  - `controller.rs` - Stream controller
  - `chunking.rs` - Text chunking
  - `commit_tick.rs` - Commit ticking
- `markdown/` - Markdown rendering
  - `markdown_render.rs` - Markdown renderer
  - `markdown_stream.rs` - Streaming markdown
- `exec_cell/` - Execution cell
  - `model.rs` - Cell model
  - `render.rs` - Cell rendering
- `status/` - Status indicators
  - `account.rs` - Account status
  - `rate_limits.rs` - Rate limit display
  - `card.rs` - Status card
- `onboarding/` - Onboarding flow
  - `auth.rs` - Authentication screen
  - `welcome.rs` - Welcome screen
  - `trust_directory.rs` - Directory trust
  - `onboarding_screen.rs` - Main onboarding
- `notifications/` - Desktop notifications
  - `bel.rs` - BEL notifications
  - `osc9.rs` - OSC9 notifications
- `render/` - Rendering utilities
  - `highlight.rs` - Syntax highlighting
  - `line_utils.rs` - Line manipulation
  - `renderable.rs` - Renderable trait
- `public_widgets/` - Public widget components
  - `composer_input.rs` - Composer input widget
- `multi_agents.rs` - Multi-agent display
- `resume_picker.rs` - Session resume UI
- `selection_list.rs` - Selection lists
- `file_search.rs` - File search
- `external_editor.rs` - External editor integration
- `diff_render.rs` - Diff rendering
- `history_cell.rs` - History display
- `pager_overlay.rs` - Pager overlay
- `status_indicator_widget.rs` - Status indicator
- `theme_picker.rs` - Theme selection
- `voice.rs` - Voice input (macOS/Windows)
- `clipboard_paste.rs` - Clipboard handling
- `cwd_prompt.rs` - Working directory prompt
- `get_git_diff.rs` - Git diff display
- `key_hint.rs` - Key hints
- `live_wrap.rs` - Live text wrapping
- `line_truncation.rs` - Line truncation
- `mention_codec.rs` - Mention encoding
- `model_migration.rs` - Model migration UI
- `session_log.rs` - Session logging
- `shimmer.rs` - Loading animations
- `skills_helpers.rs` - Skill helpers
- `slash_command.rs` - Slash commands
- `style.rs` - Styling utilities
- `text_formatting.rs` - Text formatting
- `tooltips.rs` - Tooltips
- `tui.rs` - TUI main loop
- `ui_consts.rs` - UI constants
- `wrapping.rs` - Text wrapping helpers
- `ascii_animation.rs` - ASCII animations
- `color.rs` - Color utilities
- `frames.rs` - Frame management
- `insert_history.rs` - History insertion
- `custom_terminal.rs` - Custom terminal backend
- `debug_config.rs` - Debug configuration
- `exec_command.rs` - Command execution
- `updates.rs` - Update checking
- `version.rs` - Version display

**Dependencies:** ratatui, crossterm, tokio, codex-core, codex-app-server-client

---

#### `codex-app-server`
**Path:** `codex-rs/app-server/`  
**Purpose:** JSON-RPC 2.0 server for IDE integrations

**Key Modules:**
- `message_processor.rs` - JSON-RPC message processing
- `transport.rs` - Transport layer (stdio, WebSocket)
- `codex_message_processor.rs` - Codex-specific processing
- `thread_state.rs` - Per-thread state management
- `thread_status.rs` - Thread status tracking
- `outgoing_message.rs` - Outgoing message routing
- `command_exec.rs` - Command execution endpoint
- `config_api.rs` - Configuration API
- `dynamic_tools.rs` - Dynamic tool registration
- `external_agent_config_api.rs` - External agent config
- `fuzzy_file_search.rs` - File search endpoint
- `models.rs` - Model listing
- `error_code.rs` - Error code definitions
- `filters.rs` - Request filters
- `in_process.rs` - In-process communication
- `app_server_tracing.rs` - Tracing setup
- `bespoke_event_handling.rs` - Custom event handling

**API Endpoints:**
- `thread/start` - Create new thread
- `thread/resume` - Resume existing thread
- `thread/fork` - Fork thread
- `thread/list` - List threads (cursor pagination)
- `thread/read` - Read thread by ID
- `thread/metadata/update` - Update thread metadata
- `thread/archive` - Archive thread
- `thread/unarchive` - Unarchive thread
- `thread/compact/start` - Start compaction
- `thread/rollback` - Rollback turns
- `thread/name/set` - Set thread name
- `thread/status/changed` - Status notification
- `turn/start` - Start turn
- `turn/steer` - Steer in-flight turn
- `turn/interrupt` - Interrupt turn
- `turn/completed` - Turn completion notification
- `model/list` - List models
- `command/exec` - Execute command
- `skills/list` - List skills
- `plugin/list` - List plugins
- `plugin/install` - Install plugin
- `plugin/uninstall` - Uninstall plugin
- `mcpServer/oauth/login` - MCP OAuth login
- `mcpServerStatus/list` - List MCP server status
- `config/read` - Read config
- `config/value/write` - Write config value
- `config/batchWrite` - Batch config write
- `feedback/upload` - Upload feedback
- `review/start` - Start review
- `collaborationMode/list` - List collaboration modes
- `experimentalFeature/list` - List experimental features
- `thread/realtime/start` - Start realtime session
- `thread/realtime/appendAudio` - Append audio
- `thread/realtime/appendText` - Append text
- `thread/realtime/stop` - Stop realtime
- `windowsSandbox/setupStart` - Windows sandbox setup
- `externalAgentConfig/detect` - Detect external agent config
- `externalAgentConfig/import` - Import external agent config
- `configRequirements/read` - Read config requirements

**Dependencies:** axum, tokio, tokio-tungstenite, codex-core, codex-protocol

---

#### `codex-app-server-protocol`
**Path:** `codex-rs/app-server-protocol/`  
**Purpose:** Protocol definitions for app-server

**Key Modules:**
- `protocol/`
  - `common.rs` - Common types
  - `v1.rs` - V1 API types
  - `v2.rs` - V2 API types (active development)
  - `thread_history.rs` - Thread history types
  - `serde_helpers.rs` - Serde helpers
  - `mappers.rs` - Type mappers
- `jsonrpc_lite.rs` - JSON-RPC lite types
- `export.rs` - Schema export
- `schema_fixtures.rs` - Schema fixtures
- `experimental_api.rs` - Experimental API

**Protocol Types:**
- `*Params` - Request payloads
- `*Response` - Response types
- `*Notification` - Notifications
- Thread, Turn, Item types
- Approval types
- MCP types
- Config types
- Model types
- Skill types
- Plugin types

**Dependencies:** serde, serde_json, ts-rs, schemars

---

#### `codex-protocol`
**Path:** `codex-rs/protocol/`  
**Purpose:** Shared protocol types

**Key Modules:**
- `protocol.rs` - Core protocol types
- `items.rs` - Item types
- `approvals.rs` - Approval types
- `mcp.rs` - MCP types
- `models.rs` - Model types
- `config_types.rs` - Config types
- `permissions.rs` - Permission types
- `plan_tool.rs` - Plan tool types
- `request_permissions.rs` - Permission request types
- `request_user_input.rs` - User input types
- `user_input.rs` - User input types
- `message_history.rs` - Message history
- `parse_command.rs` - Command parsing
- `custom_prompts.rs` - Custom prompts
- `dynamic_tools.rs` - Dynamic tools
- `openai_models.rs` - OpenAI model types
- `num_format.rs` - Number formatting
- `thread_id.rs` - Thread ID type
- `account.rs` - Account types

**Dependencies:** serde, serde_json, uuid

---

#### `codex-config`
**Path:** `codex-rs/config/`  
**Purpose:** Configuration loading and validation

**Key Modules:**
- `config_requirements.rs` - Config requirements
- `constraint.rs` - Constraint types
- `diagnostics.rs` - Error diagnostics
- `fingerprint.rs` - Config fingerprinting
- `merge.rs` - Config merging
- `overrides.rs` - Config overrides
- `requirements_exec_policy.rs` - Exec policy requirements
- `cloud_requirements.rs` - Cloud requirements
- `state.rs` - Config state

**Config Layers:**
1. Default config
2. User config (`~/.codex/config.toml`)
3. Project config (`AGENTS.md`, `config.toml`)
4. CLI overrides
5. Cloud requirements (MDM)

**Dependencies:** toml, serde, serde_json

---

### Security & Sandboxing

#### `codex-linux-sandbox`
**Path:** `codex-rs/linux-sandbox/`  
**Purpose:** Linux sandboxing

**Key Modules:**
- `bwrap.rs` - Bubblewrap integration
- `landlock.rs` - Landlock LSM
- `vendored_bwrap.rs` - Vendored bubblewrap
- `proxy_routing.rs` - Proxy routing
- `linux_run_main.rs` - Main entry point

**Dependencies:** landlock, nix

---

#### `codex-windows-sandbox`
**Path:** `codex-rs/windows-sandbox-rs/`  
**Purpose:** Windows Sandbox integration

**Key Modules:**
- `policy.rs` - Sandbox policy
- `setup_main_win.rs` - Setup entry point
- `elevated_impl.rs` - Elevated implementation
- `firewall.rs` - Firewall configuration
- `identity.rs` - Identity management
- `token.rs` - Token handling
- `acl.rs` - ACL management
- `allow.rs` - Allow rules
- `audit.rs` - Audit logging
- `cap.rs` - Capabilities
- `cwd_junction.rs` - CWD junctions
- `desktop.rs` - Desktop handling
- `dpapi.rs` - DPAPI
- `env.rs` - Environment
- `helper_materialization.rs` - Helper materialization
- `hide_users.rs` - User hiding
- `logging.rs` - Logging
- `path_normalization.rs` - Path normalization
- `process.rs` - Process management
- `read_acl_mutex.rs` - ACL mutex
- `sandbox_users.rs` - Sandbox users
- `setup_error.rs` - Setup errors
- `setup_orchestrator.rs` - Setup orchestration
- `winutil.rs` - Windows utilities
- `workspace_acl.rs` - Workspace ACL
- `command_runner_win.rs` - Command runner

**Dependencies:** windows-sys, widestring

---

#### `codex-execpolicy`
**Path:** `codex-rs/execpolicy/`  
**Purpose:** Execution policy parsing and checking

**Key Modules:**
- `policy.rs` - Policy types
- `rule.rs` - Rule types
- `parser.rs` - Policy parser
- `decision.rs` - Decision types
- `error.rs` - Error types
- `executable_name.rs` - Executable name handling
- `amend.rs` - Policy amendments
- `execpolicycheck.rs` - Policy checker

**Policy Format:**
```toml
[[rule]]
command = ["cargo", "run"]
decision = "allow"

[[rule]]
command_prefix = ["npm", "install"]
decision = "ask"
```

---

#### `codex-network-proxy`
**Path:** `codex-rs/network-proxy/`  
**Purpose:** HTTP/SOCKS5 proxy for network policy enforcement

**Key Modules:**
- `proxy.rs` - Main proxy logic
- `http_proxy.rs` - HTTP proxy
- `socks5.rs` - SOCKS5 proxy
- `mitm.rs` - MITM (Man-in-the-Middle)
- `policy.rs` - Network policy
- `network_policy.rs` - Network policy decisions
- `config.rs` - Configuration
- `certs.rs` - Certificate management
- `upstream.rs` - Upstream proxy
- `state.rs` - Proxy state
- `runtime.rs` - Runtime management
- `responses.rs` - Response handling
- `reasons.rs` - Block reasons

**Features:**
- HTTP proxy (default port 3128)
- SOCKS5 proxy (default port 8081)
- Domain allowlist/denylist
- Limited mode (GET/HEAD/OPTIONS only)
- MITM for HTTPS inspection
- Unix socket proxying (macOS)
- OTEL audit events

**Dependencies:** rama, rustls, tokio

---

### MCP (Model Context Protocol)

#### `codex-mcp-server`
**Path:** `codex-rs/mcp-server/`  
**Purpose:** MCP server implementation

**Key Modules:**
- `message_processor.rs` - Message processing
- `codex_tool_runner.rs` - Codex tool execution
- `codex_tool_config.rs` - Tool configuration
- `exec_approval.rs` - Execution approval
- `patch_approval.rs` - Patch approval
- `tool_handlers/` - Tool handlers
  - `mod.rs` - Handler registry

**Dependencies:** rmcp, tokio, codex-core

---

#### `codex-rmcp-client`
**Path:** `codex-rs/rmcp-client/`  
**Purpose:** MCP client implementation

**Key Modules:**
- `rmcp_client.rs` - MCP client
- `oauth.rs` - OAuth handling
- `perform_oauth_login.rs` - OAuth login flow
- `auth_status.rs` - Auth status
- `program_resolver.rs` - Program resolution
- `logging_client_handler.rs` - Logging handler
- `utils.rs` - Utilities
- `bin/` - Test servers

**Dependencies:** rmcp, reqwest, tokio

---

### Authentication

#### `codex-login`
**Path:** `codex-rs/login/`  
**Purpose:** OAuth/device code authentication

**Key Modules:**
- `device_code_auth.rs` - Device code flow
- `pkce.rs` - PKCE implementation
- `server.rs` - OAuth callback server

**Dependencies:** reqwest, tokio, axum

---

#### `codex-chatgpt`
**Path:** `codex-rs/chatgpt/`  
**Purpose:** ChatGPT API client

**Key Modules:**
- `chatgpt_client.rs` - ChatGPT client
- `chatgpt_token.rs` - Token handling
- `apply_command.rs` - Apply command
- `get_task.rs` - Task retrieval
- `connectors.rs` - Connector integration

**Dependencies:** reqwest, serde

---

#### `codex-keyring-store`
**Path:** `codex-rs/keyring-store/`  
**Purpose:** Secure credential storage

**Key Modules:**
- `lib.rs` - Keyring abstraction

**Dependencies:** keyring

---

#### `codex-secrets`
**Path:** `codex-rs/secrets/`  
**Purpose:** Secret sanitization

**Key Modules:**
- `sanitizer.rs` - Secret sanitization
- `local.rs` - Local secret storage

**Dependencies:** zeroize

---

### State & Persistence

#### `codex-state`
**Path:** `codex-rs/state/`  
**Purpose:** SQLite state database

**Key Modules:**
- `log_db.rs` - Log database
- `migrations.rs` - Database migrations
- `model/` - Data models
  - `log.rs` - Log entries
  - `memories.rs` - Memories
  - `thread_metadata.rs` - Thread metadata
  - `agent_job.rs` - Agent jobs
  - `backfill_state.rs` - Backfill state
- `runtime/` - Runtime operations
  - `logs.rs` - Log operations
  - `threads.rs` - Thread operations
  - `memories.rs` - Memory operations
  - `agent_jobs.rs` - Agent job operations
  - `backfill.rs` - Backfill operations
  - `test_support.rs` - Test support
- `paths.rs` - Path management
- `extract.rs` - Data extraction
- `bin/logs_client.rs` - Logs client

**Dependencies:** sqlx, tokio, uuid

---

### Telemetry

#### `codex-otel`
**Path:** `codex-rs/otel/`  
**Purpose:** OpenTelemetry integration

**Key Modules:**
- `config.rs` - Configuration
- `otlp.rs` - OTLP export
- `provider.rs` - Trace provider
- `trace_context.rs` - Trace context
- `targets.rs` - Trace targets
- `events/` - Event types
  - `session_telemetry.rs` - Session telemetry
  - `shared.rs` - Shared events
- `metrics/` - Metrics
  - `client.rs` - Metrics client
  - `config.rs` - Metrics config
  - `runtime_metrics.rs` - Runtime metrics
  - `tags.rs` - Metric tags
  - `timer.rs` - Timing helpers
  - `validation.rs` - Validation
  - `names.rs` - Metric names

**Dependencies:** opentelemetry, opentelemetry-otlp, opentelemetry_sdk

---

### Skills System

#### `codex-skills`
**Path:** `codex-rs/skills/`  
**Purpose:** Skill system (reusable workflows)

**Key Modules:**
- `lib.rs` - Skill definitions

**Dependencies:** serde, toml

---

#### `codex-hooks`
**Path:** `codex-rs/hooks/`  
**Purpose:** Event hooks system

**Key Modules:**
- `engine/` - Hook engine
  - `config.rs` - Hook configuration
  - `discovery.rs` - Hook discovery
  - `dispatcher.rs` - Hook dispatch
  - `command_runner.rs` - Command execution
  - `output_parser.rs` - Output parsing
  - `schema_loader.rs` - Schema loading
- `events/` - Hook events
  - `session_start.rs` - Session start hooks
  - `stop.rs` - Stop hooks
- `registry.rs` - Hook registry
- `schema.rs` - Hook schema
- `types.rs` - Hook types
- `user_notification.rs` - User notifications
- `legacy_notify.rs` - Legacy notifications

**Dependencies:** serde, tokio

---

### File Operations

#### `codex-apply-patch`
**Path:** `codex-rs/apply-patch/`  
**Purpose:** Unified diff patch application

**Key Modules:**
- `parser.rs` - Patch parser
- `invocation.rs` - Patch invocation
- `seek_sequence.rs` - Seek sequence
- `standalone_executable.rs` - Standalone binary

**Dependencies:** similar

---

#### `codex-file-search`
**Path:** `codex-rs/file-search/`  
**Purpose:** Fuzzy file search

**Key Modules:**
- `cli.rs` - CLI interface
- `lib.rs` - Search logic (ripgrep wrapper)

**Dependencies:** ripgrep, tokio

---

#### `codex-shell-command`
**Path:** `codex-rs/shell-command/`  
**Purpose:** Shell command parsing and safety

**Key Modules:**
- `parse_command.rs` - Command parsing
- `shell_detect.rs` - Shell detection
- `bash.rs` - Bash-specific handling
- `powershell.rs` - PowerShell handling
- `command_safety/` - Safety checks
  - `is_safe_command.rs` - Safety validation
  - `is_dangerous_command.rs` - Danger detection
  - `windows_safe_commands.rs` - Windows safety
  - `windows_dangerous_commands.rs` - Windows dangers

**Dependencies:** shlex

---

#### `codex-shell-escalation`
**Path:** `codex-rs/shell-escalation/`  
**Purpose:** Privilege escalation (sudo)

**Key Modules:**
- `unix/` - Unix escalation
  - `escalate_client.rs` - Escalation client
  - `escalate_server.rs` - Escalation server
  - `escalate_protocol.rs` - Escalation protocol
  - `escalation_policy.rs` - Escalation policy
  - `execve_wrapper.rs` - execve wrapper
  - `socket.rs` - Socket handling
  - `stopwatch.rs` - Timing
- `bin/main_execve_wrapper.rs` - Wrapper binary

**Dependencies:** tokio, nix

---

### Model Integrations

#### `codex-lmstudio`
**Path:** `codex-rs/lmstudio/`  
**Purpose:** LM Studio integration

**Key Modules:**
- `client.rs` - LM Studio client
- `lib.rs` - LM Studio types

**Dependencies:** reqwest

---

#### `codex-ollama`
**Path:** `codex-rs/ollama/`  
**Purpose:** Ollama integration

**Key Modules:**
- `client.rs` - Ollama client
- `parser.rs` - Response parsing
- `pull.rs` - Model pulling
- `url.rs` - URL handling

**Dependencies:** reqwest

---

#### `codex-backend-client`
**Path:** `codex-rs/backend-client/`  
**Purpose:** Backend API client

**Key Modules:**
- `client.rs` - Backend client
- `types.rs` - Backend types

**Dependencies:** reqwest

---

#### `codex-codex-client`
**Path:** `codex-rs/codex-client/`  
**Purpose:** High-level Codex API client

**Dependencies:** codex-core, codex-api

---

#### `codex-codex-api`
**Path:** `codex-rs/codex-api/`  
**Purpose:** REST API endpoints

**Key Modules:**
- `endpoint/`
  - `compact.rs` - Compaction endpoint
  - `memories.rs` - Memories endpoint
  - `models.rs` - Models endpoint

**Dependencies:** axum, codex-core

---

### Utilities

#### `codex-utils-pty`
**Path:** `codex-rs/utils/pty/`  
**Purpose:** PTY (pseudo-terminal) handling

**Key Modules:**
- `pty.rs` - PTY abstraction
- `process.rs` - Process handling
- `process_group.rs` - Process groups
- `pipe.rs` - Pipe handling
- `win/` - Windows ConPTY
  - `conpty.rs` - ConPTY bindings
  - `procthreadattr.rs` - Process thread attributes
  - `psuedocon.rs` - Pseudoconsole

**Dependencies:** portable-pty, windows-sys

---

#### `codex-utils-git`
**Path:** `codex-rs/utils/git/`  
**Purpose:** Git operations

**Key Modules:**
- `branch.rs` - Branch operations
- `apply.rs` - Patch application
- `operations.rs` - Git operations
- `ghost_commits.rs` - Ghost commits
- `platform.rs` - Platform abstraction
- `errors.rs` - Error types

**Dependencies:** git2

---

#### `codex-utils-sleep-inhibitor`
**Path:** `codex-rs/utils/sleep-inhibitor/`  
**Purpose:** Sleep inhibition

**Key Modules:**
- `macos.rs` - macOS IOKit
- `linux_inhibitor.rs` - Linux inhibitor
- `windows_inhibitor.rs` - Windows inhibitor
- `iokit_bindings.rs` - IOKit bindings

**Dependencies:** core-foundation-sys (macOS)

---

#### `codex-utils-stream-parser`
**Path:** `codex-rs/utils/stream-parser/`  
**Purpose:** Stream text parsing

**Key Modules:**
- `tagged_line_parser.rs` - Tagged line parsing
- `assistant_text.rs` - Assistant text
- `stream_text.rs` - Stream text
- `citation.rs` - Citation parsing
- `proposed_plan.rs` - Plan parsing
- `inline_hidden_tag.rs` - Hidden tags
- `utf8_stream.rs` - UTF-8 streaming

**Dependencies:** memchr

---

#### `codex-utils-image`
**Path:** `codex-rs/utils/image/`  
**Purpose:** Image handling

**Key Modules:**
- `lib.rs` - Image utilities
- `error.rs` - Error types

**Dependencies:** image

---

#### `codex-utils-cache`
**Path:** `codex-rs/utils/cache/`  
**Purpose:** Caching utilities

**Dependencies:** lru

---

#### `codex-utils-fuzzy-match`
**Path:** `codex-rs/utils/fuzzy-match/`  
**Purpose:** Fuzzy matching

**Dependencies:** nucleo

---

#### `codex-utils-approval-presets`
**Path:** `codex-rs/utils/approval-presets/`  
**Purpose:** Approval mode presets

---

#### `codex-utils-sandbox-summary`
**Path:** `codex-rs/utils/sandbox-summary/`  
**Purpose:** Sandbox configuration summary

---

#### `codex-utils-rustls-provider`
**Path:** `codex-rs/utils/rustls-provider/`  
**Purpose:** rustls TLS provider

---

#### `codex-utils-readiness`
**Path:** `codex-rs/utils/readiness/`  
**Purpose:** Readiness checks

---

#### `codex-utils-cargo-bin`
**Path:** `codex-rs/utils/cargo-bin/`  
**Purpose:** Cargo binary location helpers

---

#### `codex-utils-absolute-path`
**Path:** `codex-rs/utils/absolute-path/`  
**Purpose:** Absolute path handling

---

#### `codex-utils-home-dir`
**Path:** `codex-rs/utils/home-dir/`  
**Purpose:** Home directory resolution

**Dependencies:** dirs

---

#### `codex-utils-json-to-toml`
**Path:** `codex-rs/utils/json-to-toml/`  
**Purpose:** JSON/TOML conversion

---

#### `codex-utils-oss`
**Path:** `codex-rs/utils/oss/`  
**Purpose:** OSS utilities

---

#### `codex-utils-elapsed`
**Path:** `codex-rs/utils/elapsed/`  
**Purpose:** Elapsed time formatting

---

#### `codex-utils-string`
**Path:** `codex-rs/utils/string/`  
**Purpose:** String utilities

---

#### `codex-utils-cli`
**Path:** `codex-rs/utils/cli/`  
**Purpose:** CLI utilities

**Key Modules:**
- `approval_mode_cli_arg.rs` - Approval mode arg
- `sandbox_mode_cli_arg.rs` - Sandbox mode arg
- `config_override.rs` - Config overrides
- `format_env_display.rs` - Environment display

---

#### `codex-async-utils`
**Path:** `codex-rs/async-utils/`  
**Purpose:** Async utilities

---

#### `codex-arg0`
**Path:** `codex-rs/arg0/`  
**Purpose:** Argument 0 handling

---

#### `codex-ansi-escape`
**Path:** `codex-rs/ansi-escape/`  
**Purpose:** ANSI escape sequence parsing

---

#### `codex-process-hardening`
**Path:** `codex-rs/process-hardening/`  
**Purpose:** Process security hardening

---

#### `codex-stdio-to-uds`
**Path:** `codex-rs/stdio-to-uds/`  
**Purpose:** Stdio to Unix domain socket bridge

---

#### `codex-responses-api-proxy`
**Path:** `codex-rs/responses-api-proxy/`  
**Purpose:** OpenAI Responses API proxy

---

#### `codex-feedback`
**Path:** `codex-rs/feedback/`  
**Purpose:** Feedback submission

---

#### `codex-artifacts`
**Path:** `codex-rs/artifacts/`  
**Purpose:** Artifact runtime management

**Key Modules:**
- `client.rs` - Artifact client
- `runtime/`
  - `manager.rs` - Runtime manager
  - `installed.rs` - Installed runtime
  - `js_runtime.rs` - JavaScript runtime
  - `manifest.rs` - Manifest types
  - `error.rs` - Error types

---

#### `codex-package-manager`
**Path:** `codex-rs/package-manager/`  
**Purpose:** Package management

**Key Modules:**
- `manager.rs` - Package manager
- `package.rs` - Package types
- `archive.rs` - Archive handling
- `config.rs` - Configuration
- `platform.rs` - Platform detection

---

### CLI & Execution

#### `codex-cli`
**Path:** `codex-rs/cli/`  
**Purpose:** CLI entrypoint

**Key Modules:**
- `main.rs` - Main entry
- `app_cmd.rs` - App commands
- `mcp_cmd.rs` - MCP commands
- `login.rs` - Login command
- `debug_sandbox/` - Debug sandbox
  - `pid_tracker.rs` - PID tracking
  - `seatbelt.rs` - Seatbelt debug
- `desktop_app/` - Desktop app
  - `mac.rs` - macOS app
- `wsl_paths.rs` - WSL path handling
- `exit_status.rs` - Exit codes

---

#### `codex-exec`
**Path:** `codex-rs/exec/`  
**Purpose:** Execution engine

**Key Modules:**
- `event_processor.rs` - Event processing
- `event_processor_with_human_output.rs` - Human output
- `event_processor_with_jsonl_output.rs` - JSONL output
- `exec_events.rs` - Exec events

---

#### `codex-debug-client`
**Path:** `codex-rs/debug-client/`  
**Purpose:** Debugging utilities

**Key Modules:**
- `client.rs` - Debug client
- `commands.rs` - Debug commands
- `output.rs` - Output formatting
- `reader.rs` - Log reader
- `state.rs` - Debug state

---

### Cloud Integration

#### `codex-cloud-tasks`
**Path:** `codex-rs/cloud-tasks/`  
**Purpose:** Cloud task management

**Key Modules:**
- `app.rs` - Task app
- `cli.rs` - CLI interface
- `ui.rs` - UI components
- `new_task.rs` - Task creation
- `scrollable_diff.rs` - Scrollable diff
- `env_detect.rs` - Environment detection

---

#### `codex-cloud-tasks-client`
**Path:** `codex-rs/cloud-tasks-client/`  
**Purpose:** Cloud tasks API client

**Key Modules:**
- `api.rs` - API types
- `http.rs` - HTTP client

---

#### `codex-cloud-requirements`
**Path:** `codex-rs/cloud-requirements/`  
**Purpose:** Cloud requirement enforcement

---

#### `codex-connectors`
**Path:** `codex-rs/connectors/`  
**Purpose:** External service connectors

---

### App Server Clients

#### `codex-app-server-client`
**Path:** `codex-rs/app-server-client/`  
**Purpose:** App server client library

---

#### `codex-app-server-test-client`
**Path:** `codex-rs/app-server-test-client/`  
**Purpose:** Test client for app-server

---

### Experimental

#### `codex-codex-experimental-api-macros`
**Path:** `codex-rs/codex-experimental-api-macros/`  
**Purpose:** Experimental API macros

---

#### `codex-codex-backend-openapi-models`
**Path:** `codex-rs/codex-backend-openapi-models/`  
**Purpose:** OpenAPI models for backend

---

### Test Infrastructure

#### `codex-test-macros`
**Path:** `codex-rs/test-macros/`  
**Purpose:** Test helper macros

---

## Key Modules Deep Dive

### Tool System Architecture

**File:** `codex-rs/core/src/tools/mod.rs`

The tool system is the core execution engine. Tools are:
1. Registered in a central registry
2. Routed based on model requests
3. Executed with appropriate sandboxing
4. Results formatted for model consumption

**Tool Types:**
- **Built-in tools:** shell, apply_patch, read_file, list_dir, grep_files
- **MCP tools:** Discovered from MCP servers
- **Dynamic tools:** Runtime-registered
- **Skill tools:** From skill definitions
- **Multi-agent tools:** spawn_agent, wait_agent, send_input, close_agent

**Tool Execution Flow:**
```
Model Tool Call
    ↓
Tool Router
    ↓
Tool Handler
    ↓
Runtime (shell/patch/etc.)
    ↓
Sandbox Enforcement
    ↓
Output Formatting
    ↓
Model Response
```

---

### Sandbox Architecture

**Security Layers:**

1. **Execution Policy** (`execpolicy/`)
   - Command allowlist/denylist
   - Prefix matching
   - Decision: allow/deny/ask

2. **OS Sandboxing:**
   - **macOS:** sandbox-exec (Seatbelt)
   - **Linux:** Landlock LSM + bubblewrap
   - **Windows:** Windows Sandbox

3. **Network Policy** (`network-proxy/`)
   - HTTP/SOCKS5 proxy
   - Domain allowlist
   - Limited mode (read-only)

4. **Filesystem Restrictions:**
   - Workspace-only writes
   - Read-only outside workspace
   - Temporary file isolation

---

### Configuration System

**Config Resolution Order:**
1. Default values (compiled in)
2. User config (`~/.codex/config.toml`)
3. Project config (`./.codex/config.toml`, `AGENTS.md`)
4. CLI overrides (`--model`, `--approval-mode`)
5. Cloud requirements (MDM policies)

**Config Schema:**
```toml
model = "gpt-5.1-codex"
approval_mode = "suggest"  # suggest | auto-edit | full-auto
sandbox_mode = "workspace"  # workspace | read-only | dangerous

[permissions.workspace]
network_enabled = false
allowed_domains = ["*.openai.com"]

[mcp_servers]
[mcp_servers.my_server]
command = ["node", "mcp-server.js"]
env = { API_KEY = "${MCP_API_KEY}" }

[skills]
auto_approve = ["test", "lint"]
```

---

### State Management

**SQLite Schema:**
- `threads` - Thread metadata
- `turns` - Turn records
- `items` - Conversation items
- `memories` - Long-term memories
- `logs` - Execution logs
- `agent_jobs` - Background jobs

**Rollout Files:**
- Location: `~/.codex/sessions/YYYY-MM-DD/`
- Format: JSONL (one event per line)
- Contains: Full conversation history, tool calls, outputs

---

## Protocol & API

### JSON-RPC 2.0 Protocol

**Message Format:**
```json
{
  "jsonrpc": "2.0",
  "method": "thread/start",
  "id": 1,
  "params": {
    "model": "gpt-5.1-codex",
    "cwd": "/path/to/project",
    "approvalPolicy": "never"
  }
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "thread": {
      "id": "thread_abc123",
      "status": "notLoaded",
      "ephemeral": false
    }
  }
}
```

**Notifications:**
```json
{
  "jsonrpc": "2.0",
  "method": "turn/started",
  "params": {
    "threadId": "thread_abc123",
    "turnId": "turn_def456"
  }
}
```

---

### Transports

1. **stdio** (default)
   - Newline-delimited JSON (JSONL)
   - Bidirectional pipes

2. **WebSocket** (experimental)
   - `ws://IP:PORT`
   - One JSON-RPC message per frame
   - Health endpoints: `/readyz`, `/healthz`

---

### API Versioning

- **v1:** Legacy API (deprecated)
- **v2:** Active development
  - Cursor pagination
  - Experimental API flags
  - Notification opt-out

---

## Security Model

### Approval Modes

| Mode | Auto-Read | Auto-Write | Auto-Execute |
|------|-----------|------------|--------------|
| `suggest` | ✓ | ✗ | ✗ |
| `auto-edit` | ✓ | ✓ | ✗ |
| `full-auto` | ✓ | ✓ | ✓ (sandboxed) |

---

### Sandbox Permissions

**macOS Seatbelt Profile:**
- Read: Entire filesystem
- Write: `$PWD`, `$TMPDIR`, `~/.codex`
- Network: Blocked (unless explicitly allowed)
- Process execution: Allowed (within sandbox)

**Linux Landlock:**
- Read: Configured paths
- Write: Workspace only
- Network: Via proxy (policy-enforced)

**Windows Sandbox:**
- Isolated desktop session
- Folder mounting (read/write)
- Firewall rules (egress blocking)

---

### Network Security

**Proxy Modes:**
- **Full:** All domains (allowlist)
- **Limited:** GET/HEAD/OPTIONS only
- **Disabled:** No network access

**Default Policy:**
```toml
[network]
enabled = true
mode = "full"
allowed_domains = ["*.openai.com", "localhost"]
denied_domains = []
allow_local_binding = false
```

---

## Testing Infrastructure

### Test Types

1. **Unit Tests**
   - Module-level `#[cfg(test)]`
   - `pretty_assertions` for diffs
   - Snapshot tests with `insta`

2. **Integration Tests**
   - `tests/suite/` directories
   - Full end-to-end flows
   - Mock servers (wiremock)

3. **Snapshot Tests**
   - TUI rendering validation
   - Protocol schema fixtures
   - Update with `cargo insta review`

---

### Test Support Crates

- `core_test_support` - Core test utilities
- `mcp_test_support` - MCP test utilities
- `app_test_support` - App server test utilities

---

### Test Patterns

**Mock Server Pattern:**
```rust
let mock = responses::mount_sse_once(&server, responses::sse(vec![
    responses::ev_response_created("resp-1"),
    responses::ev_function_call(call_id, "shell", &args)?,
    responses::ev_completed("resp-1"),
])).await;

codex.submit(Op::UserTurn { ... }).await?;

let request = mock.single_request();
assert_eq!(request.path(), "/v1/responses");
```

**Snapshot Test Pattern:**
```rust
#[test]
fn test_render() {
    let output = render_widget();
    insta::assert_snapshot!(output);
}
```

---

## Build System

### Dual Build: Cargo + Bazel

**Cargo (Development):**
```bash
cargo build -p codex-tui
cargo test -p codex-core
cargo clippy -p codex-app-server
```

**Bazel (Production):**
```bash
bazel build //codex-rs/tui:codex-tui
bazel test //codex-rs/...
```

---

### Build Helpers (`just`)

```bash
just fmt              # Format code (rustfmt)
just fix -p codex-tui # Fix lints (clippy)
just test             # Run all tests
just write-config-schema      # Generate config schema
just write-app-server-schema  # Generate API schema
just bazel-lock-update        # Update Bazel locks
```

---

### Release Build

**Profile Settings:**
```toml
[profile.release]
lto = "fat"
strip = "symbols"
codegen-units = 1
```

**Platform Targets:**
- macOS: `aarch64-apple-darwin`, `x86_64-apple-darwin`
- Linux: `x86_64-unknown-linux-musl`, `aarch64-unknown-linux-musl`
- Windows: `x86_64-pc-windows-msvc`

---

## Legacy TypeScript Implementation

### codex-cli/

**Status:** Deprecated (superseded by Rust implementation)

**Structure:**
```
codex-cli/
├── bin/codex.js      # CLI entrypoint
├── package.json
└── scripts/
    ├── build_container.sh
    ├── install_native_deps.py
    └── run_in_container.sh
```

**Key Differences from Rust:**
- Node.js runtime
- Docker-based sandboxing (Linux)
- Simpler approval model
- No TUI (interactive REPL only)

---

## Documentation Files

### Key Markdown Files

- `README.md` - Project overview
- `AGENTS.md` - Contributing guidelines for Rust code
- `docs/contributing.md` - Contribution guidelines
- `docs/install.md` - Installation instructions
- `docs/open-source-fund.md` - Open source fund info
- `codex-rs/*/README.md` - Per-crate documentation

---

## Environment Variables

**Core Variables:**
```bash
OPENAI_API_KEY          # OpenAI API key
CODEX_HOME              # Config directory (~/.codex)
RUST_LOG                # Logging level
LOG_FORMAT=json         # JSON log output
DEBUG=true              # Verbose logging
CODEX_DISABLE_PROJECT_DOC=1  # Skip AGENTS.md loading
CODEX_SANDBOX=seatbelt  # Sandbox type
CODEX_SANDBOX_NETWORK_DISABLED=1  # Network disabled
```

---

## File Counts by Crate

| Crate | .rs Files | Approx. LoC |
|-------|-----------|-------------|
| core | ~200 | ~50,000 |
| tui | ~150 | ~40,000 |
| app-server | ~50 | ~15,000 |
| protocol | ~30 | ~10,000 |
| tools (handlers) | ~40 | ~12,000 |
| network-proxy | ~20 | ~8,000 |
| windows-sandbox-rs | ~30 | ~10,000 |
| state | ~20 | ~6,000 |
| mcp-server | ~15 | ~5,000 |
| rmcp-client | ~15 | ~5,000 |
| config | ~15 | ~5,000 |
| execpolicy | ~10 | ~3,000 |
| skills | ~10 | ~3,000 |
| hooks | ~15 | ~4,000 |
| otel | ~20 | ~6,000 |
| utils/* | ~50 | ~15,000 |
| **Total** | **~1,184** | **~200,000+** |

---

## Dependencies (External)

**Core Dependencies:**
- `tokio` - Async runtime
- `serde` / `serde_json` - Serialization
- `tracing` - Logging
- `reqwest` - HTTP client
- `axum` - Web framework (app-server)
- `ratatui` - TUI framework
- `crossterm` - Terminal handling
- `sqlx` - SQLite (state_db)
- `opentelemetry` - Telemetry
- `rmcp` - MCP protocol
- `rama` - Network proxy
- `keyring` - Credential storage
- `image` - Image processing
- `git2` - Git operations
- `landlock` - Linux sandboxing
- `portable-pty` - PTY handling

---

## Key Design Decisions

1. **Rust Workspace:** Modular crates for maintainability
2. **JSON-RPC 2.0:** Standard protocol for IDE integration
3. **Sandbox-First:** Security by default
4. **Approval Modes:** Configurable autonomy
5. **SQLite State:** Simple, embedded persistence
6. **Bazel + Cargo:** Fast CI (Bazel) + dev ergonomics (Cargo)
7. **Cursor Pagination:** Efficient list APIs
8. **Experimental Flags:** Safe feature rollout
9. **OTEL Integration:** Production observability
10. **MCP Support:** Extensible tool ecosystem

---

## Glossary

- **Thread:** A conversation between user and Codex
- **Turn:** One interaction (user input → agent response)
- **Item:** Individual message/tool call within a turn
- **Rollout:** Persisted session data (JSONL files)
- **Skill:** Reusable workflow definition
- **Plugin:** Bundle of skills + MCP servers + apps
- **MCP:** Model Context Protocol (tool server standard)
- **Seatbelt:** macOS sandboxing (sandbox-exec)
- **Landlock:** Linux LSM for filesystem sandboxing
- **App Server:** JSON-RPC server for IDE clients
- **TUI:** Terminal UI (ratatui-based)

---

*Generated from codex/ repository analysis*  
*Rust implementation: codex-rs/*  
*Legacy TypeScript: codex-cli/*
