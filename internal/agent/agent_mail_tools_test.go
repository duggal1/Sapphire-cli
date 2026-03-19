package agent

import (
	"testing"

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
