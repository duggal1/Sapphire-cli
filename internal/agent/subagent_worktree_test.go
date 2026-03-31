package agent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/stretchr/testify/require"
)

func TestPrepareSubAgentWorktreeRecoversLockedMissingWorktree(t *testing.T) {
	t.Parallel()

	root := initGitRepo(t)
	worktreeDir := filepath.Join(root, ".sapphire", "worktrees", "agent", "test", "analysis")

	runGitTest(t, root, "worktree", "add", "-b", "agent/test/analysis", worktreeDir, "HEAD")
	runGitTest(t, root, "worktree", "lock", "--reason", "initializing", worktreeDir)
	require.NoError(t, os.RemoveAll(worktreeDir))

	cfg, err := config.Init(root, "", false)
	require.NoError(t, err)
	coord := &coordinator{cfg: cfg}
	gotDir, branch, cleanup, err := coord.prepareSubAgentWorktree(context.Background(), "session-1", "agent-1", subAgentWorktreeSpec{
		WorktreePath: worktreeDir,
		Branch:       "agent/test/analysis",
		TaskKey:      "analysis",
		AssignmentID: "test",
	})
	require.NoError(t, err)
	require.Equal(t, worktreeDir, gotDir)
	require.Equal(t, "agent/test/analysis", branch)
	require.DirExists(t, gotDir)
	cleanup()
}

func TestRemoveWorktreeUnlocksLockedTree(t *testing.T) {
	t.Parallel()

	root := initGitRepo(t)
	worktreeDir := filepath.Join(root, "worktrees", "cleanup")

	runGitTest(t, root, "worktree", "add", "-b", "subagent/cleanup", worktreeDir, "HEAD")
	runGitTest(t, root, "worktree", "lock", "--reason", "initializing", worktreeDir)

	require.NoError(t, removeWorktree(root, worktreeDir))
	_, err := os.Stat(worktreeDir)
	require.Error(t, err)
	require.True(t, os.IsNotExist(err))
}

func TestPrepareSubAgentWorktreeRejectsActivePathCollision(t *testing.T) {
	t.Parallel()

	root := initGitRepo(t)
	cfg, err := config.Init(root, "", false)
	require.NoError(t, err)

	coord := &coordinator{
		cfg:              cfg,
		subAgents:        make(map[string]*subAgentRunner),
		subAgentRegistry: newSubAgentRegistry(),
		worktreeOps:      make(map[string]*sync.Mutex),
	}
	worktreeDir := filepath.Join(root, ".sapphire", "worktrees", "agent", "test", "shared")
	coord.subAgentRegistry.upsert("agent-existing", &subAgentRunner{
		id:      "agent-existing",
		workDir: worktreeDir,
		status:  subAgentStatusRunning,
		assignment: subAgentAssignment{
			Branch: "agent/test/shared",
		},
	})

	_, _, _, err = coord.prepareSubAgentWorktree(context.Background(), "session-1", "agent-new", subAgentWorktreeSpec{
		WorktreePath: worktreeDir,
		Branch:       "agent/test/shared",
		TaskKey:      "shared",
		AssignmentID: "test",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "already owned by active sub-agent")
}

func initGitRepo(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	runGitTest(t, root, "init")
	runGitTest(t, root, "config", "user.email", "test@example.com")
	runGitTest(t, root, "config", "user.name", "Test User")
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("hello\n"), 0o644))
	runGitTest(t, root, "add", "README.md")
	runGitTest(t, root, "commit", "-m", "init")
	return root
}

func runGitTest(t *testing.T, root string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git %v failed: %s", args, string(out))
}
