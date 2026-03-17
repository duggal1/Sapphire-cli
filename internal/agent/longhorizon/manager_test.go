package longhorizon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/sapphire/internal/db"
	"github.com/charmbracelet/sapphire/internal/session"
	"github.com/stretchr/testify/require"
)

func TestEnsurePersistsCanonicalSQLAndMaterializesFiles(t *testing.T) {
	dataDir := t.TempDir()
	conn, err := db.Connect(t.Context(), dataDir)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
	})

	q := db.New(conn)
	sessions := session.NewService(q, conn)
	sess, err := sessions.Create(t.Context(), "Long horizon task")
	require.NoError(t, err)

	manager := NewManager(q, dataDir)
	state, err := manager.Ensure(t.Context(), sess.ID, "Implement the canonical long-horizon run state.")
	require.NoError(t, err)
	require.True(t, state.Activated)

	run, err := q.GetLongHorizonRun(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Equal(t, "active", run.Status)
	require.Contains(t, run.FrozenSpecMd, "Implement the canonical long-horizon run state.")

	milestones, err := q.ListLongHorizonMilestones(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Len(t, milestones, 3)

	runbookBytes, err := os.ReadFile(state.RunbookPath)
	require.NoError(t, err)
	require.Contains(t, string(runbookBytes), "Long-Horizon Runbook")

	specBytes, err := os.ReadFile(state.SpecPath)
	require.NoError(t, err)
	require.Contains(t, string(specBytes), "canonical long-horizon run state")

	planBytes, err := os.ReadFile(state.PlanPath)
	require.NoError(t, err)
	require.Contains(t, string(planBytes), "\"milestones\"")
}

func TestBuildInjectionRematerializesMissingFilesFromSQL(t *testing.T) {
	dataDir := t.TempDir()
	conn, err := db.Connect(t.Context(), dataDir)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
	})

	q := db.New(conn)
	sessions := session.NewService(q, conn)
	sess, err := sessions.Create(t.Context(), "Long horizon task")
	require.NoError(t, err)

	manager := NewManager(q, dataDir)
	state, err := manager.Ensure(t.Context(), sess.ID, "Keep continuity across context resets.")
	require.NoError(t, err)

	manager.AppendAudit(t.Context(), sess.ID, "Checkpoint completed and ready to resume.")

	require.NoError(t, os.Remove(state.RunbookPath))
	require.NoError(t, os.Remove(state.SpecPath))
	require.NoError(t, os.Remove(state.PlanPath))

	injection := manager.BuildInjection(t.Context(), sess.ID)
	require.Contains(t, injection, "<long_horizon_runbook>")
	require.Contains(t, injection, "Checkpoint completed and ready to resume.")

	for _, path := range []string{
		filepath.Join(dataDir, "long_horizon", sess.ID, "runbook.md"),
		filepath.Join(dataDir, "long_horizon", sess.ID, "frozen_spec.md"),
		filepath.Join(dataDir, "long_horizon", sess.ID, "milestones.json"),
	} {
		_, err := os.Stat(path)
		require.NoError(t, err)
	}
}
