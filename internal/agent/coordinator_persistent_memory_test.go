package agent

import (
	"path/filepath"
	"testing"

	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/stretchr/testify/require"
)

func TestInitPersistentMemoryEnsuresMistakeProtocol(t *testing.T) {
	t.Parallel()

	cfg, err := config.Init(t.TempDir(), "", false)
	require.NoError(t, err)

	coord := &coordinator{cfg: cfg}
	projectRoot := t.TempDir()
	coord.initPersistentMemory(t.Context(), t.TempDir(), projectRoot, "")
	require.NotNil(t, coord.pmem)
	t.Cleanup(coord.pmem.Close)

	require.FileExists(t, filepath.Join(projectRoot, ".sapphire", "mistake.md"))
}
