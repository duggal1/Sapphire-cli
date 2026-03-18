package agent

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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
		worktreeDir = c.defaultSubAgentWorktreePath(root, spec.TaskKey, spec.Branch, agentID)
	}
	if !filepath.IsAbs(worktreeDir) {
		worktreeDir = filepath.Join(root, worktreeDir)
	}
	worktreeDir = filepath.Clean(worktreeDir)
	release := c.lockWorktreePath(worktreeDir)
	defer release()

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

	if owner := c.activeSubAgentUsingWorktree(worktreeDir, agentID); owner != "" {
		return "", "", func() {}, fmt.Errorf("worktree %s is already owned by active sub-agent %s", worktreeDir, owner)
	}
	if owner := c.activeSubAgentUsingBranch(branch, worktreeDir, agentID); owner != "" {
		return "", "", func() {}, fmt.Errorf("branch %s is already owned by active sub-agent %s", branch, owner)
	}

	if err := os.MkdirAll(filepath.Dir(worktreeDir), 0o755); err != nil {
		return "", "", func() {}, fmt.Errorf("create worktree parent failed: %w", err)
	}

	wtCtx, cancel := context.WithTimeout(ctx, subAgentWorktreeTimeout)
	defer cancel()
	if err := resetWorktreeState(wtCtx, root, worktreeDir); err != nil {
		return "", "", func() {}, err
	}

	baseRef := resolveWorktreeBaseRef(ctx, root)
	if err := addWorktreeWithRecovery(wtCtx, root, worktreeDir, branch, baseRef); err != nil {
		return "", "", func() {}, err
	}

	cleanup := c.subAgentWorktreeCleanup(root, worktreeDir)
	return worktreeDir, branch, cleanup, nil
}

func (c *coordinator) subAgentWorktreeRoot(root string) string {
	return filepath.Join(root, "worktrees")
}

func (c *coordinator) defaultSubAgentWorktreePath(root, taskKey, branch, agentID string) string {
	slug := worktreeTaskSlug(taskKey, branch)
	if slug == "" {
		slug = "task"
	}
	shortID := shortAgentID(agentID)
	if shortID == "" {
		shortID = "agent"
	}
	return filepath.Join(c.subAgentWorktreeRoot(root), "agent", shortID, slug)
}

func (c *coordinator) subAgentWorktreeCleanup(root, worktreeDir string) func() {
	return func() {
		release := c.lockWorktreePath(worktreeDir)
		defer release()
		if err := removeWorktree(root, worktreeDir); err != nil {
			slog.Warn("Failed to remove sub-agent worktree", "error", err)
		}
	}
}

// quarantineWorktree moves a failed worktree with changes to a quarantine
// directory instead of deleting it. This preserves evidence for review.
func (c *coordinator) quarantineWorktree(root, worktreeDir, taskSlug string) error {
	release := c.lockWorktreePath(worktreeDir)
	defer release()

	if !isSubAgentWorktree(worktreeDir) {
		return nil
	}

	// Check if worktree has changes
	ctx, cancel := context.WithTimeout(context.Background(), subAgentWorktreeTimeout)
	defer cancel()
	diffOut, err := runGitOutput(ctx, worktreeDir, "diff", "--stat")
	if err != nil || strings.TrimSpace(diffOut) == "" {
		// No changes or error reading — safe to delete
		return removeWorktree(root, worktreeDir)
	}

	// Move to quarantine
	slug := sanitizeWorktreeSlug(taskSlug)
	if slug == "" {
		slug = "unknown"
	}
	quarantineDir := filepath.Join(c.subAgentWorktreeRoot(root), "quarantine", slug)
	if err := os.MkdirAll(filepath.Dir(quarantineDir), 0o755); err != nil {
		return fmt.Errorf("create quarantine parent: %w", err)
	}
	if err := os.Rename(worktreeDir, quarantineDir); err != nil {
		slog.Warn("Quarantine rename failed, preserving in place", "error", err)
		return nil
	}
	slog.Info("Quarantined failed worktree", "from", worktreeDir, "to", quarantineDir)
	return nil
}

// resumeWorktree allows another agent to pick up an existing orphaned or
// quarantined worktree and continue work in it.
func (c *coordinator) resumeWorktree(ctx context.Context, worktreeDir string) (string, error) {
	release := c.lockWorktreePath(worktreeDir)
	defer release()

	if !isSubAgentWorktree(worktreeDir) {
		return "", fmt.Errorf("worktree %s does not exist or is not a git worktree", worktreeDir)
	}

	if owner := c.activeSubAgentUsingWorktree(worktreeDir, ""); owner != "" {
		return "", fmt.Errorf("worktree %s is still owned by active sub-agent %s", worktreeDir, owner)
	}

	branch, err := currentWorktreeBranch(ctx, worktreeDir)
	if err != nil {
		return "", fmt.Errorf("failed to read current branch in worktree %s: %w", worktreeDir, err)
	}
	return branch, nil
}

func (c *coordinator) lockWorktreePath(worktreeDir string) func() {
	c.worktreeOpsMu.Lock()
	if c.worktreeOps == nil {
		c.worktreeOps = make(map[string]*sync.Mutex)
	}
	lock := c.worktreeOps[worktreeDir]
	if lock == nil {
		lock = &sync.Mutex{}
		c.worktreeOps[worktreeDir] = lock
	}
	c.worktreeOpsMu.Unlock()
	lock.Lock()
	return lock.Unlock
}

