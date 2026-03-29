package cmd

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/duggal1/Sapphire-cli/internal/config"
	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
	"github.com/stretchr/testify/require"
)

func TestStopDurableBackgroundStateWithoutRuntimeHeartbeat(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	cfg, err := config.Init(root, filepath.Join(root, ".sapphire"), false)
	require.NoError(t, err)

	store, err := orchestrationdb.Open(ctx, cfg.Options.DataDirectory)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	now := time.Now().UTC()
	require.NoError(t, store.UpsertAgentState(ctx, orchestrationdb.AgentState{
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
	require.NoError(t, store.UpsertWorkItem(ctx, orchestrationdb.WorkItem{
		ID:          "work-1",
		Type:        "task",
		Title:       "Work One",
		Description: "Investigate lag",
		Status:      "in_progress",
		Assignee:    "agent-1",
		CreatedAt:   now.Add(-2 * time.Minute),
	}))
	_, err = store.EnqueueDispatch(ctx, orchestrationdb.DispatchQueueItem{
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
	_, err = store.SendMail(ctx, orchestrationdb.AgentMail{
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

	summary, err := stopDurableBackgroundState(ctx, cfg.Options.DataDirectory, backgroundStopReason)
	require.NoError(t, err)
	require.Equal(t, 1, summary.StoppedDispatches)
	require.Equal(t, 1, summary.BlockedWorkItems)
	require.Equal(t, 1, summary.BlockedAgentState)
	require.Equal(t, 1, summary.DeadLetteredMail)

	workItem, err := store.GetWorkItem(ctx, "work-1")
	require.NoError(t, err)
	require.Equal(t, "closed", workItem.Status)

	state, err := store.GetAgentState(ctx, "agent-1")
	require.NoError(t, err)
	require.Equal(t, "closed", state.Status)
}
