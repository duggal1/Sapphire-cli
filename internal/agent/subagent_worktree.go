package agent

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const subAgentWorktreeTimeout = 20 * time.Second

type subAgentWorktreeSpec struct {
	WorktreePath string
	Branch       string
	Reuse        bool
	TaskKey      string
}

func (c *coordinator) prepareSubAgentWorktree(ctx context.Context, sessionID, agentID string, spec subAgentWorktreeSpec) (string, string, func(), error) {
	root := c.cfg.WorkingDir()
	if root == "" {
		return "", "", func() {}, fmt.Errorf("working directory not configured")
	}

	worktreeDir := spec.WorktreePath
	if strings.TrimSpace(worktreeDir) == "" {
		worktreeDir = filepath.Join(c.subAgentWorktreeRoot(root), sanitizeWorktreeName(spec.TaskKey, agentID, spec.Branch))
	}
	if !filepath.IsAbs(worktreeDir) {
		worktreeDir = filepath.Join(root, worktreeDir)
	}
	worktreeDir = filepath.Clean(worktreeDir)

	branch := sanitizeBranchName(spec.Branch)
	if branch == "" {
		branch = defaultSubAgentBranch(spec.TaskKey, agentID)
	}

	if spec.Reuse {
		if !isSubAgentWorktree(worktreeDir) {
			return "", "", func() {}, fmt.Errorf("worktree %s does not exist for reuse", worktreeDir)
		}
		if current, err := currentWorktreeBranch(ctx, worktreeDir); err == nil && current != "" {
			branch = current
		}
		return worktreeDir, branch, func() {}, nil
	}

	if err := os.MkdirAll(filepath.Dir(worktreeDir), 0o755); err != nil {
		return "", "", func() {}, fmt.Errorf("create worktree parent failed: %w", err)
	}

	_ = removeWorktree(root, worktreeDir)

	wtCtx, cancel := context.WithTimeout(ctx, subAgentWorktreeTimeout)
	defer cancel()

	if branchExists(root, branch) {
		if err := runGit(wtCtx, root, "worktree", "add", worktreeDir, branch); err != nil {
			return "", "", func() {}, err
		}
	} else {
		if err := runGit(wtCtx, root, "worktree", "add", "-b", branch, worktreeDir, "HEAD"); err != nil {
			return "", "", func() {}, err
		}
	}

	cleanup := c.subAgentWorktreeCleanup(root, worktreeDir)
	return worktreeDir, branch, cleanup, nil
}

func (c *coordinator) subAgentWorktreeRoot(root string) string {
	return filepath.Join(root, "worktrees")
}

func (c *coordinator) subAgentWorktreeCleanup(root, worktreeDir string) func() {
	return func() {
		if err := removeWorktree(root, worktreeDir); err != nil {
			slog.Warn("Failed to remove sub-agent worktree", "error", err)
		}
	}
}

func isSubAgentWorktree(worktreeDir string) bool {
	info, err := os.Stat(worktreeDir)
	if err != nil || !info.IsDir() {
		return false
	}
	if _, err := os.Stat(filepath.Join(worktreeDir, ".git")); err != nil {
		return false
	}
	return true
}

func sanitizeWorktreeName(taskKey, agentID, branch string) string {
	name := strings.TrimSpace(taskKey)
	if name == "" {
		name = strings.TrimSpace(branch)
	}
	if name == "" {
		name = agentID
	}
	name = strings.ToLower(name)
	replacer := strings.NewReplacer(" ", "-", "/", "-", "\\", "-", ":", "-", ".", "-", "_", "-")
	name = replacer.Replace(name)
	name = strings.Trim(name, "-")
	if name == "" {
		name = "subagent"
	}
	return name
}

func sanitizeBranchName(branch string) string {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return ""
	}
	branch = strings.ToLower(branch)
	replacer := strings.NewReplacer(" ", "-", "\\", "-", ":", "-", "..", "-", "@", "-", "~", "-", "^", "-", "*", "-", "?", "-", "[", "-", "]", "-")
	branch = replacer.Replace(branch)
	branch = strings.Trim(branch, "/-")
	return branch
}

func defaultSubAgentBranch(taskKey, agentID string) string {
	slug := sanitizeWorktreeName(taskKey, agentID, "")
	shortID := strings.TrimPrefix(agentID, "agent-")
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	if shortID == "" {
		shortID = "session"
	}
	return fmt.Sprintf("subagent/%s-%s", slug, shortID)
}

func branchExists(root, branch string) bool {
	if branch == "" {
		return false
	}
	cmd := exec.Command("git", "-C", root, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func currentWorktreeBranch(ctx context.Context, worktreeDir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", worktreeDir, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	branch := strings.TrimSpace(string(out))
	if branch == "HEAD" {
		return "", nil
	}
	return branch, nil
}

func runGit(ctx context.Context, root string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s failed: %s", strings.Join(args, " "), strings.TrimSpace(string(output)))
	}
	return nil
}

func removeWorktree(root, worktreeDir string) error {
	rmCmd := exec.Command("git", "-C", root, "worktree", "remove", "--force", worktreeDir)
	rmOut, rmErr := rmCmd.CombinedOutput()
	if rmErr != nil {
		slog.Warn("Failed to remove sub-agent worktree", "error", rmErr, "output", strings.TrimSpace(string(rmOut)))
	}
	if err := os.RemoveAll(worktreeDir); err != nil {
		slog.Debug("Failed to remove sub-agent worktree directory", "error", err)
	}
	return rmErr
}
