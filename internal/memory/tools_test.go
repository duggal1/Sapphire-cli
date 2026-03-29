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

	viewResp := runMemoryTool(t, NewViewMemoryTool(nil, nil), ViewToolName, ViewMemoryParams{Mode: "recent"})
	require.Contains(t, viewResp.Content, "Memory system not initialized")

	refreshResp := runMemoryTool(t, NewRefreshTool(nil, nil), RefreshToolName, RefreshParams{})
	require.Contains(t, refreshResp.Content, "Memory system not initialized")

	recallResp := runMemoryTool(t, NewRecallTool(nil, nil), RecallToolName, RecallParams{Query: "auth"})
	require.Contains(t, recallResp.Content, "Memory system not initialized")

	saveResp := runMemoryTool(t, NewSaveTool(nil, nil), SaveToolName, SaveParams{
		EventType: "architectural_decision",
		Content:   json.RawMessage(`{"decision":"keep sqlite"}`),
	})
	require.Contains(t, saveResp.Content, "Memory system not initialized")

	healthResp := runMemoryTool(t, NewHealthTool(nil), HealthToolName, struct{}{})
	require.Contains(t, healthResp.Content, "Memory system not initialized")
}

func TestSaveToolArchitecturalDecisionPersistsConstitution(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	system, err := NewSystem(t.Context(), "session-1", Config{
		DataDir:     t.TempDir(),
		ProjectRoot: repoRoot,
	})
	require.NoError(t, err)
	require.NotNil(t, system)
	t.Cleanup(system.Close)

	saveResp := runMemoryTool(t, NewSaveTool(system, func(context.Context) string { return "session-1" }), SaveToolName, SaveParams{
		EventType: "architectural_decision",
		Content:   json.RawMessage(`{"decision":"Always read the full target file before editing it."}`),
	})
	require.Contains(t, saveResp.Content, "Memory saved: architectural_decision")

	constitution, err := system.Store.GetConstitution(context.Background())
	require.NoError(t, err)
	require.Contains(t, constitution, "Always read the full target file before editing it.")

	count, err := system.Store.CountRecords(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
}

func TestSaveToolNormalizesByteArrayArchitecturalDecisionContent(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	system, err := NewSystem(t.Context(), "session-2", Config{
		DataDir:     t.TempDir(),
		ProjectRoot: repoRoot,
	})
	require.NoError(t, err)
	require.NotNil(t, system)
	t.Cleanup(system.Close)

	raw := []byte(`{"decision":"Normalize byte-array tool payloads before persisting them."}`)
	numberList := make([]int, 0, len(raw))
	for _, b := range raw {
		numberList = append(numberList, int(b))
	}
	encoded, err := json.Marshal(numberList)
	require.NoError(t, err)

	saveResp := runMemoryTool(t, NewSaveTool(system, func(context.Context) string { return "session-2" }), SaveToolName, SaveParams{
		EventType: "architectural_decision",
		Content:   json.RawMessage(encoded),
	})
	require.Contains(t, saveResp.Content, "Memory saved: architectural_decision")

	constitution, err := system.Store.GetConstitution(context.Background())
	require.NoError(t, err)
	require.Contains(t, constitution, "Normalize byte-array tool payloads before persisting them.")

	records, err := system.Store.QueryRecords(context.Background(), "architectural", 10)
	require.NoError(t, err)
	require.NotEmpty(t, records)
	require.Contains(t, records[0].ContentJSON, `"decision":"Normalize byte-array tool payloads before persisting them."`)
}

func TestSaveToolSupportsImprovementEvalsAndStrategies(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	system, err := NewSystem(t.Context(), "session-3", Config{
		DataDir:     t.TempDir(),
		ProjectRoot: repoRoot,
	})
	require.NoError(t, err)
	require.NotNil(t, system)
	t.Cleanup(system.Close)

	evalResp := runMemoryTool(t, NewSaveTool(system, func(context.Context) string { return "session-3" }), SaveToolName, SaveParams{
		EventType: MemoryEventImprovementEval,
		Content: json.RawMessage(`{
			"task_shape":"regex parser repair",
			"failure_signature":"unknown escape sequence in matcher_test.go",
			"probe":"go test ./experiments/mistake_lab",
			"success_criteria":"targeted tests pass",
			"prevention_rule":"Persist the exact failing probe before rerunning validation."
		}`),
	})
	require.Contains(t, evalResp.Content, "Memory saved: "+MemoryEventImprovementEval)

	strategyResp := runMemoryTool(t, NewSaveTool(system, func(context.Context) string { return "session-3" }), SaveToolName, SaveParams{
		EventType: MemoryEventStrategyPattern,
		Content: json.RawMessage(`{
			"task_shape":"regex parser repair",
			"strategy":"Reproduce with a one-package go test before broadening the validation scope.",
			"why_it_worked":"It kept the failure surface narrow enough to iterate quickly.",
			"trigger_signals":["compiler escape errors","fresh mutation after heredoc edits"],
			"validation_probe":"go test ./experiments/mistake_lab"
		}`),
	})
	require.Contains(t, strategyResp.Content, "Memory saved: "+MemoryEventStrategyPattern)

	evals, err := system.Store.QueryRecords(context.Background(), MemoryFilterEvals, 10)
	require.NoError(t, err)
	require.Len(t, evals, 1)
	require.Contains(t, evals[0].ContentJSON, `"task_shape":"regex parser repair"`)

	strategies, err := system.Store.QueryRecords(context.Background(), MemoryFilterStrategies, 10)
	require.NoError(t, err)
	require.Len(t, strategies, 1)
	require.Contains(t, strategies[0].ContentJSON, `"strategy":"Reproduce with a one-package go test before broadening the validation scope."`)
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
