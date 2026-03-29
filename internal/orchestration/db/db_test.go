package orchestrationdb

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestStoreMailLifecycleAndState(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	msg, err := store.SendMail(ctx, AgentMail{
		Address:         "agent:agent-b",
		ToAgent:         "agent-b",
		ResolvedToAgent: "agent-b",
		FromAgent:       "agent-a",
		Subject:         "handoff",
		Body:            "api is ready",
		Priority:        2,
	})
	require.NoError(t, err)
	require.NotEmpty(t, msg.ID)
	require.NotEmpty(t, msg.ThreadID)
	require.Equal(t, MailDeliveryStatePending, msg.DeliveryState)
	require.Equal(t, "agent:agent-b", msg.Address)
	require.Equal(t, "agent-b", msg.ResolvedToAgent)

	inbox, err := store.ListInbox(ctx, "agent-b", true, 10)
	require.NoError(t, err)
	require.Len(t, inbox, 1)
	require.Equal(t, "agent-a", inbox[0].FromAgent)
	require.Equal(t, MailDeliveryStatePending, inbox[0].DeliveryState)

	leased, err := store.LeaseInbox(ctx, "agent-b", "agent-b", 10, time.Minute)
	require.NoError(t, err)
	require.Len(t, leased, 1)
	require.Equal(t, msg.ID, leased[0].ID)
	require.Equal(t, MailDeliveryStateLeased, leased[0].DeliveryState)
	require.Equal(t, 1, leased[0].DeliveryAttempts)
	require.Equal(t, "agent-b", leased[0].LeaseOwner)
	require.False(t, leased[0].LeaseExpiresAt.IsZero())

	actionable, err := store.ListActionableMail(ctx, "agent-b", 10)
	require.NoError(t, err)
	require.Len(t, actionable, 1)
	require.Equal(t, MailDeliveryStateLeased, actionable[0].DeliveryState)

	thread, err := store.Thread(ctx, "agent-b", msg.ThreadID, 10)
	require.NoError(t, err)
	require.Len(t, thread, 1)
	require.Equal(t, MailDeliveryStateLeased, thread[0].DeliveryState)

	acked, err := store.AckMail(ctx, "agent-b", msg.ID)
	require.NoError(t, err)
	require.Equal(t, MailDeliveryStateAcked, acked.DeliveryState)
	require.False(t, acked.AckedAt.IsZero())

	deadLettered, err := store.DeadLetterMail(ctx, msg.ID)
	require.ErrorIs(t, err, sql.ErrNoRows)
	require.Equal(t, AgentMail{}, deadLettered)

	unread, err := store.ListInbox(ctx, "agent-b", true, 10)
	require.NoError(t, err)
	require.Len(t, unread, 0)

	row, err := store.SendMail(ctx, AgentMail{
		Address:         "agent-b",
		ToAgent:         "agent-b",
		ResolvedToAgent: "agent-b",
		FromAgent:       "agent-a",
		Subject:         "retry",
		Body:            "please retry",
	})
	require.NoError(t, err)
	redelivery, err := store.LeaseInbox(ctx, "agent-b", "agent-b", 10, time.Nanosecond)
	require.NoError(t, err)
	require.Len(t, redelivery, 1)
	require.Equal(t, row.ID, redelivery[0].ID)
	time.Sleep(time.Millisecond)

	requeued, deadLetters, err := store.RequeueExpiredMailLeases(ctx, 3)
	require.NoError(t, err)
	require.Len(t, requeued, 1)
	require.Empty(t, deadLetters)
	require.Equal(t, MailDeliveryStatePending, requeued[0].DeliveryState)

	releasedAgain, err := store.LeaseInbox(ctx, "agent-b", "agent-b", 10, time.Nanosecond)
	require.NoError(t, err)
	require.Len(t, releasedAgain, 1)
	require.Equal(t, 2, releasedAgain[0].DeliveryAttempts)
	time.Sleep(time.Millisecond)

	deadLettered, err = store.DeadLetterMail(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, MailDeliveryStateDeadLetter, deadLettered.DeliveryState)

	inboxAfterDeadLetter, err := store.ListInbox(ctx, "agent-b", false, 10)
	require.NoError(t, err)
	require.NotEmpty(t, inboxAfterDeadLetter)
	foundExplicitDeadLetter := false
	for _, item := range inboxAfterDeadLetter {
		if item.ID == row.ID {
			require.Equal(t, MailDeliveryStateDeadLetter, item.DeliveryState)
			foundExplicitDeadLetter = true
			break
		}
	}
	require.True(t, foundExplicitDeadLetter)

	_, _, err = store.RequeueExpiredMailLeases(ctx, 1)
	require.NoError(t, err)
	afterDeadLetter, err := store.ListInbox(ctx, "agent-b", false, 10)
	require.NoError(t, err)
	require.NotEmpty(t, afterDeadLetter)
	foundDeadLetter := false
	for _, item := range afterDeadLetter {
		if item.ID == row.ID {
			require.Equal(t, MailDeliveryStateDeadLetter, item.DeliveryState)
			foundDeadLetter = true
			break
		}
	}
	require.True(t, foundDeadLetter)

	require.NoError(t, store.UpsertAgentState(ctx, AgentState{
		AgentID:       "agent-b",
		Role:          "subagent",
		Status:        "running",
		SessionID:     "session-1",
		WorktreePath:  "/tmp/worktree",
		Branch:        "agent/test/task",
		HookBeadID:    "work-1",
		ParentAgentID: "main:session-parent",
		LastHeartbeat: time.Now().UTC(),
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}))
	state, err := store.GetAgentState(ctx, "agent-b")
	require.NoError(t, err)
	require.Equal(t, "work-1", state.HookBeadID)

	require.NoError(t, store.RecordActivity(ctx, AgentActivity{
		AgentID:     "agent-b",
		EventType:   "mail_received",
		DetailsJSON: `{"count":1}`,
		CreatedAt:   time.Now().UTC(),
	}))
	activity, err := store.ListRecentActivity(ctx, "agent-b", 10)
	require.NoError(t, err)
	require.Len(t, activity, 1)
	require.Greater(t, activity[0].RowID, int64(0))

	require.NoError(t, store.UpsertWorkItem(ctx, WorkItem{
		ID:           "work-1",
		Type:         "task",
		Title:        "Implement auth",
		Description:  "Complete the auth flow",
		Status:       "in_progress",
		Assignee:     "agent-b",
		Dependencies: `["dep-1"]`,
		CreatedAt:    time.Now().UTC(),
	}))
	workItem, err := store.GetWorkItem(ctx, "work-1")
	require.NoError(t, err)
	require.Equal(t, "agent-b", workItem.Assignee)

	workItems, err := store.ListWorkItemsByAssignee(ctx, "agent-b", 10)
	require.NoError(t, err)
	require.Len(t, workItems, 1)

	stale, err := store.ListStaleAgentStates(ctx, time.Now().UTC().Add(time.Minute), 10)
	require.NoError(t, err)
	require.Len(t, stale, 1)

	mailRowID, err := store.LatestMailRowID(ctx, "agent-b")
	require.NoError(t, err)
	require.Greater(t, mailRowID, int64(0))

	activityRowID, err := store.LatestActivityRowID(ctx, []string{"agent-b"})
	require.NoError(t, err)
	require.Greater(t, activityRowID, int64(0))
}

