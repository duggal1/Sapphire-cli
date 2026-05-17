# Sapphire Agentics CLI

Sapphire CLI is a terminal-first autonomous agent runtime for serious agentic engineering.

It is built for long-horizon engineering tasks on large codebases: deep codebase scanning, indexed code search, planning, editing, testing, persistent memory, sub-agent coordination, worktree orchestration, and machine-readable runtime telemetry from the terminal.

## Summary

Sapphire includes:

- Local AI CLI execution.
- Codebase-aware agent runtime.
- Structured tool execution.
- Deep repository scanning.
- Indexed agent search.
- Persistent memory.
- Durable codebase graph.
- Sub-agent coordination.
- Worktree orchestration.
- Machine-facing terminal interface.
- Interactive terminal interface.
- Long-horizon task execution.
- Million-line codebase orientation.

## Positioning

Sapphire is positioned alongside modern agentic coding systems such as Claude Code and Codex.

The focus is the local runtime layer:

- Repository reading.
- Repository search.
- Codebase indexing.
- Codebase-scale scan and retrieval.
- Persistent memory.
- Failure recovery.
- Sub-agent coordination.
- Task complexity routing.
- Durable context management.

## Long-Horizon Agentics

Sapphire is built for tasks that do not fit in one prompt, one file, one tool call, or one model context window.

The runtime is designed around long-horizon engineering:

- Multi-turn task continuity.
- Persistent project memory.
- Resume points after compaction.
- Codebase graph reuse across turns.
- Sub-agent delegation.
- Worktree isolation.
- Task hardness routing.
- Recovery after repeated failures.
- Runtime telemetry for headless agent execution.

This is the core difference: Sapphire is not only a coding assistant. It is an agent runtime that keeps state, coordinates workers, searches the repo, remembers durable findings, and continues after context pressure.

## Million-Line Codebase Orientation

Sapphire is designed for repositories where the codebase is too large to read directly.

The target environment is large engineering codebases: hundreds of thousands of lines, millions of lines, many directories, many subsystems, and long-lived project state.

The implementation does not try to paste a repository into the model. It builds a retrieval layer around the repository:

- Discover supported source and text files.
- Hash files and reuse unchanged index state.
- Split files into searchable chunks.
- Parse Go declarations into symbol-oriented chunks where possible.
- Embed code chunks.
- Store vector metadata in the local index.
- Search the durable graph before reading.
- Use `agentic_view` only after locating relevant regions.
- Launch semantic survey sub-agents after indexing.
- Persist codebase findings into memory and survey artifacts.

This makes Sapphire suitable for very large codebases in the way agentic systems actually need: scan first, search second, read selectively, act with memory.

## Large-Codebase Agentics

Sapphire Agentics is designed for large repositories where manual browsing and unindexed text search are insufficient.

The runtime can scan supported source files, split code into searchable chunks, embed those chunks, persist the index, and route future agent searches through the durable codebase graph.

This is the main capability:

- Build a durable map of the repository.
- Search for unknown files, symbols, integrations, and subsystems.
- Return ranked navigation candidates instead of dumping broad context.
- Move from search to `agentic_view` for the relevant implementation files.
- Preserve codebase knowledge for later turns and sessions.
- Launch semantic survey sub-agents after indexing.
- Produce long-horizon codebase artifacts through the semantic survey.

The implementation is built for large codebases with many files and many subsystems. Current indexing is file-based and chunk-based: supported text files are discovered, hashed, chunked, embedded, and stored. Binary files and very large individual files are excluded from the code index path; this keeps the index useful for real code repositories instead of wasting the runtime on irrelevant blobs.

The practical result: Agentics can scan and search a large codebase before it edits.

## Scale Model

Sapphire does not rely on one prompt containing the repository.

The scale model is:

