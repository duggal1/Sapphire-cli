// Package app wires together services, coordinates agents, and manages
// application lifecycle.
package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
	"github.com/duggal1/Sapphire-cli/internal/agent"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools/mcp"
	"github.com/duggal1/Sapphire-cli/internal/codeindex"
	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/duggal1/Sapphire-cli/internal/db"
	"github.com/duggal1/Sapphire-cli/internal/event"
	"github.com/duggal1/Sapphire-cli/internal/filetracker"
	"github.com/duggal1/Sapphire-cli/internal/format"
	"github.com/duggal1/Sapphire-cli/internal/history"
	"github.com/duggal1/Sapphire-cli/internal/log"
	"github.com/duggal1/Sapphire-cli/internal/lsp"
	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/duggal1/Sapphire-cli/internal/permission"
	"github.com/duggal1/Sapphire-cli/internal/pubsub"
	"github.com/duggal1/Sapphire-cli/internal/session"
	"github.com/duggal1/Sapphire-cli/internal/shell"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
	"github.com/duggal1/Sapphire-cli/internal/update"
	"github.com/duggal1/Sapphire-cli/internal/version"
)

// UpdateAvailableMsg is sent when a new version of the application is available.
// It contains information about the current and latest versions to notify the user.
type UpdateAvailableMsg struct {
	CurrentVersion string
	LatestVersion  string
	IsDevelopment  bool
}

// App represents the core application container that wires together various services,
// coordinates agent interactions, and manages the overall application lifecycle.
type App struct {
	Sessions    session.Service
	Messages    message.Service
	History     history.Service
	Permissions permission.Service
	FileTracker filetracker.Service
	Conn        *sql.DB

	AgentCoordinator agent.Coordinator

	LSPManager *lsp.Manager

	config *config.Config

	serviceEventsWG *sync.WaitGroup
	eventsCtx       context.Context
	events          chan tea.Msg
	tuiWG           *sync.WaitGroup

	// global context and cleanup functions
	globalCtx    context.Context
	cleanupFuncs []func(context.Context) error
}

func ensureGoBinOnPath() {
	pathEnv := os.Getenv("PATH")
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
			gopath = filepath.Join(homeDir, "go")
		}
	}
	gobin := os.Getenv("GOBIN")
	if gobin == "" && gopath != "" {
		gobin = filepath.Join(gopath, "bin")
	}
	if gobin == "" {
		return
	}
	for _, entry := range strings.Split(pathEnv, string(os.PathListSeparator)) {
		if entry == gobin {
			return
		}
	}
	if pathEnv == "" {
		_ = os.Setenv("PATH", gobin)
		return
	}
	_ = os.Setenv("PATH", gobin+string(os.PathListSeparator)+pathEnv)
}

func isNonInteractiveRuntime() bool {
	return strings.TrimSpace(os.Getenv("SAPPHIRE_NON_INTERACTIVE")) == "1"
}

