package orchestrationdb

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStoreStopBackgroundActivityBlocksDurableBackgroundState(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	now := time.Now().UTC()
	require.NoError(t, store.UpsertAgentState(ctx, AgentState{
		AgentID:       "agent-1",
		Role:          "subagent",
		Status:        "running",
		SessionID:     "session-1",
		HookBeadID:    "work-1",
		ParentAgentID: "main:session-1",
		LastHeartbeat: now.Add(-time.Minute),
		CreatedAt:     now.Add(-2 * time.Minute),
		UpdatedAt:     now.Add(-time.Minute),
	}))
	_, err = store.EnqueueDispatch(ctx, DispatchQueueItem{
		ID:              "dispatch-1",
		SessionID:       "session-1",
		WorkItemID:      "work-1",
		TargetScope:     "subagent",
		Status:          "running",
		PayloadJSON:     `{"prompt":"continue"}`,
		AssignedAgentID: "agent-1",
		SubmissionID:    "submission-1",
		AvailableAt:     now,
		CreatedAt:       now.Add(-2 * time.Minute),
		UpdatedAt:       now.Add(-time.Minute),
	})
	require.NoError(t, err)
	require.NoError(t, store.UpsertWorkItem(ctx, WorkItem{
		ID:          "work-1",
		Type:        "task",
		Title:       "Work One",
		Description: "Investigate lag",
		Status:      "in_progress",
		Assignee:    "agent-1",
		CreatedAt:   now.Add(-2 * time.Minute),
	}))
	_, err = store.SendMail(ctx, AgentMail{
		ID:              "mail-1",
		Address:         "agent-1",
		ToAgent:         "agent-1",
		ResolvedToAgent: "agent-1",
		FromAgent:       "main:session-1",
		Subject:         "Need update",
		Body:            "Please respond",
		CreatedAt:       now.Add(-2 * time.Minute),
	})
	require.NoError(t, err)

	summary, err := store.StopBackgroundActivity(ctx, "background activity stopped by user")
	require.NoError(t, err)
	require.Equal(t, 1, summary.StoppedDispatches)
	require.Equal(t, 1, summary.BlockedWorkItems)
	require.Equal(t, 1, summary.BlockedAgentState)
	require.Equal(t, 1, summary.DeadLetteredMail)

	dispatches, err := store.ListDispatchesByWorkItem(ctx, "work-1", []string{"blocked"}, 10)
	require.NoError(t, err)
	require.Len(t, dispatches, 1)
	require.Empty(t, dispatches[0].AssignedAgentID)
	require.Empty(t, dispatches[0].SubmissionID)

	state, err := store.GetAgentState(ctx, "agent-1")
	require.NoError(t, err)
	require.Equal(t, "closed", state.Status)

	workItem, err := store.GetWorkItem(ctx, "work-1")
	require.NoError(t, err)
	require.Equal(t, "closed", workItem.Status)
	require.Empty(t, workItem.Assignee)
	require.False(t, workItem.ClosedAt.IsZero())

	mail, err := store.ListInbox(ctx, "agent-1", false, 10)
	require.NoError(t, err)
	require.Len(t, mail, 1)
	require.Equal(t, MailDeliveryStateDeadLetter, mail[0].DeliveryState)
	require.True(t, mail[0].Read)

	actionable, err := store.ListActionableMail(ctx, "agent-1", 10)
	require.NoError(t, err)
	require.Empty(t, actionable)
}
