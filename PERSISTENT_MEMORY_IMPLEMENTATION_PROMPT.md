# Sapphire CLI: Persistent Memory Implementation Prompt

## Context

You are implementing persistent memory for Sapphire CLI. The goal: enable main agent and sub-agents to run for **20-30 hours non-stop** without context overflow, while maintaining full recall of work state, decisions, and coordination.

---

## What Sapphire CLI Currently Has (KEEP THESE)

### 1. Long-Horizon Artifacts (File-Based)
**Location:** `internal/agent/longhorizon/manager.go`

**Files created per session:**
- `frozen_spec.md` — Task specification
- `milestones.json` — Milestone plan
- `runbook.md` — Operating procedures
- `audit.log` — Decision audit trail

**KEEP:** This system provides structured task breakdown and audit trails. It is injected into context via `BuildInjection()`.

### 2. Structured Summaries (SQLite-Based)
**Location:** `internal/agent/memory/memory.go`

**Tables:**
- `structured_summaries` — Decisions, file changes, failure modes, dependency graph, todo states
- `codebase_knowledge` — Symbol documentation, signatures
- `project_constitution` — Project-level constraints

**KEEP:** Summaries are **critical** for compressing long conversations into structured, queryable state. This is one pillar of persistent memory.

### 3. Sub-Agent Metadata (Message-Based)
**Location:** `internal/agent/subagent_metadata.go`

**Stored in messages table:**
- Assignment ID, worktree path, branch
- Write manifest, definition of done, test command

**KEEP:** This tracks sub-agent assignment details.

### 4. Git Snapshot Commits
**Location:** `internal/agent/tools/git_snapshot.go`

**Behavior:**
- Automatic commits after file writes (1.5s debounce)
- Local-only, never auto-pushed
- Actor naming: `main-agent` or `<agent-id>-<task-slug>`

**KEEP:** This provides file-level recoverability.

---

## What Sapphire CLI Is Missing (ADD THESE)

### The Gap

Current persistent memory relies heavily on **summaries**. Summaries are **one pillar**, but insufficient alone for 30-hour runs because:

1. **Summaries are lossy** — Compression loses details
2. **Summaries are session-scoped** — Not shared across agents
3. **No agent state tracking** — Can't query "which agents are running?"
4. **No conversation persistence** — Agent-to-agent messages not stored
5. **No activity feed** — No audit trail of agent actions
6. **No work queue** — No hook system for task assignment
7. **No heartbeat** — Can't detect stuck/dead agents

### What To Add (From Gas-Town Reference)

**Reference:** `gastown/internal/beads/`, `gastown/internal/cmd/mail*.go`, `gastown/internal/polecat/`

**Add to `orchestration.db`:**

```sql
-- 1. Agent State (external to LLM context)
CREATE TABLE agent_state (
    agent_id TEXT PRIMARY KEY,
    session_id TEXT,
    parent_agent_id TEXT,
    role TEXT,               -- main/sub-agent/coordinator
    status TEXT,             -- queued/running/idle/completed/error
    worktree_path TEXT,
    branch TEXT,
    hook_bead_id TEXT,       -- Current assigned work item
    last_heartbeat INTEGER,
    created_at INTEGER,
    updated_at INTEGER
);

-- 2. Agent Mail (persistent conversations)
CREATE TABLE agent_mail (
    id TEXT PRIMARY KEY,
    thread_id TEXT,
    to_agent TEXT,
    from_agent TEXT,
    subject TEXT,
    body TEXT,
    priority INTEGER,        -- 0=urgent, 4=backlog
    read INTEGER,
    created_at INTEGER,
    read_at INTEGER
);

-- 3. Agent Activity (audit trail)
CREATE TABLE agent_activity (
    id TEXT PRIMARY KEY,
    agent_id TEXT,
    event_type TEXT,         -- spawn/mail_sent/mail_received/wait/complete/error/handoff
    details TEXT,            -- JSON
    created_at INTEGER
);

-- 4. Work Items (beads equivalent)
CREATE TABLE work_items (
    id TEXT PRIMARY KEY,
    type TEXT,               -- task/bug/feature/epic
    title TEXT,
    description TEXT,
    status TEXT,             -- open/in_progress/blocked/closed
    assignee TEXT,           -- agent_id
    parent_id TEXT,          -- Parent work item (for hierarchy)
    dependencies TEXT,       -- JSON array of dependent IDs
    created_at INTEGER,
    closed_at INTEGER
);
```

