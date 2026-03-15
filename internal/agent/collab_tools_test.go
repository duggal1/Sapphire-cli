package agent

import (
	"context"
	"testing"
	"time"

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

func TestSendInputParamsAcceptTaskAlias(t *testing.T) {
	t.Parallel()

	var params SendInputParams
	err := params.UnmarshalJSON([]byte(`{"agent_id":"agent-123","task":"continue and summarize","interrupt":true}`))
	require.NoError(t, err)
	require.Equal(t, "agent-123", params.ID)
	require.Equal(t, "continue and summarize", params.Message)
	require.True(t, params.Interrupt)
}

func TestWaitSubAgentsReturnsAfterLifecycleEvent(t *testing.T) {
	t.Parallel()

	coord := &coordinator{
		subAgents:        make(map[string]*subAgentRunner),
		subAgentRegistry: newSubAgentRegistry(),
	}
	runner := &subAgentRunner{
		id:          "agent-1",
		sessionID:   "session-1",
		status:      subAgentStatusRunning,
		submissions: make(map[string]*subAgentSubmission),
		assignment:  subAgentAssignment{Task: "Investigate"},
	}
	coord.subAgentRegistry.upsert("agent-1", runner)

	done := make(chan struct{})
	go func() {
		defer close(done)
		time.Sleep(25 * time.Millisecond)
		runner.mu.Lock()
		runner.status = subAgentStatusIdle
		runner.lastResult = "done"
		payload := runner.lifecycleEventLocked("submission-1", SubAgentStageCompleted, "")
		runner.mu.Unlock()
		publishSubAgentLifecycleEvent(SubAgentCompletedEvent, payload)
	}()

	start := time.Now()
	snapshots, timedOut := coord.waitSubAgents(context.Background(), []string{"agent-1"}, 2*time.Second)
	require.False(t, timedOut)
	require.Len(t, snapshots, 1)
	require.Equal(t, subAgentStatusIdle, snapshots[0].Status)
	require.Less(t, time.Since(start), 500*time.Millisecond)
	<-done
}