// New initializes and returns a new application instance with the provided context,
// database connection, and configuration. It sets up all internal services and background tasks.
func New(ctx context.Context, conn *sql.DB, cfg *config.Config) (*App, error) {
	ensureGoBinOnPath()
	q := db.New(conn)
	sessions := session.NewService(q, conn)
	messages := message.NewService(q)
	files := history.NewService(q, conn)
	// YOLO mode is enabled by default - auto-approve permissions
	// Only disable if explicitly set in config file
	skipPermissionsRequests := true
	if cfg.Permissions != nil {
		// Config exists, use its value (but default struct value is false, so we
		// need a way to know if it was explicitly set - for now, always default to true)
		// TODO: Add explicit JSON tag to detect if set vs default
		skipPermissionsRequests = true // Always default to YOLO mode enabled
	}
	var allowedTools []string
	if cfg.Permissions != nil && cfg.Permissions.AllowedTools != nil {
		allowedTools = cfg.Permissions.AllowedTools
	}

	app := &App{
		Sessions:    sessions,
		Messages:    messages,
		History:     files,
		Permissions: permission.NewPermissionService(cfg.WorkingDir(), skipPermissionsRequests, allowedTools),
		FileTracker: filetracker.NewService(q),
		LSPManager:  lsp.NewManager(cfg),
		Conn:        conn,

		globalCtx: ctx,

		config: cfg,

		events:          make(chan tea.Msg, 100),
		serviceEventsWG: &sync.WaitGroup{},
		tuiWG:           &sync.WaitGroup{},
	}

	app.setupEvents()
	app.startRuntimeControlLoop()

	// Skip update checks on headless runs to keep the cold-start path focused on
	// task execution rather than background network work.
	if !isNonInteractiveRuntime() {
		go app.checkForUpdates(ctx)
	}

	// MCP clients initialize lazily on first use to avoid startup stalls.

	// cleanup database upon app shutdown
	app.cleanupFuncs = append(
		app.cleanupFuncs,
		func(context.Context) error { return conn.Close() },
		mcp.Close,
	)

	// TODO: remove the concept of agent config, most likely.
	if !cfg.IsConfigured() {
		slog.Warn("No agent configuration found")
		return app, nil
	}
	if err := app.InitCoderAgent(ctx, conn); err != nil {
		return nil, fmt.Errorf("failed to initialize coder agent: %w", err)
	}
	if closer, ok := app.AgentCoordinator.(interface{ Close() error }); ok {
		app.cleanupFuncs = append(app.cleanupFuncs, func(context.Context) error {
			return closer.Close()
		})
	}

	// Set up callback for LSP state updates.
	app.LSPManager.SetCallback(func(name string, client *lsp.Client) {
		if client == nil {
			updateLSPState(name, lsp.StateUnstarted, nil, nil, 0)
			return
		}
		client.SetDiagnosticsCallback(updateLSPDiagnostics)
		updateLSPState(name, client.GetServerState(), nil, client, 0)
	})
	go app.LSPManager.TrackConfigured()

	return app, nil
}

// Config returns the current application configuration settings.
func (app *App) Config() *config.Config {
	return app.config
}