---

## Gas-Town Reference Files (Study These)

### Persistent State Core
- `gastown/internal/beads/beads.go` — Core persistent state engine
- `gastown/internal/beads/beads_types.go` — Issue types (task, bug, feature, convoy, molecule)
- `gastown/internal/beads/beads_agent.go` — Agent bead tracking (state, assignee, heartbeat, cleanup)
- `gastown/internal/beads/beads_dependencies.go` — Dependency tracking (blocks, tracks, child-of)
- `gastown/internal/beads/beads_convoy_fields.go` — Convoy metadata (owner, notify, merge strategy)

### Agent-to-Agent Communication
- `gastown/internal/cmd/mail.go` — Main mail command entrypoint
- `gastown/internal/cmd/mail_send.go` — Send persistent messages
- `gastown/internal/cmd/mail_inbox.go` — Read agent inbox
- `gastown/internal/cmd/mail_thread.go` — Conversation threading
- `gastown/internal/cmd/mail_reply.go` — Reply with auto-threading
- `gastown/internal/cmd/nudge.go` — Immediate delivery (tmux send-keys)
- `gastown/internal/cmd/nudge_poller.go` — Background poller for non-Claude agents
- `gastown/internal/mail/resolver.go` — Address routing (mayor/, rig/polecat, crew/)
- `gastown/internal/mail/router.go` — Message routing to inbox
- `gastown/internal/mail/mailbox.go` — Inbox/outbox storage

### Agent Lifecycle + State
- `gastown/internal/polecat/manager.go` — Polecat manager (2385 lines)
- `gastown/internal/polecat/session_manager.go` — tmux session lifecycle (845 lines)
- `gastown/internal/polecat/namepool.go` — Persistent identity allocation
- `gastown/internal/polecat/types.go` — Polecat states and models
- `gastown/internal/polecat/heartbeat.go` — Liveness detection

### Activity + Monitoring
- `gastown/internal/cmd/feed.go` — Activity feed entrypoint
- `gastown/internal/tui/feed/model.go` — Three-panel TUI dashboard
- `gastown/internal/tui/feed/problems.go` — Stuck agent detection (GUPP/stalled/zombie)
- `gastown/internal/tui/feed/health.go` — Health classification
- `gastown/internal/activity/tracker.go` — Activity event tracking
- `gastown/internal/events/feed.go` — Event stream logging

### Context Recovery
- `gastown/internal/cmd/prime.go` — Context recovery entrypoint
- `gastown/internal/session/beacon.go` — Startup beacon formatting
- `gastown/internal/session/prime.go` — Prime instruction generation
- `gastown/internal/session/manager.go` — Session lifecycle management
- `gastown/internal/session/heartbeat.go` — Session-level liveness
- `gastown/internal/session/pid_tracking.go` — Process identity tracking

### Work Tracking
- `gastown/internal/cmd/convoy.go` — Convoy tracking (2715 lines)
- `gastown/internal/cmd/hook.go` — Hook/work assignment
- `gastown/internal/cmd/hooks.go` — Hook initialization
- `gastown/internal/cmd/sling.go` — Work dispatch (1138 lines)

---

## Implementation Requirements

### Architecture Principles

1. **External State Machine** — LLM context is a **view** into persistent state, not the source of truth
2. **Load on Demand** — Only load what's needed for current turn (~5K tokens max)
3. **Persist Every Turn** — Write state changes to DB before clearing context
4. **Queryable** — State must be SQL-queryable (not just files)
5. **Cross-Agent** — State shared across all agents in a session
6. **Survives Restarts** — Agent can crash/restart and recover full state