- Discover files concurrently.
- Skip ignored and unsupported directories.
- Skip binary content.
- Hash file contents.
- Reuse unchanged indexed files.
- Split large files into chunks.
- Parse Go declarations into symbol-oriented chunks where possible.
- Embed chunks with Jina code embeddings.
- Store vectors and metadata in the local code index.
- Search the durable index before reading.
- Use sub-agents for semantic repository survey.
- Compile boot packets from durable memory and codebase state.

This is the reason Sapphire can target very large engineering work. It is not asking the model to read an entire monorepo in one context window.

## Install

### Homebrew

```bash
brew install duggal1/sapphire-cli/sapphire
```

### Tap First

```bash
brew tap duggal1/sapphire-cli
brew install sapphire
```

### Update

```bash
brew update
brew upgrade sapphire
```

### Build From Source

```bash
go build -o sapphire .
./sapphire
```

## First Run

```bash
cd /path/to/project
sapphire
```

## Non-Interactive Run

```bash
sapphire run "index the codebase and explain the architecture"
```

## Pipe Input

```bash
cat README.md | sapphire run "rewrite this more clearly"
```

## Machine-Facing Agent Run

```bash
sapphire agent run "review this repository"
```

## Machine-Facing Inspect

```bash
sapphire agent inspect
```

## Binary

The shipped binary is:

```bash
sapphire
```

## Provider Environment

| Variable | Provider |
| --- | --- |
| `ANTHROPIC_API_KEY` | Anthropic |
| `OPENAI_API_KEY` | OpenAI |
| `VERCEL_API_KEY` | Vercel AI Gateway |
| `GEMINI_API_KEY` | Google Gemini |
| `SYNTHETIC_API_KEY` | Synthetic |
| `ZAI_API_KEY` | Z.ai |
| `MINIMAX_API_KEY` | MiniMax |
| `HF_TOKEN` | Hugging Face Inference |
| `CEREBRAS_API_KEY` | Cerebras |
| `OPENROUTER_API_KEY` | OpenRouter |
| `IONET_API_KEY` | io.net |
| `GROQ_API_KEY` | Groq |
| `VERTEXAI_PROJECT` | Google Cloud Vertex AI |
| `VERTEXAI_LOCATION` | Google Cloud Vertex AI |
| `AWS_ACCESS_KEY_ID` | Amazon Bedrock |
| `AWS_SECRET_ACCESS_KEY` | Amazon Bedrock |
| `AWS_REGION` | Amazon Bedrock |
| `AWS_PROFILE` | Amazon Bedrock profile |
| `AWS_BEARER_TOKEN_BEDROCK` | Amazon Bedrock bearer token |
| `AZURE_OPENAI_API_ENDPOINT` | Azure OpenAI |
| `AZURE_OPENAI_API_KEY` | Azure OpenAI |
| `AZURE_OPENAI_API_VERSION` | Azure OpenAI |

## Store API Keys

```bash
sapphire api-key openrouter sk-or-...
sapphire api-key anthropic sk-ant-...
sapphire api-key sapphire sapp_user_...
```

## Core Capabilities

- Autonomous terminal execution.
- Interactive TUI.
- Non-interactive execution.
- Machine-readable agent mode.
- Runtime telemetry.
- Model override.
- Reasoning effort override.
- Provider abstraction.
- Structured tool calling.
- File reading.
- File editing.
- Patch application.
- Shell execution.
- Background jobs.
- Job output polling.
- Job killing.
- Codebase indexing.
- Deep codebase scan.
- Durable codebase graph.
- Semantic code search.
- Tool search.
- Agent scan.
- Agent search.
- Durable memory.
- Persistent memory.
- Triple memory.
- Triple-triple memory.
- Memory health checks.
- Memory recall.
- Memory save.
- Memory refresh.
- Resume points.
- Context compaction recovery.
- Long-horizon state.
- Mistake-aware recovery.
- Doom-loop detection.
- Completion guardrails.
- Structured task plans.
- Todo reconciliation.
- Sub-agent spawning.
- Sub-agent resume.
- Sub-agent messaging.
- Sub-agent waiting.
- Sub-agent result collection.
- Sub-agent closing.
- Worktree orchestration.
- Parallel execution.
- Supervisor patrol.
- Agent mailbox.
- Agent nudging.
- Agent hooks.
- MCP discovery.
- MCP install.
- MCP connection.
- MCP tool calls.
- MCP resources.
- LSP diagnostics.
- LSP references.
- LSP restart.
- Skills loading.
- Extended skills.
- Read-only exploration.
- Web search.
- Web fetch.
- Sourcegraph search.
- Google search.
- Structured downloads.
- Python tool execution.
- Image attachment support.
- Terminal stats.
- Logs.
- Project registry.
- Directory inspection.

