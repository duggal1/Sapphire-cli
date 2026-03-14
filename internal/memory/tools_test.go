package memory

import (
	"context"
	"encoding/json"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

func TestMemoryToolsHandleUninitializedSystem(t *testing.T) {
	t.Parallel()

	recallResp := runMemoryTool(t, NewRecallTool(nil), RecallToolName, RecallParams{Query: "auth"})
	require.Contains(t, recallResp.Content, "Memory system not initialized")

	saveResp := runMemoryTool(t, NewSaveTool(nil), SaveToolName, SaveParams{
		EventType: "architectural_decision",
		Content:   json.RawMessage(`{"decision":"keep sqlite"}`),
	})
	require.Contains(t, saveResp.Content, "Memory system not initialized")

	healthResp := runMemoryTool(t, NewHealthTool(nil), HealthToolName, struct{}{})
	require.Contains(t, healthResp.Content, "Memory system not initialized")
}

func runMemoryTool[T any](t *testing.T, tool fantasy.AgentTool, name string, params T) fantasy.ToolResponse {
	t.Helper()

	input, err := json.Marshal(params)
	require.NoError(t, err)

	resp, err := tool.Run(context.Background(), fantasy.ToolCall{
		ID:    name + "-1",
		Name:  name,
		Input: string(input),
	})
	require.NoError(t, err)
	return resp
}