// RunNonInteractive executes the application in a headless mode using the provided prompt.
// It streams agent responses directly to the output writer without a TUI.
func (app *App) RunNonInteractive(ctx context.Context, output io.Writer, prompt, largeModel, smallModel, reasoningEffort string, hideSpinner bool) error {
	slog.Info("Running in non-interactive mode")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if largeModel != "" || smallModel != "" || strings.TrimSpace(reasoningEffort) != "" {
		if err := app.overrideModelsForNonInteractive(ctx, largeModel, smallModel, reasoningEffort); err != nil {
			return fmt.Errorf("failed to override models: %w", err)
		}
	}

	var (
		spinner   *format.Spinner
		stdoutTTY bool
		stderrTTY bool
		stdinTTY  bool
		progress  bool
	)

	if f, ok := output.(*os.File); ok {
		stdoutTTY = term.IsTerminal(f.Fd())
	}
	stderrTTY = term.IsTerminal(os.Stderr.Fd())
	stdinTTY = term.IsTerminal(os.Stdin.Fd())
	progress = app.config.Options.Progress == nil || *app.config.Options.Progress

	if !hideSpinner && stderrTTY {
		t := styles.DefaultStyles(false)

		// Detect background color to set the appropriate color for the
		// spinner's 'Generating...' text. Without this, that text would be
		// unreadable in light terminals.
		hasDarkBG := true
		if f, ok := output.(*os.File); ok && stdinTTY && stdoutTTY {
			hasDarkBG = lipgloss.HasDarkBackground(os.Stdin, f)
		}
		defaultFG := lipgloss.LightDark(hasDarkBG)(t.Primary, t.FgBase)
		spinner = format.NewSpinner(ctx, cancel, "Generating", t.Base.Foreground(defaultFG))
		spinner.Start()
	}

	// Helper function to stop spinner once.
	stopSpinner := func() {
		if !hideSpinner && spinner != nil {
			spinner.Stop()
			spinner = nil
		}
	}

	defer stopSpinner()

	const maxPromptLengthForTitle = 100
	const titlePrefix = "Non-interactive: "
	var titleSuffix string

	if len(prompt) > maxPromptLengthForTitle {
		titleSuffix = prompt[:maxPromptLengthForTitle] + "..."
	} else {
		titleSuffix = prompt
	}
	title := titlePrefix + titleSuffix

	sess, err := app.Sessions.Create(ctx, title)
	if err != nil {
		return fmt.Errorf("failed to create session for non-interactive mode: %w", err)
	}
	slog.Info("Created session for non-interactive run", "session_id", sess.ID)

	// Automatically approve all permission requests for this non-interactive
	// session.
	app.Permissions.AutoApproveSession(sess.ID)

	type response struct {
		result *fantasy.AgentResult
		err    error
	}
	done := make(chan response, 1)

	go func(ctx context.Context, sessionID, prompt string) {
		result, err := app.AgentCoordinator.Run(ctx, sess.ID, prompt)
		if err != nil {
			done <- response{
				err: fmt.Errorf("failed to start agent processing stream: %w", err),
			}
			return
		}
		done <- response{
			result: result,
		}
	}(ctx, sess.ID, prompt)

	messageEvents := app.Messages.Subscribe(ctx)
	messageReadBytes := make(map[string]int)
	var printed bool

	defer func() {
		if progress && stderrTTY {
			_, _ = fmt.Fprintf(os.Stderr, ansi.ResetProgressBar)
		}

		// Always print a newline at the end. If output is a TTY this will
		// prevent the prompt from overwriting the last line of output.
		_, _ = fmt.Fprintln(output)
	}()

	for {
		if progress && stderrTTY {
			// HACK: Reinitialize the terminal progress bar on every iteration
			// so it doesn't get hidden by the terminal due to inactivity.
			_, _ = fmt.Fprintf(os.Stderr, ansi.SetIndeterminateProgressBar)
		}

		select {
		case result := <-done:
			stopSpinner()
			if result.err != nil {
				if errors.Is(result.err, context.Canceled) || errors.Is(result.err, agent.ErrRequestCancelled) {
					slog.Debug("Non-interactive: agent processing cancelled", "session_id", sess.ID)
					return nil
				}
				return fmt.Errorf("agent processing failed: %w", result.err)
			}
			if !printed {
				fmt.Fprint(output, "Done.")
			}
			return nil

		case event := <-messageEvents:
			msg := event.Payload
			if msg.SessionID == sess.ID && msg.Role == message.Assistant && len(msg.Parts) > 0 {
				stopSpinner()

				content := msg.Content().String()
				readBytes := messageReadBytes[msg.ID]

				if len(content) < readBytes {
					slog.Error("Non-interactive: message content is shorter than read bytes", "message_length", len(content), "read_bytes", readBytes)
					return fmt.Errorf("message content is shorter than read bytes: %d < %d", len(content), readBytes)
				}

				part := content[readBytes:]
				// Trim leading whitespace. Sometimes the LLM includes leading
				// formatting and intentation, which we don't want here.
				if readBytes == 0 {
					part = strings.TrimLeft(part, " \t")
				}
				// Ignore initial whitespace-only messages.
				if printed || strings.TrimSpace(part) != "" {
					printed = true
					fmt.Fprint(output, part)
				}
				messageReadBytes[msg.ID] = len(content)
			}

		case <-ctx.Done():
			stopSpinner()
			return ctx.Err()
		}
	}
}

// UpdateAgentModel updates the internal models used by the agent coordinator based on the current configuration.
func (app *App) UpdateAgentModel(ctx context.Context) error {
	if app.AgentCoordinator == nil {
		return fmt.Errorf("agent configuration is missing")
	}
	return app.AgentCoordinator.UpdateModels(ctx)
}

