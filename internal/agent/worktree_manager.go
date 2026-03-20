package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
	"github.com/duggal1/Sapphire-cli/internal/worktreepolicy"
)

const (
	worktreeKindMain        = "main"
	worktreeKindSubagent    = "subagent"
	worktreeKindIntegration = "integration"
)

type managedWorktreeHandle struct {
	Run orchestrationdb.WorktreeRun
}

type worktreeManager struct {
	coordinator *coordinator
}

func newWorktreeManager(c *coordinator) *worktreeManager {
	return &worktreeManager{coordinator: c}
}

func (m *worktreeManager) PrepareMain(ctx context.Context, sessionID, title string, policy worktreepolicy.Policy) (managedWorktreeHandle, error) {
	root := m.coordinator.cfg.WorkingDir()
	baseRef := resolveWorktreeBaseRef(ctx, root)
	policy = worktreepolicy.Normalize(policy)

	if policy != worktreepolicy.Isolated {
		return managedWorktreeHandle{
			Run: orchestrationdb.WorktreeRun{
				SessionID:    strings.TrimSpace(sessionID),
				Kind:         worktreeKindMain,
				Policy:       string(policy),
				Status:       "ready",
				RepoRoot:     root,
				WorktreePath: root,
				Branch:       baseRef,
				BaseRef:      baseRef,
				Title:        strings.TrimSpace(title),
			},
		}, nil
	}

	existingRuns, err := m.coordinator.orchestrationStore.ListWorktreeRuns(ctx, sessionID, []string{"allocating", "ready", "running", "quarantined", "broken"}, 20)
	if err == nil {
		for _, run := range existingRuns {
			if run.Kind != worktreeKindMain {
				continue
			}
			if strings.TrimSpace(run.WorktreePath) == "" {
				continue
			}
			if isSubAgentWorktree(run.WorktreePath) {
				run.Status = "ready"
				run.UpdatedAt = time.Now().UTC()
				if saveErr := m.save(ctx, run); saveErr != nil {
					return managedWorktreeHandle{}, saveErr
				}
				return managedWorktreeHandle{Run: run}, nil
			}
		}
	}

	sessionSlug := sanitizeWorktreeSlug(sessionID)
	if sessionSlug == "" {
		sessionSlug = "session"
	}
	titleSlug := sanitizeWorktreeSlug(title)
	if titleSlug == "" {
		titleSlug = "main"
	}
	worktreePath := filepath.Join(root, ".sapphire", "worktrees", "main", sessionSlug, titleSlug)
	branch := fmt.Sprintf("session/%s/%s", sessionSlug, titleSlug)

	run := orchestrationdb.WorktreeRun{
		ID:           uuid.NewString(),
		SessionID:    strings.TrimSpace(sessionID),
		Kind:         worktreeKindMain,
		Policy:       string(policy),
		Status:       "allocating",
		RepoRoot:     root,
		WorktreePath: worktreePath,
		Branch:       branch,
		BaseRef:      baseRef,
		Title:        strings.TrimSpace(title),
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	release := m.coordinator.lockWorktreePath(worktreePath)
	defer release()

	if err := os.MkdirAll(filepath.Dir(worktreePath), 0o755); err != nil {
		return managedWorktreeHandle{}, fmt.Errorf("create main worktree parent failed: %w", err)
	}
	if err := resetWorktreeState(ctx, root, worktreePath); err != nil {
		return managedWorktreeHandle{}, err
	}
	if err := addWorktreeWithRecovery(ctx, root, worktreePath, branch, baseRef); err != nil {
		return managedWorktreeHandle{}, err
	}
	run.Status = "ready"
	run.UpdatedAt = time.Now().UTC()
	if err := m.save(ctx, run); err != nil {
		return managedWorktreeHandle{}, err
	}
	return managedWorktreeHandle{Run: run}, nil
}

func (m *worktreeManager) PrepareSubAgent(ctx context.Context, sessionID, agentID, parentAgentID string, spec subAgentWorktreeSpec, title string, policy worktreepolicy.Policy) (managedWorktreeHandle, func(), error) {
	root := m.coordinator.cfg.WorkingDir()
	baseRef := resolveWorktreeBaseRef(ctx, root)
	policy = worktreepolicy.Normalize(policy)

	run := orchestrationdb.WorktreeRun{
		ID:            uuid.NewString(),
		SessionID:     strings.TrimSpace(sessionID),
		AgentID:       strings.TrimSpace(agentID),
		ParentAgentID: strings.TrimSpace(parentAgentID),
		Kind:          worktreeKindSubagent,
		Policy:        string(policy),
		Status:        "allocating",
		RepoRoot:      root,
		BaseRef:       baseRef,
		TaskKey:       strings.TrimSpace(spec.TaskKey),
		Title:         strings.TrimSpace(title),
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}

	if policy != worktreepolicy.Isolated {
		run.Status = "ready"
		run.WorktreePath = root
		run.Branch = baseRef
		if err := m.save(ctx, run); err != nil {
			return managedWorktreeHandle{}, func() {}, err
		}
		return managedWorktreeHandle{Run: run}, func() {}, nil
	}

	worktreeDir := spec.WorktreePath
	if strings.TrimSpace(worktreeDir) == "" {
		worktreeDir = m.coordinator.defaultSubAgentWorktreePath(root, spec.TaskKey, spec.AssignmentID)
	}
	if !filepath.IsAbs(worktreeDir) {
		worktreeDir = filepath.Join(root, worktreeDir)
	}
	worktreeDir = filepath.Clean(worktreeDir)
	release := m.coordinator.lockWorktreePath(worktreeDir)
	defer release()

	branch := sanitizeBranchName(spec.Branch)
	if branch == "" {
		branch = defaultSubAgentBranch(spec.TaskKey, spec.AssignmentID)
	}

	run.WorktreePath = worktreeDir
	run.Branch = branch

	if spec.Reuse && !spec.AllowReuse {
		return managedWorktreeHandle{}, func() {}, fmt.Errorf("worktree reuse is forbidden; use resume_agent to continue an existing worktree")
	}

	if spec.Reuse {
		if !isParseableWorktreePath(root, worktreeDir) {
			return managedWorktreeHandle{}, func() {}, fmt.Errorf("worktree path %s is not allowed; expected .sapphire/worktrees/agent/<id>/<task-slug>", worktreeDir)
		}
		if !isSubAgentWorktree(worktreeDir) {
			return managedWorktreeHandle{}, func() {}, fmt.Errorf("worktree %s does not exist for reuse", worktreeDir)
		}
		if current, err := currentWorktreeBranch(ctx, worktreeDir); err == nil && current != "" {
			branch = current
			run.Branch = current
		}
		if !isParseableAgentBranch(branch) {
			return managedWorktreeHandle{}, func() {}, fmt.Errorf("branch %s is not allowed; expected format agent/<id>/<task-slug>", branch)
		}
		run.Status = "ready"
		run.UpdatedAt = time.Now().UTC()
		if err := m.save(ctx, run); err != nil {
			return managedWorktreeHandle{}, func() {}, err
		}
		return managedWorktreeHandle{Run: run}, func() {}, nil
	}

	if err := ensureCleanBaseWorktree(ctx, root); err != nil {
		return managedWorktreeHandle{}, func() {}, err
	}
	if !isParseableAgentBranch(branch) {
		return managedWorktreeHandle{}, func() {}, fmt.Errorf("branch %s is not allowed; expected format agent/<id>/<task-slug>", branch)
	}
	if !isParseableWorktreePath(root, worktreeDir) {
		return managedWorktreeHandle{}, func() {}, fmt.Errorf("worktree path %s is not allowed; expected .sapphire/worktrees/agent/<id>/<task-slug>", worktreeDir)
	}
	if owner := m.coordinator.activeSubAgentUsingWorktree(worktreeDir, agentID); owner != "" {
		return managedWorktreeHandle{}, func() {}, fmt.Errorf("worktree %s is already owned by active sub-agent %s", worktreeDir, owner)
	}
	if owner := m.coordinator.activeSubAgentUsingBranch(branch, worktreeDir, agentID); owner != "" {
		return managedWorktreeHandle{}, func() {}, fmt.Errorf("branch %s is already owned by active sub-agent %s", branch, owner)
	}

	ensureSapphireWorktreesGitignored(root)
	if err := os.MkdirAll(filepath.Dir(worktreeDir), 0o755); err != nil {
		return managedWorktreeHandle{}, func() {}, fmt.Errorf("create worktree parent failed: %w", err)
	}

	wtCtx, cancel := context.WithTimeout(ctx, subAgentWorktreeTimeout)
	defer cancel()
	if err := resetWorktreeState(wtCtx, root, worktreeDir); err != nil {
		return managedWorktreeHandle{}, func() {}, err
	}
	if err := addWorktreeWithRecovery(wtCtx, root, worktreeDir, branch, baseRef); err != nil {
		return managedWorktreeHandle{}, func() {}, err
	}

	run.Status = "ready"
	run.UpdatedAt = time.Now().UTC()
	if err := m.save(ctx, run); err != nil {
		return managedWorktreeHandle{}, func() {}, err
	}

	cleanup := func() {
		_ = m.Remove(context.Background(), run.ID, true)
	}
	return managedWorktreeHandle{Run: run}, cleanup, nil
}

func (m *worktreeManager) PrepareIntegration(ctx context.Context, sessionID, title, worktreePath, branch, baseRef string) (managedWorktreeHandle, error) {
	root := m.coordinator.cfg.WorkingDir()
	worktreePath = filepath.Clean(strings.TrimSpace(worktreePath))
	if worktreePath == "" {
		return managedWorktreeHandle{}, fmt.Errorf("integration worktree path is required")
	}
	if branch == "" {
		return managedWorktreeHandle{}, fmt.Errorf("integration branch is required")
	}
	release := m.coordinator.lockWorktreePath(worktreePath)
	defer release()

	run := orchestrationdb.WorktreeRun{
		ID:           uuid.NewString(),
		SessionID:    strings.TrimSpace(sessionID),
		Kind:         worktreeKindIntegration,
		Policy:       string(worktreepolicy.Isolated),
		Status:       "allocating",
		RepoRoot:     root,
		WorktreePath: worktreePath,
		Branch:       branch,
		BaseRef:      baseRef,
		Title:        strings.TrimSpace(title),
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	if err := os.MkdirAll(filepath.Dir(worktreePath), 0o755); err != nil {
		return managedWorktreeHandle{}, err
	}
	if err := resetWorktreeState(ctx, root, worktreePath); err != nil {
		return managedWorktreeHandle{}, err
	}
	if err := addWorktreeWithRecovery(ctx, root, worktreePath, branch, baseRef); err != nil {
		return managedWorktreeHandle{}, err
	}
	run.Status = "ready"
	run.UpdatedAt = time.Now().UTC()
	if err := m.save(ctx, run); err != nil {
		return managedWorktreeHandle{}, err
	}
	return managedWorktreeHandle{Run: run}, nil
}

func (m *worktreeManager) MarkStatusByPath(ctx context.Context, worktreePath, status string) {
	worktreePath = strings.TrimSpace(worktreePath)
	status = strings.TrimSpace(status)
	if worktreePath == "" || status == "" {
		return
	}
	run, err := m.coordinator.orchestrationStore.GetWorktreeRunByPath(ctx, worktreePath)
	if err != nil {
		return
	}
	run.Status = status
	run.UpdatedAt = time.Now().UTC()
	switch status {
	case "landed":
		run.LandedAt = time.Now().UTC()
	case "removed":
		run.RemovedAt = time.Now().UTC()
	}
	_ = m.save(ctx, run)
}

func (m *worktreeManager) List(ctx context.Context, sessionID string, statuses []string, limit int) ([]orchestrationdb.WorktreeRun, error) {
	return m.coordinator.orchestrationStore.ListWorktreeRuns(ctx, sessionID, statuses, limit)
}

func (m *worktreeManager) Repair(ctx context.Context, idOrPath string) (orchestrationdb.WorktreeRun, error) {
	run, err := m.lookup(ctx, idOrPath)
	if err != nil {
		return orchestrationdb.WorktreeRun{}, err
	}
	if run.Policy != string(worktreepolicy.Isolated) {
		return run, nil
	}
	root := run.RepoRoot
	if root == "" {
		root = m.coordinator.cfg.WorkingDir()
	}
	baseRef := strings.TrimSpace(run.BaseRef)
	if baseRef == "" {
		baseRef = resolveWorktreeBaseRef(ctx, root)
	}

	release := m.coordinator.lockWorktreePath(run.WorktreePath)
	defer release()
	if err := resetWorktreeState(ctx, root, run.WorktreePath); err != nil {
		return orchestrationdb.WorktreeRun{}, err
	}
	if err := addWorktreeWithRecovery(ctx, root, run.WorktreePath, run.Branch, baseRef); err != nil {
		return orchestrationdb.WorktreeRun{}, err
	}
	run.Status = "ready"
	run.UpdatedAt = time.Now().UTC()
	if err := m.save(ctx, run); err != nil {
		return orchestrationdb.WorktreeRun{}, err
	}
	return run, nil
}

func (m *worktreeManager) Remove(ctx context.Context, idOrPath string, force bool) error {
	run, err := m.lookup(ctx, idOrPath)
	if err != nil {
		return err
	}
	if run.Policy != string(worktreepolicy.Isolated) {
		run.Status = "removed"
		run.RemovedAt = time.Now().UTC()
		run.UpdatedAt = time.Now().UTC()
		return m.save(ctx, run)
	}
	root := run.RepoRoot
	if root == "" {
		root = m.coordinator.cfg.WorkingDir()
	}
	release := m.coordinator.lockWorktreePath(run.WorktreePath)
	defer release()
	if force {
		if err := forceRemoveWorktree(ctx, root, run.WorktreePath); err != nil {
			return err
		}
		_ = os.RemoveAll(run.WorktreePath)
	} else if err := removeWorktree(root, run.WorktreePath); err != nil {
		return err
	}
	run.Status = "removed"
	run.RemovedAt = time.Now().UTC()
	run.UpdatedAt = time.Now().UTC()
	return m.save(ctx, run)
}

func (m *worktreeManager) Quarantine(ctx context.Context, idOrPath, taskSlug string) (orchestrationdb.WorktreeRun, error) {
	run, err := m.lookup(ctx, idOrPath)
	if err != nil {
		return orchestrationdb.WorktreeRun{}, err
	}
	if run.Policy != string(worktreepolicy.Isolated) {
		run.Status = "quarantined"
		run.UpdatedAt = time.Now().UTC()
		_ = m.save(ctx, run)
		return run, nil
	}
	root := run.RepoRoot
	if root == "" {
		root = m.coordinator.cfg.WorkingDir()
	}
	if err := m.coordinator.quarantineWorktree(root, run.WorktreePath, taskSlug); err != nil {
		return orchestrationdb.WorktreeRun{}, err
	}
	run.Status = "quarantined"
	run.UpdatedAt = time.Now().UTC()
	if err := m.save(ctx, run); err != nil {
		return orchestrationdb.WorktreeRun{}, err
	}
	return run, nil
}

func (m *worktreeManager) Land(ctx context.Context, idOrPath, strategy string) (orchestrationdb.WorktreeRun, error) {
	run, err := m.lookup(ctx, idOrPath)
	if err != nil {
		return orchestrationdb.WorktreeRun{}, err
	}
	if run.Policy != string(worktreepolicy.Isolated) {
		return run, fmt.Errorf("worktree %s is not isolated", run.ID)
	}
	root := run.RepoRoot
	if root == "" {
		root = m.coordinator.cfg.WorkingDir()
	}
	branch := strings.TrimSpace(run.Branch)
	if branch == "" {
		return orchestrationdb.WorktreeRun{}, fmt.Errorf("worktree branch is required")
	}
	strategy = strings.ToLower(strings.TrimSpace(strategy))
	if strategy == "" {
		strategy = "merge"
	}
	switch strategy {
	case "merge":
		if err := runGit(ctx, root, "merge", "--no-ff", "--no-edit", branch); err != nil {
			return orchestrationdb.WorktreeRun{}, err
		}
	case "squash":
		if err := runGit(ctx, root, "merge", "--squash", branch); err != nil {
			return orchestrationdb.WorktreeRun{}, err
		}
		if err := runGit(ctx, root, "commit", "-m", fmt.Sprintf("squash merge %s", branch)); err != nil {
			return orchestrationdb.WorktreeRun{}, err
		}
	case "cherry_pick":
		baseRef := strings.TrimSpace(run.BaseRef)
		if baseRef == "" {
			baseRef = resolveWorktreeBaseRef(ctx, root)
		}
		commits, err := gitOutputAt(root, "rev-list", "--reverse", fmt.Sprintf("%s..%s", baseRef, branch))
		if err != nil {
			return orchestrationdb.WorktreeRun{}, err
		}
		for _, commit := range strings.Fields(commits) {
			if err := runGit(ctx, root, "cherry-pick", commit); err != nil {
				return orchestrationdb.WorktreeRun{}, err
			}
		}
	case "manual_review":
		run.Status = "ready_for_review"
		run.UpdatedAt = time.Now().UTC()
		if err := m.save(ctx, run); err != nil {
			return orchestrationdb.WorktreeRun{}, err
		}
		return run, nil
	default:
		return orchestrationdb.WorktreeRun{}, fmt.Errorf("unsupported merge strategy %q", strategy)
	}
	run.Status = "landed"
	run.LandedAt = time.Now().UTC()
	run.UpdatedAt = time.Now().UTC()
	if err := m.save(ctx, run); err != nil {
		return orchestrationdb.WorktreeRun{}, err
	}
	return run, nil
}

func (m *worktreeManager) lookup(ctx context.Context, idOrPath string) (orchestrationdb.WorktreeRun, error) {
	idOrPath = strings.TrimSpace(idOrPath)
	if idOrPath == "" {
		return orchestrationdb.WorktreeRun{}, fmt.Errorf("worktree id or path is required")
	}
	if strings.Contains(idOrPath, string(filepath.Separator)) || strings.HasPrefix(idOrPath, ".") {
		return m.coordinator.orchestrationStore.GetWorktreeRunByPath(ctx, idOrPath)
	}
	return m.coordinator.orchestrationStore.GetWorktreeRun(ctx, idOrPath)
}

func (m *worktreeManager) save(ctx context.Context, run orchestrationdb.WorktreeRun) error {
	if m == nil || m.coordinator == nil || m.coordinator.orchestrationStore == nil {
		return nil
	}
	return m.coordinator.orchestrationStore.UpsertWorktreeRun(ctx, run)
}