### Turn Cycle (Implement This Pattern)

```
1. Agent starts turn
2. Load agent_state from DB (current status, assigned work)
3. Load recent mail from inbox (last 10 unread)
4. Load work_item details (hook_bead_id)
5. Load long-horizon artifacts (spec, plan, audit tail)
6. Load structured summary (decisions, changes, failures)
7. Build prompt with ONLY what's needed (~5K tokens)
8. LLM generates response
9. Persist results to DB (agent_state, agent_mail, agent_activity, work_items)
10. Update heartbeat
11. Clear context (let GC collect)
12. Next turn: repeat from step 2
```

**Result:** Context stays ~5K tokens forever. Agent can run 100 hours.

---

## What To Build

### Phase 1: Database Schema + Services

**Create:**
```
internal/orchestration/db/
├── db.go              # Open orchestration.db, run migrations
├── migrations.go      # Schema: agent_state, agent_mail, agent_activity, work_items
└── models.go          # Go types for tables
```

### Phase 2: Mailbox Service

**Create:**
```
internal/agent/mailbox/
├── mailbox.go         # Send(), Inbox(), MarkRead(), Thread()
├── nudge.go           # Wake waiting subagent via control plane
└── types.go           # Message, Thread, Priority types
```

### Phase 3: State Service

**Create:**
```
internal/agent/state/
├── state.go           # Register(), Heartbeat(), Status()
└── types.go           # AgentStatus enum
```

### Phase 4: Activity Service

**Create:**
```
internal/agent/activity/
├── activity.go        # Log(), Feed()
└── types.go           # EventType enum
```

### Phase 5: Wire Into Sub-Agent Runner

**Modify:**
- `internal/agent/subagent_manager.go` — Poll inbox, send mail, update heartbeat
- `internal/agent/subagent_control_plane.go` — Add nudge hook
- `internal/agent/subagent_events.go` — Log activity events

### Phase 6: Wire Into Main Agent

**Modify:**
- `internal/agent/coordinator.go` — Access to mailbox, state, activity services
- Main agent can send mail, inspect state, view activity feed

---

## Strict Constraints

### DO
- Use SQLite only (same as existing Sapphire CLI)
- Keep summaries (they are one pillar of persistent memory)
- Keep long-horizon artifacts (spec, plan, runbook, audit)
- Keep git snapshot commits
- Make state queryable via SQL
- Load on demand, persist every turn
- Support 20-30 hour non-stop runs

### DO NOT
- Do not use Dolt (too slow, unnecessary complexity)
- Do not use tmux (Sapphire doesn't use tmux for agent sessions)
- Do not remove summaries (they are critical)
- Do not remove long-horizon artifacts
- Do not pollute main `sapphire.db` with orchestration tables
- Do not make orchestration dependent on Gas-Town runtime (use as reference only)

---

## Success Criteria

1. **Agent runs 30 hours** — Context never exceeds 10K tokens
2. **Full recall** — Agent can query any past decision, message, or state
3. **Crash recovery** — Agent restarts and recovers full context from DB
4. **Agent-to-agent mail** — Sub-agents coordinate directly without routing through main agent
5. **Activity audit** — Full trail of agent actions queryable by time/agent/event type
6. **Heartbeat detection** — Stuck/dead agents detected via heartbeat timeout
7. **No data loss** — All state persisted before context clear

---

## Output Format

Produce:
1. SQL schema for `orchestration.db` (4 tables: agent_state, agent_mail, agent_activity, work_items)
2. Go service implementations (mailbox, state, activity)
3. Integration code for subagent_manager.go (poll inbox, send mail, heartbeat)
4. Integration code for coordinator.go (main agent access to services)
5. Migration functions (idempotent schema creation)

**Tone:** Ultra-concise, structured, no hype. Code-first.

**Reference:** Gas-Town files listed above for patterns, but implement Sapphire-native packages.
