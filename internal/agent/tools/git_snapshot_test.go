package tools

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/sapphire/internal/config"
	"github.com/charmbracelet/sapphire/internal/permission"
	"github.com/stretchr/testify/require"
)

func TestQueueGitSnapshotCreatesLocalCommitInWorktree(t *testing.T) {
	t.Parallel()

	root := initSnapshotGitRepo(t)
	worktreeDir := filepath.Join(root, ".sapphire", "worktrees", "agent", "agent-1", "render-fix")
	runGitSnapshotTest(t, root, "worktree", "add", "-b", "agent/agent-1/render-fix", worktreeDir, "HEAD")

	targetFile := filepath.Join(worktreeDir, "notes.txt")
	require.NoError(t, os.WriteFile(targetFile, []byte("snapshot me\n"), 0o644))

	ctx := context.WithValue(context.Background(), WorkingDirContextKey, worktreeDir)
	QueueGitSnapshot(ctx, targetFile)
	require.NoError(t, FlushGitSnapshot(ctx, worktreeDir))

	count := strings.TrimSpace(runGitSnapshotOutput(t, worktreeDir, "rev-list", "--count", "HEAD"))
	require.Equal(t, "2", count)

	runGitSnapshotTest(t, worktreeDir, "diff", "--quiet")
	runGitSnapshotTest(t, worktreeDir, "diff", "--cached", "--quiet")
}

func TestBashToolBlocksDestructiveGitCommandsInIsolatedWorktree(t *testing.T) {
	t.Parallel()

	root := initSnapshotGitRepo(t)
	worktreeDir := filepath.Join(root, ".sapphire", "worktrees", "agent", "agent-2", "git-guard")
	runGitSnapshotTest(t, root, "worktree", "add", "-b", "agent/agent-2/git-guard", worktreeDir, "HEAD")

	permissions := permission.NewPermissionService(worktreeDir, true, nil)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "session-1")

	tool := NewBashTool(permissions, worktreeDir, &config.Attribution{}, "gemini-3-flash")
	input, err := json.Marshal(BashParams{
		Description: "attempt push",
		Command:     "git push origin agent/agent-2/git-guard",
	})
	require.NoError(t, err)

	resp, err := tool.Run(ctx, fantasy.ToolCall{
		ID:    "bash-git-push",
		Name:  BashToolName,
		Input: string(input),
	})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "git push is blocked")
}

func initSnapshotGitRepo(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	runGitSnapshotTest(t, root, "init")
	runGitSnapshotTest(t, root, "config", "user.email", "test@example.com")
	runGitSnapshotTest(t, root, "config", "user.name", "Test User")
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("hello\n"), 0o644))
	runGitSnapshotTest(t, root, "add", "README.md")
	runGitSnapshotTest(t, root, "commit", "-m", "init")
	return root
}

func runGitSnapshotTest(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git %v failed: %s", args, string(out))
}

func runGitSnapshotOutput(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git %v failed: %s", args, string(out))
	return string(out)
}
