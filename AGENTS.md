# Crush Development Guide

## Essential Commands

### Build & Run
- **Build**: `task build` (or `go build -v .`) - Creates the `sapphire` binary with version-injected ldflags.
- **Run**: `task run` - Builds and runs the application.
- **Install**: `task install` - Builds and installs the binary to `GOBIN`.
- **Development**: `task dev` - Runs `go run .` with `CRUSH_PROFILE: true`.

### Test & Debug
- **Run Tests**: `task test` - Runs `go test -race -failfast ./...`.
  - Single test: `go test ./internal/llm/prompt -run TestGetContextFromPaths`
- **Update Golden Files**: `go test ./... -update` - Regenerates `.golden` files.
- **Record VCR Tests**: `task test:record` - Records new VCR cassettes for agent tests.
- **Profiling**: `task profile:cpu`, `task profile:heap`, `task profile:allocs` - Triggers `go tool pprof` via local endpoints.

### Lint & Format
- **Lint**: `task lint` - Runs `golangci-lint` and custom capitalization checks.
- **Fix Lint**: `task lint:fix` - Runs `golangci-lint` with auto-fix.
- **Format**: `task fmt` - Runs `gofumpt -w .`.
- **Modernize**: `task modernize` - Applies code simplifications using `go/analysis/passes/modernize`.
- **Schema**: `task schema` - Generates `schema.json` for configuration.

## Project Architecture

This is a CLI-based AI coding assistant built on a modular architecture.

- **Core Orchestration** (`internal/agent/`): Manages conversation loops, tool execution, and hierarchical multi-agent delegation.
- **Agent Tools** (`internal/agent/tools/`): Implementation of all agent capabilities (bash, edit, view, mcp, etc.).
- **TUI (UI)** (`internal/ui/`): Bubbletea-based terminal interface using `Ultraviolet` for coordinate-based drawing.
- **Storage** (`internal/db/`): SQLite database with WAL and triggers, managed via `SQLC` and `goose` migrations.
- **LSP** (`internal/lsp/`): Language Server Protocol integration for semantic code intelligence.
- **Communication** (`internal/pubsub/`): Internal event broker for reactive UI updates and backend synchronization.
- **Process Management** (`internal/shell/`): Background shell execution with ringbuffer-based observability and absolute addressing.

## Code Conventions

- **Formatting**: Strictly follow `gofumpt`.
- **Error Handling**: Wrap errors with `fmt.Errorf("context: %w", err)`.
- **Concurrency**: Use the `csync` package (`internal/csync/`) for thread-safe collections and values.
- **Context**: Always pass `context.Context` as the first parameter.
- **Naming**: Standard Go PascalCase for exported, camelCase for unexported symbols.
- **Logging**: Messages must start with a Capital letter (enforced by custom lint).
- **Comments**: End with a period. Wrap at 78 columns.
- **FileSystem**: Use `fsext` for traversals to respect ownership boundaries and ignore patterns (global, home, and `.gitignore`).

## Configuration & Environment

- **Resolution Hierarchy**: Merges Global Defaults -> Global Config (`~/.config/sapphire/`) -> Project Config (`sapphire.json`) -> Environment (`CRUSH_*`).
- **Variable Expansion**: Supports shell-like substitution (`$VAR`, `$(command)`) via `VariableResolver`.
- **Environment**: Decoupled via an `Env` interface; specialized `CRUSH_` variables are mapped to standard names (e.g., `CRUSH_OPENAI_API_KEY` -> `OPENAI_API_KEY`) during initialization.

## Testing Approach

- **Framework**: Use `testify/require` for assertions.
- **Parallelism**: Enable `t.Parallel()` and `t.SetEnv()` where applicable.
- **VCR**: Agent tests use `charm.land/x/vcr` to record/replay API interactions.
- **Mocks**: Enable `config.UseMockProviders = true` to isolate tests from real APIs.

## Working on the TUI

Before working on the UI, read `internal/ui/AGENTS.md`.

- **Architecture**: Composite model pattern where a root `UI` model delegates to specialized sub-components.
- **Rendering**: Uses `uv.Screen` and `uv.Rectangle` for coordinate-based drawing instead of simple string concatenation.
- **Non-Blocking**: Never do IO or heavy computation in `Update`; always use `tea.Cmd`.
- **Events**: Subscribe to `pubsub.Event` types for reactive state synchronization.

## Agent Orchestration (Coordinator)

- **Autonomous Delegation**: The `Coordinator` automatically spawns parallel sub-agents (mapping, risk review) for complex tasks.
- **Recursive Logic**: Agents can invoke other agents via the `agent` or `agentic_fetch` tools.
- **Protocol Enforcement**: Strict **System Reminders** force agents to use the `todos` tool for planning before action.
- **Caching**: Anthropic-compatible models use `ephemeral` cache control headers on system prompts and recent history.

## Common Gotchas

- **SQLC Generation**: Do not edit `internal/db/*.sql.go` directly. Edit `.sql` files in `internal/db/sql/` and run generation.
- **SQLite Pragma**: Uses WAL mode and busy timeouts (30s). Transactions are supported via `WithTx`.
- **LSP Lifecycle**: Servers are lazy-loaded based on file types or root markers (`go.mod`, etc.). Use `lsp_restart` if stale.
- **Process Cap**: Background shells are capped at 50 concurrent jobs with auto-purging after 8 hours.
- **YOLO Mode**: Permission requests are auto-approved unless restricted in config.