## New Agent Features

### Agent Scan

Agent scan is Sapphire's repo-awareness foundation.

It is the capability that lets Agentics inspect a large repository before acting. It is the durable scan path that turns a codebase into indexed files, chunks, embeddings, metadata, and semantic survey artifacts.

It covers:

- Repository discovery.
- File discovery.
- Changed-file detection.
- Content hashing.
- Chunk preparation.
- Symbol capture.
- Go declaration parsing.
- Text chunking.
- Embedding batches.
- Vector index writes.
- Incremental re-indexing.
- Semantic indexing.
- Mandatory AI codebase survey.
- Multi-agent semantic survey.
- Manifest output.
- Overview output.
- Graph warmup.
- Durable state refresh.

The concrete tool is:

```text
index_codebase
```

The core implementation lives in:

```text
internal/agent/index_codebase_tool.go
internal/codeindex/service.go
internal/codeindex/types.go
internal/agent/memory/compiler.go
```

Agent scan is the basis for large-codebase work. It gives the runtime a durable repository map before the model starts making implementation decisions.

### Agent Search

Agent search is Sapphire's bounded repo locator.

Agentics search is supported through the indexed codebase graph, path matches, symbol candidates, and bounded text fallback. The goal is to return the highest-value files and regions for the next read.

It routes search by task shape:

- Unknown file or symbol: `tool_search`.
- Known path shape: `rg_files`.
- Known exact string: `rg`.
- Broad repo reading: `agentic_view`.
- Narrow verified file reading: `single_view`.
- File pattern: `glob`.
- Directory structure: `ls`.
- Size or density: `wc`.
- Line counts: `wc_l`.

The concrete tool is:

```text
tool_search
```

The core implementation lives in:

```text
internal/agent/tool_search_tool.go
internal/agent/tools/tool_search.go
internal/agent/tools/tool_search.md
```

Agent search is intentionally not a broad dump. It is a ranking system for finding the smallest useful set of files, symbols, signatures, snippets, and line ranges before reading or editing.

### Agentic Deep Scan

Agentic deep scan combines:

- `index_codebase` for durable repository scan.
- `tool_search` for indexed locator search.
- `agentic_view` for broad implementation reads.
- `run_harness` for complexity and routing policy.
- Semantic survey sub-agents for repo-level maps.
- Persistent memory for retaining findings.

This is the core Sapphire Agentics loop:

1. Scan the codebase.
2. Search the indexed graph.
3. Read the relevant implementation files.
4. Plan according to task hardness.
5. Edit with structured tools.
6. Validate with commands, diagnostics, or tests.
7. Persist durable knowledge.
8. Resume from memory when context changes.

### Large Repository Behavior

Sapphire is designed for repositories that are too large for a single context window.

The runtime avoids the failure mode where an agent tries to read everything directly. Instead it uses indexed search, chunking, ranked candidates, broad-but-bounded reads, and persistent memory.

Current code index behavior:

- Supported source and text files are indexed.
- Ignored directories are skipped.
- Binary files are skipped.
- Empty files are skipped.
- Individual files larger than the current index threshold are skipped by the code index.
- Changed files are detected by content hash.
- Unchanged files are reused.
- Chunks are embedded and stored for semantic search.

