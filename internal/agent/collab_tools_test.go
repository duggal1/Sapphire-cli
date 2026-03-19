package agent

import (
	"context"
	"testing"
	"time"

	"github.com/duggal1/Sapphire-cli/internal/pubsub"
	"github.com/stretchr/testify/require"
)

func TestSpawnAgentParamsAcceptPromptAlias(t *testing.T) {
	t.Parallel()

	var params SpawnAgentParams
	err := params.UnmarshalJSON([]byte(`{
		"prompt":"Investigate flaky tests",
		"agent_type":"coder",
		"branch_name":"fix/flaky-tests",
		"allowed_paths":["internal/agent","internal/ui"],
		"acceptance_criteria":"tests pass"
	}`))
	require.NoError(t, err)
	require.Equal(t, "Investigate flaky tests", params.Message)
	require.Equal(t, "coder", params.Agent)
	require.Equal(t, "fix/flaky-tests", params.Branch)
	require.Equal(t, []string{"internal/agent", "internal/ui"}, params.WriteManifest)
	require.Equal(t, "tests pass", params.DefinitionOfDone)
}

func TestSpawnAgentParamsAcceptItemsPayload(t *testing.T) {
	t.Parallel()

	var params SpawnAgentParams
	err := params.UnmarshalJSON([]byte(`{
		"items":[
			{"type":"text","text":"Investigate the failing worktree flow"},
			{"content":"Return a concise report"}
		]
	}`))
	require.NoError(t, err)
	require.Equal(t, "Investigate the failing worktree flow\nReturn a concise report", params.Message)
}

func TestSpawnAgentParamsAcceptIsolation(t *testing.T) {
	t.Parallel()

	var params SpawnAgentParams
	err := params.UnmarshalJSON([]byte(`{
		"message":"Investigate renderer",
		"isolation":"worktree"
	}`))
	require.NoError(t, err)
	require.Equal(t, "worktree", params.Isolation)

	useWorktree, err := resolveSpawnIsolation(params.Worktree, params.Isolation)
	require.NoError(t, err)
	require.True(t, useWorktree)
}

func TestSendInputParamsAcceptTaskAlias(t *testing.T) {
	t.Parallel()

	var params SendInputParams
	err := params.UnmarshalJSON([]byte(`{"agent_id":"agent-123","task":"continue and summarize","interrupt":true}`))
	require.NoError(t, err)
	require.Equal(t, "agent-123", params.ID)
	require.Equal(t, "continue and summarize", params.Message)
	require.True(t, params.Interrupt)
}

func TestSendInputParamsAcceptItemsPayload(t *testing.T) {
	t.Parallel()

	var params SendInputParams
	err := params.UnmarshalJSON([]byte(`{
		"agent_id":"agent-123",
		"items":[{"text":"continue analysis"},{"content":"return a concise status"}]
	}`))
	require.NoError(t, err)
	require.Equal(t, "agent-123", params.ID)
	require.Equal(t, "continue analysis\nreturn a concise status", params.Message)
	require.Equal(t, []string{"continue analysis", "return a concise status"}, params.Items)
}

func TestWaitSubAgentsReturnsAfterLifecycleEvent(t *testing.T) {
	t.Parallel()

	coord := &coordinator{
		subAgents:        make(map[string]*subAgentRunner),
		subAgentRegistry: newSubAgentRegistry(),
	}
	runner := &subAgentRunner{
		id:           "agent-1",
		sessionID:    "session-1",
		status:       subAgentStatusRunning,
		submissions:  make(map[string]*subAgentSubmission),
		assignment:   subAgentAssignment{Task: "Investigate"},
		statusBroker: pubsub.NewBroker[subAgentStatus](),
	}
	coord.subAgentRegistry.upsert("agent-1", runner)

	done := make(chan struct{})
	go func() {
		defer close(done)
		time.Sleep(25 * time.Millisecond)
		runner.mu.Lock()
		runner.status = subAgentStatusCompleted
		runner.lastResult = "done"
		broker := runner.statusBroker
		payload := runner.lifecycleEventLocked("submission-1", SubAgentStageCompleted, "")
		runner.mu.Unlock()
		publishSubAgentStatus(broker, subAgentStatusCompleted)
		publishSubAgentLifecycleEvent(SubAgentCompletedEvent, payload)
	}()

	start := time.Now()
	snapshots, timedOut := coord.waitSubAgents(context.Background(), []string{"agent-1"}, 2*time.Second)
	require.False(t, timedOut)
	require.Len(t, snapshots, 1)
	require.Equal(t, subAgentStatusCompleted, snapshots[0].Status)
	require.Less(t, time.Since(start), 500*time.Millisecond)
	<-done
}

