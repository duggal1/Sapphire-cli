# Sapphire CLI Sub-Agent + Main Agent: Complete File Reference

**Excludes:** Git worktree files (already implemented)  
**Includes:** Sub-agent orchestration, communication, lifecycle, main-agent coordination

---

## CORE SUB-AGENT FILES (13 files)

### Sub-Agent Lifecycle Management

| File | Purpose | Lines |
|------|---------|-------|
| `internal/agent/subagent_manager.go` | Sub-agent lifecycle: spawn, resume, wait, close | ~1100 |
| `internal/agent/subagent_control_plane.go` | Control plane API for sub-agent operations | ~150 |
| `internal/agent/subagent_coordination.go` | Assignment building, prompt generation, report parsing | ~300 |
| `internal/agent/subagent_events.go` | Sub-agent event publishing | ~100 |
| `internal/agent/subagent_events_helpers.go` | Event helper functions | ~50 |
| `internal/agent/subagent_metadata.go` | Metadata persistence for sub-agents | ~150 |
| `internal/agent/subagent_supervisor.go` | Supervision and monitoring | ~200 |
| `internal/agent/subagent_task_context.go` | TASK.md injection into worktrees | ~100 |
| `internal/agent/subagent_guardrails.go` | Spawn limits, depth validation | ~150 |
| `internal/agent/subagent_validation_gate.go` | Post-completion validation (build, test, lint) | ~400 |

### Sub-Agent Worktree (KEEP - Already Implemented)

| File | Purpose |
|------|---------|
| `internal/agent/subagent_worktree.go` | Worktree creation, cleanup, quarantine |
| `internal/agent/main_worktree.go` | Main worktree preparation |
| `internal/agent/worktree_orchestrator.go` | Parallel worktree orchestration |

---

## MAIN AGENT / COORDINATOR FILES (6 files)

| File | Purpose | Lines |
|------|---------|-------|
| `internal/agent/coordinator.go` | Main agent coordinator: manages all sub-agents | ~2200 |
| `internal/agent/coordinator_response.go` | Coordinator response handling | ~100 |
| `internal/agent/agent.go` | Session agent implementation | ~2500 |
| `internal/agent/agent_tool.go` | Agent tool definitions | ~200 |
| `internal/agent/event.go` | Agent event definitions | ~100 |
| `internal/agent/errors.go` | Agent error types | ~50 |

---

## COLLABORATION TOOLS (Agent-to-Agent Communication) (1 file)

| File | Purpose | Lines |
|------|---------|-------|
| `internal/agent/collab_tools.go` | spawn_agent, resume_agent, send_input, wait, collect_result, close_agent tools | ~600 |

**Tools Defined:**
- `spawn_agent` — Spawn new sub-agent
- `resume_agent` — Resume closed agent
- `send_input` — Send follow-up task
- `wait` — Wait for completion
- `collect_result` — Collect results
- `close_agent` — Close agent

---

## BACKGROUND ORCHESTRATION (2 files)

| File | Purpose | Lines |
|------|---------|-------|
| `internal/agent/background_subagents.go` | Autonomous background sub-agent spawning | ~300 |
| `internal/agent/background_wait.go` | Background sub-agent wait handling | ~100 |

---

## JOB MANAGEMENT (Background Agents) (3 files)

| File | Purpose | Lines |
|------|---------|-------|
| `internal/agent/agent_job_manager.go` | Background agent job management | ~200 |
| `internal/agent/agent_job_runner.go` | Agent job execution | ~300 |
| `internal/agent/agent_job_tools.go` | Agent job tool implementations | ~200 |

---

## LONG-HORIZON TASK MANAGEMENT (1 file)

| File | Purpose | Lines |
|------|---------|-------|
| `internal/agent/longhorizon/manager.go` | Long-horizon task manager: spec/plan/runbook/audit | ~400 |

---

## MONITORING & DETECTION (2 files)