func (c *coordinator) activeSubAgentUsingWorktree(worktreeDir, excludeAgentID string) string {
	for _, runner := range c.ensureSubAgentRegistry().list() {
		id := runner.id
		if id == excludeAgentID || runner == nil {
			continue
		}
		runner.mu.Lock()
		active := !runner.closed && isSubAgentActiveStatus(runner.status)
		samePath := filepath.Clean(runner.workDir) == worktreeDir
		runner.mu.Unlock()
		if active && samePath {
			return id
		}
	}
	return ""
}

func (c *coordinator) activeSubAgentUsingBranch(branch, worktreeDir, excludeAgentID string) string {
	if branch == "" {
		return ""
	}
	for _, runner := range c.ensureSubAgentRegistry().list() {
		id := runner.id
		if id == excludeAgentID || runner == nil {
			continue
		}
		runner.mu.Lock()
		active := !runner.closed && isSubAgentActiveStatus(runner.status)
		sameBranch := strings.TrimSpace(runner.assignment.Branch) == branch
		samePath := filepath.Clean(runner.workDir) == worktreeDir
		runner.mu.Unlock()
		if active && sameBranch && !samePath {
			return id
		}
	}
	return ""
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

func worktreeTaskSlug(taskKey, branch string) string {
	name := strings.TrimSpace(taskKey)
	if name == "" {
		name = strings.TrimSpace(branch)
	}
	return sanitizeWorktreeSlug(name)
}

func sanitizeWorktreeSlug(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return ""
	}
	replacer := strings.NewReplacer(" ", "-", "/", "-", "\\", "-", ":", "-", ".", "-", "_", "-")
	name = replacer.Replace(name)
	return strings.Trim(name, "-")
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
	slug := worktreeTaskSlug(taskKey, "")
	if slug == "" {
		slug = "task"
	}
	shortID := shortAgentID(agentID)
	if shortID == "" {
		shortID = "session"
	}
	return fmt.Sprintf("agent/%s/%s", shortID, slug)
}

func shortAgentID(agentID string) string {
	shortID := strings.TrimPrefix(agentID, "agent-")
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	return shortID
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
	_, err := runGitOutput(ctx, root, args...)
	return err
}

func runGitOutput(ctx context.Context, root string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(output)), fmt.Errorf("git %s failed: %s", strings.Join(args, " "), strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func removeWorktree(root, worktreeDir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), subAgentWorktreeTimeout)
	defer cancel()
	if err := forceRemoveWorktree(ctx, root, worktreeDir); err != nil {
		slog.Warn("Failed to remove sub-agent worktree", "error", err)
		return err
	}
	if err := os.RemoveAll(worktreeDir); err != nil {
		slog.Debug("Failed to remove sub-agent worktree directory", "error", err)
	}
	return nil
}

func addWorktreeWithRecovery(ctx context.Context, root, worktreeDir, branch, baseRef string) error {
	if err := runGit(ctx, root, worktreeAddArgs(root, branch, worktreeDir, baseRef, false)...); err == nil {
		return nil
	}
	if err := resetWorktreeState(ctx, root, worktreeDir); err != nil {
		return err
	}
	return runGit(ctx, root, worktreeAddArgs(root, branch, worktreeDir, baseRef, true)...)
}

func worktreeAddArgs(root, branch, worktreeDir, baseRef string, force bool) []string {
	args := []string{"worktree", "add"}
	if force {
		args = append(args, "-f", "-f")
	}
	if branchExists(root, branch) {
		return append(args, worktreeDir, branch)
	}
	ref := strings.TrimSpace(baseRef)
	if ref == "" {
		ref = "HEAD"
	}
	return append(args, "-b", branch, worktreeDir, ref)
}

func resetWorktreeState(ctx context.Context, root, worktreeDir string) error {
	if err := unlockWorktree(ctx, root, worktreeDir); err != nil {
		return err
	}
	if err := forceRemoveWorktree(ctx, root, worktreeDir); err != nil {
		return err
	}
	if err := os.RemoveAll(worktreeDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale worktree directory: %w", err)
	}
	if _, err := runGitOutput(ctx, root, "worktree", "prune", "--expire", "now"); err != nil && !isIgnorableWorktreeStateError(err) {
		return err
	}
	return nil
}

func unlockWorktree(ctx context.Context, root, worktreeDir string) error {
	_, err := runGitOutput(ctx, root, "worktree", "unlock", worktreeDir)
	if err != nil && !isIgnorableWorktreeStateError(err) {
		return err
	}
	return nil
}

func forceRemoveWorktree(ctx context.Context, root, worktreeDir string) error {
	_, err := runGitOutput(ctx, root, "worktree", "remove", "--force", "--force", worktreeDir)
	if err == nil || isIgnorableWorktreeStateError(err) {
		return nil
	}
	if err := unlockWorktree(ctx, root, worktreeDir); err != nil {
		return err
	}
	_, err = runGitOutput(ctx, root, "worktree", "remove", "--force", "--force", worktreeDir)
	if err != nil && !isIgnorableWorktreeStateError(err) {
		return err
	}
	return nil
}

func isIgnorableWorktreeStateError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not a working tree") ||
		strings.Contains(msg, "not found") ||
		strings.Contains(msg, "no such file or directory") ||
		strings.Contains(msg, "cannot find worktree") ||
		strings.Contains(msg, "is not registered") ||
		strings.Contains(msg, "worktree prune")
}

func resolveWorktreeBaseRef(ctx context.Context, root string) string {
	if root == "" {
		return "HEAD"
	}
	if ref, err := runGitOutput(ctx, root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD"); err == nil && ref != "" {
		return ref
	}
	if branchExists(root, "main") {
		return "main"
	}
	if branchExists(root, "master") {
		return "master"
	}
	return "HEAD"
}
