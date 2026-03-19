package tools

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

const (
	gitSnapshotDebounce      = 1500 * time.Millisecond
	gitSnapshotCommitTimeout = 15 * time.Second
)

type gitSnapshotManager struct {
	mu   sync.Mutex
	jobs map[string]*gitSnapshotJob
}

type gitSnapshotJob struct {
	worktreeDir string
	triggerCh   chan struct{}
	flushCh     chan chan error
}

var sharedGitSnapshotManager = &gitSnapshotManager{
	jobs: make(map[string]*gitSnapshotJob),
}

func QueueGitSnapshot(ctx context.Context, mutatedPath string) {
	gitDir := resolveSnapshotWorktreeDir(ctx, mutatedPath)
	if gitDir == "" {
		return
	}
	sharedGitSnapshotManager.notify(gitDir)
}

func FlushGitSnapshot(ctx context.Context, worktreeDir string) error {
	worktreeDir = filepath.Clean(strings.TrimSpace(worktreeDir))
	if !isManagedGitSnapshotDir(worktreeDir) {
		return nil
	}
	return sharedGitSnapshotManager.flush(ctx, worktreeDir)
}

func resolveSnapshotWorktreeDir(ctx context.Context, mutatedPath string) string {
	if workingDir := filepath.Clean(strings.TrimSpace(GetWorkingDirFromContext(ctx))); workingDir != "" {
		if gitDir := findGitSnapshotDir(workingDir); gitDir != "" {
			return gitDir
		}
	}

	if mutatedPath == "" {
		return ""
	}

	path := filepath.Clean(mutatedPath)
	if gitDir := findGitSnapshotDir(path); gitDir != "" {
		return gitDir
	}
	return ""
}

func isSapphireWorktreeDir(path string) bool {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" {
		return false
	}
	slashed := filepath.ToSlash(path)
	if !strings.Contains(slashed, "/.sapphire/worktrees/") && !strings.HasPrefix(slashed, ".sapphire/worktrees/") {
		return false
	}
	if _, err := os.Stat(filepath.Join(path, ".git")); err != nil {
		return false
	}
	return true
}

func isManagedGitSnapshotDir(path string) bool {
	return findGitSnapshotDir(path) != ""
}

func findGitSnapshotDir(path string) string {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" {
		return ""
	}
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	if !info.IsDir() {
		path = filepath.Dir(path)
	}
	for {
		if _, err := os.Stat(filepath.Join(path, ".git")); err == nil {
			return path
		}
		parent := filepath.Dir(path)
		if parent == path {
			return ""
		}
		path = parent
	}
}

func (m *gitSnapshotManager) notify(worktreeDir string) {
	job := m.ensureJob(worktreeDir)
	select {
	case job.triggerCh <- struct{}{}:
	default:
	}
}

func (m *gitSnapshotManager) flush(ctx context.Context, worktreeDir string) error {
	job := m.ensureJob(worktreeDir)
	select {
	case job.triggerCh <- struct{}{}:
	default:
	}
	resp := make(chan error, 1)
	select {
	case job.flushCh <- resp:
	case <-ctx.Done():
		return ctx.Err()
	}

	select {
	case err := <-resp:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *gitSnapshotManager) ensureJob(worktreeDir string) *gitSnapshotJob {
	m.mu.Lock()
	defer m.mu.Unlock()

	worktreeDir = filepath.Clean(worktreeDir)
	if job := m.jobs[worktreeDir]; job != nil {
		return job
	}

	job := &gitSnapshotJob{
		worktreeDir: worktreeDir,
		triggerCh:   make(chan struct{}, 1),
		flushCh:     make(chan chan error),
	}
	m.jobs[worktreeDir] = job
	go job.run()
	return job
}

func (j *gitSnapshotJob) run() {
	var (
		timer   *time.Timer
		timerCh <-chan time.Time
		pending bool
	)

	stopTimer := func() {
		if timer == nil {
			return
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer = nil
		timerCh = nil
	}

	resetTimer := func() {
		if timer == nil {
			timer = time.NewTimer(gitSnapshotDebounce)
		} else {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(gitSnapshotDebounce)
		}
		timerCh = timer.C
	}

	for {
		select {
		case <-j.triggerCh:
			pending = true
			resetTimer()
		case resp := <-j.flushCh:
			stopTimer()
			err := commitGitSnapshot(j.worktreeDir)
			pending = false
			resp <- err
		case <-timerCh:
			stopTimer()
			if !pending {
				continue
			}
			if err := commitGitSnapshot(j.worktreeDir); err != nil {
				slog.Warn("Failed to create git snapshot commit", "worktree", j.worktreeDir, "error", err)
			}
			pending = false
		}
	}
}

func commitGitSnapshot(worktreeDir string) error {
	if !isManagedGitSnapshotDir(worktreeDir) {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), gitSnapshotCommitTimeout)
	defer cancel()

	if _, err := runGitSnapshotCommand(ctx, worktreeDir, "rev-parse", "--is-inside-work-tree"); err != nil {
		return err
	}
	if _, err := runGitSnapshotCommand(ctx, worktreeDir, "add", "-A"); err != nil {
		return err
	}
	hasChanges, err := gitSnapshotHasStagedChanges(ctx, worktreeDir)
	if err != nil {
		return err
	}
	if !hasChanges {
		return nil
	}

	message := fmt.Sprintf("snapshot: %s %s", gitSnapshotActorName(worktreeDir), time.Now().UTC().Format("20060102-150405"))
	cmd := exec.CommandContext(
		ctx,
		"git",
		"-C", worktreeDir,
		"-c", "user.name=Sapphire Snapshot",
		"-c", "user.email=sapphire@local",
		"commit",
		"--no-verify",
		"-m", message,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		text := strings.TrimSpace(string(output))
		if strings.Contains(text, "nothing to commit") {
			return nil
		}
		return fmt.Errorf("git snapshot commit failed: %w: %s", err, text)
	}
	return nil
}

func gitSnapshotActorName(worktreeDir string) string {
	if isSapphireWorktreeDir(worktreeDir) {
		slashed := filepath.ToSlash(filepath.Clean(worktreeDir))
		parts := strings.Split(slashed, "/")
		for i := 0; i+4 < len(parts); i++ {
			if parts[i] == ".sapphire" && parts[i+1] == "worktrees" && parts[i+2] == "agent" {
				return parts[i+3] + "-" + parts[i+4]
			}
		}
		return "sub-agent"
	}
	return "main-agent"
}

func runGitSnapshotCommand(ctx context.Context, worktreeDir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", worktreeDir}, args...)...)
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func gitSnapshotHasStagedChanges(ctx context.Context, worktreeDir string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", worktreeDir, "diff", "--cached", "--quiet", "--ignore-submodules", "--exit-code")
	output, err := cmd.CombinedOutput()
	if err == nil {
		return false, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return true, nil
	}
	return false, fmt.Errorf("git diff --cached failed: %w: %s", err, strings.TrimSpace(string(output)))
}
