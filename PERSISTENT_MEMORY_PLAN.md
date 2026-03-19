# Sapphire CLI: Persistent Memory Implementation Plan

## Objective

Implement extreme persistent memory for long-term conversations (hours to days, 500+ messages) so that:

1. **User preferences persist forever** (e.g., "My name is X", "Prefer Go over Python")
2. **Key decisions persist forever** (e.g., "Use PostgreSQL", "JWT for auth")
3. **Conversation context survives session cycles** (checkpoint every 50 messages or 30 minutes)
4. **Zero memory loss** across days of back-and-forth conversation
5. **Pure SQLite** — no Git, no Dolt, no external dependencies for memory

---

## Architecture

### Database Schema (orchestration.db)

Add three tables to `internal/orchestration/db/migrations.go`:

1. **session_checkpoints** — Conversation checkpoints created every 50 messages or 30 minutes. Fields: id, session_id, parent_id (link to previous checkpoint), message_count, user_preferences (JSON), key_decisions (JSON), files_modified (JSON array), pending_tasks (JSON array), summary (compressed ~500 tokens), mail_cursor, activity_cursor, created_at.

2. **decisions** — Key decisions persisted across all sessions. Fields: id, session_id, category (architecture/preference/tool), key, value, confidence (confirmed/tentative), created_at.

3. **user_preferences** — Global preferences that persist forever. Fields: key (PRIMARY KEY), value, updated_at.

Add indexes on session_checkpoints(session_id, created_at DESC), decisions(session_id), decisions(category).

---

## Implementation Files

### 1. Checkpoint Service

**File:** `internal/agent/memory/checkpoint.go`

Create `CheckpointService` with the following methods:

- `NewCheckpointService(db)` — Initialize with database connection
- `MaybeCheckpoint(ctx, sessionID, messageCount)` — Create checkpoint if thresholds met (50 messages or 30 minutes since last checkpoint)
- `Resume(ctx, sessionID)` — Load latest checkpoint for session, returns ErrNoCheckpoint if none exists
- `SaveUserPreference(ctx, key, value)` — Persist user preference forever (upsert on conflict)
- `SaveDecision(ctx, sessionID, category, key, value, confidence)` — Persist key decision

Checkpoint struct contains: ID, SessionID, ParentID, MessageCount, UserPreferences (map), KeyDecisions (slice), FilesModified (slice), PendingTasks (slice), Summary (string), MailCursor, ActivityCursor, CreatedAt.

Decision struct contains: Category, Key, Value, Confidence.

Helper methods to implement: getLastCheckpoint, saveCheckpoint, extractUserPreferences, extractKeyDecisions, getModifiedFiles, getPendingTasks, summarizeConversation (use small LLM call to compress ~50 messages to ~500 tokens), getCurrentMailCursor, getCurrentActivityCursor.

---

### 2. Summary Service

**File:** `internal/agent/memory/summary.go`

Create `SummaryService` with the following methods:

- `NewSummaryService(messages, agent)` — Initialize with message service and agent
- `SummarizeConversation(ctx, sessionID, fromMessageID)` — Compress N messages into ~500 tokens. Load messages since last checkpoint, build summary prompt asking to extract user preferences, key decisions, tasks completed/pending, files modified. Use small model for cost-effectiveness.
- `ExtractUserPreferences(ctx, sessionID)` — Pattern-match messages for preferences ("My name is X", "I prefer X over Y", "Always use X", "Never use Y"). Return as map.
- `ExtractKeyDecisions(ctx, sessionID)` — Pattern-match messages for decisions ("Let's use X", "We decided on X", "Go with X approach"). Return as slice of Decision structs.

---

### 3. Wire Into Coordinator

**File:** `internal/agent/coordinator.go`

Add two fields to coordinator struct: checkpointService and summaryService.

Initialize in `NewCoordinator()`: create CheckpointService with database connection, create SummaryService with message service and agent.

---

### 4. Update Run Loop

**File:** `internal/agent/agent.go` (SessionAgent.Run)

After each turn completes:

1. Get message count for session
2. Call checkpointService.MaybeCheckpoint() — creates checkpoint if thresholds met
3. Call summaryService.ExtractUserPreferences() — save each preference via checkpointService.SaveUserPreference()
4. Call summaryService.ExtractKeyDecisions() — save each decision via checkpointService.SaveDecision()

---

### 5. Build Resume Context

**File:** `internal/agent/orchestration_runtime.go`

Create `buildResumeContext(ctx, sessionID)` function:

1. Load latest checkpoint via checkpointService.Resume() — return empty if no checkpoint exists
2. Build sections array:
   - USER PREFERENCES: Include if checkpoint has preferences (format as JSON)
   - KEY DECISIONS: Include if checkpoint has decisions (format as category: key = value)
   - CONVERSATION SUMMARY: Include checkpoint summary with message range
   - PENDING TASKS: Include if checkpoint has pending tasks
   - FILES MODIFIED: Include if checkpoint has modified files
   - UNREAD MAIL: Query mailbox since mail_cursor, include if messages exist
