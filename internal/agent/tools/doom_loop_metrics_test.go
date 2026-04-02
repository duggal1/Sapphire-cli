package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type deterministicMetricsTracker struct {
	reads map[string]time.Time
}

func (m *deterministicMetricsTracker) RecordRead(context.Context, string, string) {}

func (m *deterministicMetricsTracker) LastReadTime(_ context.Context, _ string, path string) time.Time {
	if m == nil {
		return time.Time{}
	}
	return m.reads[path]
}

func (m *deterministicMetricsTracker) ListReadFiles(context.Context, string) ([]string, error) {
	return nil, nil
}

func TestRecordPreparedToolUsageTracksDeterministicLoopMetrics(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	alreadyRead := filepath.Join(workingDir, "internal", "service.go")
	require.NoError(t, os.MkdirAll(filepath.Dir(alreadyRead), 0o755))
	require.NoError(t, os.WriteFile(alreadyRead, []byte("package internal\n"), 0o644))

	state := NewToolUsageState()
	ctx := context.WithValue(context.Background(), ToolUsageStateContextKey, state)
	ctx = context.WithValue(ctx, SessionIDContextKey, "session-1")
	ctx = context.WithValue(ctx, WorkingDirContextKey, workingDir)

	SetValidationFileTracker(&deterministicMetricsTracker{
		reads: map[string]time.Time{
			alreadyRead: time.Now(),
		},
	})
	t.Cleanup(func() {
		SetValidationFileTracker(nil)
	})

	recordPreparedToolUsage(ctx, AgenticViewToolName, map[string]any{"file_path": "internal/service.go"})
	recordPreparedToolUsage(ctx, AgenticViewToolName, map[string]any{"file_path": "internal/service.go"})
	recordPreparedToolUsage(ctx, AgenticViewToolName, map[string]any{"file_path": "internal/service.go"})
	recordPreparedToolUsage(ctx, WriteToolName, map[string]any{"file_path": "new/feature.go", "content": "package newfeature\n"})
	recordPreparedToolUsage(ctx, WriteToolName, map[string]any{"file_path": "internal/service.go", "content": "package internal\n// changed\n"})

	metrics := state.SnapshotDeterministicLoopMetrics()
	require.Equal(t, 5, metrics.TotalCalls)
	require.Equal(t, 3, metrics.ReadCounts[filepath.ToSlash(alreadyRead)])
	require.Equal(t, 1, metrics.BlindWriteCounts[filepath.ToSlash(filepath.Join(workingDir, "new", "feature.go"))])
	require.Contains(t, metrics.CreatedFiles, filepath.ToSlash(filepath.Join(workingDir, "new", "feature.go")))
	require.Contains(t, metrics.ModifiedFiles, filepath.ToSlash(alreadyRead))
}
