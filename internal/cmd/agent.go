package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"os/user"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"charm.land/log/v2"
	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/duggal1/Sapphire-cli/internal/event"
	"github.com/duggal1/Sapphire-cli/internal/version"
	"github.com/spf13/cobra"
	"log/slog"
)

const (
	defaultAgentModel         = "openrouter/qwen/qwen3.6-plus-preview:free"
	defaultAgentReasoning     = "high"
	defaultAgentRuntimePeriod = time.Second
	agentRunKind              = "agent.run"
	agentInspectKind          = "agent.inspect"
	agentRuntimeSampleKind    = "agent.runtime.sample"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Machine-facing terminal interface for Sapphire",
	Long:  "Emit structured runtime telemetry and run Sapphire headlessly for other terminal-only agents.",
}

var agentRunCmd = &cobra.Command{
	Use:   "run [prompt...]",
	Short: "Run a single prompt with machine-readable runtime telemetry",
	Long: `Run a single prompt in non-interactive mode.
The model defaults to OpenRouter Qwen 3.6 Plus with high reasoning effort.
Runtime snapshots are streamed to stderr as JSON lines so other agents can monitor CPU, memory, and latency in real time.`,
	Example: `
# Run with the default agent model and structured telemetry
sapphire agent run "Refactor this package"

# Override the model while keeping the terminal-only interface
sapphire agent run --model "openrouter/anthropic/claude-sonnet-4-5" "Review this diff"

# Lower reasoning effort for a faster pass
sapphire agent run --reasoning-effort medium "Summarize this repo"
`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_ = os.Setenv("SAPPHIRE_NON_INTERACTIVE", "1")
		event.SetNonInteractive(true)

		quiet, _ := cmd.Flags().GetBool("quiet")
		verbose, _ := cmd.Flags().GetBool("verbose")
		model, _ := cmd.Flags().GetString("model")
		smallModel, _ := cmd.Flags().GetString("small-model")
		reasoningEffort, _ := cmd.Flags().GetString("reasoning-effort")
		interval, _ := cmd.Flags().GetDuration("runtime-interval")

		if verbose {
			slog.SetDefault(slog.New(log.New(os.Stderr)))
		}
		if interval <= 0 {
			interval = defaultAgentRuntimePeriod
		}

		modelChanged := cmd.Flags().Changed("model")
		smallModelChanged := cmd.Flags().Changed("small-model")
		switch {
		case !modelChanged && !smallModelChanged:
			model = defaultAgentModel
			smallModel = defaultAgentModel
		case modelChanged && !smallModelChanged:
			smallModel = model
		case !modelChanged && smallModelChanged:
			model = smallModel
		}
		if strings.TrimSpace(reasoningEffort) == "" {
			reasoningEffort = defaultAgentReasoning
		}

		ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		telemetry := newAgentRuntimeTelemetry(os.Stderr, interval)
		telemetry.SetSelection(model, smallModel, reasoningEffort)
		telemetry.Start(ctx)
		defer telemetry.Stop()

		appInstance, err := setupApp(cmd)
		if err != nil {
			telemetry.EmitError(agentRunKind, err)
			return err
		}
		defer appInstance.Shutdown()

		if !appInstance.Config().IsConfigured() {
			err := fmt.Errorf("no providers configured - please run 'sapphire' interactively to set up a provider")
			telemetry.EmitError(agentRunKind, err)
			return err
		}

		prompt := strings.Join(args, " ")
		prompt, err = MaybePrependStdin(prompt)
		if err != nil {
			telemetry.EmitError(agentRunKind, err)
			return err
		}
		if strings.TrimSpace(prompt) == "" {
			err := fmt.Errorf("no prompt provided")
			telemetry.EmitError(agentRunKind, err)
			return err
		}

		event.AppInitialized()
		startedAt := time.Now().UTC()
		telemetry.EmitStart(agentRunKind, startedAt, prompt)

		runErr := appInstance.RunNonInteractive(ctx, os.Stdout, prompt, model, smallModel, reasoningEffort, quiet || verbose)
		finishedAt := time.Now().UTC()
		if runErr != nil {
			telemetry.EmitError(agentRunKind, runErr)
			return runErr
		}

		telemetry.EmitFinal(agentRunKind, startedAt, finishedAt, nil)
		return nil
	},
}

var agentInspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Emit a JSON snapshot of the current machine-facing Sapphire state",
	Long:  "Return a structured JSON snapshot of the current configuration, selected models, and runtime state.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := ResolveCwd(cmd)
		if err != nil {
			return err
		}
		dataDir, _ := cmd.Flags().GetString("data-dir")
		debugEnabled, _ := cmd.Flags().GetBool("debug")
		cfg, err := config.Init(cwd, dataDir, debugEnabled)
		if err != nil {
			return err
		}

		hostname, _ := os.Hostname()
		currentUser, _ := user.Current()
		now := time.Now().UTC()
		payload := agentInspectSnapshot{
			Kind:                   agentInspectKind,
			Timestamp:              now,
			Hostname:               hostname,
			Username:               userNameOrEmpty(currentUser),
			WorkingDir:             cwd,
			DataDir:                cfg.Options.DataDirectory,
			AppVersion:             version.Display(),
			GoVersion:              runtime.Version(),
			Configured:             cfg.IsConfigured(),
			DefaultModel:           defaultAgentModel,
			DefaultReasoningEffort: defaultAgentReasoning,
			Capabilities: []string{
				"run",
				"model_override",
				"reasoning_override",
				"runtime_sampling",
				"exact_errors",
			},
			Runtime: captureAgentRuntimeSnapshot(now),
			SelectedModels: map[string]agentModelSnapshot{
				string(config.SelectedModelTypeLarge): snapshotSelectedModel(cfg.Models[config.SelectedModelTypeLarge]),
				string(config.SelectedModelTypeSmall): snapshotSelectedModel(cfg.Models[config.SelectedModelTypeSmall]),
			},
		}

		return writeJSON(cmd.OutOrStdout(), payload)
	},
}

func init() {
	agentRunCmd.Flags().BoolP("quiet", "q", false, "Hide the spinner")
	agentRunCmd.Flags().BoolP("verbose", "v", false, "Show debug logs")
	agentRunCmd.Flags().StringP("model", "m", defaultAgentModel, "Model to use. Accepts 'model' or 'provider/model'")
	agentRunCmd.Flags().String("small-model", defaultAgentModel, "Small model to use. Defaults to the agent model")
	agentRunCmd.Flags().String("reasoning-effort", defaultAgentReasoning, "Reasoning effort override (low, medium, high)")
	agentRunCmd.Flags().Duration("runtime-interval", defaultAgentRuntimePeriod, "Interval for runtime telemetry samples")

	agentCmd.AddCommand(agentRunCmd, agentInspectCmd)
}

