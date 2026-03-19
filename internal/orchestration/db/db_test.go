package orchestrationdb

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStoreMailLifecycleAndState(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	msg, err := store.SendMail(ctx, AgentMail{
		ToAgent:   "agent-b",
		FromAgent: "agent-a",
		Subject:   "handoff",
		Body:      "api is ready",
		Priority:  2,
	})
	require.NoError(t, err)
	require.NotEmpty(t, msg.ID)
	require.NotEmpty(t, msg.ThreadID)

	inbox, err := store.ListInbox(ctx, "agent-b", true, 10)
	require.NoError(t, err)
	require.Len(t, inbox, 1)
	require.Equal(t, "agent-a", inbox[0].FromAgent)

	thread, err := store.Thread(ctx, "agent-b", msg.ThreadID, 10)
	require.NoError(t, err)
	require.Len(t, thread, 1)

	require.NoError(t, store.MarkRead(ctx, "agent-b", msg.ID))
	unread, err := store.ListInbox(ctx, "agent-b", true, 10)
	require.NoError(t, err)
	require.Len(t, unread, 0)

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
		SessionID:      "session-1",
		AgentID:        "main:session-1",
		WorkItemID:     "work-queue-1",
		SummaryJSON:    `{"phase":"resume","status":"running"}`,
		AuditTail:      "latest audit line",
		MailCursor:     11,
		ActivityCursor: 22,
		CreatedAt:      time.Now().UTC(),
	})
	require.NoError(t, err)
	require.NotEmpty(t, checkpoint.ID)

	latest, err := store.LatestCheckpoint(ctx, "session-1", "main:session-1")
	require.NoError(t, err)
	require.Equal(t, "work-queue-1", latest.WorkItemID)
	require.Contains(t, latest.SummaryJSON, `"phase":"resume"`)
}
