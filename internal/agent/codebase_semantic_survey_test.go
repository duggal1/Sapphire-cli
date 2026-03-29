package agent

import (
	"testing"

	agentmemory "github.com/duggal1/Sapphire-cli/internal/agent/memory"
	"github.com/stretchr/testify/require"
)

func TestBuildSemanticSurveyShardsPreservesCoverage(t *testing.T) {
	files := []agentmemory.IndexedFileInfo{
		{Path: "main.go", Language: "go", SymbolCount: 4},
		{Path: "internal/agent/coordinator.go", Language: "go", SymbolCount: 12},
		{Path: "internal/agent/subagent_manager.go", Language: "go", SymbolCount: 10},
		{Path: "internal/agent/templates/codebase_indexing.md", Language: "markdown", SymbolCount: 0},
		{Path: "internal/memory/memory_md.go", Language: "go", SymbolCount: 6},
		{Path: "internal/orchestration/db/db.go", Language: "go", SymbolCount: 9},
		{Path: "internal/ui/model/ui.go", Language: "go", SymbolCount: 7},
		{Path: "scripts/dev.sh", Language: "shell", SymbolCount: 0},
	}

	shards := buildSemanticSurveyShards(files, 3)
	require.Len(t, shards, 3)

	seen := make(map[string]struct{}, len(files))
	for _, shard := range shards {
		require.NotEmpty(t, shard.ID)
		require.NotEmpty(t, shard.Label)
		require.NotEmpty(t, shard.Files)
		require.NotEmpty(t, shard.CriticalFiles)
		for _, file := range shard.Files {
			seen[file.Path] = struct{}{}
		}
	}

	require.Len(t, seen, len(files))
	_, ok := seen["internal/agent/coordinator.go"]
	require.True(t, ok)
}

func TestNormalizeSemanticSurveyAgentCount(t *testing.T) {
	require.Equal(t, 0, normalizeSemanticSurveyAgentCount(3, 0))
	require.Equal(t, 1, normalizeSemanticSurveyAgentCount(0, 1))
	require.Equal(t, defaultSemanticSurveyAgents, normalizeSemanticSurveyAgentCount(0, 20))
	require.Equal(t, maxSemanticSurveyAgents, normalizeSemanticSurveyAgentCount(99, 20))
	require.Equal(t, 2, normalizeSemanticSurveyAgentCount(4, 2))
}