| File | Purpose | Lines |
|------|---------|-------|
| `internal/agent/loop_detection.go` | Agent loop detection | ~150 |
| `internal/agent/indexer.go` | Codebase indexer | ~200 |

---

## RUNTIME & CONTROL (2 files)

| File | Purpose | Lines |
|------|---------|-------|
| `internal/agent/runtime_control.go` | Runtime control for tool execution | ~150 |
| `internal/agent/tool_cache.go` | Tool caching | ~100 |
| `internal/agent/tool_filter.go` | Tool filtering | ~100 |

---

## MEMORY SYSTEM (4 files)

| File | Purpose | Lines |
|------|---------|-------|
| `internal/agent/memory_pipeline.go` | Memory pipeline for agent context | ~200 |
| `internal/agent/memory/memory.go` | Memory service for agent context | ~150 |
| `internal/agent/memory/format.go` | Memory formatting | ~100 |
| `internal/agent/memory/drift_detector.go` | Memory drift detection | ~100 |

---

## PROMPTS & TEMPLATES (3 files)

| File | Purpose | Lines |
|------|---------|-------|
| `internal/agent/prompts.go` | System prompt templates | ~300 |
| `internal/agent/prompt/prompt.go` | Prompt building and management | ~200 |
| `internal/agent/templates/` | Prompt templates directory | - |

---

## TOOLS (Sub-Agent Capabilities) (~50 files)

### Core Tools

| File | Purpose |
|------|---------|
| `internal/agent/tools/tools.go` | Tool context keys and helpers |
| `internal/agent/tools/bash.go` | Bash command execution |
| `internal/agent/tools/edit.go` | File editing with precision matching |
| `internal/agent/tools/write.go` | Full file write |
| `internal/agent/tools/view.go` | File viewing |
| `internal/agent/tools/fast_view.go` | Fast file view |
| `internal/agent/tools/glob.go` | Pattern-based file search |
| `internal/agent/tools/grep.go` | Text search with regex |
| `internal/agent/tools/rg.go` | Ripgrep text search |
| `internal/agent/tools/ls.go` | Directory listing |
| `internal/agent/tools/search.go` | Code search |
| `internal/agent/tools/search_tools.go` | Search tool helpers |
| `internal/agent/tools/references.go` | Reference management |
| `internal/agent/tools/todos.go` | Todo management |
| `internal/agent/tools/python.go` | Python code execution |
| `internal/agent/tools/fetch.go` | Web fetching |
| `internal/agent/tools/web_fetch.go` | Web fetch helpers |
| `internal/agent/tools/web_search.go` | Web search |
| `internal/agent/tools/google_search.go` | Google search |
| `internal/agent/tools/download.go` | File download |

### Tool Safety & Validation

| File | Purpose |
|------|---------|
| `internal/agent/tools/safe.go` | Safe execution helpers |
| `internal/agent/tools/edit_guard.go` | Edit protection |
| `internal/agent/tools/tool_call_validation.go` | Tool call validation |
| `internal/agent/tools/tool_call_preflight.go` | Tool call preflight |
| `internal/agent/tools/tool_call_normalize.go` | Tool call normalization |
| `internal/agent/tools/multiedit.go` | Multi-edit operations |
| `internal/agent/tools/apply_patch.go` | Apply patch tool |
| `internal/agent/tools/apply_patch_parser.go` | Patch parser |

### LSP & Diagnostics

| File | Purpose |
|------|---------|
| `internal/agent/tools/diagnostics.go` | LSP diagnostics |
| `internal/agent/tools/compiler_diagnostics.go` | Compiler diagnostics |
| `internal/agent/tools/lsp_restart.go` | LSP restart tool |

### Background Jobs

| File | Purpose |
|------|---------|
| `internal/agent/tools/background_jobs.go` | Background job management |
| `internal/agent/tools/job_output.go` | Job output viewing |
| `internal/agent/tools/job_kill.go` | Job termination |
| `internal/agent/tools/job_list.go` | Job listing |

