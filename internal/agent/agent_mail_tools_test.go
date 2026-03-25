package agent

import (
	"context"
	"testing"
	"time"

	agentmailbox "github.com/duggal1/Sapphire-cli/internal/agent/mailbox"
	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
	"github.com/duggal1/Sapphire-cli/internal/pubsub"
	"github.com/stretchr/testify/require"
)

func TestMailboxIdentityAndTargetResolution(t *testing.T) {
	coord := &coordinator{
		subAgentRegistry: newSubAgentRegistry(),
	}
	runner := &subAgentRunner{
		id:            "agent-child",
		sessionID:     "child-session",
		parentSession: "parent-session",
	}
	coord.subAgentRegistry.upsert(runner.id, runner)

	require.Equal(t, "agent-child", coord.mailboxIdentityForSession("child-session"))
	require.Equal(t, "main:parent-session", mainAgentMailboxID("parent-session"))

	target, err := coord.resolveMailTarget("child-session", "main")
	require.NoError(t, err)
	require.Equal(t, "main:parent-session", target)

	target, err = coord.resolveMailTarget("child-session", "self")
	require.NoError(t, err)
	require.Equal(t, "agent-child", target)

	target, err = coord.resolveMailTarget("parent-session", "self")
	require.NoError(t, err)
	require.Equal(t, "main:parent-session", target)
}

func TestNudgeMailboxRecipientQueuesDurableDispatchWithoutHiddenRun(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, err := orchestrationdb.Open(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	coord := &coordinator{
		orchestrationStore: store,
		mailbox:          agentmailbox.NewService(store, nil),
		subAgentRegistry: newSubAgentRegistry(),
	}
	runner := &subAgentRunner{
		id:            "agent-child",
		sessionID:     "child-session",
		parentSession: "parent-session",
		status:        subAgentStatusCompleted,
		submissions:   make(map[string]*subAgentSubmission),
		inputCh:       make(chan subAgentInput, 1),
		assignment: subAgentAssignment{
			CreatedAt: time.Now().UTC(),
		},
	}
	coord.subAgentRegistry.upsert(runner.id, runner)
	_, err = store.SendMail(ctx, orchestrationdb.AgentMail{
		ToAgent:   runner.id,
		FromAgent: "main:parent-session",
		Subject:   "SUBAGENT_DONE",
		Body:      "check inbox",
		CreatedAt: time.Now().UTC(),
	})
	require.NoError(t, err)

	err = coord.nudgeMailboxRecipient(ctx, runner.id)
	require.NoError(t, err)

	dispatches, err := store.ListDispatchesByWorkItem(ctx, mailNudgeDispatchWorkItem(runner.id), []string{"queued"}, 10)
	require.NoError(t, err)
	require.Len(t, dispatches, 1)
	require.Equal(t, dispatchTargetMailNudge, dispatches[0].TargetScope)

	runner.mu.Lock()
	defer runner.mu.Unlock()
	require.Equal(t, 0, runner.pending)
	require.Empty(t, runner.lastSubmission)
	require.Len(t, runner.submissions, 0)
	require.Equal(t, subAgentStatusCompleted, runner.status)
}

func TestDispatchMailNudgeItemRequeuesBusyAgentAndWakesIdleAgent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("requeues busy agent", func(t *testing.T) {
		store, err := orchestrationdb.Open(ctx, t.TempDir())
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, store.Close())
		})

		coord := &coordinator{
			orchestrationStore: store,
			mailbox:            agentmailbox.NewService(store, nil),
			subAgentRegistry:   newSubAgentRegistry(),
		}
		runner := &subAgentRunner{
			id:            "agent-busy",
			sessionID:     "child-session",
			parentSession: "parent-session",
			status:        subAgentStatusRunning,
			submissions:   make(map[string]*subAgentSubmission),
			inputCh:       make(chan subAgentInput, 1),
			assignment: subAgentAssignment{
				CreatedAt: time.Now().UTC(),
			},
		}
		coord.subAgentRegistry.upsert(runner.id, runner)
		_, err = coord.mailbox.Send(ctx, runner.id, "main:parent-session", "STATUS_CHECK", "follow up", agentmailbox.SendOptions{SkipNudge: true})
		require.NoError(t, err)

		dispatchID, err := coord.enqueueAgentNudgeDispatch(ctx, runner.id, mailboxNudgePrompt)
		require.NoError(t, err)

		dispatches, err := store.ListDispatchesByWorkItem(ctx, mailNudgeDispatchWorkItem(runner.id), []string{"queued"}, 10)
		require.NoError(t, err)
		require.Len(t, dispatches, 1)
		require.Equal(t, dispatchID, dispatches[0].ID)

		err = coord.dispatchQueuedItem(ctx, dispatches[0])
		require.NoError(t, err)

		requeued, err := store.ListDispatchesByWorkItem(ctx, mailNudgeDispatchWorkItem(runner.id), []string{"queued"}, 10)
		require.NoError(t, err)
		require.Len(t, requeued, 1)
		require.Equal(t, "recipient still busy", requeued[0].LastError)
		require.True(t, requeued[0].AvailableAt.After(time.Now().UTC()))
	})

	t.Run("wakes idle agent", func(t *testing.T) {
		store, err := orchestrationdb.Open(ctx, t.TempDir())
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, store.Close())
		})

		coord := &coordinator{
			orchestrationStore: store,
			mailbox:            agentmailbox.NewService(store, nil),
			subAgentRegistry:   newSubAgentRegistry(),
		}
		runner := &subAgentRunner{
			id:            "agent-idle",
			sessionID:     "child-session",
			parentSession: "parent-session",
			status:        subAgentStatusCompleted,
			submissions:   make(map[string]*subAgentSubmission),
			inputCh:       make(chan subAgentInput, 1),
			statusBroker:  pubsub.NewBroker[subAgentStatus](),
			assignment: subAgentAssignment{
				CreatedAt: time.Now().UTC(),
			},
		}
		coord.subAgentRegistry.upsert(runner.id, runner)
		_, err = coord.mailbox.Send(ctx, runner.id, "main:parent-session", "STATUS_CHECK", "follow up", agentmailbox.SendOptions{SkipNudge: true})
		require.NoError(t, err)

		_, err = coord.enqueueAgentNudgeDispatch(ctx, runner.id, mailboxNudgePrompt)
		require.NoError(t, err)

		dispatches, err := store.ListDispatchesByWorkItem(ctx, mailNudgeDispatchWorkItem(runner.id), []string{"queued"}, 10)
		require.NoError(t, err)
		require.Len(t, dispatches, 1)

		err = coord.dispatchQueuedItem(ctx, dispatches[0])
		require.NoError(t, err)

		running, err := store.ListDispatchesByWorkItem(ctx, mailNudgeDispatchWorkItem(runner.id), []string{"running"}, 10)
		require.NoError(t, err)
		require.Len(t, running, 1)
		require.Equal(t, runner.id, running[0].AssignedAgentID)
		require.NotEmpty(t, running[0].SubmissionID)

		runner.mu.Lock()
		defer runner.mu.Unlock()
		require.Equal(t, 1, runner.pending)
		require.NotNil(t, runner.submissions[running[0].SubmissionID])
		require.Equal(t, subAgentStatusQueued, runner.status)
	})
}
