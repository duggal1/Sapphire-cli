# Codex Codebase Architecture Map

## 1. Codebase Overview
Codex is a highly autonomous engineering agent orchestration platform built in Rust. It utilizes a multi-mode architecture (Plan, Pair, Execute) and leverages Git worktrees for parallel, isolated task execution. The system features a sophisticated memory pipeline (Phase 1/2) and a robust tool-calling infrastructure.

### Main Architectural Layers
- **Core Orchestration**: `agent/control.rs` and `agent/role.rs`. Manages the lifecycle of sub-agents and role-based prompt composition.
- **Tool Infrastructure**: `tools/handlers/` and `tools/spec.rs`. Defines how the agent interacts with the filesystem, shell, and other agents.
- **Persistence & State**: `state_db.rs` and `rollout/recorder.rs`. Tracks session history, task status, and rollout events.
- **Worktree Management**: `git_info.rs` and shell utilities. Ensures every task runs in a clean, isolated environment.

## 2. Directory Map
- `core/src/agent/`: Core lifecycle management (spawning, status, guards).
- `core/src/tools/handlers/`: Implementation of specific tools (bash, multi-agent coordination).
- `core/src/rollout/`: Recording and reading of session events (rollouts).
- `core/templates/`: Markdown-based prompt templates for roles and capabilities.

## 3. File-by-File Architecture Map

### [control.rs](file:///Users/harshitduggal/Desktop/sapphire-cli/codex/codex-rs/core/src/agent/control.rs)
- **Responsibility**: Central control plane for agent threads.
- **Key Types**: `AgentControl`, `SpawnAgentOptions`.
- **Why it matters**: Handles spawning, resuming, and interrupting agents. Supports "forked" history for sub-agents.

### [multi_agents.rs](file:///Users/harshitduggal/Desktop/sapphire-cli/codex/codex-rs/core/src/tools/handlers/multi_agents.rs)
- **Responsibility**: Outer tool handler for `spawn_agent`, `wait`, `send_input`, etc.
- **Key Functions**: `spawn_agent()`, `wait_for_agents()`.
- **Why it matters**: Bridge between the LLM's tool calls and the internal `AgentControl` logic.

### [spawn.rs](file:///Users/harshitduggal/Desktop/sapphire-cli/codex/codex-rs/core/src/tools/handlers/multi_agents/spawn.rs)
- **Responsibility**: Logic for creating a new sub-agent instance.
- **Key Types**: `SpawnAgentArgs`, `Handler`.
- **Why it matters**: Injects the parent's context and handles the forking logic.

### [shell_snapshot.rs](file:///Users/harshitduggal/Desktop/sapphire-cli/codex/codex-rs/core/src/shell_snapshot.rs)
- **Responsibility**: Captures Git and shell state at a specific point in time.
- **Why it matters**: Critical for reproducing a state across different agents or session resumes.

## 4. Execution Flow
1. **Entry**: CLI command starts the primary agent loop.
2. **Setup**: Config is loaded, and a `ShellSnapshot` of the repo is taken.
3. **Execution**: The LLM emits tool calls (e.g., `spawn_agent`).
4. **Orchestration**: `AgentControl` reserves a slot, creates a new thread, and initializes it with a `SubAgentSource`.
5. **Tool Calling**: Sub-agents use `bash` or `single_edit` in their local worktrees.
6. **Validation**: Parent agent (or gate logic) verifies the sub-agent's diff/build before merge.
7. **Cleanup**: Finished worktrees are cleaned up or quarantined on failure.

## 5. Fast Navigation Section
1. `core/src/agent/control.rs`: read for spawning/lifecycle logic.
2. `core/src/tools/handlers/multi_agents/spawn.rs`: read for sub-agent init.
3. `core/src/agent/role.rs`: read for prompt composition logic.
4. `core/templates/agents/orchestrator.md`: read for parent-agent coordination rules.
5. `core/templates/memories/consolidation.md`: read for memory pipeline logic.