### MCP Integration

| File | Purpose |
|------|---------|
| `internal/agent/tools/mcp-tools.go` | MCP tool execution |
| `internal/agent/tools/call_mcp_tool.go` | MCP tool calling |
| `internal/agent/tools/list_mcp_tools.go` | List MCP tools |
| `internal/agent/tools/list_mcp_resources.go` | List MCP resources |
| `internal/agent/tools/read_mcp_resource.go` | Read MCP resource |
| `internal/agent/tools/connect_mcp.go` | Connect MCP server |
| `internal/agent/tools/list_available_mcps.go` | List available MCPs |
| `internal/agent/tools/mcp/init.go` | MCP client initialization |
| `internal/agent/tools/mcp/manage.go` | MCP client management |
| `internal/agent/tools/mcp/tools.go` | MCP tool execution |
| `internal/agent/tools/mcp/resources.go` | MCP resource handling |
| `internal/agent/tools/mcp/prompts.go` | MCP prompt handling |
| `internal/agent/tools/mcp/timeout.go` | MCP timeout handling |

### Memory & Skills

| File | Purpose |
|------|---------|
| `internal/agent/tools/memory_query.go` | Memory query tool |
| `internal/agent/tools/tool_suggest.go` | Tool suggestion |
| `internal/agent/tools/skill_tool.go` | Skill loading and execution |

### Plan Mode

| File | Purpose |
|------|---------|
| `internal/agent/tools/set_mode.go` | Set mode tool |
| `internal/agent/tools/plan_mode_filter.go` | Plan mode filtering |
| `internal/agent/tools/update_plan.go` | Update plan tool |

### Write Scope

| File | Purpose |
|------|---------|
| `internal/agent/tools/write_scope.go` | Write scope management |
| `internal/agent/tools/request_user_input.go` | User input tool |

### Git Snapshot (KEEP - Already Implemented)

| File | Purpose |
|------|---------|
| `internal/agent/tools/git_snapshot.go` | Automatic snapshot commits |

---

## MCP INTEGRATION (7 files)

| File | Purpose | Lines |
|------|---------|-------|
| `internal/agent/mcp_registry.go` | MCP registry management | ~200 |
| `internal/agent/mcp_selection.go` | MCP selection based on prompts | ~200 |
| `internal/agent/mcp_preflight.go` | MCP preflight discovery | ~150 |
| `internal/agent/mcp_inventory.go` | MCP inventory building | ~150 |
| `internal/agent/mcp_async.go` | Async MCP operations | ~100 |
| `internal/agent/mcp_autonomy.go` | MCP autonomy settings | ~100 |
| `internal/agent/mcp_prompt.go` | MCP prompt handling | ~100 |
| `internal/agent/mcp_runtime.go` | MCP runtime integration | ~100 |

---

## PLAN MODE (4 files)

| File | Purpose | Lines |
|------|---------|-------|
| `internal/agent/planmode/mode.go` | Plan mode implementation | ~200 |
| `internal/agent/planmode/plan_parser.go` | Plan parsing | ~150 |
| `internal/agent/planmode/prompt.go` | Plan mode prompts | ~100 |
| `internal/agent/planmode/restrictions.go` | Plan mode restrictions | ~100 |

---

## HYPER PROVIDER (1 file)

| File | Purpose | Lines |
|------|---------|-------|
| `internal/agent/hyper/provider.go` | Hyper provider configuration | ~100 |

---

## TOTAL FILE COUNT

