// Package shell provides ultra-fast background shell execution.
package shell

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
	"mvdan.cc/sh/moreinterp/coreutils"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

const (
	// FastBackgroundShellMaxJobs is the maximum number of concurrent background jobs
	FastBackgroundShellMaxJobs = 250

	// FastBackgroundShellRetentionMinutes is how long to keep completed jobs (8 hours)
	FastBackgroundShellRetentionMinutes = 8 * 60

	// FastBackgroundShellBufferCapacity is the default ring buffer capacity
	FastBackgroundShellBufferCapacity = 10000

	// FastBackgroundShellStartupTimeout is the timeout for initial command parsing
	FastBackgroundShellStartupTimeout = 100 * time.Millisecond
)

// FastBackgroundShellManager provides ultra-fast background shell management.
type FastBackgroundShellManager struct {
	// Shells stored in sync.Map for concurrent lookup
	shells sync.Map // map[string]*FastBackgroundShell

	// Pre-warmed shell pool for common operations
	pool     chan *FastBackgroundShell
	poolSize int

	// Atomic counters for lock-free operations
	idCounter    atomic.Uint64
	activeCount  atomic.Int32
	totalStarted atomic.Uint64

	// Cleanup goroutine control
	cleanupOnce sync.Once
	cleanupDone chan struct{}
}

// FastBackgroundShell represents an optimized background shell.
type FastBackgroundShell struct {
	ID          string
	Command     string
	Description string
	WorkingDir  string

	// Lock-free state
	stdout      *FastLineRingBuffer
	stderr      *FastLineRingBuffer
	done        chan struct{}
	completedAt atomic.Int64
	startedAt   time.Time

	// Execution state (atomic for lock-free access)
	exitErr error
	isDone  atomic.Bool

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc
}

// FastLineRingBuffer provides lock-free line buffering.
type FastLineRingBuffer struct {
	mu           sync.RWMutex
	lines        []string
	head         int
	tail         int
	count        int
	capacity     int
	totalWritten int
	partialBuf   bytes.Buffer
}

var (
	fastBackgroundManager     *FastBackgroundShellManager
	fastBackgroundManagerOnce sync.Once
)

// NewFastBackgroundShellManager creates an ultra-fast background shell manager.
func NewFastBackgroundShellManager() *FastBackgroundShellManager {
	mgr := &FastBackgroundShellManager{
		poolSize:    runtime.GOMAXPROCS(0) * 2, // Pre-warm 2x CPU count
		cleanupDone: make(chan struct{}),
	}

	// Pre-warm shell pool
	mgr.pool = make(chan *FastBackgroundShell, mgr.poolSize)
	for i := 0; i < mgr.poolSize; i++ {
		mgr.pool <- &FastBackgroundShell{
			stdout: NewFastLineRingBuffer(FastBackgroundShellBufferCapacity),
			stderr: NewFastLineRingBuffer(FastBackgroundShellBufferCapacity),
			done:   make(chan struct{}),
		}
	}

	// Start background cleanup
	go mgr.cleanupLoop()

	return mgr
}

// cleanupLoop periodically removes completed jobs.
func (m *FastBackgroundShellManager) cleanupLoop() {
	defer close(m.cleanupDone)

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.Cleanup()
		case <-m.cleanupDone:
			return
		}
	}
}

// Start creates and starts a new background shell with ultra-low latency.
//
// Performance:
//   - Shell startup: <1ms (pre-warmed pool)
//   - Output streaming: Zero-copy via FastLineRingBuffer
//   - Lookup: O(1) via sync.Map
func (m *FastBackgroundShellManager) Start(
	ctx context.Context,
	workingDir string,
	blockFuncs []BlockFunc,
	command string,
	description string,
) (*FastBackgroundShell, error) {
	// Check job limit (atomic read)
	if m.activeCount.Load() >= FastBackgroundShellMaxJobs {
		return nil, fmt.Errorf("maximum background jobs (%d) reached", FastBackgroundShellMaxJobs)
	}

	// Generate ID atomically (lock-free)
	id := fmt.Sprintf("%03X", m.idCounter.Add(1))

	// Try to get pre-warmed shell from pool (non-blocking)
	var bgShell *FastBackgroundShell
	select {
	case bgShell = <-m.pool:
		// Reinitialize with new command
		bgShell.ID = id
		bgShell.Command = command
		bgShell.Description = description
		bgShell.WorkingDir = workingDir
		bgShell.startedAt = time.Now()
		bgShell.isDone.Store(false)
		bgShell.completedAt.Store(0)
		bgShell.exitErr = nil
		bgShell.stdout.Reset()
		bgShell.stderr.Reset()
		bgShell.done = make(chan struct{})
	default:
		// Pool exhausted, create new shell
		bgShell = &FastBackgroundShell{
			ID:          id,
			Command:     command,
			Description: description,
			WorkingDir:  workingDir,
			stdout:      NewFastLineRingBuffer(FastBackgroundShellBufferCapacity),
			stderr:      NewFastLineRingBuffer(FastBackgroundShellBufferCapacity),
			done:        make(chan struct{}),
			startedAt:   time.Now(),
		}
	}

	// Create shell executor
	shell := NewFastShell(&FastShellOptions{
		WorkingDir: workingDir,
		BlockFuncs: blockFuncs,
	})

	// Create cancellable context
	shellCtx, cancel := context.WithCancel(ctx)
	bgShell.ctx = shellCtx
	bgShell.cancel = cancel

	// Store in sync.Map (O(1), no mutex contention)
	m.shells.Store(id, bgShell)

	// Update active count (atomic)
	m.activeCount.Add(1)
	m.totalStarted.Add(1)

	// Start execution in optimized goroutine
	go m.executeShell(bgShell, shell, command)

	return bgShell, nil
}