func TestWaitSubAgentsReturnsWhenAnyAgentFinishes(t *testing.T) {
	t.Parallel()

	coord := &coordinator{
		subAgents:        make(map[string]*subAgentRunner),
		subAgentRegistry: newSubAgentRegistry(),
	}
	completedRunner := &subAgentRunner{
		id:           "agent-done",
		sessionID:    "session-done",
		status:       subAgentStatusRunning,
		submissions:  make(map[string]*subAgentSubmission),
		assignment:   subAgentAssignment{Task: "Done soon"},
		statusBroker: pubsub.NewBroker[subAgentStatus](),
	}
	runningRunner := &subAgentRunner{
		id:           "agent-running",
		sessionID:    "session-running",
		status:       subAgentStatusRunning,
		submissions:  make(map[string]*subAgentSubmission),
		assignment:   subAgentAssignment{Task: "Still running"},
		statusBroker: pubsub.NewBroker[subAgentStatus](),
	}
	coord.subAgentRegistry.upsert("agent-done", completedRunner)
	coord.subAgentRegistry.upsert("agent-running", runningRunner)

	go func() {
		time.Sleep(25 * time.Millisecond)
		completedRunner.mu.Lock()
		completedRunner.status = subAgentStatusCompleted
		completedRunner.lastResult = "done"
		broker := completedRunner.statusBroker
		payload := completedRunner.lifecycleEventLocked("submission-done", SubAgentStageCompleted, "")
		completedRunner.mu.Unlock()
		publishSubAgentStatus(broker, subAgentStatusCompleted)
		publishSubAgentLifecycleEvent(SubAgentCompletedEvent, payload)
	}()

	start := time.Now()
	snapshots, timedOut := coord.waitSubAgents(context.Background(), []string{"agent-done", "agent-running"}, 2*time.Second)
	require.False(t, timedOut)
	require.Len(t, snapshots, 2)
	require.Less(t, time.Since(start), 500*time.Millisecond)

	statuses := map[string]subAgentStatus{}
	for _, snap := range snapshots {
		statuses[snap.ID] = snap.Status
	}
	require.Equal(t, subAgentStatusCompleted, statuses["agent-done"])
	require.Equal(t, subAgentStatusRunning, statuses["agent-running"])
}

func TestWaitSubAgentsMarksStaleRunnerStuck(t *testing.T) {
	t.Parallel()

	coord := &coordinator{
		subAgents:        make(map[string]*subAgentRunner),
		subAgentRegistry: newSubAgentRegistry(),
	}
	runner := &subAgentRunner{
		id:             "agent-stale",
		sessionID:      "session-stale",
		status:         subAgentStatusRunning,
		lastSubmission: "submission-stale",
		lastHeartbeat:  time.Now().UTC().Add(-1 * time.Minute),
		submissions: map[string]*subAgentSubmission{
			"submission-stale": {
				ID:          "submission-stale",
				Status:      subAgentStatusRunning,
				HeartbeatAt: time.Now().UTC().Add(-1 * time.Minute),
			},
		},
		assignment:   subAgentAssignment{Task: "Investigate stall"},
		statusBroker: pubsub.NewBroker[subAgentStatus](),
	}
	coord.subAgentRegistry.upsert("agent-stale", runner)

	snapshots, timedOut := coord.waitSubAgents(context.Background(), []string{"agent-stale"}, 3*time.Second)
	require.False(t, timedOut)
	require.Len(t, snapshots, 1)
	require.Equal(t, subAgentStatusStuck, snapshots[0].Status)
}

func TestCollectSubAgentResultsReturnsLatestSubmissionPayload(t *testing.T) {
	t.Parallel()

	coord := &coordinator{
		subAgents:        make(map[string]*subAgentRunner),
		subAgentRegistry: newSubAgentRegistry(),
	}
	runner := &subAgentRunner{
		id:             "agent-2",
		sessionID:      "session-2",
		status:         subAgentStatusCompleted,
		lastSubmission: "submission-2",
		lastResult:     "summary",
		lastProgress:   "done",
		submissions: map[string]*subAgentSubmission{
			"submission-2": {
				ID:     "submission-2",
				Status: subAgentStatusCompleted,
				Result: "final report",
			},
		},
		assignment: subAgentAssignment{Branch: "feature/test"},
	}
	coord.subAgentRegistry.upsert("agent-2", runner)

	results := coord.collectSubAgentResults([]string{"agent-2"})
	require.Len(t, results, 1)
	require.Equal(t, "agent-2", results[0].ID)
	require.Equal(t, "submission-2", results[0].SubmissionID)
	require.Equal(t, subAgentStatusCompleted, results[0].Status)
	require.Equal(t, "final report", results[0].Result)
	require.Equal(t, "done", results[0].Progress)
	require.Equal(t, "feature/test", results[0].Branch)
}
