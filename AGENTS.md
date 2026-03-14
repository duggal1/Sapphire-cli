# Sapphire Agent Guidelines

Sapphire is an autonomous software engineering CLI tool built in Go, designed to orchestrate complex technical tasks through AI agents.

## Repository Complexity
- **Scale**: Large (>200 files, multi-language/workspace).
- **Architecture**: Domain-driven Go monolith with sub-agent workspaces (`long_horizon/`) and cross-language tool integration via the `codex/` Rust workspace.

## Core Architecture
- **Engine (`internal/agent/`)**: Core session management, sub-agent coordination, and MCP orchestration.
- **Persistence (`internal/db/`)**: SQLite storage utilizing `sqlc` for typed database queries.
- **UI (`internal/ui/`)**: Terminal user interface built with `charmbracelet` libraries (Bubble Tea, Lipgloss).
- **Codex (`codex/`)**: Cross-language workspace containing the Rust-based application server and TS/Python SDKs for agent integration.
- **Skills (`internal/skills/`)**: Extensible agent capabilities (YAML/MD driven).

## Testing & Infrastructure
- **Provider Tests (`third_party/fantasy/`)**: Critical framework for testing LLM interactions across multiple model providers. Use this for verifying agent behavior shifts.
- **Standard Testing**: Go unit tests utilizing `testify`.

## Agent Lifecycle & Coordination
1. **Planning**: Complex requests trigger long-horizon planning via the `long_horizon/` state-tracker.
2. **Execution**: The `Coordinator` manages agent state and tool invocation.
3. **Tracking**: Multi-step workflows require the `todos` CLI tool.
4. **Resilience**: Agents recover from tool failures (e.g., Python retries) and manage context via automated summarization.

## Operational Conventions
- **Tooling**: Prefer `internal/agent/tools` over manual shell operations.
- **Sub-agents**: Autonomous sub-agent priming is triggered by specific keywords in user prompts (e.g., "across the codebase").
- **Concurrency**: Parallel sub-agents can conflict on shared SQLite databases or filesystem paths; always verify state lock.

## Development & Operations
- **Build/Test**: Use `Taskfile.yaml`.
- **Environment**: Configuration requires LLM API keys and MCP server connectivity.
- **Style**: Strict Go idioms. Maintain `internal/` package encapsulation.
