package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"charm.land/fantasy"
	agentactivity "github.com/duggal1/Sapphire-cli/internal/agent/activity"
	agentmailbox "github.com/duggal1/Sapphire-cli/internal/agent/mailbox"
	agentstate "github.com/duggal1/Sapphire-cli/internal/agent/state"
	agentsupervisor "github.com/duggal1/Sapphire-cli/internal/agent/supervisor"
	"github.com/duggal1/Sapphire-cli/internal/config"
	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
	"github.com/stretchr/testify/require"
)

func TestSpawnedSubAgentReceivesSupervisorRecoveryNudge(t *testing.T) {
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
	}
	coord.supervisor = agentsupervisor.NewService(store, coord.stateService, coord.activityService, coord.mailbox, agentsupervisor.Hooks{
		GetRuntimeSnapshot:   coord.supervisorRuntimeSnapshot,
		ResolveMainMailboxID: func(sessionID string) string { return "main:" + sessionID },
	})

	blockRun := make(chan struct{})
	coord.subAgentFactory = func(ctx context.Context, workDir string, normalizedManifest []string, opts spawnAgentOptions) (SessionAgent, error) {
		agent := newMockAgent("test-provider", 4096, func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		})
		agent.busy = true
		return agent, nil
	}

	parentSession, err := env.sessions.Create(context.Background(), "Parent")
	require.NoError(t, err)

	agentID, _, err := coord.spawnSubAgent(context.Background(), parentSession.ID, spawnAgentOptions{
		Prompt: "Stay running so the supervisor can inspect the live sub-agent",
		Title:  "Supervised Sub-Agent",
	})
	require.NoError(t, err)
	defer func() {
		close(blockRun)
		_ = coord.closeSubAgent(agentID)
	}()

	require.Eventually(t, func() bool {
		runner := coord.ensureSubAgentRegistry().get(agentID)
		if runner == nil {
			return false
		}
		snap := runner.snapshot()
		return snap.Status == subAgentStatusRunning || snap.Status == subAgentStatusReady || snap.Status == subAgentStatusStarting
	}, 2*time.Second, 20*time.Millisecond)

	runner := coord.ensureSubAgentRegistry().get(agentID)
	require.NotNil(t, runner)
	runner.mu.Lock()
	runner.status = subAgentStatusRunning
	runner.lastHeartbeat = time.Now().UTC().Add(-16 * time.Minute)
	runner.heartbeatContext = "integration-test forced stale heartbeat"
	runner.mu.Unlock()
	coord.syncRunnerOrchestrationState(context.Background(), runner)

	require.NoError(t, coord.supervisor.RunPatrolCycle(context.Background()))

	agentInbox, err := store.ListInbox(context.Background(), agentID, false, 20)
	require.NoError(t, err)
	mainInbox, err := store.ListInbox(context.Background(), "main:"+parentSession.ID, false, 20)
	require.NoError(t, err)

	activities, err := coord.activityService.Recent(context.Background(), agentID, 100)
	require.NoError(t, err)
	foundReceipt := false
	foundRecoveryIntervention := false
	for _, item := range activities {
		if item.EventType == "supervisor_patrol_receipt" {
			foundReceipt = true
		}
		if item.EventType == "supervisor_intervention" && strings.Contains(item.DetailsJSON, `"action":"recovery_nudge"`) {
			foundRecoveryIntervention = true
		}
	}

	foundSupervisorMail := false
	for _, msg := range agentInbox {
		if msg.Subject == "RECOVERY_NUDGE" {
			foundSupervisorMail = true
			break
		}
	}
	if !foundSupervisorMail {
		for _, msg := range mainInbox {
			if msg.Subject == "REASSIGNMENT_NOTICE" || msg.Subject == "ESCALATION" {
				foundSupervisorMail = true
				break
			}
		}
	}
	require.True(t, foundReceipt, "expected a patrol receipt for the launched sub-agent")
	require.True(t, foundRecoveryIntervention || foundSupervisorMail, "expected supervisor intervention evidence for the launched sub-agent")
}
