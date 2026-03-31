package agent

import (
	"os"
	"testing"

	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/stretchr/testify/require"
)

func TestInitializePromptEnforcesDeepCoverageAndSubstantialOutput(t *testing.T) {
	t.Parallel()

	workingDir, err := os.MkdirTemp("", "sapphire-init-prompt-working-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(workingDir) })

	dataDir, err := os.MkdirTemp("", "sapphire-init-prompt-data-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dataDir) })

	cfg, err := config.Init(workingDir, dataDir, false)
	require.NoError(t, err)

	built, err := InitializePrompt(*cfg)
	require.NoError(t, err)
	require.Contains(t, built, "`agentic_view` is the default read tool for initialization")
	require.Contains(t, built, "aggressive broad sweeps of about 20-30 files")
	require.Contains(t, built, "at least 30 meaningful files if available")
	require.Contains(t, built, "under roughly 100 lines for a real repository is presumptively incomplete")
	require.Contains(t, built, "often about 300-600 lines when the evidence supports it")
	require.Contains(t, built, "Read the main important files for each major domain deeply enough")
	require.Contains(t, built, "Do not stop after root files, dependency files, and one or two entry points.")
	require.Contains(t, built, "## AGENTS.md Semantics")
	require.Contains(t, built, "### Conflict Resolution")
	require.Contains(t, built, "Before modifying any file, check whether an `AGENTS.md` file applies to that file's path.")
}
