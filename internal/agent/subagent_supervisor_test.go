package agent

import (
	"testing"

	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/stretchr/testify/require"
)

func TestValidateSubAgentLaunchReturnsConfiguredThreadLimit(t *testing.T) {
	env := testEnv(t)
	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)

	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.Options.AgentMaxThreads = 2

	coord := &coordinator{
		cfg:              cfg,
		sessions:         env.sessions,
		subAgentRegistry: newSubAgentRegistry(),
	}

	coord.subAgentRegistry.upsert("agent-1", &subAgentRunner{
		parentSession: parent.ID,
		status:        subAgentStatusRunning,
	})
	coord.subAgentRegistry.upsert("agent-2", &subAgentRunner{
		parentSession: parent.ID,
		status:        subAgentStatusRunning,
	})

	_, err = coord.validateSubAgentLaunch(t.Context(), parent.ID, "delegate this task")
	require.Error(t, err)
	require.Equal(t, "sub-agent limit reached: system currently allows up to 2 concurrent sub-agents; 2 already active", err.Error())
}
