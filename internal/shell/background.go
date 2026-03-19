package shell

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/duggal1/Sapphire-cli/internal/csync"
)

const (
	// MaxBackgroundJobs is the maximum number of concurrent background jobs allowed
	MaxBackgroundJobs = 120
	// CompletedJobRetentionMinutes is how long to keep completed jobs before auto-cleanup (8 hours)
	CompletedJobRetentionMinutes = 8 * 60
)

// BackgroundShell represents a shell running in the background.
type BackgroundShell struct {
	ID          string
	Command     string
	Description string
	Shell       *Shell
	WorkingDir  string
	ctx         context.Context
	cancel      context.CancelFunc
	stdout      *LineRingBuffer
	stderr      *LineRingBuffer
	done        chan struct{}
	exitErr     error
	completedAt int64 // Unix timestamp when job completed (0 if still running)
}

// BackgroundShellManager manages background shell instances.
type BackgroundShellManager struct {
	shells *csync.Map[string, *BackgroundShell]
}

var (
	backgroundManager     *BackgroundShellManager
	backgroundManagerOnce sync.Once
	idCounter             atomic.Uint64
)

// newBackgroundShellManager creates a new BackgroundShellManager instance.
func newBackgroundShellManager() *BackgroundShellManager {
	return &BackgroundShellManager{
		shells: csync.NewMap[string, *BackgroundShell](),
	}
}

// GetBackgroundShellManager returns the singleton background shell manager.
func GetBackgroundShellManager() *BackgroundShellManager {
	backgroundManagerOnce.Do(func() {
		backgroundManager = newBackgroundShellManager()
	})
	return backgroundManager
}

// Start creates and starts a new background shell with the given command.
func (m *BackgroundShellManager) Start(ctx context.Context, workingDir string, blockFuncs []BlockFunc, command string, description string, backend string) (*BackgroundShell, error) {
	// Check job limit
	if m.shells.Len() >= MaxBackgroundJobs {
		return nil, fmt.Errorf("maximum number of background jobs (%d) reached. Please terminate or wait for some jobs to complete", MaxBackgroundJobs)
	}

	id := fmt.Sprintf("%03X", idCounter.Add(1))

	shell := NewShell(&Options{
		WorkingDir: workingDir,
		BlockFuncs: blockFuncs,
		Backend:    backend,
	})

	shellCtx, cancel := context.WithCancel(ctx)

	bgShell := &BackgroundShell{
		ID:          id,
		Command:     command,
		Description: description,
		WorkingDir:  workingDir,
		Shell:       shell,
		ctx:         shellCtx,
		cancel:      cancel,
		stdout:      NewLineRingBuffer(10000),
		stderr:      NewLineRingBuffer(10000),
		done:        make(chan struct{}),
	}

	m.shells.Set(id, bgShell)

	go func() {
		defer close(bgShell.done)

		err := shell.ExecStream(shellCtx, command, bgShell.stdout, bgShell.stderr)

		bgShell.exitErr = err
		atomic.StoreInt64(&bgShell.completedAt, time.Now().Unix())
	}()

	return bgShell, nil
}

// Get retrieves a background shell by ID.
func (m *BackgroundShellManager) Get(id string) (*BackgroundShell, bool) {
	return m.shells.Get(id)
}

// Remove removes a background shell from the manager without terminating it.
// This is useful when a shell has already completed and you just want to clean up tracking.
func (m *BackgroundShellManager) Remove(id string) error {
	_, ok := m.shells.Take(id)
	if !ok {
		return fmt.Errorf("background shell not found: %s", id)
	}
	return nil
}

// Kill terminates a background shell by ID.
func (m *BackgroundShellManager) Kill(id string) error {
	shell, ok := m.shells.Take(id)
	if !ok {
		return fmt.Errorf("background shell not found: %s", id)
	}

	shell.cancel()
	<-shell.done
	return nil
}

