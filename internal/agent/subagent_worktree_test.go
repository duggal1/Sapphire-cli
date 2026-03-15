package agent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/sapphire/internal/config"
	"github.com/stretchr/testify/require"
)

func TestPrepareSubAgentWorktreeRecoversLockedMissingWorktree(t *testing.T) {
	t.Parallel()

	root := initGitRepo(t)
	worktreeDir := filepath.Join(root, "worktrees", "analysis")

	runGitTest(t, root, "worktree", "add", "-b", "subagent/analysis", worktreeDir, "HEAD")
	runGitTest(t, root, "worktree", "lock", "--reason", "initializing", worktreeDir)
	require.NoError(t, os.RemoveAll(worktreeDir))

	cfg, err := config.Init(root, "", false)
	require.NoError(t, err)
	coord := &coordinator{cfg: cfg}
	gotDir, branch, cleanup, err := coord.prepareSubAgentWorktree(context.Background(), "session-1", "agent-1", subAgentWorktreeSpec{
		WorktreePath: worktreeDir,
		Branch:       "subagent/analysis",
		TaskKey:      "analysis",
	})
	require.NoError(t, err)
	require.Equal(t, worktreeDir, gotDir)
	require.Equal(t, "subagent/analysis", branch)
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
