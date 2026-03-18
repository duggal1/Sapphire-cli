package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const mainWorktreeBaseName = "main-agent"

func (c *coordinator) prepareMainWorktree(ctx context.Context) (string, string, error) {
	root := c.cfg.WorkingDir()
	if root == "" {
		return "", "", fmt.Errorf("working directory not configured")
	}
	if _, err := runGitOutput(ctx, root, "rev-parse", "--git-dir"); err != nil {
		return root, "", nil
	}

	baseDir := filepath.Join(c.subAgentWorktreeRoot(root), mainWorktreeBaseName)
	baseBranch := "agent/main"
	worktreeDir := baseDir
	branch := baseBranch

	if pathExists(baseDir) {
		if isSubAgentWorktree(baseDir) {
			clean, err := isWorktreeClean(ctx, baseDir)
			if err == nil && clean {
				return baseDir, baseBranch, nil
			}
		}
		suffix := time.Now().UTC().Format("20060102-150405")
		worktreeDir = filepath.Join(c.subAgentWorktreeRoot(root), fmt.Sprintf("%s-%s", mainWorktreeBaseName, suffix))
		branch = fmt.Sprintf("%s/%s", baseBranch, suffix)
	}

	if err := os.MkdirAll(filepath.Dir(worktreeDir), 0o755); err != nil {
		return root, "", fmt.Errorf("create worktree parent failed: %w", err)
	}

	release := c.lockWorktreePath(worktreeDir)
	defer release()

	wtCtx, cancel := context.WithTimeout(ctx, subAgentWorktreeTimeout)
	defer cancel()
	if err := addWorktreeWithRecovery(wtCtx, root, worktreeDir, branch); err != nil {
		return root, "", err
	}

	return worktreeDir, branch, nil
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
