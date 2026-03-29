package memory

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestViewMemoryToolReturnsStructuredHistory(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "main.go"), []byte("package main\n"), 0o644))

	system, err := NewSystem(t.Context(), "", Config{
		DataDir:     t.TempDir(),
		ProjectRoot: projectRoot,
	})
	require.NoError(t, err)
	t.Cleanup(system.Close)

	const sessionID = "session-history"
	system.RecordUserTurn(context.Background(), sessionID, "Implement the memory system")
	system.RecordToolCall(context.Background(), sessionID, "agentic_view", `{"files":["internal/memory/system.go"]}`)
	system.RecordToolResult(context.Background(), sessionID, "agentic_view", "Read 3 files successfully", false)
	system.RecordAssistantTurn(context.Background(), sessionID, "Planned the hybrid memory implementation.")
	system.RecordSavedMemory(context.Background(), sessionID, "architectural_decision", `{"decision":"use badger for per-session history"}`)

	resp := runMemoryTool(t, NewViewMemoryTool(system, func(context.Context) string { return sessionID }), ViewToolName, ViewMemoryParams{
		Mode:  "recent",
		Limit: 10,
	})

	var result ViewMemoryResult
	require.NoError(t, json.Unmarshal([]byte(resp.Content), &result))
	require.Equal(t, "recent", result.Mode)
	require.Equal(t, sessionID, result.SessionID)
	require.GreaterOrEqual(t, len(result.Entries), 4)
	require.Contains(t, result.Sources[0], sessionID)
}

func TestBuildContextInjectionForSessionIncludesDynamicMemoryFile(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "main.go"), []byte("// entrypoint\npackage main\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "README.md"), []byte("# Sapphire CLI\n"), 0o644))

	system, err := NewSystem(t.Context(), "", Config{
		DataDir:     t.TempDir(),
		ProjectRoot: projectRoot,
	})
	require.NoError(t, err)
	t.Cleanup(system.Close)

	const sessionID = "session-map"
	system.RecordUserTurn(context.Background(), sessionID, "Fix memory continuity")
	system.RecordAssistantTurn(context.Background(), sessionID, "I am implementing the memory runtime.")
	system.RecordSavedMemory(context.Background(), sessionID, "architectural_decision", `{"decision":"inject memory.md on every turn"}`)

	injection := system.BuildContextInjectionForSession(context.Background(), sessionID, 4000)
	require.Contains(t, injection, "<persistent_memory_map>")
	require.Contains(t, injection, "current_task: Fix memory continuity")
	require.Contains(t, injection, "## Active Workstreams")
	require.True(t, strings.Contains(injection, "main.go") || strings.Contains(injection, "README.md"))
}

func TestBuildContextInjectionForSessionStagesMemoryMap(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "internal", "agent"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "main.go"), []byte("// entrypoint\npackage main\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "README.md"), []byte("# Sapphire CLI\n"), 0o644))
	for i := 0; i < 24; i++ {
		path := filepath.Join(projectRoot, "internal", "agent", "file"+strconv.Itoa(i)+".go")
		require.NoError(t, os.WriteFile(path, []byte("package agent\n"), 0o644))
	}

	system, err := NewSystem(t.Context(), "", Config{
		DataDir:     t.TempDir(),
		ProjectRoot: projectRoot,
	})
	require.NoError(t, err)
	t.Cleanup(system.Close)

	const sessionID = "session-staged-map"
	system.RecordUserTurn(context.Background(), sessionID, "Stabilize long-horizon prompt assembly")
	system.RecordSavedMemory(context.Background(), sessionID, "architectural_decision", `{"decision":"load memory in context buckets"}`)
	system.RecordSavedMemory(context.Background(), sessionID, MemoryEventStrategyPattern, `{"task_shape":"long-horizon prompt assembly","strategy":"Read constitution and prior strategy records before changing staged injection.","validation_probe":"go test ./internal/memory"}`)
	system.RecordSavedMemory(context.Background(), sessionID, MemoryEventImprovementEval, `{"task_shape":"long-horizon prompt assembly","failure_signature":"missing stage-gated memory sections","probe":"go test ./internal/memory -run TestBuildContextInjectionForSessionStagesMemoryMap","success_criteria":"stage assertions pass"}`)

	stage10 := system.BuildContextInjectionForSessionAtStage(context.Background(), sessionID, 4000, ContextLoadStage10)
	require.Contains(t, stage10, "<persistent_memory_map>")
	require.Contains(t, stage10, "## Session Snapshot")
	require.Contains(t, stage10, "## Active Workstreams")
	require.NotContains(t, stage10, "## Architecture Overview")
	require.NotContains(t, stage10, "<persistent_memory_strategies>")

	stage20 := system.BuildContextInjectionForSessionAtStage(context.Background(), sessionID, 4000, ContextLoadStage20)
	require.Contains(t, stage20, "<persistent_memory_strategies>")
	require.NotContains(t, stage20, "<persistent_memory_evals>")

	stage30 := system.BuildContextInjectionForSessionAtStage(context.Background(), sessionID, 4000, ContextLoadStage30)
	require.Contains(t, stage30, "## Architecture Overview")
	require.Contains(t, stage30, "<persistent_memory_evals>")
	require.NotContains(t, stage30, "## Critical Files")

	stage40 := system.BuildContextInjectionForSessionAtStage(context.Background(), sessionID, 4000, ContextLoadStage40)
	require.Contains(t, stage40, "## Critical Files")
	require.NotContains(t, stage40, "## Supporting Files")

	stage50 := system.BuildContextInjectionForSessionAtStage(context.Background(), sessionID, 4000, ContextLoadStage50)
	require.Contains(t, stage50, "## Supporting Files")
	require.Contains(t, stage50, "load memory in context buckets")
}

func TestBuildSessionStateFlagsMajorAchievementSignals(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "main.go"), []byte("package main\n"), 0o644))

	system, err := NewSystem(t.Context(), "", Config{
		DataDir:     t.TempDir(),
		ProjectRoot: projectRoot,
	})
	require.NoError(t, err)
	t.Cleanup(system.Close)

	const sessionID = "session-achievement-state"
	system.RecordUserTurn(context.Background(), sessionID, "Upgrade durable memory")
	system.RecordToolCall(context.Background(), sessionID, "apply_patch", `{"file":"internal/agent/a.go"}`)
	system.RecordToolCall(context.Background(), sessionID, "apply_patch", `{"file":"internal/agent/b.go"}`)
	system.RecordToolCall(context.Background(), sessionID, "apply_patch", `{"file":"internal/memory/c.go"}`)
	system.RecordToolResult(context.Background(), sessionID, "index_codebase", "semantic graph refreshed", false)
	system.RecordSavedMemory(context.Background(), sessionID, "architectural_decision", `{"decision":"persist a deeper handbook"}`)

	state, err := system.History.BuildSessionState(context.Background(), sessionID)
	require.NoError(t, err)
	require.True(t, state.MajorAchievementLikely)
	require.Contains(t, state.AchievementSignals, "architectural_decision")
	require.Contains(t, state.AchievementSignals, "multi_file_write")
	require.Contains(t, state.AchievementSignals, "semantic_codebase_refresh")
}
