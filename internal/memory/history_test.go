package memory

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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
	require.True(t, strings.Contains(injection, "main.go") || strings.Contains(injection, "README.md"))
}
