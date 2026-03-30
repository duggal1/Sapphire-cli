# Sapphire CLI

Sapphire CLI is a terminal-first autonomous agent system for serious software development. It combines graph-based code understanding, durable memory, lifecycle sub-agents, worktree orchestration, MCP and LSP integration, and mistake-aware self-healing in one runtime.

Sapphire is built for extreme longer-horizon engineering work. It indexes the repository structurally, compiles task-specific boot packets from real symbols and edges, supervises active agents, carries forward durable memory across sessions, and can trigger a recursive mistake-recovery loop when a code-changing run fails repeatedly.

## Install

### Homebrew

The shipped binary name is `sapphire`.

Fresh install:

```bash
brew install duggal1/sapphire-cli/sapphire
```

Then launch it with:

```bash
sapphire
```

If you want to tap first:

```bash
brew tap duggal1/sapphire-cli
brew install sapphire
```

To update later:

```bash
brew update
brew upgrade sapphire
```

Note: plain `brew install sapphire` on a completely clean machine only works after `duggal1/sapphire-cli` has already been tapped, or if Sapphire is ever accepted into `homebrew/core`.

### Build From Source

```bash
go build -o sapphire .
./sapphire
```

## Getting Started

Install Sapphire, open your project directory, and run `sapphire`. On first use, Sapphire can prompt you to configure a provider key.

You can also set provider credentials up front with environment variables or store them directly in Sapphire with `sapphire api-key`.

### Provider Environment Variables

| Environment Variable        | Provider                                           |
| --------------------------- | -------------------------------------------------- |
| `ANTHROPIC_API_KEY`         | Anthropic                                          |
| `OPENAI_API_KEY`            | OpenAI                                             |
| `VERCEL_API_KEY`            | Vercel AI Gateway                                  |
| `GEMINI_API_KEY`            | Google Gemini                                      |
| `SYNTHETIC_API_KEY`         | Synthetic                                          |
| `ZAI_API_KEY`               | Z.ai                                               |
| `MINIMAX_API_KEY`           | MiniMax                                            |
| `HF_TOKEN`                  | Hugging Face Inference                             |
| `CEREBRAS_API_KEY`          | Cerebras                                           |
| `OPENROUTER_API_KEY`        | OpenRouter                                         |
| `IONET_API_KEY`             | io.net                                             |
| `GROQ_API_KEY`              | Groq                                               |
| `VERTEXAI_PROJECT`          | Google Cloud VertexAI (Gemini)                     |
| `VERTEXAI_LOCATION`         | Google Cloud VertexAI (Gemini)                     |
| `AWS_ACCESS_KEY_ID`         | Amazon Bedrock (Claude)                            |
| `AWS_SECRET_ACCESS_KEY`     | Amazon Bedrock (Claude)                            |
| `AWS_REGION`                | Amazon Bedrock (Claude)                            |
| `AWS_PROFILE`               | Amazon Bedrock (Custom Profile)                    |
| `AWS_BEARER_TOKEN_BEDROCK`  | Amazon Bedrock                                     |
| `AZURE_OPENAI_API_ENDPOINT` | Azure OpenAI models                                |
| `AZURE_OPENAI_API_KEY`      | Azure OpenAI models (optional when using Entra ID) |
| `AZURE_OPENAI_API_VERSION`  | Azure OpenAI models                                |

### Optional Key Storage Commands

Store a provider key in Sapphire:

```bash
sapphire api-key openrouter sk-or-123456
sapphire api-key anthropic sk-ant-123456
```

Store a Sapphire skills key:

```bash
sapphire api-key sapphire sapp_user_xxx
```

### First Run

Interactive mode:

```bash
cd /path/to/project
sapphire
```

Non-interactive mode:

```bash
sapphire run "index the codebase and explain the architecture"
```

Pipe content in:

```bash
cat README.md | sapphire run "rewrite this more clearly"
```

## Core Capabilities

- Graph-based code understanding for Go using AST parsing, symbol extraction, and repository edges such as `imports`, `calls`, `defines`, and `test_covers`.
- Boot-packet context injection that turns indexed code structure, runtime state, and required reads into task-specific system context.
- Durable memory with SQLite-backed retrieval, staged context injection, checkpoints, and persistent project history across sessions.
- Mistake-aware recursive self-healing that can detect repeated hard failures after mutation, require the mistake protocol, update the failure register, persist the lesson, and continue.
- Lifecycle sub-agent orchestration with explicit spawn, resume, wait, collect, close, and supervisor-driven monitoring.
- Git worktree isolation for parallel execution when tasks need branch-level separation and validation gates.
- MCP integration with registry-backed discovery, installation, connection, and tool execution.
- LSP diagnostics and semantic references for compiler-aware and symbol-aware code work.
- Interactive TUI, non-interactive CLI execution, and multiple collaboration modes including `default`, `plan`, `architect`, `debug`, `security`, `review`, and `orchestrator`.

