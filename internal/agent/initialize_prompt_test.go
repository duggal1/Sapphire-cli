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
	require.Contains(t, built, "`agentic_view` is the default read tool for initialization.")
	require.Contains(t, built, "broad sweeps of about 12-20 files")
	require.Contains(t, built, "at least 20 meaningful files if available")
	require.Contains(t, built, "around 300-400 lines of verified material")
	require.Contains(t, built, "Do not stop after root files, dependency files, and one or two entry points.")
}
