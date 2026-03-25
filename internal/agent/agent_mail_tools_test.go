package agent

import (
	"context"
	"testing"

	agentmailbox "github.com/duggal1/Sapphire-cli/internal/agent/mailbox"
	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
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

func TestNudgeMailboxRecipientDoesNotEnqueueHiddenSubAgentRun(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, err := orchestrationdb.Open(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	coord := &coordinator{
		mailbox:          agentmailbox.NewService(store, nil),
		subAgentRegistry: newSubAgentRegistry(),
	}
	runner := &subAgentRunner{
		id:           "agent-child",
		sessionID:    "child-session",
		status:       subAgentStatusCompleted,
		submissions:  make(map[string]*subAgentSubmission),
		statusBroker: nil,
	}
	coord.subAgentRegistry.upsert(runner.id, runner)

	err = coord.nudgeMailboxRecipient(ctx, runner.id)
	require.NoError(t, err)

	runner.mu.Lock()
	defer runner.mu.Unlock()
	require.Equal(t, 0, runner.pending)
	require.Empty(t, runner.lastSubmission)
	require.Len(t, runner.submissions, 0)
	require.Equal(t, subAgentStatusCompleted, runner.status)
}