This means the system is aimed at large codebases and large engineering tasks, not unlimited single-file ingestion. The implementation is strongest when a repository is composed of many real source files and modules.

### Hardness Scales

Sapphire classifies work before it acts.

Hardness is expressed through task complexity and policy requirements.

It affects:

- Planning requirement.
- Validation requirement.
- Recovery requirement.
- Harness requirement.
- Parallel-agent preference.
- Repository-reading depth.
- Long-horizon memory preference.
- Tool-routing strictness.

The core implementation lives in:

```text
internal/agent/singularity_cognitive.go
internal/agent/harness_tool.go
internal/agent/subagent_reasoning.go
```

### Triple Memory

Sapphire uses memory as part of the runtime.

Triple memory means:

- Active conversation memory.
- Repo-local durable memory.
- SQLite-backed persistent memory.

### Triple-Triple Memory

Triple-triple memory expands the model into nine durable surfaces:

- Active conversation history.
- Session database.
- Structured summaries.
- `.sapphire-memory/MEMORY.md`.
- `.sapphire-memory/memory_summary.md`.
- Raw rollout memories.
- Rollout summaries.
- Codebase knowledge graph.
- Mistake memory.

### Persistent Memory

Persistent memory survives context loss.

It supports:

- Recall.
- Explicit save.
- Refresh.
- Health reporting.
- Summary generation.
- Rollout extraction.
- Project continuity.
- Mistake prevention.
- Resume boot packets.

The core implementation lives in:

```text
internal/memory/system.go
internal/memory/store.go
internal/memory/tools.go
internal/memory/history.go
internal/agent/memory_pipeline.go
internal/agent/memory/lifecycle.go
```

## Architecture

| Layer | Responsibility |
| --- | --- |
| CLI | Cobra commands and command flags |
| App | Session wiring and lifecycle |
| UI | Bubble Tea terminal interface |
| Agent | Model loop and tool execution |
| Tools | Structured local and external actions |
| Memory | Persistent long-horizon context |
| Code Index | Semantic code graph and search |
| Orchestration | Sub-agents, worktrees, hooks, mail |
| DB | SQLite state and migrations |
| Config | Providers, models, permissions, MCP |

## Primary Runtime Files

| File | Role |
| --- | --- |
| `main.go` | Binary entrypoint |
| `internal/cmd/root.go` | Root CLI command |
| `internal/cmd/run.go` | Non-interactive run command |
| `internal/cmd/agent.go` | Machine-facing agent command |
| `internal/cmd/subagents.go` | Sub-agent limit command |
| `internal/app/app.go` | App container |
| `internal/agent/agent.go` | Session agent loop |
| `internal/agent/coordinator.go` | Runtime coordinator |
| `internal/agent/index_codebase_tool.go` | Codebase indexing tool |
| `internal/agent/tool_search_tool.go` | Indexed tool search |
| `internal/agent/harness_tool.go` | Harness requirement logic |
| `internal/agent/singularity_cognitive.go` | Complexity policy |
| `internal/agent/memory_pipeline.go` | Rollout memory pipeline |
| `internal/memory/system.go` | Persistent memory system |
| `internal/codeindex/service.go` | Code indexing service |
| `internal/codeindex/types.go` | Code index data model |
| `internal/db/sql/tiered_memory.sql` | Tiered memory queries |

## Tool Surface

