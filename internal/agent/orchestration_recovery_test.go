package agent

import (
	"context"
	"testing"
	"time"

	agentactivity "github.com/duggal1/Sapphire-cli/internal/agent/activity"
	agentmailbox "github.com/duggal1/Sapphire-cli/internal/agent/mailbox"
	agentstate "github.com/duggal1/Sapphire-cli/internal/agent/state"
	agentsupervisor "github.com/duggal1/Sapphire-cli/internal/agent/supervisor"
	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
	"github.com/stretchr/testify/require"
)

func TestRecoverOrchestrationStateReclaimsDispatchesAndMail(t *testing.T) {
	ctx := context.Background()
	env := testEnv(t)
	store, err := orchestrationdb.Open(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	stateService := agentstate.NewService(store)
	activityService := agentactivity.NewService(store)
	mailbox := agentmailbox.NewService(store, nil)
	coord := &coordinator{
		sessions:           env.sessions,
		messages:           env.messages,
		orchestrationStore: store,
		stateService:       stateService,
		activityService:    activityService,
		mailbox:            mailbox,
		subAgentRegistry:   newSubAgentRegistry(),
	}
	coord.supervisor = agentsupervisor.NewService(store, stateService, activityService, mailbox, agentsupervisor.Hooks{
		GetRuntimeSnapshot:        coord.supervisorRuntimeSnapshot,
		ResolveMainMailboxID:      mainAgentMailboxID,
		EnsureDispatchForWorkItem: coord.ensureDispatchForWorkItem,
	})

	now := time.Now().UTC()
	require.NoError(t, store.UpsertAgentState(ctx, orchestrationdb.AgentState{
		AgentID:       "agent-failed",
		Role:          "subagent",
		Status:        "running",
		SessionID:     "sub-session",
		HookBeadID:    "work-running",
		ParentAgentID: mainAgentMailboxID("parent-session"),
		LastHeartbeat: now.Add(-5 * time.Minute),
		CreatedAt:     now.Add(-10 * time.Minute),
		UpdatedAt:     now.Add(-2 * time.Minute),
	}))
	require.NoError(t, store.UpsertAgentState(ctx, orchestrationdb.AgentState{
		AgentID:       "agent-mail",
		Role:          "subagent",
		Status:        "running",
		SessionID:     "mail-session",
		HookBeadID:    "work-mail",
		ParentAgentID: mainAgentMailboxID("parent-session"),
		LastHeartbeat: now,
		CreatedAt:     now.Add(-3 * time.Minute),
		UpdatedAt:     now,
	}))
	require.NoError(t, store.UpsertWorkItem(ctx, orchestrationdb.WorkItem{
		ID:          "work-running",
		Type:        "task",
		Title:       "Recover running work",
		Description: "Continue after restart",
		Status:      "in_progress",
		Assignee:    "agent-failed",
		ParentID:    "parent-session",
		CreatedAt:   now.Add(-10 * time.Minute),
	}))
	require.NoError(t, store.UpsertWorkItem(ctx, orchestrationdb.WorkItem{
		ID:          "work-leased",
		Type:        "task",
		Title:       "Queued work",
		Description: "Dispatch was only leased",
		Status:      "open",
		ParentID:    "parent-session",
		CreatedAt:   now.Add(-10 * time.Minute),
	}))
	require.NoError(t, store.UpsertWorkItem(ctx, orchestrationdb.WorkItem{
		ID:          "work-mail",
		Type:        "task",
		Title:       "Recover mail recipient",
		Description: "Recipient crashed while coordination mail was pending",
		Status:      "open",
		ParentID:    "parent-session",
		CreatedAt:   now.Add(-10 * time.Minute),
	}))
	_, err = store.EnqueueDispatch(ctx, orchestrationdb.DispatchQueueItem{
		ID:              "dispatch-running",
		SessionID:       "parent-session",
		WorkItemID:      "work-running",
		TargetScope:     dispatchTargetSubAgent,
		Status:          "running",
		AssignedAgentID: "agent-failed",
		SubmissionID:    "submission-old",
		PayloadJSON:     `{"prompt":"continue"}`,
		AvailableAt:     now,
		CreatedAt:       now.Add(-10 * time.Minute),
		UpdatedAt:       now,
	})
	require.NoError(t, err)
	_, err = store.EnqueueDispatch(ctx, orchestrationdb.DispatchQueueItem{
		ID:          "dispatch-leased",
		SessionID:   "parent-session",
		WorkItemID:  "work-leased",
		TargetScope: dispatchTargetSubAgent,
		Status:      "leased",
		LeasedBy:    dispatchLeaseOwner,
		LeasedAt:    now.Add(-time.Minute),
		PayloadJSON: `{"prompt":"continue"}`,
		AvailableAt: now,
		CreatedAt:   now.Add(-10 * time.Minute),
		UpdatedAt:   now,
	})
	require.NoError(t, err)

	msg, err := store.SendMail(ctx, orchestrationdb.AgentMail{
		Address:         "agent-mail",
		ToAgent:         "agent-mail",
		ResolvedToAgent: "agent-mail",
		FromAgent:       "main:parent-session",
		Subject:         "recover",
		Body:            "mail should be requeued",
		CreatedAt:       now.Add(-2 * time.Minute),
	})
	require.NoError(t, err)
	leasedMail, err := store.LeaseInbox(ctx, "agent-mail", "agent-mail", 10, time.Nanosecond)
	require.NoError(t, err)
	require.Len(t, leasedMail, 1)
	require.Equal(t, msg.ID, leasedMail[0].ID)
	time.Sleep(time.Millisecond)

	require.NoError(t, coord.recoverOrchestrationState(ctx))

	leasedDispatches, err := store.ListDispatchesByWorkItem(ctx, "work-leased", []string{"queued"}, 10)
	require.NoError(t, err)
	require.Len(t, leasedDispatches, 1)
	require.Empty(t, leasedDispatches[0].LeasedBy)

	requeuedRunning, err := store.ListDispatchesByWorkItem(ctx, "work-running", []string{"queued"}, 10)
	require.NoError(t, err)
	require.Len(t, requeuedRunning, 1)
	require.Empty(t, requeuedRunning[0].AssignedAgentID)
	require.Empty(t, requeuedRunning[0].SubmissionID)
	require.Contains(t, requeuedRunning[0].LastError, "resume sub-agent")

	workItem, err := store.GetWorkItem(ctx, "work-running")
	require.NoError(t, err)
	require.Equal(t, "open", workItem.Status)
	require.Empty(t, workItem.Assignee)

	state, err := store.GetAgentState(ctx, "agent-failed")
	require.NoError(t, err)
	require.Equal(t, string(subAgentStatusError), state.Status)

	actionable, err := mailbox.Actionable(ctx, "agent-mail", 10)
	require.NoError(t, err)
	require.Len(t, actionable, 1)
	require.Equal(t, orchestrationdb.MailDeliveryStatePending, actionable[0].DeliveryState)

	mailRecoveryDispatches, err := store.ListDispatchesByWorkItem(ctx, "work-mail", []string{"queued"}, 10)
	require.NoError(t, err)
	require.Len(t, mailRecoveryDispatches, 1)
	require.Equal(t, dispatchTargetSubAgent, mailRecoveryDispatches[0].TargetScope)
}

func TestRecoverOrchestrationStateBlocksStaleRunningDispatches(t *testing.T) {
	ctx := context.Background()
	env := testEnv(t)
	store, err := orchestrationdb.Open(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	stateService := agentstate.NewService(store)
	activityService := agentactivity.NewService(store)
	mailbox := agentmailbox.NewService(store, nil)
	coord := &coordinator{
		sessions:           env.sessions,
		messages:           env.messages,
		orchestrationStore: store,
		stateService:       stateService,
		activityService:    activityService,
		mailbox:            mailbox,
		subAgentRegistry:   newSubAgentRegistry(),
	}
	coord.supervisor = agentsupervisor.NewService(store, stateService, activityService, mailbox, agentsupervisor.Hooks{
		GetRuntimeSnapshot:        coord.supervisorRuntimeSnapshot,
		ResolveMainMailboxID:      mainAgentMailboxID,
		EnsureDispatchForWorkItem: coord.ensureDispatchForWorkItem,
	})

	now := time.Now().UTC()
	staleAt := now.Add(-startupRecoveryResumeWindow - 10*time.Minute)
	require.NoError(t, store.UpsertAgentState(ctx, orchestrationdb.AgentState{
		AgentID:       "agent-stale",
		Role:          "subagent",
		Status:        "running",
		SessionID:     "stale-session",
		HookBeadID:    "work-stale",
		ParentAgentID: mainAgentMailboxID("parent-session"),
		LastHeartbeat: staleAt,
		CreatedAt:     staleAt,
		UpdatedAt:     staleAt,
	}))
	require.NoError(t, store.UpsertWorkItem(ctx, orchestrationdb.WorkItem{
		ID:          "work-stale",
		Type:        "task",
		Title:       "Stale work",
		Description: "Should not auto-resume",
		Status:      "in_progress",
		Assignee:    "agent-stale",
		ParentID:    "parent-session",
		CreatedAt:   staleAt,
	}))
	_, err = store.EnqueueDispatch(ctx, orchestrationdb.DispatchQueueItem{
		ID:              "dispatch-stale",
		SessionID:       "parent-session",
		WorkItemID:      "work-stale",
		TargetScope:     dispatchTargetSubAgent,
		Status:          "running",
		AssignedAgentID: "agent-stale",
		SubmissionID:    "submission-stale",
		PayloadJSON:     `{"prompt":"continue"}`,
		AvailableAt:     staleAt,
		CreatedAt:       staleAt,
		UpdatedAt:       staleAt,
	})
	require.NoError(t, err)

	require.NoError(t, coord.recoverOrchestrationState(ctx))

	dispatches, err := store.ListDispatchesByWorkItem(ctx, "work-stale", []string{"blocked"}, 10)
	require.NoError(t, err)
	require.Len(t, dispatches, 1)
	require.Empty(t, dispatches[0].AssignedAgentID)
	require.Empty(t, dispatches[0].SubmissionID)
	require.Contains(t, dispatches[0].LastError, "startup recovery skipped stale dispatch")

	state, err := store.GetAgentState(ctx, "agent-stale")
	require.NoError(t, err)
	require.Equal(t, string(subAgentStatusBlocked), state.Status)

	workItem, err := store.GetWorkItem(ctx, "work-stale")
	require.NoError(t, err)
	require.Equal(t, "blocked", workItem.Status)
	require.Empty(t, workItem.Assignee)
}

func TestRecoverOrchestrationStateDeadLettersSupervisorMail(t *testing.T) {
	ctx := context.Background()
	env := testEnv(t)
	store, err := orchestrationdb.Open(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	stateService := agentstate.NewService(store)
	activityService := agentactivity.NewService(store)
	mailbox := agentmailbox.NewService(store, nil)
	coord := &coordinator{
		sessions:           env.sessions,
		messages:           env.messages,
		orchestrationStore: store,
		stateService:       stateService,
		activityService:    activityService,
		mailbox:            mailbox,
		subAgentRegistry:   newSubAgentRegistry(),
	}

	now := time.Now().UTC()
	msg, err := store.SendMail(ctx, orchestrationdb.AgentMail{
		Address:         "supervisor",
		ToAgent:         "supervisor",
		ResolvedToAgent: "supervisor",
		FromAgent:       "agent-a",
		Subject:         "RE: LOOP DETECTED",
		Body:            "ack",
		CreatedAt:       now.Add(-2 * time.Minute),
	})
	require.NoError(t, err)

	require.NoError(t, coord.recoverOrchestrationState(ctx))

	inbox, err := store.ListInbox(ctx, "supervisor", false, 10)
	require.NoError(t, err)
	require.Len(t, inbox, 1)
	require.Equal(t, msg.ID, inbox[0].ID)
	require.Equal(t, orchestrationdb.MailDeliveryStateDeadLetter, inbox[0].DeliveryState)
}
