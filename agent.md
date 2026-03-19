# Sapphire CLI - Agent.md (Source of Truth)

**Repository:** https://github.com/duggal1/Sapphire-cli
**Language:** Go 1.26.1
**Last Updated:** 2026-03-17

---

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [File Inventory](#file-inventory)
4. [Core Components](#core-components)
5. [Agent System](#agent-system)
6. [Sub-Agent System](#sub-agent-system)
7. [Tool System](#tool-system)
8. [UI System](#ui-system)
9. [Configuration System](#configuration-system)
10. [Database Schema](#database-schema)
11. [Event System](#event-system)
12. [MCP Integration](#mcp-integration)
13. [LSP Integration](#lsp-integration)
14. [Skills System](#skills-system)
15. [Long-Horizon Tasks](#long-horizon-tasks)
16. [Shell System](#shell-system)

---

## Overview

Sapphire CLI is a terminal-first AI assistant for software development. It provides session-based AI agent functionality with support for:

- Multi-provider LLM integration (OpenAI, Anthropic, Google Gemini, Azure, Bedrock, OpenRouter, Vercel)
- Sub-agent spawning and coordination with worktree isolation
- Tool execution with permission management
- Automatic snapshot commits for recoverability
- Git safety policy (no autonomous push/merge/rebase)
- Model Context Protocol (MCP) integration
- Language Server Protocol (LSP) integration
- Skills system (Agent Skills open standard)
- Long-horizon task management
- Worktree-based parallel execution
- Validation gate for sub-agent output (build, test, lint, security)
- TUI and non-interactive modes

---

## Architecture

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Sapphire CLI                                     │
├─────────────────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │   main.go   │  │  cmd/root   │  │  app/app    │  │  config/config      │ │
│  │  (Entry)    │─▶│  (CLI)      │─▶│  (Wire)     │─▶│  (Load/Init)        │ │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────────────┘ │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────────┐ │
│  │                         Agent Coordinator                                │ │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐                  │ │
│  │  │ SessionAgent │  │ SubAgentMgr  │  │ ToolRegistry │                  │ │
│  │  │ (Run/Queue)  │  │ (Spawn/Wait) │  │ (MCP/Built)  │                  │ │
│  │  └──────────────┘  └──────────────┘  └──────────────┘                  │ │
│  └─────────────────────────────────────────────────────────────────────────┘ │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────────┐ │
│  │                          Services Layer                                  │ │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  │ │
│  │  │ Session  │  │ Message  │  │ History  │  │ Permission│  │ FileTrack│  │ │
│  │  │ Service  │  │ Service  │  │ Service  │  │ Service  │  │ Service  │  │ │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────┘  └──────────┘  │ │
│  └─────────────────────────────────────────────────────────────────────────┘ │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────────┐ │
│  │                          Data Layer                                      │ │
│  │  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐      │ │
│  │  │  SQLite (sqlc)   │  │  Memory Store    │  │  Config Files    │      │ │
│  │  │  (db/*.sql.go)   │  │  (vector search) │  │  (crush.json)    │      │ │
│  │  └──────────────────┘  └──────────────────┘  └──────────────────┘      │ │
│  └─────────────────────────────────────────────────────────────────────────┘ │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────────┐ │
│  │                          UI Layer                                        │ │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌────────────┐  │ │
│  │  │  ui/model    │  │  ui/chat     │  │  ui/dialog   │  │ ui/common  │  │ │
│  │  │  (BubbleTea) │  │  (Messages)  │  │  (Overlays)  │  │ (Helpers)  │  │ │
│  │  └──────────────┘  └──────────────┘  └──────────────┘  └────────────┘  │ │
│  └─────────────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Request Flow

```
User Input (TUI or CLI)
         │
         ▼
┌─────────────────┐
│  cmd/root.go    │ ──▶ setupApp() ──▶ config.Init()
└─────────────────┘
         │
         ▼
┌─────────────────┐
│  app/app.go     │ ──▶ InitCoderAgent() ──▶ agent.NewCoordinator()
└─────────────────┘
         │
         ▼
┌─────────────────┐
│  coordinator.go │ ──▶ Run() / Submit()
└─────────────────┘
         │
         ├─────────────────────┐
         │                     │
         ▼                     ▼
┌─────────────────┐   ┌─────────────────┐
│  sessionAgent   │   │  subAgentMgr    │
│  .Run()         │   │  .spawnSubAgent()│
└─────────────────┘   └─────────────────┘
         │
         ▼
┌─────────────────┐
│  fantasy.Agent  │ ──▶ LLM Stream
└─────────────────┘
         │
         ▼
┌─────────────────┐
│  Tool Execution │ ──▶ MCP / Bash / Edit / etc.
└─────────────────┘
         │
         ▼
┌─────────────────┐
│  Message Update │ ──▶ DB + PubSub ──▶ UI
└─────────────────┘
```

---

## File Inventory

### Root Level Files

| File | Purpose |
|------|---------|
| `main.go` | Entry point; starts profiling server and executes root CLI command |
| `crush.json` | Main configuration file (providers, models, MCP, LSP, options) |
| `schema.json` | JSON Schema for crush.json validation |
| `Taskfile.yaml` | Task runner configuration |
| `go.mod` / `go.sum` | Go module dependencies |
| `README.md` | Project documentation |
| `LICENSE.md` | MIT License |
| `install.sh` | Installation script |
| `testing.go` | Test helpers |
| `sqlc.yaml` | sqlc configuration for database queries |

### `/internal/agent/` - Agent Core

| File | Purpose |
|------|---------|
| `coordinator.go` | Agent coordinator; manages multiple AI agents, sub-agents, MCP selection, skill matching |
| `agent.go` | Session agent implementation; handles LLM streaming, tool execution, message management |
| `subagent_manager.go` | Sub-agent lifecycle management; spawn, resume, wait, close operations |
| `subagent_coordination.go` | Sub-agent coordination helpers |
| `subagent_events.go` / `subagent_events_helpers.go` | Sub-agent event publishing |
| `subagent_guardrails.go` | Sub-agent spawn limits and validation |
| `subagent_metadata.go` | Sub-agent metadata persistence |
| `subagent_supervisor.go` | Sub-agent supervision |
| `subagent_worktree.go` | Sub-agent worktree preparation, lifecycle, cleanup, quarantine |
| `subagent_validation_gate.go` | Validation gate for completed sub-agent work (diff, build, test, lint, security) |
| `agent_tool.go` | Agent tool definitions |
| `agent_job_manager.go` | Background agent job management |
| `agent_job_runner.go` | Agent job execution |
| `agent_job_tools.go` | Agent job tool implementations |
| `background_subagents.go` | Autonomous sub-agent priming |
| `background_wait.go` | Background sub-agent wait handling |
| `collab_tools.go` | Collaboration tool handlers |
| `worktree_orchestrator.go` | Worktree orchestration for parallel execution |
| `mcp_registry.go` | MCP registry management |
| `mcp_selection.go` | MCP selection based on user prompts |
| `mcp_preflight.go` | MCP preflight discovery |
| `mcp_inventory.go` | MCP inventory building |
| `mcp_async.go` | Async MCP operations |
| `mcp_autonomy.go` | MCP autonomy settings |
| `skill_tool.go` | Skill loading and execution |
| `prompts.go` | System prompt templates |
| `loop_detection.go` | Agent loop detection |
| `indexer.go` | Codebase indexer |
| `tool_cache.go` | Tool caching |
| `tool_filter.go` | Tool filtering |
| `runtime_control.go` | Runtime control for tool execution |
| `errors.go` | Agent error definitions |
| `event.go` | Agent event definitions |

### `/internal/agent/tools/` - Built-in Tools

| File | Purpose |
|------|---------|
| `tools.go` | Tool context keys and helpers |
| `bash.go` | Bash command execution tool |
| `edit.go` | File edit tool (multi-edit support) |
| `write.go` | File write tool |
| `view.go` / `fast_view.go` | File view tools |
| `glob.go` | Glob pattern file search |
| `grep.go` / `rg.go` | Grep/ripgrep text search |
| `ls.go` | Directory listing |
| `fetch.go` | Web fetching |
| `web_fetch.go` / `web_search.go` | Web search tools |
| `download.go` | File download |
| `python.go` | Python code execution |
| `todos.go` | Todo management |
| `references.go` | Reference management |
| `search.go` | Code search |
| `sourcegraph.go` | Sourcegraph integration |
| `google_search.go` | Google search |
| `diagnostics.go` | LSP diagnostics |
| `lsp_restart.go` | LSP restart tool |
| `multiedit.go` | Multi-edit operations |
| `job_output.go` / `job_kill.go` | Background job management |
| `mcp-tools.go` | MCP tool execution |
| `call_mcp_tool.go` | MCP tool calling |
| `list_mcp_tools.go` / `list_mcp_resources.go` | MCP listing |
| `memory_query.go` | Memory query tool |
| `tool_suggest.go` | Tool suggestion |
| `edit_guard.go` | Edit protection |
| `tool_call_validation.go` | Tool call validation |
| `tool_call_preflight.go` | Tool call preflight |
| `safe.go` | Safe execution helpers |
| `dispatcher.go` / `fast_dispatcher.go` | Tool dispatch |

### `/internal/agent/tools/mcp/` - MCP Tools

| File | Purpose |
|------|---------|
| `init.go` | MCP client initialization |
| `manage.go` | MCP client management |
| `tools.go` | MCP tool execution |
| `resources.go` | MCP resource handling |
| `prompts.go` | MCP prompt handling |
| `timeout.go` | MCP timeout handling |

### `/internal/agent/memory/` - Agent Memory

| File | Purpose |
|------|---------|
| `memory.go` | Memory service for agent context |

### `/internal/agent/prompt/` - Prompt System

| File | Purpose |
|------|---------|
| `prompt.go` | Prompt building and management |

### `/internal/agent/longhorizon/` - Long-Horizon Tasks

| File | Purpose |
|------|---------|
| `manager.go` | Long-horizon task manager; spec/plan/runbook/audit |

### `/internal/agent/hyper/` - Hyper Provider

| File | Purpose |
|------|---------|
| `provider.go` | Hyper provider configuration |

### `/internal/app/` - Application Wire

| File | Purpose |
|------|---------|
| `app.go` | Application container; wires services, coordinates agents |
| `lsp_events.go` | LSP event subscription |
| `provider.go` | Provider helpers |

### `/internal/cmd/` - CLI Commands

| File | Purpose |
|------|---------|
| `root.go` | Root CLI command; setup, TUI initialization |
| `run.go` | Non-interactive run command |
| `login.go` | OAuth login |
| `mcp.go` | MCP management commands |
| `models.go` | Model listing |
| `projects.go` | Project management |
| `dirs.go` | Directory commands |
| `logs.go` | Log viewing |
| `stats.go` | Usage statistics |
| `worktrees.go` | Worktree management (orchestrate, clean --merged) |
| `update_providers.go` | Provider updates |
| `schema.go` | Schema generation |

### `/internal/config/` - Configuration

| File | Purpose |
|------|---------|
| `config.go` | Config types and methods |
| `load.go` | Config loading from files |
| `init.go` | Config initialization |
| `provider.go` | Provider configuration |
| `mcp.go` | MCP configuration |
| `mcp_registry.go` | MCP registry |
| `mcp_catalog.go` | MCP catalog |
| `lsp_defaults.go` | LSP defaults |
| `hyper.go` | Hyper configuration |
| `copilot.go` | GitHub Copilot OAuth |
| `catwalk.go` | Catwalk provider catalog |
| `resolve.go` | Variable resolution |
| `recent_models.go` | Recent model tracking |

### `/internal/db/` - Database Layer

| File | Purpose |
|------|---------|
| `db.go` | Database connection |
| `connect.go` | Connection helpers |
| `models.go` | Database models |
| `querier.go` | Querier interface |
| `files.sql.go` | File queries (sqlc generated) |
| `messages.sql.go` | Message queries |
| `sessions.sql.go` | Session queries |
| `read_files.sql.go` | Read file tracking |
| `tiered_memory.sql.go` | Tiered memory |
| `stats.sql.go` | Statistics queries |
| `embed.go` | Embedded migrations |
| `supabase.go` | Supabase integration |

### `/internal/db/sql/` - SQL Queries

| File | Purpose |
|------|---------|
| `files.sql` | File table queries |
| `messages.sql` | Message table queries |
| `sessions.sql` | Session table queries |
| `read_files.sql` | Read file tracking queries |
| `tiered_memory.sql` | Tiered memory queries |
| `stats.sql` | Statistics queries |

### `/internal/db/migrations/` - Database Migrations

| File | Purpose |
|------|---------|
| `*.sql` | SQLite migrations |

### `/internal/session/` - Session Service

| File | Purpose |
|------|---------|
| `session.go` | Session service; CRUD, todos, pubsub |

### `/internal/message/` - Message Service

| File | Purpose |
|------|---------|
| `message.go` | Message service; CRUD, parts marshaling |
| `content.go` | Message content types |
| `attachment.go` | Attachment handling |

### `/internal/history/` - File History Service

| File | Purpose |
|------|---------|
| `file.go` | File history service |

### `/internal/permission/` - Permission Service

| File | Purpose |
|------|---------|
| `permission.go` | Permission service; tool approval |

### `/internal/filetracker/` - File Tracker Service

| File | Purpose |
|------|---------|
| `service.go` | File tracking service |

### `/internal/skills/` - Skills System

| File | Purpose |
|------|---------|
| `skills.go` | Skill discovery and parsing |
| `bundle.go` | Skill bundling |
| `embedding.go` | Embedding-based skill retrieval |
| `fast_loader.go` | Fast skill loading |

### `/internal/shell/` - Shell Execution

| File | Purpose |
|------|---------|
| `shell.go` | Shell execution (POSIX via mvdan.cc/sh) |
| `background.go` | Background shell management |
| `fast_background.go` | Fast background execution |
| `coreutils.go` | Coreutils integration |
| `ringbuffer.go` | Output ring buffer |

### `/internal/lsp/` - LSP Integration

| File | Purpose |
|------|---------|
| `client.go` | LSP client |
| `manager.go` | LSP manager; lifecycle |
| `handlers.go` | LSP handlers |
| `util/edit.go` | LSP edit helpers |

### `/internal/memory/` - Persistent Memory

| File | Purpose |
|------|---------|
| `system.go` | Persistent memory system |
| `store.go` | Memory store |
| `embedding.go` | Memory embedding |
| `extraction.go` | Memory extraction |
| `pipeline.go` | Memory pipeline |
| `tools.go` | Memory tools |

### `/internal/llm/` - LLM Providers

| Directory | Purpose |
|-----------|---------|
| `provider/gemini/` | Google Gemini provider |

### `/internal/oauth/` - OAuth Integration

| Directory | Purpose |
|-----------|---------|
| `copilot/` | GitHub Copilot OAuth |
| `hyper/` | Hyper OAuth |

### `/internal/ui/` - User Interface

| Directory | Purpose |
|-----------|---------|
| `model/` | Main UI model (Bubble Tea) |
| `chat/` | Chat message rendering |
| `dialog/` | Dialog overlays |
| `common/` | Common UI components |
| `list/` | List components |
| `diffview/` | Diff viewing |
| `styles/` | Styling |
| `anim/` | Animations |
| `logo/` | Logo rendering |
| `completions/` | Completions UI |
| `attachments/` | Attachment UI |
| `image/` | Image rendering |
| `util/` | UI utilities |
| `notification/` | Notifications |

### `/internal/event/` - Event System

| File | Purpose |
|------|---------|
| `event.go` | PostHog analytics events |
| `logger.go` | Event logger |
| `identifier.go` | User identification |
| `all.go` | Event helpers |

### `/internal/pubsub/` - Pub/Sub System

| File | Purpose |
|------|---------|
| `broker.go` | Pub/sub broker |
| `events.go` | Event types |

### `/internal/csync/` - Concurrent Collections

| File | Purpose |
|------|---------|
| `maps.go` | Concurrent maps |
| `slices.go` | Concurrent slices |
| `value.go` | Concurrent value |
| `versionedmap.go` | Versioned maps |

### `/internal/fsext/` - Filesystem Extensions

| File | Purpose |
|------|---------|
| `fileutil.go` | File utilities |
| `lookup.go` | File lookup |
| `ls.go` | Directory listing |
| `expand.go` | Path expansion |
| `ignore.go` | Gitignore handling |
| `paste.go` | Clipboard paste |

### `/internal/format/` - Formatting

| File | Purpose |
|------|---------|
| `spinner.go` | Progress spinner |

### `/internal/diff/` - Diff Utilities

| File | Purpose |
|------|---------|
| `diff.go` | Diff generation |

### `/internal/ansiext/` - ANSI Extensions

| File | Purpose |
|------|---------|
| `ansi.go` | ANSI escape codes |

### `/internal/stringext/` - String Extensions

| File | Purpose |
|------|---------|
| `string.go` | String utilities |

### `/internal/filepathext/` - Filepath Extensions

| File | Purpose |
|------|---------|
| `filepath.go` | Filepath utilities |

### `/internal/env/` - Environment

| File | Purpose |
|------|---------|
| `env.go` | Environment helpers |

### `/internal/home/` - Home Directory

| File | Purpose |
|------|---------|
| `home.go` | Home directory helpers |

### `/internal/version/` - Version

| File | Purpose |
|------|---------|
| `version.go` | Version information |

### `/internal/update/` - Update Checking

| File | Purpose |
|------|---------|
| `update.go` | Update checking |

### `/internal/log/` - Logging

| File | Purpose |
|------|---------|
| `log.go` | Logging setup |
| `http.go` | HTTP logging |

### `/internal/projects/` - Project Management

| File | Purpose |
|------|---------|
| `projects.go` | Project registration |

### `/internal/runtimeopt/` - Runtime Options

| File | Purpose |
|------|---------|
| `runtime.go` | Runtime options |

### `/internal/commands/` - Custom Commands

| File | Purpose |
|------|---------|
| `commands.go` | Custom command loading |

### `/long_horizon/` - Long-Horizon Artifacts

| Directory | Purpose |
|-----------|---------|
| `<session-id>/` | Per-session long-horizon artifacts |

### `/.sapphire/` - Data Directory

| Directory | Purpose |
|-----------|---------|
| `skills/` | Skill definitions |
| `subagents/` | Sub-agent worktrees |
| `logs/` | Application logs |

---

## Core Components

### Entry Point (`main.go`)

```go
func main() {
    // Optional profiling server
    if os.Getenv("CRUSH_PROFILE") != "" {
        go http.ListenAndServe("localhost:6060", nil)
    }
    cmd.Execute()
}
```

### CLI Root (`internal/cmd/root.go`)

**Purpose:** CLI command setup and TUI initialization.

**Key Functions:**
- `Execute()` - Runs root command with fang wrapper
- `setupApp()` - Creates app instance with config, DB, services
- `setupAppWithProgressBar()` - Adds progress bar for TUI

**Flags:**
- `--cwd` / `-c` - Working directory
- `--data-dir` / `-D` - Custom data directory
- `--debug` / `-d` - Debug logging
- `--yolo` / `-y` - Auto-approve permissions

### Application Container (`internal/app/app.go`)

**Purpose:** Wires together services, coordinates agents, manages lifecycle.

**Key Types:**
- `App` - Main application struct

**Fields:**
- `Sessions` - Session service
- `Messages` - Message service
- `History` - File history service
- `Permissions` - Permission service
- `FileTracker` - File tracker service
- `Conn` - Database connection
- `AgentCoordinator` - Agent coordinator
- `LSPManager` - LSP manager
- `config` - Configuration

**Key Methods:**
- `New()` - Initialize app with services
- `InitCoderAgent()` - Initialize coder agent
- `RunNonInteractive()` - Headless execution
- `Subscribe()` - Forward events to TUI
- `Shutdown()` - Graceful termination

---

## Agent System

### Agent Coordinator (`internal/agent/coordinator.go`)

**Purpose:** Manages multiple AI agents, sub-agents, MCP selection, skill matching.

**Key Types:**
- `Coordinator` interface - Agent management API
- `coordinator` struct - Implementation

**Methods:**
- `Run(ctx, sessionID, prompt)` - Execute agent synchronously
- `Submit(ctx, sessionID, prompt)` - Submit for async execution
- `Cancel(sessionID)` - Cancel session
- `CancelAll()` - Cancel all sessions
- `IsSessionBusy(sessionID)` - Check if session is busy
- `Summarize(ctx, sessionID)` - Summarize conversation
- `UpdateModels(ctx)` - Update model configurations
- `spawnSubAgent()` - Spawn sub-agent
- `waitSubAgents()` - Wait for sub-agents
- `closeSubAgent()` - Close sub-agent
- `OrchestrateWorktrees()` - Worktree orchestration

**Key Logic:**
- Skill keyword matching via `skillKeywordMap`
- MCP preflight and selection
- Google Search failure tracking
- Background sub-agent limiting
- Tool caching

### Session Agent (`internal/agent/agent.go`)

**Purpose:** Session-based AI agent with LLM streaming and tool execution.

**Key Types:**
- `SessionAgent` interface
- `sessionAgent` struct

**Fields:**
- `largeModel` / `smallModel` - Model configurations
- `systemPrompt` - System prompt
- `tools` - Available tools
- `sessions` / `messages` - Services
- `memory` - Memory service
- `pmem` - Persistent memory
- `longHorizon` - Long-horizon manager

**Key Methods:**
- `Run(ctx, call)` - Execute agent turn
- `Enqueue(call)` - Queue prompt
- `SetModels()` / `SetTools()` / `SetSystemPrompt()` - Configuration
- `Summarize()` - Auto-summarization

**Stream Handling:**
- `PrepareStep` - Message preparation
- `OnTextDelta` - Text streaming
- `OnToolCallStart` / `OnToolCallEnd` - Tool lifecycle
- `OnReasoningStart` / `OnReasoningDelta` / `OnReasoningEnd` - Reasoning content

**Auto-Summarization:**
- Large context: 200K tokens, 20K buffer
- Small context: 20% ratio, 3K min buffer

### Sub-Agent Manager (`internal/agent/subagent_manager.go`)

**Purpose:** Sub-agent lifecycle management.

**Key Types:**
- `subAgentRunner` - Sub-agent execution context
- `subAgentRegistry` - Sub-agent tracking
- `subAgentSnapshot` - Sub-agent state

**Status Values:**
- `subAgentStatusQueued`
- `subAgentStatusRunning`
- `subAgentStatusCompleted`
- `subAgentStatusError`
- `subAgentStatusClosed`

**Key Methods:**
- `spawnSubAgent()` - Create sub-agent
- `runSubAgentLoop()` - Execution loop
- `runSubAgentTurn()` - Single turn execution
- `resumeSubAgent()` - Resume closed agent
- `waitSubAgents()` - Wait for completion
- `closeSubAgent()` - Terminate agent
- `sendSubAgentInput()` - Send follow-up

**Worktree Integration:**
- Isolated worktrees per sub-agent
- Branch management
- Write scope enforcement

---

## Sub-Agent System

### Architecture

Sapphire implements a multi-agent orchestration system with:

- Worktree-based isolation for sub-agents
- Automatic snapshot commits for recoverability
- Destructive Git command denial for safety
- Human-controlled integration (no auto-push/merge)
- Hierarchical spawning with depth limits
- Status tracking and pub/sub
- Context forking
- Completion signals

### Worktree Isolation

Each sub-agent operates in an isolated Git worktree under `.sapphire/worktrees/agent/<agent-id>/<task-slug>/`.

**Worktree structure:**
```
repo-root/
├── .sapphire/
│   ├── worktrees/
│   │   └── agent/
│   │       ├── <agent-id-1>/
│   │       │   └── <task-slug>/
│   │       └── <agent-id-2>/
│   │           └── <task-slug>/
│   └── quarantine/
│       └── <failed-worktree>/
```

**Branch naming:** `agent/<agent-id>/<task-slug>`

**Base branch selection:**
1. `main` if it exists
2. `master` if `main` does not exist
3. `origin/HEAD` if remote tracking exists
4. `HEAD` as final fallback

### Snapshot Commits

Automatic local snapshot commits are created after meaningful file writes with a 1.5-second debounce.

**Snapshot manager:** `internal/agent/tools/git_snapshot.go`

**Behavior:**
- Triggered after file writes in any Git worktree
- Debounced to batch rapid writes
- Flushable on demand before task completion
- Local-only commits (never auto-pushed)
- Actor naming: `main-agent` for main workspace, `<agent-id>-<task-slug>` for sub-agents

**Commit format:**
```
snapshot: <actor-name> <timestamp>
```

**Example:**
```
snapshot: main-agent 20260319-143022
snapshot: agent-1-render-fix 20260319-143045
```

### Git Safety Policy

All agents are blocked from destructive Git operations by default.

**Blocked commands:**
- `git push` - agents never push autonomously
- `git merge` - integration is human-controlled
- `git rebase` - history rewriting blocked
- `git restore` - destructive recovery blocked
- `git clean` - file deletion blocked
- `git reset --hard` - hard reset blocked
- `git worktree remove` - worktree deletion blocked
- `git branch -d/-D` - branch deletion blocked

**Implementation:** `internal/agent/tools/bash.go::isForbiddenGitAgentCommand()`

### Spawn Flow

```
Model calls spawn_agent tool
         │
         ▼
validateSubAgentLaunch() - Check depth limit
         │
         ▼
prepareSubAgentWorktree() - Create isolated worktree
         │
         ▼
buildAgentWithWorkingDirOverrides() - Configure agent
         │
         ▼
sessions.CreateTaskSession() - Create DB session
         │
         ▼
subAgentRunner initialization
         │
         ▼
runSubAgentLoop() - Start execution goroutine
         │
         ▼
enqueue() - Submit initial prompt
```

### Depth Tracking

Depth encoded in session metadata:
```
CLI session (depth=0)
  └─ spawn_agent → depth=1
       └─ spawn_agent → depth=2
            └─ spawn_agent → depth=3 (max)
```

### Context Forking

- Copies last N messages from parent
- Excludes tool history
- Includes summary message if present
- Maximum 40 messages

### Status Subscription

```go
runner.subscribeStatus(ctx) <-chan pubsub.Event[subAgentStatus]
```

Events published on:
- Spawn
- Running
- Completed
- Failed
- Closed

### Worktree Lifecycle

**Creation:**
1. Validate worktree path format (`.sapphire/worktrees/agent/<id>/<slug>`)
2. Validate branch format (`agent/<id>/<task-slug>`)
3. Ensure base worktree is clean
4. Create worktree from base branch
5. Add `.sapphire/worktrees/` to `.gitignore`

**Cleanup:**
- Zero-change worktrees: deleted immediately
- Failed worktrees with changes: quarantined to `.sapphire/quarantine/`
- Merged worktrees: removed via human-triggered `sapphire worktree clean --merged`
- Crashed worktrees: preserved for `--resume` flow

**Review branch archival:**
Failed worktrees with validation failures can be archived to `review/<task-slug>` branches for later inspection.

### Validation Gate

**File:** `internal/agent/subagent_validation_gate.go`

**Validation phases:**
1. Git diff stat against base branch
2. Build verification (auto-detected: `go build`, `npm run build`, `cargo build`)
3. Test verification (auto-detected: `go test`, `npm test`, `cargo test`)
4. Lint verification (auto-detected: `golangci-lint`, `npm run lint`, `task lint`)
5. Security scan (auto-detected: `gosec`, `npm run security`, `task security`)

**Output:** Validation report with pass/fail status, diff summary, and error messages.

### Worktree Orchestration CLI

**Command:** `sapphire worktrees` (alias: `sapphire worktree`)

**Subcommands:**
- `sapphire worktrees orchestrate` - Spawn sub-agents from spec file
- `sapphire worktrees clean --merged` - Remove merged worktrees

**File:** `internal/cmd/worktrees.go`

---

## Tool System

### Tool Architecture

Tools are implemented using `fantasy.AgentTool` from charm.land/fantasy.

**Context Keys:**
- `SessionIDContextKey` - Session ID
- `MessageIDContextKey` - Message ID
- `WorkingDirContextKey` - Working directory
- `WriteScopeContextKey` - Sub-agent write constraints
- `RuntimeControlContextKey` - Runtime control

### Built-in Tools

#### Bash Tool (`internal/agent/tools/bash.go`)

**Purpose:** Execute shell commands.

**Parameters:**
- `command` - Command to execute
- `description` - Brief description
- `working_dir` - Working directory
- `run_in_background` - Background execution
- `backend` - Execution backend (posix/native)
- `justification` - Audit trail
- `prefix_rule` - Commands to prepend

**Banned Commands:**
- Network tools: curl, wget, ssh, scp
- Package managers: apt, yum, pacman, brew
- System modification: sudo, systemctl, mount
- Network config: iptables, ifconfig

**Git Safety Policy:**
The following Git commands are blocked for all agents:
- `git push` - push remains human-controlled
- `git merge` - integration remains human-controlled
- `git rebase` - history rewriting blocked
- `git restore` - destructive recovery blocked
- `git clean` - file deletion blocked
- `git reset --hard` - hard reset blocked
- `git worktree remove` - worktree deletion blocked
- `git branch -d/-D` - branch deletion blocked

**Implementation:** `isForbiddenGitAgentCommand()`

#### Edit Tool (`internal/agent/tools/edit.go`)

**Purpose:** File editing with precision matching.

**Parameters:**
- `file_path` - Target file
- `old_string` - Text to replace
- `new_string` - Replacement text
- `replace_all` - Replace all occurrences

**Errors:**
- `old_string not found` - Precision violation
- `old_string matches multiple` - Ambiguity

**Features:**
- Edit guard for file locking
- LSP diagnostics integration
- Diff generation
- File creation support
- Automatic snapshot commit queuing

#### Git Snapshot Tool (`internal/agent/tools/git_snapshot.go`)

**Purpose:** Automatic local snapshot commits for recoverability.

**Behavior:**
- Automatically triggered after file writes
- 1.5-second debounce for batched writes
- Flushable on demand before task completion
- Creates local-only commits (never pushed)
- Actor naming based on worktree type

**Functions:**
- `QueueGitSnapshot(ctx, mutatedPath)` - Queue a snapshot commit
- `FlushGitSnapshot(ctx, worktreeDir)` - Flush pending snapshots
- `commitGitSnapshot(worktreeDir)` - Create snapshot commit

**Commit format:**
```
snapshot: <actor-name> <timestamp>
```

**Actor naming:**
- Main workspace: `main-agent`
- Sub-agent worktrees: `<agent-id>-<task-slug>`

#### View Tool (`internal/agent/tools/view.go`)

**Purpose:** File viewing with smart truncation.

#### Write Tool (`internal/agent/tools/write.go`)

**Purpose:** Full file write.

#### Glob Tool (`internal/agent/tools/glob.go`)

**Purpose:** Pattern-based file search.

#### Grep Tool (`internal/agent/tools/grep.go`)

**Purpose:** Text search with regex.

#### LS Tool (`internal/agent/tools/ls.go`)

**Purpose:** Directory listing.

**Limits:**
- Max depth (default 0, configurable)
- Max items (default 1000)

#### Python Tool (`internal/agent/tools/python.go`)

**Purpose:** Python code execution.

**Features:**
- Sandboxed execution
- Failure tracking (quit after 3 failures)
- Output capture

#### Todo Tool (`internal/agent/tools/todos.go`)

**Purpose:** Todo management.

**Operations:**
- Create
- Update status
- List

#### MCP Tools (`internal/agent/tools/mcp-*.go`)

**Purpose:** Model Context Protocol integration.

**Tools:**
- `call_mcp_tool` - Execute MCP tool
- `list_mcp_tools` - List available tools
- `list_mcp_resources` - List resources
- `read_mcp_resource` - Read resource
- `connect_mcp` - Connect server

### Tool Execution Flow

```
Model invokes tool call
         │
         ▼
PrepareToolCall() - Validate and normalize
         │
         ▼
permission.Request() - Request approval (if needed)
         │
         ▼
Tool execution
         │
         ▼
LSP diagnostics (for file edits)
         │
         ▼
Response formatting
         │
         ▼
Tool result to LLM
```

---

## UI System

### Architecture

Built with Bubble Tea (charm.land/bubbletea/v2).

**Main Model:** `internal/ui/model/ui.go`

**States:**
- `uiOnboarding` - First-run setup
- `uiInitialize` - Loading state
- `uiLanding` - Session selection
- `uiChat` - Active chat

**Focus States:**
- `uiFocusNone`
- `uiFocusEditor`
- `uiFocusMain`

### Components

#### Chat (`internal/ui/chat/`)

**Files:**
- `assistant.go` - Assistant messages
- `user.go` - User messages
- `tools.go` - Tool call rendering
- `bash.go` - Bash output
- `file.go` - File attachments
- `todos.go` - Todo rendering
- `agent.go` - Sub-agent messages
- `mcp.go` - MCP messages
- `search.go` - Search results
- `fetch.go` - Fetch results
- `diagnostics.go` - LSP diagnostics
- `messages.go` - Message helpers

#### Dialogs (`internal/ui/dialog/`)

**Overlays:**
- `filepicker.go` - File selection
- `models.go` - Model selection
- `sessions.go` - Session list
- `permissions.go` - Permission prompts
- `api_key_input.go` - API key entry
- `oauth.go` - OAuth flow
- `mcp_config.go` - MCP configuration
- `commands.go` - Custom commands
- `reasoning.go` - Reasoning settings

#### Common (`internal/ui/common/`)

**Components:**
- `button.go` - Buttons
- `markdown.go` - Markdown rendering
- `diff.go` - Diff viewing
- `scrollbar.go` - Scrollbars
- `highlight.go` - Syntax highlighting

### Event Handling

UI receives events via `app.Subscribe(program)`:

- Session events (created, updated, deleted)
- Message events (created, updated)
- Permission events (requested, approved, denied)
- MCP events (connected, disconnected)
- LSP events (diagnostics, state changes)

---

## Configuration System

### Configuration File (`crush.json`)

**Schema:** `schema.json`

**Structure:**

```json
{
  "$schema": "https://charm.land/crush.json",
  "models": {
    "large": {"model": "gpt-4o", "provider": "openai"},
    "small": {"model": "gpt-4o-mini", "provider": "openai"}
  },
  "providers": {
    "openai": {
      "api_key": "$OPENAI_API_KEY",
      "models": [...]
    }
  },
  "mcp": {...},
  "lsp": {...},
  "options": {...},
  "permissions": {...}
}
```

### Config Loading (`internal/config/load.go`)

**Flow:**
1. Lookup config paths (project, home, global)
2. Load and merge configs
3. Set defaults
4. Load known providers from Catwalk
5. Configure providers (API keys, headers)
6. Configure selected models
7. Setup agents

### Provider Configuration

**Supported Providers:**
- OpenAI
- Anthropic
- Google Gemini
- Azure
- AWS Bedrock
- OpenRouter
- Vercel
- Hyper
- GitHub Copilot (OAuth)

**Configuration:**
- `base_url` - API endpoint
- `api_key` - API key (supports env vars)
- `type` - Provider type
- `models` - Available models
- `extra_headers` - Custom headers
- `extra_body` - Custom body fields

### MCP Configuration

```json
{
  "mcp": {
    "server-name": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server"],
      "type": "stdio",
      "env": {...},
      "disabled_tools": [...]
    }
  }
}
```

### LSP Configuration

```json
{
  "lsp": {
    "gopls": {
      "command": "gopls",
      "filetypes": ["go", "mod"],
      "root_markers": ["go.mod"],
      "options": {...}
    }
  }
}
```

### Options

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `context_paths` | []string | [...] | Context file paths |
| `skills_paths` | []string | [...] | Skill directories |
| `data_directory` | string | `.sapphire` | Data storage |
| `disabled_tools` | []string | [] | Disabled built-in tools |
| `debug` | bool | false | Debug logging |
| `debug_lsp` | bool | false | LSP debug logging |
| `auto_lsp` | *bool | true | Auto-setup LSP |
| `progress` | *bool | true | Show progress |
| `google_grounding` | bool | false | Gemini grounding |
| `agent_max_depth` | int | 2 | Max sub-agent depth |
| `agent_max_threads` | int | 6 | Max concurrent sub-agents |

---

## Database Schema

### Tables

#### `sessions`

| Column | Type | Description |
|--------|------|-------------|
| `id` | TEXT | Primary key |
| `parent_session_id` | TEXT | Parent session (sub-agents) |
| `title` | TEXT | Session title |
| `message_count` | INTEGER | Message count |
| `prompt_tokens` | INTEGER | Prompt tokens used |
| `completion_tokens` | INTEGER | Completion tokens |
| `cost` | REAL | Total cost |
| `summary_message_id` | TEXT | Summary message reference |
| `todos` | TEXT | JSON todo list |
| `created_at` | INTEGER | Unix timestamp |
| `updated_at` | INTEGER | Unix timestamp |

#### `messages`

| Column | Type | Description |
|--------|------|-------------|
| `id` | TEXT | Primary key |
| `session_id` | TEXT | Foreign key to sessions |
| `role` | TEXT | user/assistant/system |
| `parts` | TEXT | JSON message parts |
| `model` | TEXT | Model used |
| `provider` | TEXT | Provider used |
| `is_summary_message` | INTEGER | Summary flag |
| `created_at` | INTEGER | Unix timestamp |
| `updated_at` | INTEGER | Unix timestamp |
| `finished_at` | INTEGER | Completion timestamp |

#### `files`

| Column | Type | Description |
|--------|------|-------------|
| `id` | TEXT | Primary key |
| `session_id` | TEXT | Foreign key |
| `path` | TEXT | File path |
| `content` | TEXT | File content |
| `version` | INTEGER | Version number |
| `created_at` | INTEGER | Unix timestamp |
| `updated_at` | INTEGER | Unix timestamp |

#### `codebase_knowledge`

| Column | Type | Description |
|--------|------|-------------|
| `id` | TEXT | Primary key |
| `file_path` | TEXT | File path |
| `symbol_name` | TEXT | Symbol name |
| `symbol_type` | TEXT | Symbol type |
| `signature` | TEXT | Symbol signature |
| `documentation` | TEXT | Documentation |
| `location_range` | TEXT | Location range |
| `created_at` | INTEGER | Unix timestamp |
| `updated_at` | INTEGER | Unix timestamp |

#### `read_files`

| Column | Type | Description |
|--------|------|-------------|
| `session_id` | TEXT | Session ID |
| `path` | TEXT | File path |
| `read_at` | INTEGER | Read timestamp |

#### `project_constitution`

| Column | Type | Description |
|--------|------|-------------|
| `id` | TEXT | Primary key |
| `content` | TEXT | Constitution content |
| `created_at` | INTEGER | Unix timestamp |
| `updated_at` | INTEGER | Unix timestamp |

#### `structured_summaries`

| Column | Type | Description |
|--------|------|-------------|
| `id` | TEXT | Primary key |
| `session_id` | TEXT | Session ID |
| `summary_data` | TEXT | Summary JSON |
| `created_at` | INTEGER | Unix timestamp |
| `updated_at` | INTEGER | Unix timestamp |

### Migrations

Located in `internal/db/migrations/`:

1. `20250424200609_initial.sql` - Initial schema
2. `20250515105448_add_summary_message_id.sql` - Summary message reference
3. `20250624000000_add_created_at_indexes.sql` - Index additions
4. `20250627000000_add_provider_to_messages.sql` - Provider tracking
5. `20250810000000_add_is_summary_message.sql` - Summary flag
6. `20250812000000_add_todos_to_sessions.sql` - Todo support
7. `20260127000000_add_read_files_table.sql` - Read file tracking
8. `20260309000000_add_tiered_memory_tables.sql` - Tiered memory

---

## Event System

### PostHog Analytics (`internal/event/event.go`)

**Events Tracked:**
- `SessionCreated`
- `SessionDeleted`
- `MessageCreated`
- `ToolExecuted`
- `AppInitialized`
- `AppExited`
- `Error`

**Properties:**
- GOOS / GOARCH
- TERM
- SHELL
- Version
- GoVersion
- NonInteractive flag

**User Identification:**
- Machine ID (distinct_id)
- Optional user ID aliasing

### Pub/Sub System (`internal/pubsub/`)

**Broker Pattern:**
```go
broker := pubsub.NewBroker[T]()
broker.Subscribe(ctx) <-chan Event[T]
broker.Publish(eventType, payload)
```

**Event Types:**
- `CreatedEvent`
- `UpdatedEvent`
- `DeletedEvent`

**Subscribers:**
- Sessions
- Messages
- Permissions
- History
- MCP
- LSP

---

## MCP Integration

### Architecture (`internal/agent/tools/mcp/`)

**Components:**
- `ClientSession` - MCP client connection
- `sessions` - Global session registry
- `allTools` - Tool registry

**Lifecycle:**
1. Initialize client (stdio/HTTP/SSE)
2. List available tools
3. Filter disabled tools
4. Register in global tool registry
5. Handle tool calls

### MCP Operations

**Initialize:**
```go
mcp.Initialize(ctx, &mcp.InitializeParams{...})
```

**List Tools:**
```go
mcp.ListTools(ctx, &mcp.ListToolsParams{})
```

**Call Tool:**
```go
mcp.CallTool(ctx, &mcp.CallToolParams{
    Name: "tool-name",
    Arguments: map[string]any{...},
})
```

### MCP Registry

**Purpose:** Dynamic MCP discovery and selection.

**Flow:**
1. User prompt analysis
2. MCP preflight discovery
3. MCP selection based on relevance
4. Tool injection into agent context

---

## LSP Integration

### Architecture (`internal/lsp/`)

**Components:**
- `Client` - LSP client
- `Manager` - LSP lifecycle manager

**Lifecycle:**
1. Detect file type
2. Find root marker
3. Start server
4. Initialize
5. Open documents
6. Handle diagnostics

### Diagnostics

**Callback:**
```go
client.SetDiagnosticsCallback(func(uri string, diagnostics []Diagnostic) {
    updateLSPDiagnostics(uri, diagnostics)
})
```

**Integration:**
- Edit tool triggers diagnostics
- Diagnostics injected into tool response
- Edit guard locks files with errors

---

## Skills System

### Specification (`internal/skills/skills.go`)

**Standard:** Agent Skills open standard (https://agentskills.io)

**Skill Structure:**
```yaml
---
name: skill-name
description: Skill description
license: MIT
compatibility: sapphire>=1.0
metadata:
  key: value
---
# Skill instructions (markdown)
```

### Discovery

```go
skills.Discover(paths []string) []*Skill
```

**Process:**
1. Walk directories (follows symlinks)
2. Find SKILL.md files
3. Parse frontmatter
4. Validate (name, description, path match)
5. Return valid skills

### Skill Matching

**Keyword-based matching:**
- `skillKeywordMap` defines category keywords
- Whole-word matching
- Folder path matching
- Category aliases

**Categories:**
- frontend
- backend
- debugging
- architect
- devops
- security

### Embedding-based Retrieval

**Requirements:**
- Gemini API key
- Embedding service initialization

**Flow:**
1. User prompt embedding
2. Similarity search against skill embeddings
3. Return top matches above threshold

---

## Long-Horizon Tasks

### Architecture (`internal/agent/longhorizon/`)

**Purpose:** Structured long-horizon task management.

**Artifacts:**
- `frozen_spec.md` - Task specification
- `milestones.json` - Milestone plan
- `runbook.md` - Operating procedures
- `audit.log` - Decision audit trail

### Manager

**Methods:**
- `Ensure(ctx, sessionID, prompt)` - Initialize artifacts
- `AppendAudit(ctx, sessionID, lines)` - Log decisions
- `BuildInjection(sessionID)` - Build context block

### Runbook Rules

- Work milestone-by-milestone
- Keep diffs scoped to active milestone
- Validate completion before moving on
- Write decisions to audit log
- Handle failures with recovery attempts

### Context Injection

```xml
<long_horizon_runbook>
  ...runbook content...
</long_horizon_runbook>

<long_horizon_frozen_spec>
  ...spec content...
</long_horizon_frozen_spec>

<long_horizon_milestones>
  ...milestones JSON...
</long_horizon_milestones>

<long_horizon_audit>
  ...audit log tail...
</long_horizon_audit>
```

---

## Shell System

### Architecture (`internal/shell/`)

**Implementation:** POSIX shell via mvdan.cc/sh/v3

**Features:**
- Cross-platform (POSIX emulation on Windows)
- Environment variable management
- Working directory tracking
- Command blocking
- Background execution
- Streaming output

### Shell Instance

```go
shell := NewShell(&Options{
    WorkingDir: "/path",
    Env: []string{"VAR=value"},
    BlockFuncs: []BlockFunc{...},
})
```

### Execution

**Foreground:**
```go
stdout, stderr, err := shell.Exec(ctx, "command")
```

**Streaming:**
```go
err := shell.ExecStream(ctx, "command", stdoutWriter, stderrWriter)
```

**Background:**
```go
jobID, err := backgroundManager.Start(ctx, "command")
```

### Command Blocking

**BlockFunc:**
```go
type BlockFunc func(args []string) bool
```

**Built-in Blockers:**
- `CommandsBlocker([]string)` - Exact command match
- `ArgumentsBlocker(cmd, args, flags)` - Subcommand/flag match

**Banned Commands:**
- Network: curl, wget, ssh, scp
- Package managers: apt, yum, pacman
- System: sudo, systemctl, mount

---

## Key Integration Points

### Permission System

**Service:** `internal/permission/permission.go`

**Flow:**
1. Tool requests permission
2. Permission service checks policy
3. If YOLO mode: auto-approve
4. Otherwise: prompt user (TUI)
5. Store decision
6. Execute or deny

**Session-based Approval:**
```go
permissions.AutoApproveSession(sessionID)
```

### File Tracker

**Service:** `internal/filetracker/service.go`

**Purpose:** Track file modifications per session.

### History Service

**Service:** `internal/history/file.go`

**Purpose:** File version history.

### Memory System

**Service:** `internal/memory/system.go`

**Purpose:** Persistent memory with vector search.

**Requirements:**
- Gemini API key
- Embedding model

**Operations:**
- Extract memories from conversations
- Store with embeddings
- Query by similarity

---

## Testing

### Test Files

Located alongside source files with `_test.go` suffix.

**Key Test Files:**
- `internal/agent/agent_test.go`
- `internal/agent/coordinator_test.go`
- `internal/agent/tools/*.go`
- `internal/config/*_test.go`
- `internal/db/*_test.go`
- `internal/ui/chat/*_test.go`

### Test Data

- `internal/agent/testdata/` - Agent test fixtures
- `internal/agent/tools/testdata/` - Tool test data

---

## Environment Variables

| Variable | Purpose |
|----------|---------|
| `CRUSH_PROFILE` | Enable profiling server |
| `CRUSH_DISABLE_METRICS` | Disable analytics |
| `DO_NOT_TRACK` | Disable analytics |
| `SAPPHIRE_NON_INTERACTIVE` | Non-interactive mode flag |
| `OPENAI_API_KEY` | OpenAI API key |
| `ANTHROPIC_API_KEY` | Anthropic API key |
| `GEMINI_API_KEY` | Google API key |
| `AZURE_API_KEY` | Azure API key |
| `AWS_*` | AWS credentials |

---

## Dependencies

### Core

- `charm.land/bubbletea/v2` - TUI framework
- `charm.land/bubbles/v2` - TUI components
- `charm.land/fantasy` - Agent framework
- `charm.land/catwalk` - Provider catalog
- `charm.land/lipgloss/v2` - Styling
- `charm.land/glamour/v2` - Markdown rendering

### Database

- `modernc.org/sqlite` - SQLite driver
- `github.com/ncruces/go-sqlite3` - Alternative SQLite
- `github.com/pressly/goose/v3` - Migrations

### LLM Providers

- `github.com/openai/openai-go/v3` - OpenAI
- `github.com/charmbracelet/anthropic-sdk-go` - Anthropic
- `google.golang.org/genai` - Google Gemini
- `github.com/aws/aws-sdk-go-v2` - AWS Bedrock

### MCP

- `github.com/modelcontextprotocol/go-sdk` - MCP SDK

### LSP

- `github.com/sourcegraph/jsonrpc2` - JSON-RPC
- `github.com/charmbracelet/x/ansi` - ANSI handling

### Shell

- `mvdan.cc/sh/v3` - POSIX shell
- `mvdan.cc/sh/moreinterp` - Shell interpreter

### Utilities

- `github.com/spf13/cobra` - CLI framework
- `github.com/charmbracelet/fang` - Command wrapper
- `github.com/posthog/posthog-go` - Analytics
- `github.com/google/uuid` - UUID generation
- `gopkg.in/yaml.v3` - YAML parsing
- `github.com/bmatcuk/doublestar/v4` - Glob patterns

---

## Build and Release

### Build

```bash
go build -o sapphire .
```

### Release

**Tool:** GoReleaser

**Config:** `.goreleaser.yml`

**Platforms:**
- darwin/amd64
- darwin/arm64
- linux/amd64
- linux/arm64
- windows/amd64

### Linting

**Tool:** golangci-lint

**Config:** `.golangci.yml`

```bash
golangci-lint run
```

---

## Directory Conventions

- `/internal/` - Private application code
- `/long_horizon/` - Long-horizon artifacts
- `/.sapphire/` - Data directory (gitignored)
- `/worktrees/` - Sub-agent worktrees (gitignored)
- `/crush-repo/` - Reference Crush CLI code (for alignment)

---

## Unimplemented / Incomplete Features

1. **Multi-agent coordination** - Multiple main agents not yet implemented
2. **Guardian review** - Approval review sub-agent not implemented
3. **Memory consolidation** - Phase-2 memory consolidation not implemented
4. **Role-based config** - Role configuration system referenced but not fully implemented
5. **Thread forking** - Context forking partially implemented

---

## Notes

- YOLO mode (auto-approve) is enabled by default
- Non-interactive mode bypasses MCP init wait (1s timeout)
- Progress bars shown only in supported terminals
- Transparent mode auto-enabled for Apple Terminal
- Git repository detection limits file walk operations
- Database uses WAL mode for concurrency
- LSP clients start lazily on first file open
- MCP clients start lazily on first use

---

**End of agent.md**