## Key Architectural Decisions and Patterns

- **Modular Monolith**: The project is a single Go application with distinct internal packages for different functional areas.
- **Agent-Native Design**: Core `internal/agent` package with a `Coordinator` managing `SessionAgent` instances, enabling hierarchical and recursive agent calls.
- **Reactive TUI**: Built on Charm's Bubble Tea framework and `Ultraviolet` for sophisticated terminal UI layouts.
- **SQLite Persistence**: `internal/db` uses SQLite for all session, message, and file history data. Migrations are managed with `goose`, and `sqlc` generates type-safe Go code from SQL files.
- **LSP Server Interaction**: Utilizes `github.com/charmbracelet/x/powernap`. A `Manager` maintains lazily initialized `Client` instances for `jsonrpc2` communication.
- **Diagnostics**: Incoming `textDocument/publishDiagnostics` are stored in a `csync.VersionedMap` with a thread-safe `Mutex`-protected cache to notify external components (`onDiagnosticsChanged`) of updates.
- **Lifecycle**: `Manager` handles lazy initialization based on file types. `Client` instances handle `Restart` (with timeout/kill fallback) and `StopAll`/`KillAll` methods.
- **Event-Driven Communication**: `internal/pubsub` implements a generic publish/subscribe broker for asynchronous communication; TUI components react to `pubsub.Event` types (e.g., session updates, message creation) for decoupled state synchronization.
- **Background Shell Management**: Orchestrated via singleton `BackgroundShellManager` using `mvdan.cc/sh/v3` for POSIX-compliant shell emulation, with output captured in circular buffers.
- **SQLite Database Access**: Employs `sqlc`-generated type-safe wrappers with specific SQLite pragmas (`WAL`, `foreign_keys`, `secure_delete`). Migrations via `goose`.
- **Filesystem Traversals**: Optimized via `fastwalk`, respecting `.gitignore` and `.crushignore` with O(1) lookup of ignore patterns.
- **Context Window Management**: Automatic summarization is triggered in `internal/agent/agent.go` to keep conversations within model limits.
- **Embedding**: Generates 768-dimensional vectors using `gemini-embedding-001`.
- **Retrieval**: Uses **cosine similarity** to identify relevant skills against a `DefaultSimilarityThreshold` of 0.45, injecting instructions into agent context.
- **Dependency Injection**: Services are typically constructed with their dependencies.
- **Multi-Tier Configuration**: Settings are loaded from Global Defaults -> Project Configs (recursive search) -> Environment. 
  - **Resolution**: `VariableResolver` supports shell-like substitution (`$VAR`, `$(command)`) for dynamic values.
  - **Environment**: Abstracted via `env.Env` interface; `CRUSH_`-prefixed variables are temporarily applied to the process environment and restored to maintain isolation.
- **System Reminders**: Agents are strictly guided by system prompts (`TODO PROTOCOL`, `COMPLEXITY DETECTED`, `SUB-AGENT`) to enforce tool usage and orchestration patterns.
- **Singleton Pattern**: `BackgroundShellManager` is a singleton.
- **Cached Rendering**: UI components use `cachedMessageItem` to avoid redundant Lipgloss rendering.
- **Concurrency**: Extensive use of `csync` package for thread-safe collections and values.

## Libraries and Frameworks

- **Go 1.26.0**: Core language version.
- `charm.land/bubbletea/v2`: TUI framework.
- `charm.land/lipgloss/v2`: Styling for TUI.
- `github.com/charmbracelet/ultraviolet`: Custom TUI rendering engine.
- `charm.land/fantasy`: AI agent framework.
- `github.com/pressly/goose/v3`: Database migration tool.
- `github.com/ncruces/go-sqlite3`, `modernc.org/sqlite`: SQLite drivers.
- `github.com/google/uuid`: UUID generation.
- `github.com/bmatcuk/doublestar/v4`: Glob pattern matching.
- `github.com/charlievieth/fastwalk`: Optimized directory traversal.
- `github.com/charmbracelet/x/powernap`: LSP client implementation.
- `golang.org/x/sync/errgroup`: For concurrent task execution.
- `github.com/golangci/golangci-lint`: Linting.
- `github.com/spf13/cobra`: CLI command structure.
- `gopkg.in/natefinch/lumberjack.v2`: Log rotation.
- `github.com/stretchr/testify`: Testing assertions.

## Environment Details

- **Language**: Go 1.26.0
- **OS**: Detection of `runtime.GOOS` for platform-specific behaviors.
- **Dependencies**: Extensive list in `go.mod`, including Charm libraries, AI providers, and utilities.
- **Environment Variables**: `CRUSH_PROFILE`, `CRUSH_GLOBAL_DATA`, `CRUSH_GLOBAL_CONFIG`, `CRUSH_DISABLE_ANTHROPIC_CACHE`, `CRUSH_DISABLE_PROVIDER_AUTO_UPDATE`, `CRUSH_DISABLE_DEFAULT_PROVIDERS`, `CRUSH_DISABLE_METRICS`, `TERM_PROGRAM`, `WT_SESSION`, XDG variables for config paths.