3. Return formatted context with "## CONVERSATION RESUME" header

Wire into `buildContext()`: call buildResumeContext() first, prepend to existing context (long-horizon, summaries, etc.).

---

## Test Plan

### Checkpoint Service Tests

**File:** `internal/agent/memory/checkpoint_test.go`

Create tests:

- TestCheckpointService_MaybeCheckpoint_Every50Messages — Create service, simulate 50 messages, assert checkpoint created
- TestCheckpointService_MaybeCheckpoint_Every30Minutes — Create checkpoint at T0, advance time 30 minutes, simulate 1 message, assert checkpoint created
- TestCheckpointService_Resume_LoadsLatest — Create 3 checkpoints, resume, assert latest checkpoint loaded
- TestCheckpointService_SaveUserPreference_PersistsForever — Save preference, close DB, reopen, assert preference still there

### Integration Tests

**File:** `internal/agent/coordinator_test.go`

Create tests:

- TestCoordinator_Run_CheckpointsAfterTurn — Run agent for 50 turns, assert checkpoint created, assert user preferences extracted, assert key decisions extracted
- TestCoordinator_buildResumeContext_IncludesAllLayers — Create checkpoint with all fields, build resume context, assert all sections present

---

## Acceptance Criteria

| Criterion | How to Verify |
|-----------|---------------|
| **Checkpoint every 50 messages** | Run 50 turns, check session_checkpoints table |
| **Checkpoint every 30 minutes** | Wait 30 min, run 1 turn, check table |
| **User preferences persist** | Save "name = Harshit", restart process, load checkpoint, assert present |
| **Key decisions persist** | Save "database = PostgreSQL", restart, assert present |
| **Summary ~500 tokens** | Check summary field length after checkpoint |
| **Mail cursor tracks** | Send mail, checkpoint, send more mail, resume, assert only new mail shown |
| **Zero memory loss** | 500 messages over 10 hours, resume, assert all preferences/decisions/tasks present |

---

## Migration Path

### Existing Users

For users with existing orchestration.db:

Create `MigrateToCheckpoints(ctx)` method on coordinator:

1. Check if tables exist (session_checkpoints, decisions, user_preferences)
2. For each missing table, run migration
3. Return error if any migration fails

### Backfill Checkpoints

For existing sessions without checkpoints:

Create `BackfillCheckpoints(ctx)` method on coordinator:

1. List all active sessions
2. For each session, create initial checkpoint from current state (message count, summarize existing conversation, current timestamp)
3. Save checkpoint

---

## Performance Considerations

| Operation | Target | Optimization |
|-----------|--------|--------------|
| Checkpoint creation | < 2 seconds | Run async, don't block turn |
| Resume from checkpoint | < 1 second | Cache in memory |
| Summary generation | < 5 seconds | Use small model (Haiku/Sonnet) |
| DB size after 1000 messages | < 10 MB | Prune old checkpoints (keep last 10) |

---

## Rollout Plan

### Phase 1: Core Checkpointing (2-3 days)
- [ ] Add database tables
- [ ] Implement CheckpointService
- [ ] Wire into coordinator.Run()
- [ ] Add tests

### Phase 2: Summary Service (2-3 days)
- [ ] Implement SummaryService
- [ ] Add LLM summarization prompt
- [ ] Test compression quality (50 messages to ~500 tokens)

### Phase 3: Resume Context (1-2 days)
- [ ] Implement buildResumeContext()
- [ ] Wire into context building
- [ ] Test resume flow

### Phase 4: User Preferences + Decisions (1-2 days)
- [ ] Add extraction patterns
- [ ] Add SaveUserPreference() / SaveDecision()
- [ ] Test persistence across restarts

### Phase 5: Migration + Backfill (1 day)
- [ ] Add migration logic
- [ ] Backfill existing sessions
- [ ] Test upgrade path

**Total: 7-11 days**

---

## Success Metrics

| Metric | Target |
|--------|--------|
| **Memory retention after 500 messages** | 100% (user prefs, decisions, tasks) |
| **Checkpoint creation latency** | < 2 seconds (async) |
| **Resume latency** | < 1 second |
| **DB size after 1000 messages** | < 10 MB |
| **Summary quality** | Human review: captures all key info |

---

## Notes

- **Do NOT use Git for conversation memory** — SQLite only
- **Do NOT store full messages** — summaries only (~500 tokens per checkpoint)
- **Do NOT block turns** — checkpoint async, don't wait
- **DO persist user preferences forever** — these are global, not session-scoped
- **DO extract decisions automatically** — pattern-match conversation, don't require manual tagging

---

## End of Plan
