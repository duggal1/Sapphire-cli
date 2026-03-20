package agent

import (
	"context"
	"os"
	"strings"
)

const mainWorktreeBaseName = "main-agent"

func (c *coordinator) prepareMainWorktree(ctx context.Context) (string, string, error) {
	root := c.cfg.WorkingDir()
	if _, err := runGitOutput(ctx, root, "rev-parse", "--git-dir"); err != nil {
		return root, "", nil
	}
	baseRef := resolveWorktreeBaseRef(ctx, root)
	return root, baseRef, nil
}

func isWorktreeClean(ctx context.Context, worktreeDir string) (bool, error) {
	out, err := runGitOutput(ctx, worktreeDir, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "", nil
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