| Category | File Count |
|----------|------------|
| **Core Sub-Agent** | 10 |
| **Sub-Agent Worktree** | 3 (KEEP) |
| **Main Agent/Coordinator** | 6 |
| **Collaboration Tools** | 1 |
| **Background Orchestration** | 2 |
| **Job Management** | 3 |
| **Long-Horizon** | 1 |
| **Monitoring** | 2 |
| **Runtime & Control** | 3 |
| **Memory System** | 4 |
| **Prompts** | 3 |
| **Tools (Core)** | ~20 |
| **Tools (Safety)** | ~8 |
| **Tools (LSP)** | ~3 |
| **Tools (Jobs)** | ~4 |
| **Tools (MCP)** | ~13 |
| **Tools (Memory/Skills)** | ~3 |
| **Tools (Plan Mode)** | ~3 |
| **Tools (Write Scope)** | ~2 |
| **Tools (Git Snapshot)** | 1 (KEEP) |
| **MCP Integration** | 8 |
| **Plan Mode** | 4 |
| **Hyper Provider** | 1 |
| **TOTAL** | **~115 files** |

---

## WHAT SAPPHIRE CLI HAS VS GAS-TOWN

| Capability | Sapphire CLI | Gas-Town |
|------------|--------------|----------|
| **Sub-agent spawning** | ✅ `subagent_manager.go` | ✅ `polecat/manager.go` |
| **Worktree isolation** | ✅ `subagent_worktree.go` | ✅ `polecat/session_manager.go` |
| **Snapshot commits** | ✅ `git_snapshot.go` | ❌ Manual |
| **Agent-to-agent mail** | ❌ None | ✅ 18 mail files |
| **Agent-to-agent nudge** | ❌ None | ✅ 3 nudge files |
| **Persistent mailbox** | ❌ None | ✅ beads-backed |
| **Work queue (hook)** | ❌ None | ✅ 10 hook files |
| **Convoy tracking** | ❌ None | ✅ 9 convoy files |
| **Activity monitoring** | ❌ None | ✅ 5 feed files |
| **Health detection** | ❌ None | ✅ problems.go, health.go |
| **Context recovery** | ❌ None | ✅ 6 prime/beacon files |
| **Persistent identity** | ❌ UUIDs | ✅ namepool.go |
| **Formula system** | ❌ None | ✅ 4 formula files |
| **Merge queue** | ❌ Human-only | ✅ refinery/ |
| **Capacity control** | ❌ Simple limit | ✅ scheduler/capacity/ |

---

## KEY DIFFERENCES

### Sapphire CLI Strengths
- Git snapshot commits (automatic, debounced)
- Git safety policy (blocked commands)
- Worktree overlap detection
- Validation gate (build, test, lint, security)
- Long-horizon artifacts (spec, plan, runbook, audit)

### Sapphire CLI Gaps (vs Gas-Town)
- **No persistent communication** — agents can't talk directly
- **No mailbox system** — no inbox/outbox that survives restarts
- **No work queue** — no hook mechanism for task assignment
- **No convoy tracking** — no work bundling across agents
- **No activity monitoring** — no real-time dashboard
- **No health detection** — no GUPP/stalled/zombie detection
- **No context recovery** — no prime/beacon for session restart
- **No formula system** — no predefined workflows
- **No capacity management** — simple thread limit only

---

## FILES TO ADD FOR GAS-TOWN CAPABILITIES

To match Gas-Town's sub-agent communication and orchestration:

### 1. Agent-to-Agent Communication (New Folder)

```
internal/agent/mailbox/
├── mailbox.go            # Simple inbox/outbox (~150 lines)
├── nudge.go              # tmux send-keys wrapper (~50 lines)
└── resolver.go           # Address routing (~100 lines)
```

### 2. Work Queue (Hook System)

```
internal/agent/hook/
├── hook.go               # Work assignment (~200 lines)
└── queue.go              # Work queue (~100 lines)
```

### 3. Activity Monitoring

```
internal/agent/feed/
├── feed.go               # Activity feed (~200 lines)
└── problems.go           # Stuck agent detection (~200 lines)
```

### 4. Context Recovery

```
internal/agent/recovery/
├── prime.go              # Context reload (~150 lines)
└── beacon.go             # Startup beacon (~100 lines)
```

**Total: ~10 new files, ~1200 lines**

---

## END OF SAPPHIRE CLI SUB-AGENT FILE REFERENCE
