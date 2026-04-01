package agent

import (
	"context"
	"testing"

	"charm.land/fantasy"
	agenttools "github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/stretchr/testify/require"
)

type recordingToolObserver struct {
	inputStarts []string
	calls       []string
	results     []string
}

func (r *recordingToolObserver) OnToolInputStart(sessionID, toolName string) {
	r.inputStarts = append(r.inputStarts, toolName)
}

func (r *recordingToolObserver) OnToolCall(sessionID, toolName, rawInput string) {
	r.calls = append(r.calls, toolName)
}

func (r *recordingToolObserver) OnToolResult(sessionID, toolName, content, metadata string, isError bool) {
	if isError {
		r.results = append(r.results, toolName+":error")
		return
	}
	r.results = append(r.results, toolName)
}

func TestAutoVerifyPendingArtifactsClearsVerificationGuardrail(t *testing.T) {
	t.Parallel()

	usage := agenttools.NewToolUsageState()
	ctx := context.WithValue(context.Background(), agenttools.ToolUsageStateContextKey, usage)
	ctx = context.WithValue(ctx, agenttools.WorkingDirContextKey, "/repo")

	agenttools.ObserveSuccessfulTurnGuardrailResult(ctx, agenttools.WriteToolName, `{"file_path":"AGENTS.md","content":"# guide"}`, false)
	require.Len(t, usage.PendingArtifactVerificationPaths(), 1)

	verifyTool := fantasy.NewAgentTool(
		agenttools.SingleViewToolName,
		"",
		func(ctx context.Context, params agenttools.ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse("# Repository Agent Instructions"), nil
		},
	)

	observer := &recordingToolObserver{}
	a := &sessionAgent{toolObserver: observer}

	err := a.autoVerifyPendingArtifacts(ctx, "session-auto-verify", []fantasy.AgentTool{verifyTool})
	require.NoError(t, err)
	require.Empty(t, usage.PendingArtifactVerificationPaths())
	require.Equal(t, []string{agenttools.SingleViewToolName}, observer.inputStarts)
	require.Equal(t, []string{agenttools.SingleViewToolName}, observer.calls)
	require.Equal(t, []string{agenttools.SingleViewToolName}, observer.results)
}