| Tool | Purpose |
| --- | --- |
| `agentic_view` | Broad repo reading |
| `single_view` | Narrow file read |
| `view` | File display |
| `agentic_edit` | Multi-line or multi-file edit |
| `single_edit` | Trivial one-file edit |
| `apply_patch` | Unified patch application |
| `write` | File creation or overwrite |
| `tool_search` | Bounded repo locator |
| `rg_files` | Path-shape discovery |
| `rg` | Exact content search |
| `grep` | Content search |
| `glob` | Pattern file search |
| `ls` | Directory structure |
| `wc` | Size and density |
| `wc_l` | Line counts |
| `index_codebase` | Durable code graph warmup |
| `update_plan` | Live task planning |
| `run_harness` | Complexity and route contract |
| `bash` | Build, test, and process work |
| `job_output` | Background job polling |
| `job_kill` | Background job stop |
| `python` | Python execution |
| `web_search` | Current web research |
| `web_fetch` | Fetch web content |
| `agentic_fetch` | AI-assisted web retrieval |
| `google_search` | Google search |
| `sourcegraph` | Public code search |
| `spawn_agent` | Start sub-agent |
| `resume_agent` | Resume sub-agent |
| `send_input` | Message sub-agent |
| `wait` | Wait for sub-agent |
| `collect_result` | Collect sub-agent output |
| `close_agent` | Close sub-agent |
| `view_memory` | Inspect memory |
| `recall_memory` | Query memory |
| `save_memory` | Persist fact |
| `refresh_memory` | Regenerate memory |
| `memory_health` | Inspect memory state |
| `list_available_mcps` | Discover MCP servers |
| `install_mcp` | Install MCP server |
| `connect_mcp` | Connect MCP server |
| `call_mcp_tool` | Run MCP tool |
| `list_mcp_tools` | List MCP tools |
| `list_mcp_resources` | List MCP resources |
| `read_mcp_resource` | Read MCP resource |
| `lsp_diagnostics` | Compiler diagnostics |
| `lsp_references` | Symbol references |
| `lsp_restart` | Restart LSP |

## Command Surface

```bash
sapphire
sapphire run "prompt"
sapphire agent run "prompt"
sapphire agent inspect
sapphire models
sapphire skills
sapphire api-key <provider> <key>
sapphire mcp sync
sapphire sub-agents
sapphire sub-agents 20
sapphire dirs
sapphire dirs config
sapphire dirs data
sapphire logs -f
sapphire stats
sapphire projects
sapphire schema
sapphire login
sapphire stop
sapphire worktrees list
sapphire worktrees clean --merged
sapphire worktrees orchestrate -s spec.json
```

## Machine-Facing Agent Mode

`sapphire agent` exists for other terminal-only agents.

It emits structured telemetry.

It supports:

- Runtime samples.
- CPU tracking.
- Memory tracking.
- Heap tracking.
- GC tracking.
- Goroutine tracking.
- Selected model metadata.
- Reasoning effort metadata.
- Exact error envelopes.
- Quiet mode.
- Verbose mode.

## Codebase Graph

The codebase graph is durable.

It tracks:

- Files.
- Chunks.
- Embeddings.
- Hashes.
- Languages.
- Symbols.
- Snippets.
- Search text.
- Line ranges.
- Index timestamps.
- Semantic matches.

## Search Routing

| Situation | Preferred Tool |
| --- | --- |
| Unknown file or symbol | `tool_search` |
| Known path shape | `rg_files` |
| Known exact string | `rg` |
| Broad repo read | `agentic_view` |
| One verified trivial file | `single_view` |
| File pattern | `glob` |
| Directory layout | `ls` |
| Size question | `wc` |
| Line count | `wc_l` |

## Memory Model

| Memory Layer | Purpose |
| --- | --- |
| Conversation | Immediate turn context |
| Session DB | Message and tool history |
| Structured summary | Compact durable state |
| Repo memory | `.sapphire-memory` project continuity |
| Raw memory | Unconsolidated extracted facts |
| Rollout summary | Per-run evidence |
| Memory summary | Fast boot summary |
| `MEMORY.md` | Consolidated durable memory |
| Codebase graph | Symbol and file knowledge |
| Mistake memory | Failure prevention |

## Memory Files

| Path | Role |
| --- | --- |
| `.sapphire-memory/MEMORY.md` | Durable project memory |
| `.sapphire-memory/memory_summary.md` | Fast memory summary |
| `.sapphire-memory/raw_memories.md` | Pending extracted memory |
| `.sapphire-memory/rollout_summaries/` | Per-run summaries |

