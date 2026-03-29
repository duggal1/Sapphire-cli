package codebasesurvey

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWriteAndReadManifest(t *testing.T) {
	dataDir := t.TempDir()
	manifest := Manifest{
		RepoRoot:      "/tmp/repo",
		ScopePath:     "/tmp/repo",
		HeadCommit:    "abc123",
		IndexEpoch:    7,
		GeneratedAt:   time.Unix(1700000000, 0).UTC(),
		Status:        "ready",
		AgentCount:    3,
		TotalFiles:    42,
		Overview:      []string{"Agent layer owns orchestration.", "Memory layer owns durable recall."},
		CriticalFiles: []string{"internal/agent/coordinator.go", "internal/memory/memory_md.go"},
		ShardArtifacts: []ShardArtifact{
			{ShardID: "shard-01", Label: "Shard 1", Status: "completed", FileCount: 21, ArtifactPath: "/tmp/graph-1.md"},
		},
	}

	require.NoError(t, WriteManifest(dataDir, manifest))

	loaded, ok, err := ReadManifest(dataDir)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, manifest.RepoRoot, loaded.RepoRoot)
	require.Equal(t, manifest.AgentCount, loaded.AgentCount)
	require.Equal(t, manifest.TotalFiles, loaded.TotalFiles)

	overviewPath := OverviewPath(dataDir)
	data, err := os.ReadFile(overviewPath)
	require.NoError(t, err)
	require.Contains(t, string(data), "# AI Codebase Graph")
	require.Contains(t, string(data), "Agent layer owns orchestration.")
}

func TestWriteShardInputCreatesJSON(t *testing.T) {
	dataDir := t.TempDir()
	path, err := WriteShardInput(dataDir, ShardInput{
		ShardID:       "shard-01",
		Label:         "Shard 1",
		RepoRoot:      "/tmp/repo",
		ScopePath:     "/tmp/repo",
		AssignedFiles: []string{"main.go", "internal/agent/coordinator.go"},
	})
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(data), `"shard_id": "shard-01"`)
	require.Contains(t, string(data), `"main.go"`)
}