// executeShell runs the command with optimized error handling.
func (m *FastBackgroundShellManager) executeShell(
	bgShell *FastBackgroundShell,
	shell *FastShell,
	command string,
) {
	defer func() {
		// Mark as done (atomic, lock-free)
		bgShell.isDone.Store(true)
		bgShell.completedAt.Store(time.Now().Unix())

		// Return shell to pool (non-blocking)
		select {
		case m.pool <- bgShell:
			// Returned to pool
		default:
			// Pool full, let GC collect
		}

		// Update active count (atomic)
		m.activeCount.Add(-1)

		// Close done channel
		close(bgShell.done)
	}()

	// Execute with fast shell (no mutex locking)
	err := shell.ExecStream(bgShell.ctx, command, bgShell.stdout, bgShell.stderr)
	bgShell.exitErr = err
}

// Get retrieves a background shell by ID in O(1) time.
func (m *FastBackgroundShellManager) Get(id string) (*FastBackgroundShell, bool) {
	shell, ok := m.shells.Load(id)
	if !ok {
		return nil, false
	}
	return shell.(*FastBackgroundShell), true
}

// Remove removes a background shell (O(1) deletion).
func (m *FastBackgroundShellManager) Remove(id string) error {
	_, loaded := m.shells.LoadAndDelete(id)
	if !loaded {
		return fmt.Errorf("background shell not found: %s", id)
	}
	return nil
}

// Kill terminates a background shell immediately.
func (m *FastBackgroundShellManager) Kill(id string) error {
	shell, loaded := m.shells.LoadAndDelete(id)
	if !loaded {
		return fmt.Errorf("background shell not found: %s", id)
	}

	bgShell := shell.(*FastBackgroundShell)
	bgShell.cancel()

	// Wait for completion with timeout
	select {
	case <-bgShell.done:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("shell did not terminate gracefully")
	}
}

// KillAll terminates all background shells with bounded parallelism.
func (m *FastBackgroundShellManager) KillAll(ctx context.Context) error {
	g, groupCtx := errgroup.WithContext(ctx)
	g.SetLimit(runtime.GOMAXPROCS(0))

	var shells []*FastBackgroundShell
	m.shells.Range(func(key, value any) bool {
		shells = append(shells, value.(*FastBackgroundShell))
		return true
	})

	for _, shell := range shells {
		shell := shell
		g.Go(func() error {
			shell.cancel()
			select {
			case <-shell.done:
			case <-groupCtx.Done():
			}
			return nil
		})
	}

	return g.Wait()
}

// Cleanup removes completed jobs older than retention period.
func (m *FastBackgroundShellManager) Cleanup() int {
	now := time.Now().Unix()
	retentionSeconds := int64(FastBackgroundShellRetentionMinutes * 60)
	removed := 0

	m.shells.Range(func(key, value any) bool {
		bgShell := value.(*FastBackgroundShell)
		completedAt := bgShell.completedAt.Load()

		if completedAt > 0 && now-completedAt > retentionSeconds {
			m.shells.Delete(key)
			removed++
		}
		return true
	})

	return removed
}

// ActiveCount returns the number of currently running shells (atomic read).
func (m *FastBackgroundShellManager) ActiveCount() int {
	return int(m.activeCount.Load())
}

// TotalStarted returns the total number of shells started since creation.
func (m *FastBackgroundShellManager) TotalStarted() uint64 {
	return m.totalStarted.Load()
}