func TestStoreDispatchQueueAndCheckpointLifecycle(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	dispatch, err := store.EnqueueDispatch(ctx, DispatchQueueItem{
		SessionID:   "session-1",
		WorkItemID:  "work-queue-1",
		TargetScope: "subagent",
		Status:      "queued",
		Priority:    1,
		PayloadJSON: `{"prompt":"implement health checks"}`,
		AvailableAt: time.Now().UTC(),
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	})
	require.NoError(t, err)
	require.NotEmpty(t, dispatch.ID)

	leased, err := store.LeaseDispatch(ctx, "dispatcher-test", 5)
	require.NoError(t, err)
	require.Len(t, leased, 1)
	require.Equal(t, dispatch.ID, leased[0].ID)
	require.Equal(t, "leased", leased[0].Status)

	leased[0].Status = "running"
	leased[0].LeasedBy = ""
	leased[0].LeasedAt = time.Time{}
	leased[0].AssignedAgentID = "agent-queue-1"
	leased[0].SubmissionID = "submission-1"
	leased[0].UpdatedAt = time.Now().UTC()
	require.NoError(t, store.UpdateDispatch(ctx, leased[0]))

	dispatches, err := store.ListDispatches(ctx, "session-1", []string{"running"}, 10)
	require.NoError(t, err)
	require.Len(t, dispatches, 1)
	require.Equal(t, "agent-queue-1", dispatches[0].AssignedAgentID)

	checkpoint, err := store.SaveCheckpoint(ctx, SessionCheckpoint{
		SessionID:          "session-1",
		AgentID:            "main:session-1",
		WorkItemID:         "work-queue-1",
		ParentCheckpointID: "checkpoint-0",
		MessageCount:       51,
		SummaryJSON:        `{"phase":"resume","status":"running"}`,
		AuditTail:          "latest audit line",
		PendingTasksJSON:   `["finish health checks"]`,
		FilesModifiedJSON:  `["internal/agent/coordinator.go"]`,
		MailCursor:         11,
		ActivityCursor:     22,
		CreatedAt:          time.Now().UTC(),
	})
	require.NoError(t, err)
	require.NotEmpty(t, checkpoint.ID)

	latest, err := store.LatestCheckpoint(ctx, "session-1", "main:session-1")
	require.NoError(t, err)
	require.Equal(t, "work-queue-1", latest.WorkItemID)
	require.Equal(t, 51, latest.MessageCount)
	require.Contains(t, latest.SummaryJSON, `"phase":"resume"`)

	_, err = store.SaveDecision(ctx, DecisionRecord{
		SessionID:          "session-1",
		Category:           "architecture",
		Key:                "database",
		Value:              "postgresql",
		Confidence:         "confirmed",
		SourceCheckpointID: checkpoint.ID,
		CreatedAt:          time.Now().UTC(),
	})
	require.NoError(t, err)
	decisions, err := store.ListDecisionRecords(ctx, "session-1", 10)
	require.NoError(t, err)
	require.Len(t, decisions, 1)

	require.NoError(t, store.UpsertUserPreference(ctx, UserPreference{
		Key:             "user.name",
		Value:           "Harshit",
		Confidence:      "confirmed",
		SourceSessionID: "session-1",
		UpdatedAt:       time.Now().UTC(),
	}))
	prefs, err := store.ListUserPreferences(ctx, 10)
	require.NoError(t, err)
	require.Len(t, prefs, 1)
}

