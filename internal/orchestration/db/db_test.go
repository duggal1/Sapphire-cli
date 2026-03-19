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
		ParentAgentID: "main:session-parent",
		LastHeartbeat: time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}))

	require.NoError(t, store.RecordActivity(ctx, AgentActivity{
		AgentID:     "agent-b",
		EventType:   "mail_received",
		DetailsJSON: `{"count":1}`,
		CreatedAt:   time.Now().UTC(),
	}))
}