## Autonomous Recovery

Sapphire includes recovery behavior for long-horizon work.

It can:

- Detect repeated hard failures.
- Track tool failure loops.
- Trigger mistake self-healing.
- Reconcile stale todos.
- Repair structured block failures.
- Continue after compaction.
- Create resume points.
- Persist lessons.
- Resume from durable boot packets.

## Sub-Agents

Sapphire supports lifecycle sub-agents.

They can:

- Spawn.
- Resume.
- Receive input.
- Run independently.
- Return results.
- Be waited on.
- Be collected.
- Be closed.
- Use bounded context.
- Support parallel task slices.

## Worktrees

Sapphire supports git worktree orchestration.

Use worktrees when:

- Agents need isolated branches.
- Tasks can proceed in parallel.
- Changes must avoid collision.
- Validation should happen independently.
- Multiple approaches should be explored safely.

## MCP

Sapphire includes MCP integration.

It can:

- Discover servers.
- Sync registries.
- Install servers.
- Connect servers.
- List tools.
- Call tools.
- List resources.
- Read resources.

## Skills

Sapphire includes local and extended skills.

Skills provide:

- Domain instructions.
- Tool workflows.
- Reusable scripts.
- Project conventions.
- Agent behavior constraints.
- Task-specific operating patterns.

## LSP

Sapphire integrates LSP support.

It can:

- Fetch diagnostics.
- Find references.
- Restart language servers.
- Improve edit validation.
- Ground changes in compiler feedback.

## Safety Model

Sapphire keeps autonomy visible.

It supports:

- Permission configuration.
- YOLO mode when explicitly requested.
- Read-only exploration wrapping.
- Structured tool validation.
- Edit guardrails.
- Tool call normalization.
- Runtime control.
- Failure tracking.
- Recovery prompts.

## Done

- [x] Terminal-first app shell.
- [x] Interactive TUI.
- [x] Non-interactive `run`.
- [x] Machine-facing `agent run`.
- [x] Machine-facing `agent inspect`.
- [x] Provider configuration.
- [x] API key storage.
- [x] Model selection.
- [x] Reasoning effort override.
- [x] Structured tool registry.
- [x] File tools.
- [x] Search tools.
- [x] Patch tools.
- [x] Shell tools.
- [x] Background job tools.
- [x] Web retrieval tools.
- [x] MCP tools.
- [x] LSP tools.
- [x] Todo planning.
- [x] Codebase indexing.
- [x] Semantic search.
- [x] Tool search.
- [x] Agent scan foundation.
- [x] Agent search foundation.
- [x] Durable memory.
- [x] Persistent memory.
- [x] Triple memory model.
- [x] Triple-triple memory model.
- [x] Rollout summaries.
- [x] Memory health.
- [x] Mistake memory.
- [x] Resume points.
- [x] Compaction recovery.
- [x] Sub-agent lifecycle.
- [x] Worktree orchestration.
- [x] Runtime telemetry.
- [x] Stats command.
- [x] Logs command.
- [x] Skills browser.
- [x] Release workflow.
- [x] Homebrew formula flow.

## Remaining

- [ ] Stabilize orchestration imports.
- [ ] Resolve missing Gas Town package dependencies.
- [ ] Resolve missing orchestration module dependencies.
- [ ] Fix currently failing agent tests.
- [ ] Expose agent scan as a dedicated user-facing command if desired.
- [ ] Expose agent search as a dedicated user-facing command if desired.
- [ ] Document every orchestration command after dependency cleanup.
- [ ] Add end-to-end CLI smoke tests.
- [ ] Add release-blocking integration test set.
- [ ] Add memory migration validation tests.
- [ ] Add worktree orchestration fixture tests.
- [ ] Add public docs for memory internals.
- [ ] Add public docs for hardness scales.
- [ ] Add public docs for machine telemetry JSON.
- [ ] Add public docs for MCP registry format.
- [ ] Add public docs for skills format.
- [ ] Add example worktree spec.
- [ ] Add example agent telemetry output.
- [ ] Add example memory health output.
- [ ] Add benchmark automation.