// overrideModelsForNonInteractive parses the model strings and temporarily
// overrides the model configurations, then rebuilds the agent.
// Format: "model-name" (searches all providers) or "provider/model-name".
// Model matching is case-insensitive.
// If largeModel is provided but smallModel is not, the small model defaults to
// the provider's default small model.
func (app *App) overrideModelsForNonInteractive(ctx context.Context, largeModel, smallModel, reasoningEffort string) error {
	providers := app.config.Providers.Copy()
	reasoningEffort = strings.ToLower(strings.TrimSpace(reasoningEffort))
	if reasoningEffort != "" && !isSupportedReasoningEffort(reasoningEffort) {
		return fmt.Errorf("invalid reasoning effort %q (expected low, medium, or high)", reasoningEffort)
	}

	largeMatches, smallMatches, err := findModels(providers, largeModel, smallModel)
	if err != nil {
		return err
	}

	var largeProviderID string

	// Override large model.
	if largeModel != "" {
		found, err := validateMatches(largeMatches, largeModel, "large")
		if err != nil {
			return err
		}
		largeProviderID = found.provider
		slog.Info("Overriding large model for non-interactive run", "provider", found.provider, "model", found.modelID)
		app.config.Models[config.SelectedModelTypeLarge] = config.SelectedModel{
			Provider: found.provider,
			Model:    found.modelID,
		}
	}

	// Override small model.
	switch {
	case smallModel != "":
		found, err := validateMatches(smallMatches, smallModel, "small")
		if err != nil {
			return err
		}
		slog.Info("Overriding small model for non-interactive run", "provider", found.provider, "model", found.modelID)
		app.config.Models[config.SelectedModelTypeSmall] = config.SelectedModel{
			Provider: found.provider,
			Model:    found.modelID,
		}

	case largeModel != "":
		// No small model specified, but large model was - use provider's default.
		smallCfg := app.GetDefaultSmallModel(largeProviderID)
		app.config.Models[config.SelectedModelTypeSmall] = smallCfg
	}

	if reasoningEffort != "" {
		if err := applyReasoningOverrideToSelection(app.config, config.SelectedModelTypeLarge, reasoningEffort); err != nil {
			return err
		}
		if err := applyReasoningOverrideToSelection(app.config, config.SelectedModelTypeSmall, reasoningEffort); err != nil {
			return err
		}
	}

	return app.AgentCoordinator.UpdateModels(ctx)
}

func isSupportedReasoningEffort(effort string) bool {
	switch strings.TrimSpace(strings.ToLower(effort)) {
	case "low", "medium", "high":
		return true
	default:
		return false
	}
}

func applyReasoningOverrideToSelection(cfg *config.Config, modelType config.SelectedModelType, effort string) error {
	if cfg == nil {
		return fmt.Errorf("config is required")
	}
	effort = strings.ToLower(strings.TrimSpace(effort))
	if effort == "" {
		return nil
	}
	if !isSupportedReasoningEffort(effort) {
		return fmt.Errorf("invalid reasoning effort %q", effort)
	}
	selected, ok := cfg.Models[modelType]
	if !ok {
		return fmt.Errorf("%s model is not configured", modelType)
	}
	model := cfg.GetModel(selected.Provider, selected.Model)
	if model == nil {
		return fmt.Errorf("%s model %s/%s is not available", modelType, selected.Provider, selected.Model)
	}
	cfg.Models[modelType] = config.ApplyReasoningSelection(model, selected, effort)
	return nil
}