type agentModelSnapshot struct {
	Provider        string `json:"provider,omitempty"`
	Model           string `json:"model,omitempty"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
	Think           bool   `json:"think,omitempty"`
	MaxTokens       int64  `json:"max_tokens,omitempty"`
}

type agentRuntimeSnapshot struct {
	CapturedAt      time.Time `json:"captured_at"`
	ElapsedMs       int64     `json:"elapsed_ms"`
	UserCPUMs       int64     `json:"user_cpu_ms"`
	SystemCPUMs     int64     `json:"system_cpu_ms"`
	HeapAllocBytes  uint64    `json:"heap_alloc_bytes"`
	HeapInuseBytes  uint64    `json:"heap_inuse_bytes"`
	HeapSysBytes    uint64    `json:"heap_sys_bytes"`
	TotalAllocBytes uint64    `json:"total_alloc_bytes"`
	HeapObjects     uint64    `json:"heap_objects"`
	Goroutines      int       `json:"goroutines"`
	NumGC           uint32    `json:"num_gc"`
	GOMAXPROCS      int       `json:"gomaxprocs"`
	GOGC            int       `json:"gogc"`
	GOMEMLIMITBytes int64     `json:"gomemlimit_bytes"`
}

type agentRuntimeSummary struct {
	Baseline            agentRuntimeSnapshot `json:"baseline"`
	Latest              agentRuntimeSnapshot `json:"latest"`
	Peak                agentRuntimeSnapshot `json:"peak"`
	Samples             int                  `json:"samples"`
	DurationMs          int64                `json:"duration_ms"`
	HeapAllocSpikeBytes int64                `json:"heap_alloc_spike_bytes"`
	HeapInuseSpikeBytes int64                `json:"heap_inuse_spike_bytes"`
	UserCPUSpikeMs      int64                `json:"user_cpu_spike_ms"`
	SystemCPUSpikeMs    int64                `json:"system_cpu_spike_ms"`
}

type agentRuntimeSampleEnvelope struct {
	Kind            string               `json:"kind"`
	Timestamp       time.Time            `json:"timestamp"`
	SessionID       string               `json:"session_id,omitempty"`
	Prompt          string               `json:"prompt,omitempty"`
	Model           string               `json:"model,omitempty"`
	SmallModel      string               `json:"small_model,omitempty"`
	ReasoningEffort string               `json:"reasoning_effort,omitempty"`
	Runtime         agentRuntimeSnapshot `json:"runtime"`
}

type agentRuntimeReportEnvelope struct {
	Kind            string              `json:"kind"`
	Timestamp       time.Time           `json:"timestamp"`
	SessionID       string              `json:"session_id,omitempty"`
	Prompt          string              `json:"prompt,omitempty"`
	Model           string              `json:"model,omitempty"`
	SmallModel      string              `json:"small_model,omitempty"`
	ReasoningEffort string              `json:"reasoning_effort,omitempty"`
	Runtime         agentRuntimeSummary `json:"runtime"`
	Error           *agentErrorEnvelope `json:"error,omitempty"`
}

type agentInspectSnapshot struct {
	Kind                   string                        `json:"kind"`
	Timestamp              time.Time                     `json:"timestamp"`
	Hostname               string                        `json:"hostname,omitempty"`
	Username               string                        `json:"username,omitempty"`
	WorkingDir             string                        `json:"working_dir,omitempty"`
	DataDir                string                        `json:"data_dir,omitempty"`
	AppVersion             string                        `json:"app_version,omitempty"`
	GoVersion              string                        `json:"go_version,omitempty"`
	Configured             bool                          `json:"configured"`
	DefaultModel           string                        `json:"default_model"`
	DefaultReasoningEffort string                        `json:"default_reasoning_effort"`
	Capabilities           []string                      `json:"capabilities"`
	Runtime                agentRuntimeSnapshot          `json:"runtime"`
	SelectedModels         map[string]agentModelSnapshot `json:"selected_models"`
}

type agentErrorEnvelope struct {
	Message string `json:"message"`
}

type agentRuntimeTelemetry struct {
	mu              sync.Mutex
	out             io.Writer
	interval        time.Duration
	startedAt       time.Time
	stopOnce        sync.Once
	stopCh          chan struct{}
	doneCh          chan struct{}
	baseline        agentRuntimeSnapshot
	latest          agentRuntimeSnapshot
	peak            agentRuntimeSnapshot
	sampleCount     int
	model           string
	smallModel      string
	reasoningEffort string
}

func newAgentRuntimeTelemetry(out io.Writer, interval time.Duration) *agentRuntimeTelemetry {
	if interval <= 0 {
		interval = defaultAgentRuntimePeriod
	}
	now := time.Now().UTC()
	start := captureAgentRuntimeSnapshot(now)
	return &agentRuntimeTelemetry{
		out:       out,
		interval:  interval,
		startedAt: now,
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
		baseline:  start,
		latest:    start,
		peak:      start,
	}
}

func (t *agentRuntimeTelemetry) SetSelection(model, smallModel, reasoningEffort string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.model = strings.TrimSpace(model)
	t.smallModel = strings.TrimSpace(smallModel)
	t.reasoningEffort = strings.TrimSpace(reasoningEffort)
	t.mu.Unlock()
}

func (t *agentRuntimeTelemetry) Start(ctx context.Context) {
	if t == nil {
		return
	}
	go func() {
		defer close(t.doneCh)
		ticker := time.NewTicker(t.interval)
		defer ticker.Stop()
		t.emitSample(agentRuntimeSampleKind, t.baseline)
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.stopCh:
				return
			case <-ticker.C:
				t.emitSample(agentRuntimeSampleKind, captureAgentRuntimeSnapshot(t.startedAt))
			}
		}
	}()
}

func (t *agentRuntimeTelemetry) Stop() {
	if t == nil {
		return
	}
	t.stopOnce.Do(func() {
		close(t.stopCh)
	})
	<-t.doneCh
}

func (t *agentRuntimeTelemetry) EmitStart(kind string, startedAt time.Time, prompt string) {
	if t == nil {
		return
	}
	payload := agentRuntimeSampleEnvelope{
		Kind:            kind + ".start",
		Timestamp:       time.Now().UTC(),
		Prompt:          prompt,
		Model:           t.currentModel(),
		SmallModel:      t.currentSmallModel(),
		ReasoningEffort: t.currentReasoningEffort(),
		Runtime:         captureAgentRuntimeSnapshot(startedAt),
	}
	_ = writeJSONLine(t.out, payload)
}

func (t *agentRuntimeTelemetry) EmitFinal(kind string, startedAt, finishedAt time.Time, runErr error) {
	if t == nil {
		return
	}
	payload := agentRuntimeReportEnvelope{
		Kind:            kind + ".finish",
		Timestamp:       finishedAt.UTC(),
		Model:           t.currentModel(),
		SmallModel:      t.currentSmallModel(),
		ReasoningEffort: t.currentReasoningEffort(),
		Runtime:         t.summary(startedAt, finishedAt),
	}
	if runErr != nil {
		payload.Error = &agentErrorEnvelope{Message: runErr.Error()}
	}
	_ = writeJSONLine(t.out, payload)
}

func (t *agentRuntimeTelemetry) EmitError(kind string, err error) {
	if t == nil || err == nil {
		return
	}
	now := time.Now().UTC()
	payload := agentRuntimeReportEnvelope{
		Kind:            kind + ".error",
		Timestamp:       now,
		Model:           t.currentModel(),
		SmallModel:      t.currentSmallModel(),
		ReasoningEffort: t.currentReasoningEffort(),
		Runtime:         t.summary(t.startedAt, now),
		Error:           &agentErrorEnvelope{Message: err.Error()},
	}
	_ = writeJSONLine(t.out, payload)
}

func (t *agentRuntimeTelemetry) emitSample(kind string, snapshot agentRuntimeSnapshot) {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.latest = snapshot
	t.sampleCount++
	if snapshot.HeapAllocBytes > t.peak.HeapAllocBytes {
		t.peak.HeapAllocBytes = snapshot.HeapAllocBytes
	}
	if snapshot.HeapInuseBytes > t.peak.HeapInuseBytes {
		t.peak.HeapInuseBytes = snapshot.HeapInuseBytes
	}
	if snapshot.Goroutines > t.peak.Goroutines {
		t.peak.Goroutines = snapshot.Goroutines
	}
	if snapshot.UserCPUMs > t.peak.UserCPUMs {
		t.peak.UserCPUMs = snapshot.UserCPUMs
	}
	if snapshot.SystemCPUMs > t.peak.SystemCPUMs {
		t.peak.SystemCPUMs = snapshot.SystemCPUMs
	}
	model := t.model
	smallModel := t.smallModel
	reasoningEffort := t.reasoningEffort
	t.mu.Unlock()

	payload := agentRuntimeSampleEnvelope{
		Kind:            kind,
		Timestamp:       snapshot.CapturedAt,
		Model:           model,
		SmallModel:      smallModel,
		ReasoningEffort: reasoningEffort,
		Runtime:         snapshot,
	}
	_ = writeJSONLine(t.out, payload)
}

func (t *agentRuntimeTelemetry) currentModel() string {
	if t == nil {
		return ""
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.model
}

func (t *agentRuntimeTelemetry) currentSmallModel() string {
	if t == nil {
		return ""
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.smallModel
}

func (t *agentRuntimeTelemetry) currentReasoningEffort() string {
	if t == nil {
		return ""
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.reasoningEffort
}

func (t *agentRuntimeTelemetry) summary(startedAt, finishedAt time.Time) agentRuntimeSummary {
	if t == nil {
		return agentRuntimeSummary{}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return agentRuntimeSummary{
		Baseline:            t.baseline,
		Latest:              t.latest,
		Peak:                t.peak,
		Samples:             t.sampleCount,
		DurationMs:          finishedAt.Sub(startedAt).Milliseconds(),
		HeapAllocSpikeBytes: int64(t.peak.HeapAllocBytes) - int64(t.baseline.HeapAllocBytes),
		HeapInuseSpikeBytes: int64(t.peak.HeapInuseBytes) - int64(t.baseline.HeapInuseBytes),
		UserCPUSpikeMs:      t.peak.UserCPUMs - t.baseline.UserCPUMs,
		SystemCPUSpikeMs:    t.peak.SystemCPUMs - t.baseline.SystemCPUMs,
	}
}

func captureAgentRuntimeSnapshot(start time.Time) agentRuntimeSnapshot {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	snapshot := agentRuntimeSnapshot{
		CapturedAt:      time.Now().UTC(),
		ElapsedMs:       time.Since(start).Milliseconds(),
		HeapAllocBytes:  mem.HeapAlloc,
		HeapInuseBytes:  mem.HeapInuse,
		HeapSysBytes:    mem.HeapSys,
		TotalAllocBytes: mem.TotalAlloc,
		HeapObjects:     mem.HeapObjects,
		Goroutines:      runtime.NumGoroutine(),
		NumGC:           mem.NumGC,
		GOMAXPROCS:      runtime.GOMAXPROCS(0),
		GOGC:            currentGCPercent(),
		GOMEMLIMITBytes: debug.SetMemoryLimit(-1),
	}

	if usage, err := captureProcessUsage(); err == nil {
		snapshot.UserCPUMs = usage.user.Milliseconds()
		snapshot.SystemCPUMs = usage.system.Milliseconds()
	}

	return snapshot
}

type processUsage struct {
	user   time.Duration
	system time.Duration
}

func captureProcessUsage() (processUsage, error) {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return processUsage{}, err
	}
	return processUsage{
		user:   time.Duration(usage.Utime.Sec)*time.Second + time.Duration(usage.Utime.Usec)*time.Microsecond,
		system: time.Duration(usage.Stime.Sec)*time.Second + time.Duration(usage.Stime.Usec)*time.Microsecond,
	}, nil
}

func currentGCPercent() int {
	if raw := strings.TrimSpace(os.Getenv("GOGC")); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil {
			return value
		}
	}
	return 100
}

func snapshotSelectedModel(selected config.SelectedModel) agentModelSnapshot {
	return agentModelSnapshot{
		Provider:        strings.TrimSpace(selected.Provider),
		Model:           strings.TrimSpace(selected.Model),
		ReasoningEffort: strings.TrimSpace(selected.ReasoningEffort),
		Think:           selected.Think,
		MaxTokens:       selected.MaxTokens,
	}
}

func writeJSON(w io.Writer, payload any) error {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}

func writeJSONLine(w io.Writer, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}

func userNameOrEmpty(u *user.User) string {
	if u == nil {
		return ""
	}
	return u.Username
}