## Competitive Context

This comparison reflects public product documentation checked in May 2026.

Anthropic describes Claude Code as an agentic coding tool that works in the terminal, understands a codebase, edits files, runs commands, and supports development workflows.

Claude Code also documents subagents, background agents, hooks, skills, MCP, and memory-oriented workflows.

OpenAI documents Codex CLI, Codex cloud tasks, code review, subagents, web search, MCP, approval modes, and skills.

Sapphire's comparison point is its local runtime model: durable memory, codebase graph search, complexity routing, and machine-facing telemetry.

## Claude Code vs Codex vs Sapphire CLI

| Area | Claude Code | Codex | Sapphire CLI |
| --- | --- | --- | --- |
| Primary interface | Terminal and app surfaces | CLI, IDE, app, cloud | Terminal-first CLI |
| Product center | Claude agentic coding | OpenAI coding agent platform | Autonomous local agent runtime |
| Agent-runtime orientation | Strong assistant agent | Strong coding agent platform | Runtime-first agent system |
| Codebase reading | Yes | Yes | Yes, with durable graph support |
| File edits | Yes | Yes | Yes |
| Test execution | Yes | Yes | Yes |
| Subagents | Yes | Yes | Yes |
| Worktrees | Yes | Yes | Yes |
| MCP | Yes | Yes | Yes |
| Skills | Yes | Yes | Yes |
| Hooks | Yes | Environment dependent | Yes |
| Persistent memory | Supported | Supported through platform/context | Repo-local plus SQLite plus graph |
| Code index | Product-managed | Product-managed | Explicit local code index |
| Tool search | Yes | Yes | Yes |
| Large-codebase scan | Product-managed | Product-managed | Explicit durable scan with chunking and semantic survey |
| Deep repo search | Product-managed | Product-managed | Indexed search before `agentic_view` reads |
| Agent scan | Explore/plan style scanning | Repo and task context scan | `index_codebase` plus semantic survey |
| Agent search | Code search tools | CLI and cloud search tools | Indexed `tool_search` plus structured routing |
| Million-line repo strategy | Product-managed context selection | Product-managed context selection | Local scan, chunk, embed, search, then read |
| Long-horizon task model | Session and product memory | Cloud tasks and context management | Memory, resume points, boot packets, sub-agents, worktrees |
| Recovery model | Agent retry behavior | Agent retry behavior | Dedicated mistake, completion, todo, and compaction recovery |
| Hardness scale | Delegation and thoroughness policies | Reasoning and approval modes | Complexity profile and harness policy |
| Machine telemetry | Product/runtime dependent | Product/runtime dependent | `sapphire agent` JSON telemetry |
| Local DB state | Product-specific | Product-specific | SQLite-backed runtime state |
| Memory files | Claude memory locations | Codex memory/config | `.sapphire-memory` project memory |
| Mistake recovery | Agent retry behavior | Agent retry behavior | Explicit mistake and recovery pipeline |
| Resume boot packet | Context management | Compaction and cloud context | Durable resume point compiler |
| Provider choice | Claude-centered | OpenAI-centered | Multi-provider |
| Runtime ownership | Product owned | Product/platform owned | Repo-owned Go runtime |

## Agentic Capability Comparison

This table focuses on the agent runtime itself, not only the user-facing coding assistant.