// BackgroundShellInfo contains information about a background shell.
type BackgroundShellInfo struct {
	ID          string
	Command     string
	Description string
}

// List returns all background shell IDs.
func (m *BackgroundShellManager) List() []string {
	ids := make([]string, 0, m.shells.Len())
	for id := range m.shells.Seq2() {
		ids = append(ids, id)
	}
	return ids
}

// RunningCount returns the number of currently running background shells.
func (m *BackgroundShellManager) RunningCount() int {
	count := 0
	for shell := range m.shells.Seq() {
		if !shell.IsDone() {
			count++
		}
	}
	return count
}

// Cleanup removes completed jobs that have been finished for more than the retention period
func (m *BackgroundShellManager) Cleanup() int {
	now := time.Now().Unix()
	retentionSeconds := int64(CompletedJobRetentionMinutes * 60)

	var toRemove []string
	for shell := range m.shells.Seq() {
		completedAt := atomic.LoadInt64(&shell.completedAt)
		if completedAt > 0 && now-completedAt > retentionSeconds {
			toRemove = append(toRemove, shell.ID)
		}
	}

	for _, id := range toRemove {
		m.Remove(id)
	}

	return len(toRemove)
}

// KillAll terminates all background shells. The provided context bounds how
// long the function waits for each shell to exit.
func (m *BackgroundShellManager) KillAll(ctx context.Context) {
	shells := slices.Collect(m.shells.Seq())
	m.shells.Reset(map[string]*BackgroundShell{})

	var wg sync.WaitGroup
	for _, shell := range shells {
		wg.Go(func() {
			shell.cancel()
			select {
			case <-shell.done:
			case <-ctx.Done():
			}
		})
	}
	wg.Wait()
}

// GetOutput returns the current output of a background shell.
func (bs *BackgroundShell) GetOutput() (stdout string, stderr string, done bool, err error) {
	select {
	case <-bs.done:
		return bs.stdout.String(), bs.stderr.String(), true, bs.exitErr
	default:
		return bs.stdout.String(), bs.stderr.String(), false, nil
	}
}

// GetOutputSince returns the new output of a background shell since the given cursors.
func (bs *BackgroundShell) GetOutputSince(stdoutCursor, stderrCursor int) (stdout string, newStdoutCursor int, stdoutMissed bool, stderr string, newStderrCursor int, stderrMissed bool, done bool, err error) {
	stdout, newStdoutCursor, stdoutMissed = bs.stdout.ReadSince(stdoutCursor)
	stderr, newStderrCursor, stderrMissed = bs.stderr.ReadSince(stderrCursor)

	select {
	case <-bs.done:
		// Ensure any unwritten strings are captured if we are done
		bs.stdout.Flush()
		bs.stderr.Flush()

		// If flush added lines we need to re-read
		if bs.stdout.totalWritten > newStdoutCursor {
			var extraStdout string
			extraStdout, newStdoutCursor, _ = bs.stdout.ReadSince(newStdoutCursor)
			if stdout != "" && extraStdout != "" {
				stdout += "\n"
			}
			stdout += extraStdout
		}
		if bs.stderr.totalWritten > newStderrCursor {
			var extraStderr string
			extraStderr, newStderrCursor, _ = bs.stderr.ReadSince(newStderrCursor)
			if stderr != "" && extraStderr != "" {
				stderr += "\n"
			}
			stderr += extraStderr
		}

		done = true
		err = bs.exitErr
	default:
		done = false
		err = nil
	}

	return
}

// IsDone checks if the background shell has finished execution.
func (bs *BackgroundShell) IsDone() bool {
	select {
	case <-bs.done:
		return true
	default:
		return false
	}
}

// Wait blocks until the background shell completes.
func (bs *BackgroundShell) Wait() {
	<-bs.done
}

func (bs *BackgroundShell) WaitContext(ctx context.Context) bool {
	select {
	case <-bs.done:
		return true
	case <-ctx.Done():
		return false
	}
}
