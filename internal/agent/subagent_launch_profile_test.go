package agent

import (
	"context"
	"testing"
	"time"

	"charm.land/fantasy"
	agentactivity "github.com/duggal1/Sapphire-cli/internal/agent/activity"
	agentmailbox "github.com/duggal1/Sapphire-cli/internal/agent/mailbox"
	agentstate "github.com/duggal1/Sapphire-cli/internal/agent/state"
	"github.com/duggal1/Sapphire-cli/internal/config"
	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
	"github.com/stretchr/testify/require"
)

func TestSubAgentLaunchProfileHarnessCapturesSpawnChurn(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.Providers.Set("test-provider", config.ProviderConfig{ID: "test-provider"})

	store, err := orchestrationdb.Open(context.Background(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	metrics := newSubAgentLaunchMetrics()
	coord := &coordinator{
		cfg:                       cfg,
		sessions:                  env.sessions,
		messages:                  env.messages,
		orchestrationStore:        store,
		stateService:              agentstate.NewService(store),
		activityService:           agentactivity.NewService(store),
		mailbox:                   agentmailbox.NewService(store, nil),
		backgroundSubAgentLimiter: make(chan struct{}, maxBackgroundSubAgents),
		subAgents:                 make(map[string]*subAgentRunner),
		subAgentRegistry:          newSubAgentRegistry(),
		subAgentLaunchProbe:       metrics,
	}
	coord.subAgentFactory = func(ctx context.Context, workDir string, normalizedManifest []string, opts spawnAgentOptions) (SessionAgent, error) {
		return newMockAgent("test-provider", 4096, func(_ context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
			time.Sleep(25 * time.Millisecond)
			return agentResultWithText("STATUS: done\nSUMMARY: profiled spawn"), nil
		}), nil
	}

	parentSession, err := env.sessions.Create(context.Background(), "Parent")
	require.NoError(t, err)

	before := captureSubAgentLaunchRuntimeSnapshot()
	agentID, submissionID, err := coord.spawnSubAgent(context.Background(), parentSession.ID, spawnAgentOptions{
		Prompt: "Profile the launch path",
		Title:  "Profiled Sub-Agent",
	})
	require.NoError(t, err)
	require.NotEmpty(t, agentID)
	require.NotEmpty(t, submissionID)

	statuses, timedOut := coord.waitSubAgentStatuses(context.Background(), []string{agentID}, 3*time.Second)
	require.False(t, timedOut)
	require.Len(t, statuses, 1)
	require.Equal(t, subAgentStatusCompleted, statuses[0].Status)

	results := coord.collectSubAgentResults([]string{agentID})
	require.Len(t, results, 1)
	require.Equal(t, subAgentStatusCompleted, results[0].Status)

	require.NoError(t, coord.closeSubAgent(agentID))
	after := captureSubAgentLaunchRuntimeSnapshot()

	steps, counters := metrics.snapshot()
	profile := SubAgentLaunchProfile{
		Before:   before,
		After:    after,
		Steps:    steps,
		Counters: counters,
	}

	t.Logf("sub-agent launch profile: %+v", profile)
	require.NotZero(t, counters["db.agent_state_upsert"])
	require.NotZero(t, counters["db.activity_write"])
	require.NotZero(t, counters["event.subagent_lifecycle_publish"])
	require.NotZero(t, counters["db.message_create"])
	require.LessOrEqual(t, counters["db.agent_state_upsert"], int64(6))
}