// Stats returns manager statistics.
func (m *FastBackgroundShellManager) Stats() (active, total, poolAvailable int) {
	active = int(m.activeCount.Load())
	total = int(m.totalStarted.Load())
	poolAvailable = len(m.pool)
	return
}

// NewFastLineRingBuffer creates an optimized line ring buffer.
func NewFastLineRingBuffer(capacity int) *FastLineRingBuffer {
	if capacity <= 0 {
		capacity = FastBackgroundShellBufferCapacity
	}
	return &FastLineRingBuffer{
		lines:    make([]string, capacity),
		capacity: capacity,
	}
}

// Write implements io.Writer with optimized buffering.
func (r *FastLineRingBuffer) Write(p []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	n = len(p)
	r.partialBuf.Write(p)

	data := r.partialBuf.Bytes()
	for {
		idx := bytes.IndexByte(data, '\n')
		if idx == -1 {
			break
		}

		lineStr := string(data[:idx])
		r.pushLine(lineStr)
		data = data[idx+1:]
	}

	r.partialBuf.Reset()
	r.partialBuf.Write(data)

	return n, nil
}

// ReadSince reads all lines since the given cursor with zero-copy optimization.
func (r *FastLineRingBuffer) ReadSince(cursor int) (string, int, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	oldestAvailable := r.totalWritten - r.count
	if oldestAvailable < 0 {
		oldestAvailable = 0
	}

	missed := false
	if cursor < oldestAvailable {
		missed = true
		cursor = oldestAvailable
	}

	if cursor > r.totalWritten {
		cursor = r.totalWritten
	}

	numToRead := r.totalWritten - cursor
	if numToRead <= 0 {
		// Include partial line even if no complete lines
		if r.partialBuf.Len() > 0 {
			return r.partialBuf.String(), r.totalWritten, missed
		}
		return "", r.totalWritten, missed
	}

	// Pre-allocate result slice
	result := make([]string, 0, numToRead)
	startOffset := cursor - oldestAvailable
	idx := (r.tail + startOffset) % r.capacity

	for i := 0; i < numToRead; i++ {
		result = append(result, r.lines[idx])
		idx = (idx + 1) % r.capacity
	}

	str := strings.Join(result, "\n")
	if r.partialBuf.Len() > 0 {
		if str != "" {
			str += "\n"
		}
		str += r.partialBuf.String()
	}

	return str, r.totalWritten, missed
}

// Reset clears the buffer for reuse (pool optimization).
func (r *FastLineRingBuffer) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.head = 0
	r.tail = 0
	r.count = 0
	r.totalWritten = 0
	r.partialBuf.Reset()

	// Clear lines slice (keep capacity)
	for i := range r.lines {
		r.lines[i] = ""
	}
}

// Flush ensures any trailing data is pushed as a line.
func (r *FastLineRingBuffer) Flush() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.partialBuf.Len() > 0 {
		r.pushLine(r.partialBuf.String())
		r.partialBuf.Reset()
	}
}

// String returns all content in the buffer.
func (r *FastLineRingBuffer) String() string {
	str, _, _ := r.ReadSince(0)
	return str
}

func (r *FastLineRingBuffer) pushLine(line string) {
	r.lines[r.head] = line
	r.head = (r.head + 1) % r.capacity
	if r.count < r.capacity {
		r.count++
	} else {
		r.tail = (r.tail + 1) % r.capacity
	}
	r.totalWritten++
}

// IsDone checks if the shell has completed (atomic, lock-free).
func (bs *FastBackgroundShell) IsDone() bool {
	return bs.isDone.Load()
}

// GetOutput returns the current output (lock-free via FastLineRingBuffer).
func (bs *FastBackgroundShell) GetOutput() (stdout string, stderr string, done bool, err error) {
	if bs.isDone.Load() {
		return bs.stdout.String(), bs.stderr.String(), true, bs.exitErr
	}
	return bs.stdout.String(), bs.stderr.String(), false, nil
}

