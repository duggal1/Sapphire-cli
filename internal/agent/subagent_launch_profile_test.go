package agent

import (
	"context"
	"sync"
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

func TestSubAgentBurstLaunchProfileUsesSharedStartupContext(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.Providers.Set("test-provider", config.ProviderConfig{ID: "test-provider"})

	metrics := newSubAgentLaunchMetrics()
	coord := &coordinator{
		cfg:                       cfg,
		sessions:                  env.sessions,
		messages:                  env.messages,
		memory:                    orchestrationMemoryStub{},
		backgroundSubAgentLimiter: make(chan struct{}, maxBackgroundSubAgents),
		subAgents:                 make(map[string]*subAgentRunner),
		subAgentRegistry:          newSubAgentRegistry(),
		subAgentLaunchProbe:       metrics,
		subAgentLaunchMemoryCache: make(map[string]subAgentLaunchMemoryCacheEntry),
		subAgentLaunchMemoryWork:  make(map[string]*subAgentLaunchMemoryFlight),
	}
	coord.subAgentFactory = func(ctx context.Context, workDir string, normalizedManifest []string, opts spawnAgentOptions) (SessionAgent, error) {
		return newMockAgent("test-provider", 4096, func(_ context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
			time.Sleep(15 * time.Millisecond)
			return agentResultWithText("STATUS: done\nSUMMARY: profiled spawn burst"), nil
		}), nil
	}

	parentSession, err := env.sessions.Create(context.Background(), "Parent")
	require.NoError(t, err)

	ids := make([]string, 0, 5)
	for i := 0; i < 5; i++ {
		agentID, submissionID, err := coord.spawnSubAgent(context.Background(), parentSession.ID, spawnAgentOptions{
			Prompt: "Inspect your assigned domain only and return a short summary.",
			Title:  "Profiled Burst Sub-Agent",
		})
		require.NoError(t, err)
		require.NotEmpty(t, submissionID)
		ids = append(ids, agentID)
	}

	deadline := time.Now().Add(5 * time.Second)
	var statuses []subAgentStatusEntry
	for {
		var timedOut bool
		statuses, timedOut = coord.waitSubAgentStatuses(context.Background(), ids, time.Until(deadline))
		require.False(t, timedOut)
		allCompleted := len(statuses) == len(ids)
		for _, status := range statuses {
			if status.Status != subAgentStatusCompleted {
				allCompleted = false
				break
			}
		}
		if allCompleted {
			break
		}
		require.True(t, time.Now().Before(deadline), "sub-agent burst did not finish before deadline")
	}
	for _, id := range ids {
		require.NoError(t, coord.closeSubAgent(id))
	}

	steps, counters := metrics.snapshot()
	t.Logf("burst sub-agent launch profile: steps=%+v counters=%+v", steps, counters)
	require.Equal(t, int64(1), counters["subagent_memory.launch_context_cache_miss"])
	require.GreaterOrEqual(t, counters["subagent_memory.launch_context_cache_hit"]+counters["subagent_memory.launch_context_flight_wait"], int64(4))
	require.Equal(t, int64(5), counters["subagent_memory.launch_lightweight"])
	require.NotZero(t, steps["turn.build_memory_context"])
}

func TestSpawnedSubAgentsUseIsolatedSessions(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.Providers.Set("test-provider", config.ProviderConfig{ID: "test-provider"})

	var (
		mu       sync.Mutex
		sessions []string
	)

	coord := &coordinator{
		cfg:                       cfg,
		sessions:                  env.sessions,
		messages:                  env.messages,
		backgroundSubAgentLimiter: make(chan struct{}, maxBackgroundSubAgents),
		subAgents:                 make(map[string]*subAgentRunner),
		subAgentRegistry:          newSubAgentRegistry(),
	}
	coord.subAgentFactory = func(ctx context.Context, workDir string, normalizedManifest []string, opts spawnAgentOptions) (SessionAgent, error) {
		return newMockAgent("test-provider", 4096, func(_ context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
			mu.Lock()
			sessions = append(sessions, call.SessionID)
			mu.Unlock()
			return agentResultWithText("STATUS: done\nSUMMARY: isolated session"), nil
		}), nil
	}

	parentSession, err := env.sessions.Create(context.Background(), "Parent")
	require.NoError(t, err)

	ids := make([]string, 0, 2)
	for i := 0; i < 2; i++ {
		agentID, _, err := coord.spawnSubAgent(context.Background(), parentSession.ID, spawnAgentOptions{
			Prompt: "Inspect a distinct subsystem and return concise findings.",
			Title:  "Isolated Session Check",
		})
		require.NoError(t, err)
		ids = append(ids, agentID)
	}

	statuses, timedOut := coord.waitSubAgentStatuses(context.Background(), ids, 3*time.Second)
	require.False(t, timedOut)
	require.Len(t, statuses, 2)

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(sessions) == 2
	}, 3*time.Second, 20*time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	require.NotEqual(t, sessions[0], sessions[1])
	require.NotEqual(t, parentSession.ID, sessions[0])
	require.NotEqual(t, parentSession.ID, sessions[1])
}