## Core Architecture

Sapphire is organized in clear layers:

- UI layer: Bubble Tea TUI, dialogs, progress views, and terminal rendering.
- Application layer: app container, session management, and message wiring.
- Agent layer: coordinator, session agent, sub-agent manager, supervisor, memory compiler, long-horizon services, and mailbox.
- Tools layer: file operations, search, shell jobs, MCP tools, memory tools, LSP tools, and agent coordination tools.
- Persistence layer: SQLite orchestration state, graph index state, memory state, and tracked worktrees.

At the center of the runtime are:

- `internal/agent/coordinator.go` for orchestration
- `internal/agent/agent.go` for the session agent loop
- `internal/agent/subagent_manager.go` for sub-agent lifecycle
- `internal/agent/memory/indexer.go` and `internal/agent/memory/compiler.go` for graph indexing and boot packet compilation
- `internal/memory/` for durable memory
- `internal/agent/supervisor/` for patrol and recovery

## Tools and Runtime Model

Sapphire exposes a structured tool surface instead of forcing everything through shell commands.

Key tool groups:

- File tools: `single_view`, `agentic_view`, `single_edit`, `agentic_edit`, `write`, `apply_patch`, `ls`, `glob`, `grep`
- Agent tools: `spawn_agent`, `resume_agent`, `send_input`, `wait`, `collect_result`, `close_agent`
- Memory tools: `view_memory`, `recall_memory`, `save_memory`, `refresh_memory`
- Web and retrieval: `agentic_fetch`, `web_search`, `web_fetch`, `google_search`, `fetch`
- LSP and diagnostics: `lsp_diagnostics`, `lsp_references`, `lsp_restart`
- MCP: `list_available_mcps`, `install_mcp`, `connect_mcp`, `call_mcp_tool`, `list_mcp_tools`, `list_mcp_resources`, `read_mcp_resource`

`bash` still exists, but it is intentionally a fallback tool for build/test/process work and shell-native operations that structured tools do not cover well.

## Common Commands

```bash
# Start the interactive TUI
sapphire

# Run a single prompt and exit
sapphire run "summarize this repository"

# List configured and available models
sapphire models

# Browse and install extended skills
sapphire skills

# Sync MCP registry data
sapphire mcp sync

# Show or set the sub-agent cap
sapphire sub-agents
sapphire sub-agents 12

# Print Sapphire directories
sapphire dirs
sapphire dirs config
sapphire dirs data

# View logs
sapphire logs -f

# Show usage statistics
sapphire stats

# Worktree orchestration from a spec file
sapphire worktrees orchestrate -s spec.json
```

## Extended Skills

Sapphire includes a local and extended skills system.

- `sapphire skills` opens the terminal skills browser.
- Set `SAPPHIRE_API_KEY` or store a Sapphire key with `sapphire api-key sapphire ...` to browse and install extended skills.
- Installed skills are written into Sapphire’s local data directory and become available as local skills on later runs.

## Worktrees and Sub-Agents

Sapphire has two parallel execution models:

- Lifecycle sub-agents for direct coordination inside the main session.
- Worktree orchestration for isolated git-based parallel execution.

Useful worktree commands:

```bash
sapphire worktrees list
sapphire worktrees clean --merged
```

Useful sub-agent configuration:

```bash
sapphire sub-agents 20
```

## Repository Documentation

- [AGENTS.md](./AGENTS.md): repository operating instructions and architectural guidance for agents

## Maintainer Release Flow

Tagged releases are published through GoReleaser and GitHub Actions.

- `.github/workflows/release.yml` builds release artifacts and updates the Homebrew tap
- `.github/workflows/snapshot.yml` validates the release pipeline on `main` and PRs
- `.goreleaser.yml` builds the `sapphire` binary, archives, checksums, completions, manpages, and the Homebrew formula

Maintainer checklist:

```bash
task build
task test
task release
```

The Homebrew formula is published into `duggal1/homebrew-sapphire-cli`.