func TestOpenMigratesLegacyWorkItemsSchema(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	dbPath := filepath.Join(dataDir, "orchestration.db")

	conn, err := sql.Open("sqlite", "file:"+dbPath)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
	})

	_, err = conn.ExecContext(ctx, `CREATE TABLE work_items (
		id TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		title TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL,
		assignee TEXT NOT NULL DEFAULT '',
		created_at INTEGER NOT NULL,
		closed_at INTEGER NOT NULL DEFAULT 0
	);`)
	require.NoError(t, err)

	store, err := Open(ctx, dataDir)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	require.NoError(t, store.UpsertWorkItem(ctx, WorkItem{
		ID:           "legacy-work-item",
		Type:         "task",
		Title:        "Legacy row",
		Status:       "open",
		ParentID:     "parent-1",
		ConvoyID:     "convoy-1",
		Dependencies: `["dep-1"]`,
		CreatedAt:    time.Now().UTC(),
	}))

	item, err := store.GetWorkItem(ctx, "legacy-work-item")
	require.NoError(t, err)
	require.Equal(t, "parent-1", item.ParentID)
	require.Equal(t, "convoy-1", item.ConvoyID)
	require.Equal(t, `["dep-1"]`, item.Dependencies)
}

func TestOpenMigratesLegacyAgentMailSchema(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	dbPath := filepath.Join(dataDir, "orchestration.db")

	conn, err := sql.Open("sqlite", "file:"+dbPath)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
	})

	_, err = conn.ExecContext(ctx, `CREATE TABLE agent_mail (
		id TEXT PRIMARY KEY,
		to_agent TEXT NOT NULL,
		from_agent TEXT NOT NULL,
		subject TEXT NOT NULL,
		body TEXT NOT NULL,
		priority INTEGER NOT NULL DEFAULT 0,
		thread_id TEXT NOT NULL,
		read INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL,
		read_at INTEGER NOT NULL DEFAULT 0
	);`)
	require.NoError(t, err)

	_, err = conn.ExecContext(ctx, `INSERT INTO agent_mail (
		id, to_agent, from_agent, subject, body, priority, thread_id, read, created_at, read_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"legacy-mail-1",
		"agent-b",
		"agent-a",
		"handoff",
		"legacy body",
		1,
		"thread-1",
		0,
		time.Now().UTC().Unix(),
		0,
	)
	require.NoError(t, err)

	store, err := Open(ctx, dataDir)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	inbox, err := store.ListInbox(ctx, "agent-b", false, 10)
	require.NoError(t, err)
	require.Len(t, inbox, 1)
	require.Equal(t, "legacy-mail-1", inbox[0].ID)
	require.Equal(t, "agent-b", inbox[0].ResolvedToAgent)
	require.Equal(t, MailDeliveryStatePending, inbox[0].DeliveryState)
}

func TestWorktreeRunLifecycle(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	run := WorktreeRun{
		ID:           "wt-1",
		SessionID:    "session-1",
		AgentID:      "main:session-1",
		Kind:         "main",
		Policy:       "isolated_worktree",
		Status:       "ready",
		RepoRoot:     "/repo",
		WorktreePath: "/repo/.sapphire/worktrees/main/session-1/main",
		Branch:       "session/session-1/main",
		BaseRef:      "main",
		Title:        "Main Worktree",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	require.NoError(t, store.UpsertWorktreeRun(ctx, run))

	loaded, err := store.GetWorktreeRun(ctx, run.ID)
	require.NoError(t, err)
	require.Equal(t, run.WorktreePath, loaded.WorktreePath)
	require.Equal(t, run.Policy, loaded.Policy)

	byPath, err := store.GetWorktreeRunByPath(ctx, run.WorktreePath)
	require.NoError(t, err)
	require.Equal(t, run.ID, byPath.ID)

	run.Status = "landed"
	run.LandedAt = time.Now().UTC()
	run.UpdatedAt = time.Now().UTC()
	require.NoError(t, store.UpsertWorktreeRun(ctx, run))

	items, err := store.ListWorktreeRuns(ctx, "session-1", []string{"landed"}, 10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "landed", items[0].Status)
}