// GetDefaultSmallModel retrieves the default small model configuration for a specific provider.
// It falls back to the configured large model if a suitable small model cannot be determined.
func (app *App) GetDefaultSmallModel(providerID string) config.SelectedModel {
	cfg := app.config
	largeModelCfg := cfg.Models[config.SelectedModelTypeLarge]

	// Find the provider in the known providers list to get its default small model.
	knownProviders, _ := config.Providers(cfg)
	var knownProvider *catwalk.Provider
	for _, p := range knownProviders {
		if string(p.ID) == providerID {
			knownProvider = &p
			break
		}
	}

	// For unknown/local providers, use the large model as small.
	if knownProvider == nil {
		slog.Warn("Using large model as small model for unknown provider", "provider", providerID, "model", largeModelCfg.Model)
		return largeModelCfg
	}

	defaultSmallModelID := knownProvider.DefaultSmallModelID
	model := cfg.GetModel(providerID, defaultSmallModelID)
	if model == nil {
		slog.Warn("Default small model not found, using large model", "provider", providerID, "model", largeModelCfg.Model)
		return largeModelCfg
	}

	slog.Info("Using provider default small model", "provider", providerID, "model", defaultSmallModelID)
	return config.SelectedModel{
		Provider:        providerID,
		Model:           defaultSmallModelID,
		MaxTokens:       model.DefaultMaxTokens,
		ReasoningEffort: model.DefaultReasoningEffort,
	}
}

// setupEvents initializes internal pubsub subscribers for various application services
// and routes their events to the main application event channel.
func (app *App) setupEvents() {
	ctx, cancel := context.WithCancel(app.globalCtx)
	app.eventsCtx = ctx
	setupSubscriber(ctx, app.serviceEventsWG, "sessions", app.Sessions.Subscribe, app.events, subscriberCritical)
	setupSubscriber(ctx, app.serviceEventsWG, "messages", app.Messages.Subscribe, app.events, subscriberCritical)
	setupSubscriber(ctx, app.serviceEventsWG, "permissions", app.Permissions.Subscribe, app.events, subscriberCritical)
	setupSubscriber(ctx, app.serviceEventsWG, "permissions-notifications", app.Permissions.SubscribeNotifications, app.events, subscriberCritical)
	setupSubscriber(ctx, app.serviceEventsWG, "history", app.History.Subscribe, app.events, subscriberCritical)
	setupSubscriber(ctx, app.serviceEventsWG, "mcp", mcp.SubscribeEvents, app.events, subscriberBestEffort)
	setupSubscriber(ctx, app.serviceEventsWG, "lsp", SubscribeLSPEvents, app.events, subscriberBestEffort)
	setupSubscriber(ctx, app.serviceEventsWG, "codeindex", codeindex.SubscribeEvents, app.events, subscriberBestEffort)
	setupSubscriber(ctx, app.serviceEventsWG, "subagents", agent.SubscribeSubAgentEvents, app.events, subscriberBestEffort)
	cleanupFunc := func(context.Context) error {
		cancel()
		app.serviceEventsWG.Wait()
		return nil
	}
	app.cleanupFuncs = append(app.cleanupFuncs, cleanupFunc)
}

const subscriberSendTimeout = 2 * time.Second

type subscriberMode uint8

const (
	subscriberCritical subscriberMode = iota
	subscriberBestEffort
)

// setupSubscriber sets up an individual service event subscriber in a background goroutine,
// handling message propagation to the output channel with a timeout-based safety mechanism.
func setupSubscriber[T any](
	ctx context.Context,
	wg *sync.WaitGroup,
	name string,
	subscriber func(context.Context) <-chan pubsub.Event[T],
	outputCh chan<- tea.Msg,
	mode subscriberMode,
) {
	wg.Go(func() {
		subCh := subscriber(ctx)
		if mode == subscriberCritical {
			for {
				select {
				case event, ok := <-subCh:
					if !ok {
						slog.Debug("Subscription channel closed", "name", name)
						return
					}
					select {
					case outputCh <- tea.Msg(event):
					case <-ctx.Done():
						slog.Debug("Subscription cancelled", "name", name)
						return
					}
				case <-ctx.Done():
					slog.Debug("Subscription cancelled", "name", name)
					return
				}
			}
		}

		sendTimer := time.NewTimer(0)
		<-sendTimer.C
		defer sendTimer.Stop()

		for {
			select {
			case event, ok := <-subCh:
				if !ok {
					slog.Debug("Subscription channel closed", "name", name)
					return
				}
				var msg tea.Msg = event
				if !sendTimer.Stop() {
					select {
					case <-sendTimer.C:
					default:
					}
				}
				sendTimer.Reset(subscriberSendTimeout)

				select {
				case outputCh <- msg:
				case <-sendTimer.C:
					slog.Debug("Message dropped due to slow consumer", "name", name)
				case <-ctx.Done():
					slog.Debug("Subscription cancelled", "name", name)
					return
				}
			case <-ctx.Done():
				slog.Debug("Subscription cancelled", "name", name)
				return
			}
		}
	})
}