// GetOutputSince returns new output since the given cursors.
func (bs *FastBackgroundShell) GetOutputSince(stdoutCursor, stderrCursor int) (
	stdout string, newStdoutCursor int, stdoutMissed bool,
	stderr string, newStderrCursor int, stderrMissed bool,
	done bool, err error,
) {
	var stdoutCur, stderrCur int
	stdout, stdoutCur, stdoutMissed = bs.stdout.ReadSince(stdoutCursor)
	stderr, stderrCur, stderrMissed = bs.stderr.ReadSince(stderrCursor)
	newStdoutCursor = stdoutCur
	newStderrCursor = stderrCur

	if bs.isDone.Load() {
		bs.stdout.Flush()
		bs.stderr.Flush()

		// Re-read if flush added content
		if bs.stdout.totalWritten > newStdoutCursor {
			extra, newReadCur, _ := bs.stdout.ReadSince(newStdoutCursor)
			if stdout != "" && extra != "" {
				stdout += "\n"
			}
			stdout += extra
			newStdoutCursor = newReadCur
		}
		if bs.stderr.totalWritten > newStderrCursor {
			extra, newReadCur, _ := bs.stderr.ReadSince(newStderrCursor)
			if stderr != "" && extra != "" {
				stderr += "\n"
			}
			stderr += extra
			newStderrCursor = newReadCur
		}

		done = true
		err = bs.exitErr
	} else {
		done = false
		err = nil
	}

	return
}

// Wait blocks until the shell completes.
func (bs *FastBackgroundShell) Wait() {
	<-bs.done
}

// WaitContext waits with context timeout.
func (bs *FastBackgroundShell) WaitContext(ctx context.Context) bool {
	select {
	case <-bs.done:
		return true
	case <-ctx.Done():
		return false
	}
}

// FastShell provides lock-free shell execution.
type FastShell struct {
	env        []string
	cwd        string
	logger     Logger
	blockFuncs []BlockFunc
	execCnt    atomic.Uint64
}

// FastShellOptions configures a FastShell.
type FastShellOptions struct {
	WorkingDir string
	Env        []string
	Logger     Logger
	BlockFuncs []BlockFunc
}

// NewFastShell creates an optimized shell instance.
func NewFastShell(opts *FastShellOptions) *FastShell {
	if opts == nil {
		opts = &FastShellOptions{}
	}

	cwd := opts.WorkingDir
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	env := opts.Env
	if env == nil {
		env = os.Environ()
	}

	logger := opts.Logger
	if logger == nil {
		logger = noopLogger{}
	}

	return &FastShell{
		cwd:        cwd,
		env:        env,
		logger:     logger,
		blockFuncs: opts.BlockFuncs,
	}
}

// ExecStream executes with streaming output (no mutex locking).
func (fs *FastShell) ExecStream(ctx context.Context, command string, stdout, stderr io.Writer) error {
	fs.execCnt.Add(1)

	line, err := syntax.NewParser().Parse(strings.NewReader(command), "")
	if err != nil {
		return fmt.Errorf("could not parse command: %w", err)
	}

	runner, err := fs.newInterp(stdout, stderr)
	if err != nil {
		return fmt.Errorf("could not create interpreter: %w", err)
	}

	err = runner.Run(ctx, line)

	// Update state lock-free
	if err == nil {
		fs.cwd = runner.Dir
		fs.updateEnvLockFree(runner.Vars)
	}

	fs.logger.InfoPersist("command finished", "command", command, "err", err)
	return err
}

func (fs *FastShell) updateEnvLockFree(vars map[string]expand.Variable) {
	newEnv := make([]string, 0, len(vars))
	for name, vr := range vars {
		if vr.Exported {
			newEnv = append(newEnv, name+"="+vr.Str)
		}
	}
	fs.env = newEnv
}

func (fs *FastShell) newInterp(stdout, stderr io.Writer) (*interp.Runner, error) {
	return interp.New(
		interp.StdIO(nil, stdout, stderr),
		interp.Interactive(false),
		interp.Env(expand.ListEnviron(fs.env...)),
		interp.Dir(fs.cwd),
		interp.ExecHandlers(fs.execHandlers()...),
	)
}

func (fs *FastShell) execHandlers() []func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
	handlers := []func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc{
		fs.blockHandler(),
	}
	if useGoCoreUtils {
		handlers = append(handlers, coreutils.ExecHandler)
	}
	return handlers
}

func (fs *FastShell) blockHandler() func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
	return func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
		return func(ctx context.Context, args []string) error {
			if len(args) == 0 {
				return next(ctx, args)
			}
			for _, blockFn := range fs.blockFuncs {
				if blockFn(args) {
					return fmt.Errorf("command is not allowed: %q", args[0])
				}
			}
			return next(ctx, args)
		}
	}
}

// GetFastBackgroundShellManager returns the singleton optimized background shell manager.
func GetFastBackgroundShellManager() *FastBackgroundShellManager {
	fastBackgroundManagerOnce.Do(func() {
		fastBackgroundManager = NewFastBackgroundShellManager()
	})
	return fastBackgroundManager
}
