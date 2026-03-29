package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/duggal1/Sapphire-cli/internal/codebasesurvey"
	"github.com/stretchr/testify/require"
)

func TestMemoryFileManagerRenderSemanticSurveySection(t *testing.T) {
	dataDir := t.TempDir()
	projectRoot := t.TempDir()

	manager, err := newMemoryFileManager(dataDir, projectRoot)
	require.NoError(t, err)

	require.NoError(t, codebasesurvey.WriteManifest(dataDir, codebasesurvey.Manifest{
		RepoRoot:      projectRoot,
		ScopePath:     projectRoot,
		GeneratedAt:   time.Unix(1700000000, 0).UTC(),
		Status:        "ready",
		AgentCount:    3,
		TotalFiles:    99,
		Overview:      []string{"Agent layer coordinates sub-agents.", "Mailbox carries durable transport."},
		OverviewPath:  codebasesurvey.OverviewPath(dataDir),
		CriticalFiles: []string{"internal/agent/coordinator.go"},
	}))

	lines := manager.renderSemanticSurveySection()
	require.NotEmpty(t, lines)
	require.Contains(t, lines[0], "status: ready")
	require.Contains(t, lines[1], "agent_count: 3")
	require.Contains(t, lines[2], "total_files: 99")
	require.Contains(t, strings.Join(lines, "\n"), "Mailbox carries durable transport.")
}

func TestTrimMemoryContentForStageUsesSectionPriority(t *testing.T) {
	t.Parallel()

	content := strings.TrimSpace(`
# Sapphire Memory Handbook

Intro.

## Session Snapshot
- session_id: abc

## Active Workstreams
- active_request: keep durable memory coherent

## Repo Constitution
Do not regress the long-horizon memory path.

## Stable Decisions
- persist a durable handbook

## Proven Strategies
- use the narrowest targeted validation first

## Failures and Guardrails
- guardrail: never drop durable memory silently

## Validated Improvement Probes
- probe: go test ./internal/memory

## Architecture Overview
- internal/memory/: memory runtime

## AI Codebase Graph
- status: ready

## Critical Files
- internal/memory/memory_md.go: memory handbook builder

## Supporting Files
- internal/memory/history.go: session history
`)

	stage10 := trimMemoryContentForStage(content, ContextLoadStage10)
	require.Contains(t, stage10, "## Session Snapshot")
	require.Contains(t, stage10, "## Active Workstreams")
	require.NotContains(t, stage10, "## Architecture Overview")

	stage30 := trimMemoryContentForStage(content, ContextLoadStage30)
	require.Contains(t, stage30, "## Stable Decisions")
	require.Contains(t, stage30, "## Proven Strategies")
	require.Contains(t, stage30, "## Failures and Guardrails")
	require.Contains(t, stage30, "## Validated Improvement Probes")
	require.Contains(t, stage30, "## Architecture Overview")
	require.NotContains(t, stage30, "## Critical Files")

	stage40 := trimMemoryContentForStage(content, ContextLoadStage40)
	require.Contains(t, stage40, "## AI Codebase Graph")
	require.Contains(t, stage40, "## Critical Files")
	require.NotContains(t, stage40, "## Supporting Files")
}

func TestMemoryFileManagerLoadCodebaseSnapshotReusesPersistedSnapshot(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	projectRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "main.go"), []byte("package main\n"), 0o644))

	manager, err := newMemoryFileManager(dataDir, projectRoot)
	require.NoError(t, err)

	snapshot, err := manager.loadCodebaseSnapshot(false)
	require.NoError(t, err)
	require.Greater(t, snapshot.TotalFiles, 0)

	manager2, err := newMemoryFileManager(dataDir, projectRoot)
	require.NoError(t, err)
	require.NoError(t, os.Remove(filepath.Join(projectRoot, "main.go")))

	reused, err := manager2.loadCodebaseSnapshot(false)
	require.NoError(t, err)
	require.Equal(t, snapshot.TotalFiles, reused.TotalFiles)
	require.Equal(t, snapshot.Architecture, reused.Architecture)
}