| Capability | Claude Code | Codex | Sapphire CLI |
| --- | --- | --- | --- |
| Agent reads code | Yes | Yes | Yes |
| Agent searches code | Yes | Yes | Yes, through indexed `tool_search`, `rg_files`, `rg`, and semantic graph search |
| Agent scans entire repo | Product-managed | Product-managed | Explicit `index_codebase` scan path |
| Agent builds durable repo map | Product-managed | Product-managed | Yes: files, chunks, hashes, embeddings, metadata, semantic survey artifacts |
| Agent handles million-line scale | Product-managed | Product-managed | Designed around scan/search/read-selectively instead of context stuffing |
| Agent uses memory as runtime state | Supported | Supported | Yes: session DB, `.sapphire-memory`, structured summaries, mistake memory, graph state |
| Agent resumes after context pressure | Supported | Supported | Yes: compaction recovery and durable resume boot packets |
| Agent delegates work | Yes | Yes | Yes: lifecycle sub-agents and worktree orchestration |
| Agent routes by task hardness | Product policy | Reasoning and approval modes | Explicit complexity profile and harness policy |
| Agent exposes machine telemetry | Product/runtime dependent | Product/runtime dependent | Yes: `sapphire agent run` emits JSON runtime telemetry |
| Agent recovery is explicit | Product behavior | Product behavior | Yes: mistake recovery, todo reconciliation, completion guardrails, compaction continuation |
| Runtime is inspectable in repo | No | Partly | Yes, Go implementation in this repository |

## Sapphire Differentiation

Sapphire is designed for local, inspectable, extensible autonomous agent execution on large engineering codebases.

The main differentiators are:

- Memory is part of the runtime layer.
- Codebase graphing is explicit.
- Agent scan is durable and repository-scale.
- Agent search is indexed before the agent reads.
- Deep scan is based on files, chunks, hashes, embeddings, and semantic survey artifacts.
- Million-line codebase work is approached through scan, search, selective read, and memory.
- Long-horizon tasks use persistent memory, boot packets, sub-agents, worktrees, and recovery loops.
- Hardness routing is built into policy.
- Recovery has a dedicated pipeline.
- Machine telemetry is exposed through `sapphire agent`.
- Provider choice is configurable.
- The runtime is Go and locally inspectable.
- Orchestration is built into the repo.
- Worktree and sub-agent workflows are native.
- Persistent memory is project-local and durable.
- Search routing is explicit.
- Long-horizon state is part of the runtime.

## Current Boundary

Claude Code and Codex are mature commercial products.

Sapphire should not claim the same ecosystem maturity yet.

Sapphire's current claim is architectural and implementation-specific.

It exposes the local runtime machinery directly in this repository.

## Benchmark Direction

Sapphire should be judged on:

- Cold repo understanding.
- Search precision.
- Long task continuity.
- Memory correctness.
- Sub-agent coordination.
- Worktree isolation.
- Failure recovery.
- Test repair.
- Multi-turn persistence.
- Human supervision cost.

## Known Repo State

- `internal/cmd` tests pass.
- `internal/memory` tests pass.
- `internal/codeindex` tests pass.
- `internal/orchestration/db` tests pass.
- Broader orchestration packages currently need dependency cleanup.
- Some agent tests currently fail and should be stabilized before release claims.

## Sources Checked

- Anthropic Claude Code product page: https://www.anthropic.com/product/claude-code
- Claude Code subagents docs: https://code.claude.com/docs/en/sub-agents
- OpenAI Codex CLI docs: https://developers.openai.com/codex/cli
- OpenAI Codex app announcement: https://openai.com/index/introducing-the-codex-app/
- OpenAI Codex upgrades announcement: https://openai.com/index/introducing-upgrades-to-codex/

## Release Flow

```bash
task build
task test
task release
```

## Homebrew Tap

The Homebrew formula is published into:

```text
duggal1/homebrew-sapphire-cli
```

## Maintainer Notes

- Keep README claims tied to actual repo capabilities.
- Do not claim full product maturity until tests are green.
- Keep the competitive comparison current.
- Re-check Claude Code and Codex docs before major public claims.
- Keep memory central.
- Keep agent runtime capabilities central.
- Keep CLI documentation direct.