// InitCoderAgent initializes the coding-specialized agent and its coordinator with all necessary service dependencies.
func (app *App) InitCoderAgent(ctx context.Context, conn *sql.DB) error {
	coderAgentCfg := app.config.Agents[config.AgentCoder]
	if coderAgentCfg.ID == "" {
		return fmt.Errorf("coder agent configuration is missing")
	}
	var err error
	app.AgentCoordinator, err = agent.NewCoordinator(
		ctx,
		app.config,
		app.Sessions,
		app.Messages,
		app.Permissions,
		app.History,
		app.FileTracker,
		app.LSPManager,
		conn,
	)
	if err != nil {
		slog.Error("Failed to create coder agent", "err", err)
		return err
	}
	return nil
}

// Subscribe begins forwarding internal application events to the provided Bubble Tea program,
// enabling the UI to respond to real-time service updates.
func (app *App) Subscribe(program *tea.Program) {
	defer log.RecoverPanic("app.Subscribe", func() {
		slog.Info("TUI subscription panic: attempting graceful shutdown")
		program.Quit()
	})

	app.tuiWG.Add(1)
	tuiCtx, tuiCancel := context.WithCancel(app.globalCtx)
	app.cleanupFuncs = append(app.cleanupFuncs, func(context.Context) error {
		slog.Debug("Cancelling TUI message handler")
		tuiCancel()
		app.tuiWG.Wait()
		return nil
	})
	defer app.tuiWG.Done()

	for {
		select {
		case <-tuiCtx.Done():
			slog.Debug("TUI message handler shutting down")
			return
		case msg, ok := <-app.events:
			if !ok {
				slog.Debug("TUI message channel closed")
				return
			}
			program.Send(msg)
		}
	}
}

// Shutdown performs an orderly termination of the application, cancelling active agents,
// closing database connections, and shutting down background processes and LSP clients.
func (app *App) Shutdown() {
	start := time.Now()
	defer func() { slog.Debug("Shutdown took " + time.Since(start).String()) }()

	// First, cancel all agents and wait for them to finish. This must complete
	// before closing the DB so agents can finish writing their state.
	if app.AgentCoordinator != nil {
		app.AgentCoordinator.CancelAll()
	}

	// Now run remaining cleanup tasks in parallel.
	var wg sync.WaitGroup

	// Shared shutdown context for all timeout-bounded cleanup.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(app.globalCtx), 5*time.Second)
	defer cancel()

	// Send exit event
	wg.Go(func() {
		event.AppExited()
	})

	// Kill all background shells.
	wg.Go(func() {
		shell.GetBackgroundShellManager().KillAll(shutdownCtx)
	})

	// Shutdown all LSP clients.
	wg.Go(func() {
		app.LSPManager.KillAll(shutdownCtx)
	})

	// Call all cleanup functions.
	for _, cleanup := range app.cleanupFuncs {
		if cleanup != nil {
			wg.Go(func() {
				if err := cleanup(shutdownCtx); err != nil {
					slog.Error("Failed to cleanup app properly on shutdown", "error", err)
				}
			})
		}
	}
	wg.Wait()
}

// checkForUpdates initiates an asynchronous check for newer versions of the application
// and notifies the application event channel if an update is available.
func (app *App) checkForUpdates(ctx context.Context) {
	checkCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	info, err := update.Check(checkCtx, version.Version, update.Default)
	if err != nil || !info.Available() {
		return
	}
	app.events <- UpdateAvailableMsg{
		CurrentVersion: info.Current,
		LatestVersion:  info.Latest,
		IsDevelopment:  info.IsDevelopment(),
	}
}
