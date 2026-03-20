package agent

import (
	"bytes"
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	agentactivity "github.com/duggal1/Sapphire-cli/internal/agent/activity"
	agentbackground "github.com/duggal1/Sapphire-cli/internal/agent/background"
	agentconvoy "github.com/duggal1/Sapphire-cli/internal/agent/convoy"
	agentdaemon "github.com/duggal1/Sapphire-cli/internal/agent/daemon"
	agentformula "github.com/duggal1/Sapphire-cli/internal/agent/formula"
	agenthook "github.com/duggal1/Sapphire-cli/internal/agent/hook"
	"github.com/duggal1/Sapphire-cli/internal/agent/hyper"
	"github.com/duggal1/Sapphire-cli/internal/agent/longhorizon"
	agentmailbox "github.com/duggal1/Sapphire-cli/internal/agent/mailbox"
	"github.com/duggal1/Sapphire-cli/internal/agent/memory"
	promptpkg "github.com/duggal1/Sapphire-cli/internal/agent/prompt"
	agentscheduler "github.com/duggal1/Sapphire-cli/internal/agent/scheduler"
	agentstate "github.com/duggal1/Sapphire-cli/internal/agent/state"
	agentsupervisor "github.com/duggal1/Sapphire-cli/internal/agent/supervisor"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/duggal1/Sapphire-cli/internal/db"
	"github.com/duggal1/Sapphire-cli/internal/filetracker"
	"github.com/duggal1/Sapphire-cli/internal/history"
	"github.com/duggal1/Sapphire-cli/internal/log"
	"github.com/duggal1/Sapphire-cli/internal/lsp"
	pmem "github.com/duggal1/Sapphire-cli/internal/memory"
	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/duggal1/Sapphire-cli/internal/oauth/copilot"
	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
	"github.com/duggal1/Sapphire-cli/internal/permission"
	"github.com/duggal1/Sapphire-cli/internal/session"
	"github.com/duggal1/Sapphire-cli/internal/skills"
	"golang.org/x/sync/errgroup"

	"charm.land/fantasy/providers/anthropic"
	"charm.land/fantasy/providers/azure"
	"charm.land/fantasy/providers/bedrock"
	"charm.land/fantasy/providers/google"
	"charm.land/fantasy/providers/openai"
	"charm.land/fantasy/providers/openaicompat"
	"charm.land/fantasy/providers/openrouter"
	"charm.land/fantasy/providers/vercel"
	openaisdk "github.com/openai/openai-go/v3/option"
	"github.com/qjebbs/go-jsons"
	"google.golang.org/genai"
)

type Coordinator interface {
	// INFO: (kujtim) this is not used yet we will use this when we have multiple agents
	// SetMainAgent(string)
	Run(ctx context.Context, sessionID, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error)
	Submit(ctx context.Context, sessionID, prompt string, attachments ...message.Attachment) (SubmissionResult, error)
	OrchestrateWorktrees(ctx context.Context, sessionID string, params OrchestrateWorktreesParams) (OrchestrateWorktreesResult, error)
	ResumeWorktree(ctx context.Context, sessionID, worktreePath, prompt, agentKey, model, reasoningEffort string) (OrchestrationAgentRef, error)
	Cancel(sessionID string)
	CancelAll()
	IsSessionBusy(sessionID string) bool
	IsBusy() bool
	QueuedPrompts(sessionID string) int
	QueuedPromptsList(sessionID string) []string
	ClearQueue(sessionID string)
	Summarize(context.Context, string) error
	Model() Model
	UpdateModels(ctx context.Context) error
	MemoryPipe() interface{}
	ConsolidateMemory(ctx context.Context, sessionID string) error
	DispatchBackground(ctx context.Context, spec agentbackground.TaskSpec) (string, error)
	GetBackgroundStatus(agentID string) (agentbackground.SubAgent, bool)
	ListBackgroundAgents() []agentbackground.SubAgent
	WaitForCompletion(ctx context.Context, agentIDs []string) ([]agentbackground.SubAgent, error)
	GetLongHorizonState(sessionID string) string
	GetLongHorizonAuditTail(sessionID string, maxBytes int) string
	RunPlanMode(ctx context.Context, sessionID, task, taskContext string) (*agentformula.ExecutionState, error)
	ResolvePlanApproval(ctx context.Context, sessionID string, approved bool) error
}

func (c *coordinator) MemoryPipe() interface{} {
	return c.memoryPipe
}

func (c *coordinator) ConsolidateMemory(ctx context.Context, sessionID string) error {
	if c.memoryPipe == nil {
		return nil
	}
	return c.memoryPipe.ConsolidateMemory(ctx, sessionID)
}

func (c *coordinator) GetLongHorizonState(sessionID string) string {
	if c.longHorizon == nil {
		return ""
	}
	return c.longHorizon.BuildInjection(sessionID)
}

func (c *coordinator) GetLongHorizonAuditTail(sessionID string, maxBytes int) string {
	if c.longHorizon == nil {
		return ""
	}
	// We'll call the BuildInjection but we might want to extend longhorizon.Manager for tail only.
	// For now, this is sufficient for metadata injection.
	return c.longHorizon.BuildInjection(sessionID)
}

// coordinator implements the Coordinator interface and manages multiple AI agents.
type coordinator struct {
	cfg         *config.Config
	sessions    session.Service
	messages    message.Service
	permissions permission.Service
	history     history.Service
	filetracker filetracker.Service
	editGuard   *tools.EditGuard
	lspManager  *lsp.Manager
	memory      memory.MemoryService
	indexer     *Indexer
	pmem        *pmem.System
	longHorizon *longhorizon.Manager

	currentAgent SessionAgent
	agents       map[string]SessionAgent

	readyWg   errgroup.Group
	readyOnce sync.Once
	readyDone chan struct{}
	readyErr  error

	// Embedding-based skill retrieval.
	embeddingService *skills.EmbeddingService
	discoveredSkills []*skills.Skill
	skillsOnce       sync.Once

	// Google search failure tracking - fallback to DDG after 2 failures
	googleSearchFailures      sync.Map // map[string]int (sessionID -> failureCount)
	googleSearchClient        *genai.Client
	backgroundSubAgentLimiter chan struct{}
	backgroundIndicatorMu     sync.Mutex
	backgroundIndicators      map[string]*backgroundIndicatorState
	backgroundRegistry        *agentbackground.Registry
	backgroundDispatcher      *agentbackground.Dispatcher
	backgroundMonitor         *agentbackground.Monitor
	toolRegistry              *tools.Registry
	formulaExecutor           *agentformula.Executor
	subAgentsMu               sync.Mutex
	subAgents                 map[string]*subAgentRunner
	subAgentRegistry          *subAgentRegistry
	orchestrationStore        *orchestrationdb.Store
	mailbox                   *agentmailbox.Service
	stateService              *agentstate.Service
	activityService           *agentactivity.Service
	hookService               *agenthook.Service
	convoyService             *agentconvoy.Service
	supervisor                *agentsupervisor.Service
	dispatcher                *agentscheduler.Dispatcher
	daemon                    *agentdaemon.Service
	orchestrationSvcCancel    context.CancelFunc
	orchestrationSvcWG        sync.WaitGroup
	mainWorktreeDir           string
	mainWorktreeBranch        string
	worktreeOpsMu             sync.Mutex
	worktreeOps               map[string]*sync.Mutex
	agentJobs                 *agentJobManager
	mcpRegistryMu             sync.Mutex
	mcpRegistryDefs           []config.RegistryMCPDefinition
	mcpRegistryLastFetch      time.Time
	mcpRegistryFetchInFlight  bool
	toolCacheMu               sync.RWMutex
	memoryPipe                *memoryPipeline
	checkpointService         *memory.CheckpointService
	cachedTools               []fantasy.AgentTool
	cachedToolNames           []string
	mcpPreflightMu            sync.Mutex
	mcpPreflightCache         map[string]mcpPreflightSnapshot
	mcpPreflightInFlight      map[string]bool
	mcpSelectionMu            sync.Mutex
	mcpSelectionCache         map[string]mcpSelectionSnapshot
	mcpSelectionInFlight      map[string]bool
	planApprovalMu            sync.Mutex
	planApprovalWaiters       map[string]chan bool
}

type autonomousSubAgentTask struct {
	Name         string
	SessionTitle string
	Prompt       string
}

type submissionEnvelope struct {
	sessionID      string
	userPrompt     string
	attachments    []message.Attachment
	userMessage    message.Message
	model          Model
	providerCfg    config.ProviderConfig
	mergedOptions  fantasy.ProviderOptions
	temp           *float64
	topP           *float64
	topK           *int64
	freqPenalty    *float64
	presPenalty    *float64
	maxTokens      int64
	deferPreflight bool
}

func isNonInteractiveMode() bool {
	return strings.TrimSpace(os.Getenv("SAPPHIRE_NON_INTERACTIVE")) == "1"
}

func (c *coordinator) waitForReady(ctx context.Context, timeout time.Duration) error {
	c.readyOnce.Do(func() {
		c.readyDone = make(chan struct{})
		go func() {
			c.readyErr = c.readyWg.Wait()
			close(c.readyDone)
		}()
	})

	if timeout <= 0 {
		timeout = 1 * time.Second
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-c.readyDone:
		return c.readyErr
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// NewCoordinator creates a new agent coordinator to manage multiple AI agents and sessions.
func NewCoordinator(
	ctx context.Context,
	cfg *config.Config,
	sessions session.Service,
	messages message.Service,
	permissions permission.Service,
	history history.Service,
	filetracker filetracker.Service,
	lspManager *lsp.Manager,
	conn *sql.DB,
) (Coordinator, error) {
	c := &coordinator{
		cfg:                       cfg,
		sessions:                  sessions,
		messages:                  messages,
		permissions:               permissions,
		history:                   history,
		filetracker:               filetracker,
		editGuard:                 tools.NewEditGuard(),
		lspManager:                lspManager,
		memory:                    memory.NewMemoryService(db.New(conn), conn),
		agents:                    make(map[string]SessionAgent),
		backgroundSubAgentLimiter: make(chan struct{}, maxBackgroundSubAgents),
		backgroundIndicators:      make(map[string]*backgroundIndicatorState),
		backgroundRegistry:        agentbackground.NewRegistry(),
		subAgents:                 make(map[string]*subAgentRunner),
		subAgentRegistry:          newSubAgentRegistry(),
		worktreeOps:               make(map[string]*sync.Mutex),
		agentJobs:                 newAgentJobManager(),
		mcpPreflightCache:         make(map[string]mcpPreflightSnapshot),
		mcpPreflightInFlight:      make(map[string]bool),
		mcpSelectionCache:         make(map[string]mcpSelectionSnapshot),
		mcpSelectionInFlight:      make(map[string]bool),
	}
	c.memoryPipe = newMemoryPipeline(c)
	if err := c.memoryPipe.EnsureMemoryFolder(); err != nil {
		slog.Warn("Failed to ensure memory folder", "error", err)
	}
	orchestrationStore, err := orchestrationdb.Open(ctx, cfg.Options.DataDirectory)
	if err != nil {
		return nil, fmt.Errorf("open orchestration database: %w", err)
	}
	c.orchestrationStore = orchestrationStore
	c.mailbox = agentmailbox.NewService(orchestrationStore, c.nudgeMailboxRecipient)
	c.stateService = agentstate.NewService(orchestrationStore)
	c.activityService = agentactivity.NewService(orchestrationStore)
	c.hookService = agenthook.NewService(orchestrationStore, c.stateService)
	c.convoyService = agentconvoy.NewService(orchestrationStore, agentconvoy.Hooks{
		EnsureDispatchForWorkItem: c.ensureDispatchForWorkItem,
	})
	c.supervisor = agentsupervisor.NewService(orchestrationStore, c.stateService, c.activityService, c.mailbox, agentsupervisor.Hooks{
		GetRuntimeSnapshot:        c.supervisorRuntimeSnapshot,
		ResolveMainMailboxID:      mainAgentMailboxID,
		EnsureDispatchForWorkItem: c.ensureDispatchForWorkItem,
	})
	c.dispatcher = agentscheduler.NewDispatcher(agentscheduler.Hooks{
		ProcessQueue: c.processDispatchQueue,
		Reconcile:    c.reconcileDispatchQueue,
		CountActive:  c.activeSubAgentCountAll,
		MaxActive:    c.dispatchActiveLimit,
	})
	if err := c.dispatcher.Validate(); err != nil {
		return nil, fmt.Errorf("initialize dispatcher: %w", err)
	}
	c.daemon = agentdaemon.NewService(c.dispatcher, c.supervisor)
	c.startOrchestrationServices()
	worktreeDir, worktreeBranch, err := c.prepareMainWorktree(ctx)
	if err == nil && worktreeDir != "" {
		c.mainWorktreeDir = worktreeDir
		c.mainWorktreeBranch = worktreeBranch
	} else if err != nil {
		slog.Warn("Failed to prepare main worktree; falling back to repo root", "error", err)
	}
	if !isNonInteractiveMode() {
		mainDir := c.mainWorkingDir()
		c.indexer = NewIndexer(mainDir, lspManager, c.memory, func() bool {
			if c.currentAgent == nil {
				return false
			}
			return c.currentAgent.IsBusy()
		})
		go c.indexer.Start(ctx)
		c.longHorizon = longhorizon.NewManager(mainDir)

		// Initialize persistent memory system (optional, requires Gemini API key).
		apiKey := c.resolveGeminiAPIKey()
		if apiKey != "" {
			pmemSys, pmemErr := pmem.NewSystem(ctx, "", pmem.Config{
				ExtractionModel: c.resolveGeminiExtractionModel(),
				APIKey:          apiKey,
				EmbeddingModel:  pmem.DefaultEmbeddingModel,
				EmbeddingDims:   pmem.DefaultEmbeddingDimensions,
				DataDir:         cfg.Options.DataDirectory,
				ProjectRoot:     mainDir,
			})
			if pmemErr == nil && pmemSys != nil {
				c.pmem = pmemSys
				slog.Debug("Persistent memory system initialized")
			}
		}
		// Initialize embedding-based skill retrieval.
		c.initEmbeddingService()

		// Initialize Google Search client if grounding is enabled.
		if c.cfg.Options.GoogleGrounding {
			for p := range c.cfg.Providers.Seq() {
				if strings.ToLower(string(p.Type)) == "gemini" || strings.ToLower(string(p.Type)) == "google" {
					searchClient, err := c.buildGeminiCodeExecutionClient(ctx, p)
					if err == nil {
						c.googleSearchClient = searchClient
						break
					}
				}
			}
		}
	}
	c.checkpointService = memory.NewCheckpointService(c.orchestrationStore, c.messages, c.memory, c.pmem)
	c.backgroundDispatcher = agentbackground.NewDispatcher(c.backgroundRegistry, agentbackground.Hooks{
		Execute:       c.executeBackgroundSubAgent,
		DefaultCtx:    context.Background,
		MaxConcurrent: c.backgroundConcurrencyLimit,
	})
	c.backgroundMonitor = agentbackground.NewMonitor(c.backgroundRegistry, agentbackground.MonitorHooks{
		Notify: c.handleBackgroundCompletion,
	})
	if ctx != nil && c.backgroundMonitor != nil {
		go c.backgroundMonitor.Start(ctx)
	}
	if err := c.initPlanMode(); err != nil {
		return nil, err
	}

	agentCfg, ok := cfg.Agents[config.AgentCoder]
	if !ok {
		return nil, errors.New("coder agent not configured")
	}

	// TODO: make this dynamic when we support multiple agents
	prompt, err := coderPrompt(promptpkg.WithWorkingDir(c.mainWorkingDir()))
	if err != nil {
		return nil, err
	}

	agent, err := c.buildAgent(ctx, prompt, agentCfg, false)
	if err != nil {
		return nil, err
	}
	c.currentAgent = agent
	c.agents[config.AgentCoder] = agent
	return c, nil
}

// Run implements Coordinator.
func (c *coordinator) Run(ctx context.Context, sessionID string, userPrompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error) {
	env, err := c.prepareSubmission(ctx, sessionID, userPrompt, attachments, false, false)
	if err != nil {
		return nil, err
	}
	run := func() (*fantasy.AgentResult, error) {
		return c.executeSubmission(ctx, env)
	}

	result, originalErr := run()

	// Reset failures on success
	if originalErr == nil {
		c.googleSearchFailures.Delete(sessionID)
	}

	if c.isUnauthorized(originalErr) {
		switch {
		case env.providerCfg.OAuthToken != nil:
			slog.Debug("Received 401. Refreshing token and retrying", "provider", env.providerCfg.ID)
			if err := c.refreshOAuth2Token(ctx, env.providerCfg); err != nil {
				return nil, originalErr
			}
			slog.Debug("Retrying request with refreshed OAuth token", "provider", env.providerCfg.ID)
			return run()
		case strings.Contains(env.providerCfg.APIKeyTemplate, "$"):
			slog.Debug("Received 401. Refreshing API Key template and retrying", "provider", env.providerCfg.ID)
			if err := c.refreshApiKeyTemplate(ctx, env.providerCfg); err != nil {
				return nil, originalErr
			}
			slog.Debug("Retrying request with refreshed API key", "provider", env.providerCfg.ID)
			return run()
		}
	}

	return result, originalErr
}

func (c *coordinator) Submit(ctx context.Context, sessionID, userPrompt string, attachments ...message.Attachment) (SubmissionResult, error) {
	env, err := c.prepareSubmission(ctx, sessionID, userPrompt, attachments, true, true)
	if err != nil {
		return SubmissionResult{}, err
	}

	if c.currentAgent.IsSessionBusy(sessionID) {
		call := SessionAgentCall{
			SessionID:       env.sessionID,
			Prompt:          env.userPrompt,
			Attachments:     env.attachments,
			PrecreatedUser:  &env.userMessage,
			SkipUserMessage: true,
		}
		if err := c.currentAgent.Enqueue(call); err != nil {
			return SubmissionResult{}, err
		}
		return SubmissionResult{
			Status:        SubmissionStatusQueued,
			SessionID:     env.sessionID,
			UserMessageID: env.userMessage.ID,
		}, nil
	}

	go func(detached submissionEnvelope) {
		runCtx := context.WithoutCancel(ctx)
		if _, runErr := c.executeSubmission(runCtx, detached); runErr != nil && !errors.Is(runErr, context.Canceled) {
			slog.Error("detached submission failed", "session_id", detached.sessionID, "error", runErr)
		}
	}(env)

	return SubmissionResult{
		Status:        SubmissionStatusRunning,
		SessionID:     env.sessionID,
		UserMessageID: env.userMessage.ID,
	}, nil
}

func (c *coordinator) prepareSubmission(
	ctx context.Context,
	sessionID string,
	userPrompt string,
	attachments []message.Attachment,
	deferPreflight bool,
	createUser bool,
) (submissionEnvelope, error) {
	if userPrompt == "" && !message.ContainsTextAttachment(attachments) {
		return submissionEnvelope{}, ErrEmptyPrompt
	}

	model := c.currentAgent.Model()
	maxTokens := model.CatwalkCfg.DefaultMaxTokens
	if model.ModelCfg.MaxTokens != 0 {
		maxTokens = model.ModelCfg.MaxTokens
	}

	if !model.CatwalkCfg.SupportsImages && attachments != nil {
		filteredAttachments := make([]message.Attachment, 0, len(attachments))
		for _, att := range attachments {
			if att.IsText() {
				filteredAttachments = append(filteredAttachments, att)
			}
		}
		attachments = filteredAttachments
	}

	var userMessage message.Message
	if createUser {
		parts := []message.ContentPart{message.TextContent{Text: userPrompt}}
		for _, attachment := range attachments {
			parts = append(parts, message.BinaryContent{
				Path:     attachment.FilePath,
				MIMEType: attachment.MimeType,
				Data:     attachment.Content,
			})
		}
		created, err := c.messages.Create(ctx, sessionID, message.CreateMessageParams{
			Role:  message.User,
			Parts: parts,
		})
		if err != nil {
			return submissionEnvelope{}, fmt.Errorf("failed to create user message: %w", err)
		}
		userMessage = created
	}

	providerCfg, ok := c.cfg.Providers.Get(model.ModelCfg.Provider)
	if !ok {
		return submissionEnvelope{}, errors.New("model provider not configured")
	}
	mergedOptions, temp, topP, topK, freqPenalty, presPenalty := c.mergeCallOptions(model, providerCfg, sessionID)

	return submissionEnvelope{
		sessionID:      sessionID,
		userPrompt:     userPrompt,
		attachments:    attachments,
		userMessage:    userMessage,
		model:          model,
		providerCfg:    providerCfg,
		mergedOptions:  mergedOptions,
		temp:           temp,
		topP:           topP,
		topK:           topK,
		freqPenalty:    freqPenalty,
		presPenalty:    presPenalty,
		maxTokens:      maxTokens,
		deferPreflight: deferPreflight,
	}, nil
}

func (c *coordinator) executeSubmission(ctx context.Context, env submissionEnvelope) (*fantasy.AgentResult, error) {
	if env.providerCfg.OAuthToken != nil && env.providerCfg.OAuthToken.IsExpired() {
		slog.Debug("Token needs to be refreshed", "provider", env.providerCfg.ID)
		if err := c.refreshOAuth2Token(ctx, env.providerCfg); err != nil {
			return nil, err
		}
	}

	if err := c.waitForReady(ctx, 1*time.Second); err != nil {
		return nil, err
	}

	var skillContext string
	var activeSkillNames []string
	selectedMCP := map[string]struct{}(nil)

	if !env.deferPreflight {
		if shouldPrimeAutonomousSubAgents(env.userPrompt) {
			subAgentContext, err := c.autonomousSubAgentContextMaybeBackground(ctx, env.sessionID, env.userPrompt)
			if err != nil {
				slog.Debug("Autonomous sub-agent priming skipped", "error", err)
			} else if strings.TrimSpace(subAgentContext) != "" {
				skillContext = subAgentContext
			}
		}

		preflightContext := ""
		if requiresMCPDiscovery(env.userPrompt) {
			preflightContext = c.getMCPPreflightContext(env.sessionID, env.userPrompt)
		}
		if strings.TrimSpace(preflightContext) != "" {
			if skillContext != "" {
				skillContext += "\n\n"
			}
			skillContext += preflightContext
		}

		mcpContext := ""
		if requiresMCPDiscovery(env.userPrompt) {
			selectedMCP, mcpContext = c.getMCPSelection(env.sessionID, env.userPrompt)
		}
		if strings.TrimSpace(mcpContext) != "" {
			if skillContext != "" {
				skillContext += "\n\n"
			}
			skillContext += mcpContext
		}
		if inventoryContext := c.buildMCPInventoryContext(ctx); strings.TrimSpace(inventoryContext) != "" {
			if skillContext != "" {
				skillContext += "\n\n"
			}
			skillContext += inventoryContext
		}
		if subAgentContext := c.buildSubAgentStatusContext(env.sessionID); strings.TrimSpace(subAgentContext) != "" {
			if skillContext != "" {
				skillContext += "\n\n"
			}
			skillContext += subAgentContext
		}
	} else {
		c.refreshMCPPreflightAsync(env.sessionID, env.userPrompt)
		c.refreshMCPSelectionAsync(env.sessionID, env.userPrompt)
	}

	cachedTools, _ := c.getToolCache()
	activeTools := buildActiveToolNames(cachedTools, selectedMCP)
	if c.longHorizon != nil {
		if c.GetLongHorizonState(env.sessionID) != "" || len(strings.Fields(env.userPrompt)) >= 80 || shouldDelegateToSubAgents(env.userPrompt) {
			c.ensureLongHorizonDispatch(ctx, env.sessionID, env.userPrompt)
		}
	}
	c.syncMainAgentOrchestrationState(ctx, env.sessionID)
	if orchestrationContext := c.buildMainOrchestrationMemoryContext(ctx, env.sessionID); strings.TrimSpace(orchestrationContext) != "" {
		if skillContext != "" {
			skillContext += "\n\n"
		}
		skillContext += orchestrationContext
	}
	call := SessionAgentCall{
		SessionID:        env.sessionID,
		Prompt:           env.userPrompt,
		SkillContext:     skillContext,
		ActiveSkills:     activeSkillNames,
		ActiveTools:      activeTools,
		Attachments:      env.attachments,
		MaxOutputTokens:  env.maxTokens,
		ProviderOptions:  env.mergedOptions,
		Temperature:      env.temp,
		TopP:             env.topP,
		TopK:             env.topK,
		FrequencyPenalty: env.freqPenalty,
		PresencePenalty:  env.presPenalty,
	}
	if env.userMessage.ID != "" {
		call.PrecreatedUser = &env.userMessage
		call.SkipUserMessage = true
	}
	mainAgentID := mainAgentMailboxID(env.sessionID)
	c.recordOrchestrationActivity(ctx, mainAgentID, "main_turn_started", map[string]any{
		"session_id": env.sessionID,
		"message_id": env.userMessage.ID,
	})
	c.writeSessionCheckpoint(ctx, env.sessionID, mainAgentID, "", env.sessionID, buildCheckpointSummary("main_turn_started", env.userPrompt, "", "running", map[string]any{
		"message_id": env.userMessage.ID,
	}))
	result, err := c.currentAgent.Run(ctx, call)
	if err != nil {
		c.recordOrchestrationActivity(ctx, mainAgentID, "main_turn_error", map[string]any{
			"session_id": env.sessionID,
			"error":      err.Error(),
		})
		c.writeSessionCheckpoint(ctx, env.sessionID, mainAgentID, "", env.sessionID, buildCheckpointSummary("main_turn_error", env.userPrompt, err.Error(), "error", nil))
		return nil, err
	}
	c.recordOrchestrationActivity(ctx, mainAgentID, "main_turn_completed", map[string]any{
		"session_id": env.sessionID,
	})
	c.writeSessionCheckpoint(ctx, env.sessionID, mainAgentID, "", env.sessionID, buildCheckpointSummary("main_turn_completed", env.userPrompt, result.Response.Content.Text(), "completed", nil))
	return result, nil
}

// initEmbeddingService resolves the Gemini API key from configured providers
// and creates the EmbeddingService for skill retrieval.
func (c *coordinator) initEmbeddingService() {
	apiKey := c.resolveGeminiAPIKey()
	if apiKey == "" {
		slog.Debug("No Gemini API key found, embedding-based skill retrieval disabled")
		return
	}
	c.embeddingService = skills.NewEmbeddingService(apiKey, skills.DefaultSimilarityThreshold)
	slog.Debug("Embedding-based skill retrieval initialized")
}

// resolveGeminiAPIKey finds a Google/Gemini API key from configured providers.
// Falls back to GEMINI_API_KEY and GOOGLE_API_KEY environment variables.
func (c *coordinator) resolveGeminiAPIKey() string {
	for p := range c.cfg.Providers.Seq() {
		pType := strings.ToLower(string(p.Type))
		if pType != "gemini" && pType != "google" {
			continue
		}
		if p.APIKey == "" {
			continue
		}
		resolved, err := c.cfg.Resolve(p.APIKey)
		if err == nil && resolved != "" {
			return resolved
		}
	}

	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		return key
	}
	if key := os.Getenv("GOOGLE_API_KEY"); key != "" {
		return key
	}
	return ""
}

// resolveGeminiExtractionModel picks a Gemini model suitable for lightweight
// extraction/search tasks, falling back to a known available Gemini model.
func (c *coordinator) resolveGeminiExtractionModel() string {
	if entry, ok := c.cfg.Models[config.SelectedModelTypeSmall]; ok {
		if entry.Model != "" && isGeminiProvider(c.cfg, entry.Provider) {
			return entry.Model
		}
	}
	return "gemini-3.1-flash-lite-preview"
}

func isGeminiProvider(cfg *config.Config, providerID string) bool {
	if cfg == nil || providerID == "" || cfg.Providers == nil {
		return false
	}
	for p := range cfg.Providers.Seq() {
		if p.ID != providerID {
			continue
		}
		pType := strings.ToLower(string(p.Type))
		return pType == "gemini" || pType == "google"
	}
	return false
}

// skillKeywordMap defines hardwired keyword patterns for skill categories.
// If ANY keyword matches the user prompt (case-insensitive), the corresponding
// skill category is force-included. This guarantees deterministic skill injection.
var skillKeywordMap = map[string][]string{
	"frontend": {
		"frontend", "front-end", "front end", "ui", "ux", "ui/ux",
		"component", "page", "layout", "css", "tailwind", "style",
		"react", "next.js", "nextjs", "html", "design", "responsive",
		"animation", "button", "modal", "navbar", "sidebar", "header",
		"footer", "card", "form", "input", "dropdown", "interface",
		"beautiful", "aesthetic", "refactor ui", "refactor ux",
	},
	"backend": {
		"backend", "back-end", "back end", "api", "server", "database",
		"prisma", "drizzle", "orm", "endpoint", "route", "middleware",
		"authentication", "auth", "session", "webhook", "cron",
		"migration", "schema", "query", "elysia", "express",
		"go safe", "type-safe", "type safe", "safety",
	},
	"debugging": {
		"debug", "debugging", "bug", "fix", "error", "crash",
		"issue", "broken", "failing", "trace", "stack trace",
		"exception", "log", "logging", "troubleshoot",
	},
	"architect": {
		"architecture", "architect", "structural", "high-level",
		"design pattern", "dependency injection", "system design",
		"modularity", "package", "composition", "solid",
	},
	"devops": {
		"deployment", "deploy", "ci", "cd", "pipeline", "docker",
		"container", "kubernetes", "cloud", "infra", "infrastructure",
		"cicd", "github actions",
	},
	"security": {
		"security", "audit", "vulnerability", "protection", "secure",
		"encryption", "hashing", "sanitize", "cleaning", "leak",
		"secret", "token", "env",
	},
}

// isWholeWordMatch checks if a keyword appears as a whole word in the text.
func isWholeWordMatch(text, keyword string) bool {
	idx := strings.Index(text, keyword)
	for idx != -1 {
		leftBoundary := idx == 0 || !isWordChar(text[idx-1])
		rightBoundary := idx+len(keyword) == len(text) || !isWordChar(text[idx+len(keyword)])
		if leftBoundary && rightBoundary {
			return true
		}
		nextIdx := strings.Index(text[idx+len(keyword):], keyword)
		if nextIdx == -1 {
			break
		}
		idx += len(keyword) + nextIdx
	}
	return false
}

func isWordChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

// matchSkillsByKeyword performs deterministic, hardwired skill matching.
// It scans the user prompt for known keyword patterns and returns all
// discovered skills whose names, descriptions, or folder paths match
// the triggered categories. Folder path matching is critical because
// bundled skills may have placeholder YAML names.
func (c *coordinator) matchSkillsByKeyword(userPrompt string) []*skills.Skill {
	lower := strings.ToLower(userPrompt)
	triggeredCategories := make(map[string]bool)

	for category, keywords := range skillKeywordMap {
		for _, kw := range keywords {
			if isWholeWordMatch(lower, kw) {
				triggeredCategories[category] = true
				break
			}
		}
	}

	if len(triggeredCategories) == 0 {
		return nil
	}

	// Map category aliases: e.g. "debugging" category should also match "debug" folder
	categoryAliases := map[string][]string{
		"frontend":  {"frontend", "front-end", "ui", "ux"},
		"backend":   {"backend", "back-end", "api", "server"},
		"debugging": {"debug", "debugging", "troubleshoot"},
		"devops":    {"deploy", "infrastructure"},
		"architect": {"structure", "design"},
		"security":  {"secure", "audit"},
	}

	var matched []*skills.Skill
	for _, s := range c.discoveredSkills {
		skillNameLower := strings.ToLower(s.Name)
		skillDescLower := strings.ToLower(s.Description)
		skillPathLower := strings.ToLower(s.Path)

		for category := range triggeredCategories {
			// Direct category match on name/description
			if strings.Contains(skillNameLower, category) || strings.Contains(skillDescLower, category) {
				matched = append(matched, s)
				break
			}
			// Match on folder path (critical for bundled skills with placeholder names)
			if strings.Contains(skillPathLower, "/"+category) || strings.HasSuffix(skillPathLower, category) {
				matched = append(matched, s)
				break
			}
			// Match on category aliases against path
			if aliases, ok := categoryAliases[category]; ok {
				found := false
				for _, alias := range aliases {
					if strings.Contains(skillPathLower, "/"+alias) || strings.HasSuffix(skillPathLower, alias) {
						matched = append(matched, s)
						found = true
						break
					}
				}
				if found {
					break
				}
			}
		}
	}

	if len(matched) > 0 {
		names := make([]string, len(matched))
		for i, s := range matched {
			// Use folder name as display name if YAML name is a placeholder
			displayName := s.Name
			if displayName == "" || displayName == "SKILLNAME" {
				displayName = filepath.Base(s.Path)
			}
			names[i] = displayName
		}
		slog.Info("Hardwired skill keyword match", "categories", maps.Keys(triggeredCategories), "matched", names)
	}

	return matched
}

// mergeSkills merges two skill slices, deduplicating by skill name.
func mergeSkills(primary, secondary []*skills.Skill) []*skills.Skill {
	seen := make(map[string]bool, len(primary))
	var result []*skills.Skill

	for _, s := range primary {
		if !seen[s.Path] {
			seen[s.Path] = true
			result = append(result, s)
		}
	}
	for _, s := range secondary {
		if !seen[s.Path] {
			seen[s.Path] = true
			result = append(result, s)
		}
	}
	return result
}

// ensureSkillsDiscovered discovers all available skills once and caches them.
func (c *coordinator) ensureSkillsDiscovered() {
	c.skillsOnce.Do(func() {
		var expandedPaths []string
		for _, pth := range c.cfg.Options.SkillsPaths {
			expandedPaths = append(expandedPaths, promptpkg.ExpandPath(pth, *c.cfg))
		}
		c.discoveredSkills = skills.Discover(expandedPaths)
		if len(c.discoveredSkills) > 0 {
			slog.Debug("Discovered skills for embedding", "count", len(c.discoveredSkills))
		}
	})
}

func shouldPrimeAutonomousSubAgents(userPrompt string) bool {
	allowed, _ := shouldAllowSubAgentLaunch(userPrompt)
	return allowed && !hasExplicitSubAgentOrWorktreePlan(userPrompt)
}

func hasExplicitSubAgentOrWorktreePlan(userPrompt string) bool {
	prompt := strings.ToLower(userPrompt)
	signals := []string{
		"spawn_agent",
		"spawn subagent",
		"spawn sub-agent",
		"use subagents",
		"use sub-agents",
		"subagent",
		"sub-agent",
		"orchestrate_worktrees",
		"worktree orchestration",
		"worktree",
		"spawn 2-4 targeted subagents",
		"spawn 5-12 subagents",
		"one subagent per domain",
		"one per distinct domain",
	}
	return hasAnySignal(prompt, signals)
}

func buildAutonomousSubAgentTasks(userPrompt string) []autonomousSubAgentTask {
	prompt := strings.ToLower(userPrompt)
	tasks := []autonomousSubAgentTask{}

	if hasAnySignal(prompt, subAgentCodebaseSignals) {
		tasks = append(tasks, autonomousSubAgentTask{
			Name:         "codebase-map",
			SessionTitle: "Autonomous Codebase Map",
			Prompt: fmt.Sprintf(
				"User task: %s\n\nMap the codebase relevant to this task. Identify the main packages, entry points, and the shortest list of absolute file paths that matter. Return a compact summary only.",
				userPrompt,
			),
		})
	}

	if hasAnySignal(prompt, subAgentDependencySignals) {
		tasks = append(tasks, autonomousSubAgentTask{
			Name:         "dependency-trace",
			SessionTitle: "Autonomous Dependency Trace",
			Prompt: fmt.Sprintf(
				"User task: %s\n\nTrace the dependency and control flow relevant to this task. Focus on call paths, shared types, and cross-package interactions. Return only concise findings with absolute file paths.",
				userPrompt,
			),
		})
	}

	if hasAnySignal(prompt, subAgentRiskSignals) {
		tasks = append(tasks, autonomousSubAgentTask{
			Name:         "risk-review",
			SessionTitle: "Autonomous Risk Review",
			Prompt: fmt.Sprintf(
				"User task: %s\n\nReview likely implementation risks, edge cases, and validation points for this task. Return a compact checklist with absolute file paths where the risks live.",
				userPrompt,
			),
		})
	}

	if hasAnySignal(prompt, subAgentSourceSignals) && hasAnySignal(prompt, subAgentMultiSourceSignals) {
		tasks = append(tasks, autonomousSubAgentTask{
			Name:         "fact-audit",
			SessionTitle: "Autonomous Knowledge Audit",
			Prompt: fmt.Sprintf(
				"User task: %s\n\nAudit the task for external knowledge requirements. If the task mentions specific versions (e.g. Next.js 16.1, React 20) or features that post-date your 2025 cutoff, USE 'agentic_fetch' IMMEDIATELY to search the web for documentation. Provide a concise factual summary with source URLs.",
				userPrompt,
			),
		})
	}

	return tasks
}

func (c *coordinator) getProviderOptions(model Model, providerCfg config.ProviderConfig, useGrounding bool) fantasy.ProviderOptions {
	options := fantasy.ProviderOptions{}

	cfgOpts := []byte("{}")
	providerCfgOpts := []byte("{}")
	catwalkOpts := []byte("{}")

	if model.ModelCfg.ProviderOptions != nil {
		data, err := json.Marshal(model.ModelCfg.ProviderOptions)
		if err == nil {
			cfgOpts = data
		}
	}

	if providerCfg.ProviderOptions != nil {
		data, err := json.Marshal(providerCfg.ProviderOptions)
		if err == nil {
			providerCfgOpts = data
		}
	}

	if model.CatwalkCfg.Options.ProviderOptions != nil {
		data, err := json.Marshal(model.CatwalkCfg.Options.ProviderOptions)
		if err == nil {
			catwalkOpts = data
		}
	}

	readers := []io.Reader{
		bytes.NewReader(catwalkOpts),
		bytes.NewReader(providerCfgOpts),
		bytes.NewReader(cfgOpts),
	}

	got, err := jsons.Merge(readers)
	if err != nil {
		slog.Error("Could not merge call config", "err", err)
		return options
	}

	mergedOptions := make(map[string]any)

	err = json.Unmarshal([]byte(got), &mergedOptions)
	if err != nil {
		slog.Error("Could not create config for call", "err", err)
		return options
	}

	providerType := providerCfg.Type
	if providerType == "hyper" {
		if strings.Contains(model.CatwalkCfg.ID, "claude") {
			providerType = anthropic.Name
		} else if strings.Contains(model.CatwalkCfg.ID, "gpt") {
			providerType = openai.Name
		} else if strings.Contains(model.CatwalkCfg.ID, "gemini") {
			providerType = google.Name
		} else {
			providerType = openaicompat.Name
		}
	}

	switch providerType {
	case openai.Name, azure.Name:
		_, hasReasoningEffort := mergedOptions["reasoning_effort"]
		if !hasReasoningEffort && model.ModelCfg.ReasoningEffort != "" {
			mergedOptions["reasoning_effort"] = model.ModelCfg.ReasoningEffort
		}
		if openai.IsResponsesModel(model.CatwalkCfg.ID) {
			if openai.IsResponsesReasoningModel(model.CatwalkCfg.ID) {
				mergedOptions["reasoning_summary"] = "auto"
				mergedOptions["include"] = []openai.IncludeType{openai.IncludeReasoningEncryptedContent}
			}
			parsed, err := openai.ParseResponsesOptions(mergedOptions)
			if err == nil {
				options[openai.Name] = parsed
			}
		} else {
			parsed, err := openai.ParseOptions(mergedOptions)
			if err == nil {
				options[openai.Name] = parsed
			}
		}
	case anthropic.Name:
		var (
			_, hasEffort = mergedOptions["effort"]
			_, hasThink  = mergedOptions["thinking"]
		)
		switch {
		case !hasEffort && model.ModelCfg.ReasoningEffort != "":
			mergedOptions["effort"] = model.ModelCfg.ReasoningEffort
		case !hasThink && model.ModelCfg.Think:
			mergedOptions["thinking"] = map[string]any{"budget_tokens": 2000}
		}
		parsed, err := anthropic.ParseOptions(mergedOptions)
		if err == nil {
			options[anthropic.Name] = parsed
		}

	case openrouter.Name:
		_, hasReasoning := mergedOptions["reasoning"]
		if !hasReasoning && model.ModelCfg.ReasoningEffort != "" {
			mergedOptions["reasoning"] = map[string]any{
				"enabled": true,
				"effort":  model.ModelCfg.ReasoningEffort,
			}
		}
		parsed, err := openrouter.ParseOptions(mergedOptions)
		if err == nil {
			options[openrouter.Name] = parsed
		}
	case vercel.Name:
		_, hasReasoning := mergedOptions["reasoning"]
		if !hasReasoning && model.ModelCfg.ReasoningEffort != "" {
			mergedOptions["reasoning"] = map[string]any{
				"enabled": true,
				"effort":  model.ModelCfg.ReasoningEffort,
			}
		}
		parsed, err := vercel.ParseOptions(mergedOptions)
		if err == nil {
			options[vercel.Name] = parsed
		}
	case google.Name:
		_, hasGoogleSearch := mergedOptions["google_search"]
		switch {
		case useGrounding && !hasGoogleSearch:
			mergedOptions["google_search"] = true
		case !useGrounding && hasGoogleSearch:
			mergedOptions["google_search"] = false
		}

		_, hasReasoning := mergedOptions["thinking_config"]
		if !hasReasoning {
			if config.IsGemini3Model(model.CatwalkCfg.ID) {
				level := model.ModelCfg.ReasoningEffort
				if level == "" {
					level = "medium" // default
				}
				mergedOptions["thinking_config"] = map[string]any{
					"thinking_level":   level,
					"include_thoughts": true,
				}
			} else if config.IsGemini25Model(model.CatwalkCfg.ID) {
				if model.ModelCfg.Think {
					mergedOptions["thinking_config"] = map[string]any{
						"thinking_budget":  2048,
						"include_thoughts": true,
					}
				}
			}
		}
		parsed, err := google.ParseOptions(mergedOptions)
		if err == nil {
			options[google.Name] = parsed
		}
	case openaicompat.Name:
		_, hasReasoningEffort := mergedOptions["reasoning_effort"]
		if !hasReasoningEffort && model.ModelCfg.ReasoningEffort != "" {
			mergedOptions["reasoning_effort"] = model.ModelCfg.ReasoningEffort
		}
		parsed, err := openaicompat.ParseOptions(mergedOptions)
		if err == nil {
			options[openaicompat.Name] = parsed
		}
	}

	return options
}

func (c *coordinator) mergeCallOptions(model Model, cfg config.ProviderConfig, sessionID string) (fantasy.ProviderOptions, *float64, *float64, *int64, *float64, *float64) {
	// Google Search failure tracking - fallback to DDG after 2 failures.
	// Failure = 2 consecutive errors with grounding enabled.
	useGrounding := c.cfg.Options.GoogleGrounding
	if v, ok := c.googleSearchFailures.Load(sessionID); ok {
		if failures, ok := v.(int); ok && failures >= 2 {
			useGrounding = false
		}
	}

	modelOptions := c.getProviderOptions(model, cfg, useGrounding)
	temp := cmp.Or(model.ModelCfg.Temperature, model.CatwalkCfg.Options.Temperature)
	topP := cmp.Or(model.ModelCfg.TopP, model.CatwalkCfg.Options.TopP)
	topK := cmp.Or(model.ModelCfg.TopK, model.CatwalkCfg.Options.TopK)
	freqPenalty := cmp.Or(model.ModelCfg.FrequencyPenalty, model.CatwalkCfg.Options.FrequencyPenalty)
	presPenalty := cmp.Or(model.ModelCfg.PresencePenalty, model.CatwalkCfg.Options.PresencePenalty)
	return modelOptions, temp, topP, topK, freqPenalty, presPenalty
}

func (c *coordinator) buildAgent(ctx context.Context, prompt *promptpkg.Prompt, agent config.Agent, isSubAgent bool) (SessionAgent, error) {
	return c.buildAgentWithWorkingDir(ctx, prompt, agent, isSubAgent, c.mainWorkingDir())
}

func (c *coordinator) mainWorkingDir() string {
	return c.cfg.WorkingDir()
}

func (c *coordinator) buildAgentWithWorkingDir(ctx context.Context, prompt *promptpkg.Prompt, agent config.Agent, isSubAgent bool, workingDir string) (SessionAgent, error) {
	return c.buildAgentWithWorkingDirInternal(ctx, prompt, agent, isSubAgent, workingDir, nil, nil)
}

func (c *coordinator) buildAgentWithWorkingDirOverrides(ctx context.Context, prompt *promptpkg.Prompt, agent config.Agent, isSubAgent bool, workingDir string, override *agentModelOverride, writeScope *tools.WriteScope) (SessionAgent, error) {
	return c.buildAgentWithWorkingDirInternal(ctx, prompt, agent, isSubAgent, workingDir, override, writeScope)
}

func (c *coordinator) buildAgentWithWorkingDirInternal(ctx context.Context, prompt *promptpkg.Prompt, agent config.Agent, isSubAgent bool, workingDir string, override *agentModelOverride, writeScope *tools.WriteScope) (SessionAgent, error) {
	large, small, err := c.buildAgentModelsWithOverride(ctx, isSubAgent, override)
	if err != nil {
		return nil, err
	}

	largeProviderCfg, _ := c.cfg.Providers.Get(large.ModelCfg.Provider)
	result := NewSessionAgent(SessionAgentOptions{
		LargeModel:           large,
		SmallModel:           small,
		SystemPromptPrefix:   largeProviderCfg.SystemPromptPrefix,
		SystemPrompt:         "",
		IsSubAgent:           isSubAgent,
		DisableAutoSummarize: c.cfg.Options.DisableAutoSummarize,
		IsYolo:               c.permissions.SkipRequests(),
		Sessions:             c.sessions,
		Messages:             c.messages,
		Tools:                nil,
		Memory:               c.memory,
		Pmem:                 c.pmem,
		LongHorizon:          c.longHorizon,
		MemoryConsolidator:   c.ConsolidateMemory,
		WaitBackground:       c.waitForBackgroundWork,
		CheckpointTurn:       c.checkpointTurn,
		WriteScope:           writeScope,
	})

	// Use a local WaitGroup for sub-agents to ensure initialization finishes before returning.
	// For the main agent, use c.readyWg to allow background initialization during startup.
	var wg interface {
		Go(func() error)
	}
	if isSubAgent {
		innerWg := &errgroup.Group{}
		wg = innerWg
		defer func() {
			if err := innerWg.Wait(); err != nil {
				slog.Error("Failed to initialize sub-agent", "error", err)
			}
		}()
	} else {
		wg = &c.readyWg
	}

	wg.Go(func() error {
		systemPrompt, err := prompt.Build(ctx, large.Model.Provider(), large.Model.Model(), *c.cfg)
		if err != nil {
			return err
		}
		result.SetSystemPrompt(systemPrompt)
		return nil
	})

	wg.Go(func() error {
		agentTools, err := c.buildToolsForWorkingDir(ctx, agent, workingDir)
		if err != nil {
			return err
		}
		if isSubAgent {
			agentTools = filterTools(agentTools, map[string]struct{}{
				AgentToolName:                {},
				SpawnAgentsOnCSVToolName:     {},
				ReportAgentJobResultToolName: {},
				OrchestrateWorktreesToolName: {},
				SpawnAgentToolName:           {},
				ResumeAgentToolName:          {},
				SendInputToolName:            {},
				WaitAgentsToolName:           {},
				CollectResultToolName:        {},
				CloseAgentToolName:           {},
				tools.WriteToolName:          {},
				tools.EditToolName:           {},
				tools.SingleEditToolName:     {},
				tools.AgenticEditToolName:    {},
				tools.ApplyPatchToolName:     {},
			})
		}
		result.SetTools(agentTools)
		if !isSubAgent {
			c.setToolCache(agentTools)
		}
		return nil
	})

	return result, nil
}

func filterTools(items []fantasy.AgentTool, disallowed map[string]struct{}) []fantasy.AgentTool {
	if len(items) == 0 || len(disallowed) == 0 {
		return items
	}
	filtered := make([]fantasy.AgentTool, 0, len(items))
	for _, tool := range items {
		if tool == nil {
			continue
		}
		if _, blocked := disallowed[tool.Info().Name]; blocked {
			continue
		}
		filtered = append(filtered, tool)
	}
	return filtered
}

func (c *coordinator) refreshSystemPrompt(ctx context.Context) error {
	if c.currentAgent == nil {
		return nil
	}
	model := c.currentAgent.Model()
	prompt, err := coderPrompt(promptpkg.WithWorkingDir(c.mainWorkingDir()))
	if err != nil {
		return err
	}
	systemPrompt, err := prompt.Build(ctx, model.Model.Provider(), model.Model.Model(), *c.cfg)
	if err != nil {
		return err
	}
	c.currentAgent.SetSystemPrompt(systemPrompt)
	return nil
}

func (c *coordinator) buildTools(ctx context.Context, agent config.Agent) ([]fantasy.AgentTool, error) {
	return c.buildToolsForWorkingDir(ctx, agent, c.mainWorkingDir())
}

func (c *coordinator) buildToolsForWorkingDir(ctx context.Context, agent config.Agent, workingDir string) ([]fantasy.AgentTool, error) {
	tools.SetValidationFileTracker(c.filetracker)

	var allTools []fantasy.AgentTool
	if slices.Contains(agent.AllowedTools, AgentToolName) {
		agentTool, err := c.agentTool(ctx)
		if err != nil {
			return nil, err
		}
		allTools = append(allTools, agentTool)
	}
	if slices.Contains(agent.AllowedTools, SpawnAgentToolName) {
		spawnTool, err := c.spawnAgentTool(ctx)
		if err != nil {
			return nil, err
		}
		allTools = append(allTools, spawnTool)
	}
	if slices.Contains(agent.AllowedTools, ResumeAgentToolName) {
		resumeTool, err := c.resumeAgentTool(ctx)
		if err != nil {
			return nil, err
		}
		allTools = append(allTools, resumeTool)
	}
	if slices.Contains(agent.AllowedTools, SendInputToolName) {
		sendTool, err := c.sendInputTool(ctx)
		if err != nil {
			return nil, err
		}
		allTools = append(allTools, sendTool)
	}
	if slices.Contains(agent.AllowedTools, WaitAgentsToolName) {
		waitTool, err := c.waitAgentsTool(ctx)
		if err != nil {
			return nil, err
		}
		allTools = append(allTools, waitTool)
	}
	if slices.Contains(agent.AllowedTools, CollectResultToolName) {
		collectTool, err := c.collectResultTool(ctx)
		if err != nil {
			return nil, err
		}
		allTools = append(allTools, collectTool)
	}
	if slices.Contains(agent.AllowedTools, CloseAgentToolName) {
		closeTool, err := c.closeAgentTool(ctx)
		if err != nil {
			return nil, err
		}
		allTools = append(allTools, closeTool)
	}
	if slices.Contains(agent.AllowedTools, SpawnAgentsOnCSVToolName) {
		jobTool, err := c.spawnAgentsOnCSVTool(ctx)
		if err != nil {
			return nil, err
		}
		allTools = append(allTools, jobTool)
	}
	if slices.Contains(agent.AllowedTools, ReportAgentJobResultToolName) {
		reportTool, err := c.reportAgentJobResultTool(ctx)
		if err != nil {
			return nil, err
		}
		allTools = append(allTools, reportTool)
	}
	if slices.Contains(agent.AllowedTools, OrchestrateWorktreesToolName) {
		planTool, err := c.orchestrateWorktreesTool(ctx)
		if err != nil {
			return nil, err
		}
		allTools = append(allTools, planTool)
	}

	if slices.Contains(agent.AllowedTools, tools.AgenticFetchToolName) {
		agenticFetchTool, err := c.agenticFetchTool(ctx, nil)
		if err != nil {
			return nil, err
		}
		allTools = append(allTools, agenticFetchTool)
	}

	// Get the model name for the agent
	modelName := ""
	if modelCfg, ok := c.cfg.Models[agent.Model]; ok {
		if model := c.cfg.GetModel(modelCfg.Provider, modelCfg.Model); model != nil {
			modelName = model.Name
		}
	}

	maxConcurrent := 250

	if pythonTool, err := c.buildPythonTool(ctx, agent); err != nil {
		return nil, err
	} else if pythonTool != nil {
		allTools = append(allTools, pythonTool)
	}

	allTools = append(allTools,
		tools.NewBashTool(c.permissions, workingDir, c.cfg.Options.Attribution, modelName),
		tools.NewJobListTool(),
		tools.NewJobOutputTool(),
		tools.NewJobKillTool(),
		tools.NewDownloadTool(c.permissions, workingDir, nil),
		tools.NewEditTool(c.lspManager, c.editGuard, c.permissions, c.history, c.filetracker, workingDir),
		tools.NewSingleEditTool(c.lspManager, c.editGuard, c.permissions, c.history, c.filetracker, workingDir),
		tools.NewMultiEditTool(c.lspManager, c.editGuard, c.permissions, c.history, c.filetracker, workingDir),
		tools.NewApplyPatchTool(workingDir),
		tools.NewFetchTool(c.permissions, workingDir, nil),
		tools.NewGlobTool(workingDir),
		tools.NewMemoryQueryTool(c.memory),
		tools.NewGrepTool(workingDir, c.cfg.Tools.Grep),
		tools.NewLsTool(c.permissions, workingDir, c.cfg.Tools.Ls),
		tools.NewSourcegraphTool(nil),
		// tools.NewTodosTool(c.sessions),  // COMMENTED OUT - Replaced with Codex-style update_plan
		tools.NewUpdatePlanTool(c.sessions),       // Codex-style plan management tool
		tools.NewRequestUserInputTool(c.sessions), // Codex-style structured questions tool (Plan Mode only)
		tools.NewSetModeTool(c.sessions),          // Codex-style mode switching tool
		tools.NewViewTool(tools.ViewToolName, c.lspManager, c.editGuard, c.permissions, c.filetracker, workingDir, 1, c.cfg.Options.SkillsPaths...),
		tools.NewViewTool(tools.SingleViewToolName, c.lspManager, c.editGuard, c.permissions, c.filetracker, workingDir, 1, c.cfg.Options.SkillsPaths...),
		tools.FastViewTool(tools.AgenticViewToolName, c.lspManager, c.editGuard, c.permissions, c.filetracker, workingDir, maxConcurrent, c.cfg.Options.SkillsPaths...),
		tools.NewWriteTool(c.lspManager, c.editGuard, c.permissions, c.history, c.filetracker, workingDir),
	)
	if slices.Contains(agent.AllowedTools, AgentMailSendToolName) {
		tool, err := c.agentMailSendTool(ctx)
		if err != nil {
			return nil, err
		}
		allTools = append(allTools, tool)
	}
	if slices.Contains(agent.AllowedTools, AgentMailInboxToolName) {
		tool, err := c.agentMailInboxTool(ctx)
		if err != nil {
			return nil, err
		}
		allTools = append(allTools, tool)
	}
	if slices.Contains(agent.AllowedTools, CheckHookToolName) {
		tool, err := c.checkHookTool(ctx)
		if err != nil {
			return nil, err
		}
		allTools = append(allTools, tool)
	}

	listTools := func() []string {
		if len(allTools) == 0 {
			return nil
		}
		names := make([]string, 0, len(allTools))
		for _, tool := range allTools {
			if tool == nil {
				continue
			}
			names = append(names, tool.Info().Name)
		}
		return names
	}
	allTools = append(allTools, tools.NewListToolsTool(listTools))
	if c.cfg.Options.GoogleGrounding && c.googleSearchClient != nil {
		// We use a small Gemini model for grounding search to keep it fast.
		searchModel := c.resolveGeminiExtractionModel()

		allTools = append(allTools, tools.NewGoogleSearchTool(
			c.googleSearchClient,
			nil,
			searchModel,
			func(sessionID string) int {
				if v, ok := c.googleSearchFailures.Load(sessionID); ok {
					return v.(int)
				}
				return 0
			},
			func(sessionID string) {
				failures := 0
				if v, ok := c.googleSearchFailures.Load(sessionID); ok {
					failures = v.(int)
				}
				c.googleSearchFailures.Store(sessionID, failures+1)
			},
			func(sessionID string) {
				c.googleSearchFailures.Delete(sessionID)
			},
			func(ctx context.Context, sessionID string) string {
				if sessionID == "" {
					return ""
				}
				messages, err := c.messages.ListUserMessages(ctx, sessionID)
				if err != nil {
					return ""
				}
				for i := len(messages) - 1; i >= 0; i-- {
					text := strings.TrimSpace(messages[i].Content().Text)
					if text != "" {
						return text
					}
				}
				return ""
			},
		))
	}

	// Add persistent memory tools (recall_memory, save_memory).
	if c.pmem != nil {
		allTools = append(allTools,
			pmem.NewRecallTool(c.pmem),
			pmem.NewSaveTool(c.pmem),
			pmem.NewHealthTool(c.pmem),
		)
	}

	// Add skill tools (list_skills, load_skill).
	loadSkillTool, err := c.loadSkillTool(ctx)
	if err == nil {
		allTools = append(allTools, loadSkillTool)
	}
	listSkillsTool, err := c.listSkillsTool(ctx)
	if err == nil {
		allTools = append(allTools, listSkillsTool)
	}

	// Add LSP tools if user has configured LSPs or auto_lsp is enabled (nil or true).
	if len(c.cfg.LSP) > 0 || c.cfg.Options.AutoLSP == nil || *c.cfg.Options.AutoLSP {
		allTools = append(allTools, tools.NewDiagnosticsTool(c.lspManager), tools.NewReferencesTool(c.lspManager), tools.NewLSPRestartTool(c.lspManager))
	}

	allTools = append(
		allTools,
		tools.NewListAvailableMCPsTool(c.cfg, c.permissions),
		tools.NewConnectMCPTool(c.cfg, c.permissions),
		tools.NewCallMCPTool(c.cfg, c.permissions),
		tools.NewListMCPToolsTool(c.cfg, c.permissions),
		tools.NewListMCPResourcesTool(c.cfg, c.permissions),
		tools.NewReadMCPResourceTool(c.cfg, c.permissions),
	)

	var filteredTools []fantasy.AgentTool
	for _, tool := range allTools {
		if tool.Info().Name == tools.GoogleSearchToolName {
			filteredTools = append(filteredTools, tool)
			continue
		}
		if slices.Contains(agent.AllowedTools, tool.Info().Name) {
			filteredTools = append(filteredTools, tool)
		}
	}

	for _, tool := range tools.GetMCPTools(c.permissions, c.cfg, workingDir) {
		if agent.AllowedMCP == nil {
			// No MCP restrictions
			filteredTools = append(filteredTools, tool)
			continue
		}
		if len(agent.AllowedMCP) == 0 {
			// No MCPs allowed
			slog.Debug("No MCPs allowed", "tool", tool.Name(), "agent", agent.Name)
			break
		}

		for mcp, tools := range agent.AllowedMCP {
			if mcp != tool.MCP() {
				continue
			}
			if len(tools) == 0 || slices.Contains(tools, tool.MCPToolName()) {
				filteredTools = append(filteredTools, tool)
				break
			}
			slog.Debug("MCP not allowed", "tool", tool.Name(), "agent", agent.Name)
		}
	}

	// list_tools is already in allTools - no need to recreate
	if slices.Contains(agent.AllowedTools, tools.SearchToolsToolName) {
		searchToolsTool := tools.NewSearchToolsTool(func() []fantasy.ToolInfo {
			infos := make([]fantasy.ToolInfo, 0, len(filteredTools))
			for _, tool := range filteredTools {
				infos = append(infos, tool.Info())
			}
			return infos
		})
		filteredTools = append(filteredTools, searchToolsTool)
	}
	if slices.Contains(agent.AllowedTools, tools.ToolSuggestToolName) {
		suggestTool := tools.NewToolSuggestTool(c.cfg, c.permissions)
		filteredTools = append(filteredTools, suggestTool)
	}
	slices.SortFunc(filteredTools, func(a, b fantasy.AgentTool) int {
		return strings.Compare(a.Info().Name, b.Info().Name)
	})

	return filteredTools, nil
}

func (c *coordinator) buildPythonTool(ctx context.Context, agent config.Agent) (fantasy.AgentTool, error) {
	modelCfg, ok := c.cfg.Models[agent.Model]
	if !ok {
		return nil, nil
	}

	if !strings.HasPrefix(strings.ToLower(modelCfg.Model), "gemini") {
		return nil, nil
	}

	providerCfg, ok := c.cfg.Providers.Get(modelCfg.Provider)
	if !ok {
		return nil, nil
	}

	switch providerCfg.Type {
	case google.Name, "gemini", "google-vertex":
	default:
		return nil, nil
	}

	client, err := c.buildGeminiCodeExecutionClient(ctx, providerCfg)
	if err != nil {
		return nil, err
	}

	return tools.NewPythonTool(client, modelCfg.Model), nil
}

func (c *coordinator) buildGeminiCodeExecutionClient(ctx context.Context, providerCfg config.ProviderConfig) (*genai.Client, error) {
	headers := maps.Clone(providerCfg.ExtraHeaders)
	if headers == nil {
		headers = make(map[string]string)
	}

	clientConfig := &genai.ClientConfig{
		HTTPClient: c.httpClient(),
	}

	if len(headers) > 0 {
		httpHeaders := http.Header{}
		for k, v := range headers {
			httpHeaders.Add(k, v)
		}
		clientConfig.HTTPOptions.Headers = httpHeaders
	}

	switch providerCfg.Type {
	case google.Name, "gemini":
		apiKey, _ := c.cfg.Resolve(providerCfg.APIKey)
		baseURL, _ := c.cfg.Resolve(providerCfg.BaseURL)
		clientConfig.Backend = genai.BackendGeminiAPI
		clientConfig.APIKey = apiKey
		if baseURL != "" {
			clientConfig.HTTPOptions.BaseURL = baseURL
		}
	case "google-vertex":
		clientConfig.Backend = genai.BackendVertexAI
		clientConfig.Project = providerCfg.ExtraParams["project"]
		clientConfig.Location = providerCfg.ExtraParams["location"]
		if err := clientConfig.UseDefaultCredentials(); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("provider type %q does not support Gemini code execution", providerCfg.Type)
	}

	return genai.NewClient(ctx, clientConfig)
}

type agentModelOverride struct {
	Provider        string
	Model           string
	ReasoningEffort string
}

func (c *coordinator) resolveSubAgentModelOverride(rawModel, reasoningEffort string) (*agentModelOverride, error) {
	rawModel = strings.TrimSpace(rawModel)
	reasoningEffort = strings.TrimSpace(reasoningEffort)
	if rawModel == "" && reasoningEffort == "" {
		return nil, nil
	}
	largeModelCfg, ok := c.cfg.Models[config.SelectedModelTypeLarge]
	if !ok {
		return nil, errors.New("large model not selected")
	}
	provider := largeModelCfg.Provider
	modelID := largeModelCfg.Model
	if rawModel != "" {
		parts := strings.SplitN(rawModel, ":", 2)
		if len(parts) == 2 {
			provider = strings.TrimSpace(parts[0])
			modelID = strings.TrimSpace(parts[1])
		} else {
			modelID = strings.TrimSpace(rawModel)
		}
	}
	if provider == "" || modelID == "" {
		return nil, errors.New("invalid model override")
	}
	if _, ok := c.cfg.Providers.Get(provider); !ok {
		return nil, fmt.Errorf("model provider %q not configured", provider)
	}
	return &agentModelOverride{
		Provider:        provider,
		Model:           modelID,
		ReasoningEffort: reasoningEffort,
	}, nil
}

// TODO: when we support multiple agents we need to change this so that we pass in the agent specific model config
func (c *coordinator) buildAgentModels(ctx context.Context, isSubAgent bool) (Model, Model, error) {
	return c.buildAgentModelsWithOverride(ctx, isSubAgent, nil)
}

func (c *coordinator) buildAgentModelsWithOverride(ctx context.Context, isSubAgent bool, override *agentModelOverride) (Model, Model, error) {
	largeModelCfg, ok := c.cfg.Models[config.SelectedModelTypeLarge]
	if !ok {
		return Model{}, Model{}, errors.New("large model not selected")
	}
	smallModelCfg, ok := c.cfg.Models[config.SelectedModelTypeSmall]
	if !ok {
		return Model{}, Model{}, errors.New("small model not selected")
	}

	if override != nil {
		if override.Provider != "" {
			largeModelCfg.Provider = override.Provider
		}
		if override.Model != "" {
			largeModelCfg.Model = override.Model
		}
		if override.ReasoningEffort != "" {
			largeModelCfg.ReasoningEffort = override.ReasoningEffort
		}
	}

	largeProviderCfg, ok := c.cfg.Providers.Get(largeModelCfg.Provider)
	if !ok {
		return Model{}, Model{}, errors.New("large model provider not configured")
	}

	largeProvider, err := c.buildProvider(largeProviderCfg, largeModelCfg, isSubAgent)
	if err != nil {
		return Model{}, Model{}, err
	}

	smallProviderCfg, ok := c.cfg.Providers.Get(smallModelCfg.Provider)
	if !ok {
		return Model{}, Model{}, errors.New("small model provider not configured")
	}

	smallProvider, err := c.buildProvider(smallProviderCfg, smallModelCfg, true)
	if err != nil {
		return Model{}, Model{}, err
	}

	var largeCatwalkModel *catwalk.Model
	var smallCatwalkModel *catwalk.Model

	for _, m := range largeProviderCfg.Models {
		if m.ID == largeModelCfg.Model {
			largeCatwalkModel = &m
		}
	}
	for _, m := range smallProviderCfg.Models {
		if m.ID == smallModelCfg.Model {
			smallCatwalkModel = &m
		}
	}

	if largeCatwalkModel == nil {
		return Model{}, Model{}, errors.New("large model not found in provider config")
	}

	if smallCatwalkModel == nil {
		return Model{}, Model{}, errors.New("small model not found in provider config")
	}

	largeModelID := largeModelCfg.Model
	smallModelID := smallModelCfg.Model

	if largeModelCfg.Provider == openrouter.Name && isExactoSupported(largeModelID) {
		largeModelID += ":exacto"
	}

	if smallModelCfg.Provider == openrouter.Name && isExactoSupported(smallModelID) {
		smallModelID += ":exacto"
	}

	largeModel, err := largeProvider.LanguageModel(ctx, largeModelID)
	if err != nil {
		return Model{}, Model{}, err
	}
	smallModel, err := smallProvider.LanguageModel(ctx, smallModelID)
	if err != nil {
		return Model{}, Model{}, err
	}

	return Model{
			Model:      largeModel,
			CatwalkCfg: *largeCatwalkModel,
			ModelCfg:   largeModelCfg,
		}, Model{
			Model:      smallModel,
			CatwalkCfg: *smallCatwalkModel,
			ModelCfg:   smallModelCfg,
		}, nil
}

func (c *coordinator) buildAnthropicProvider(baseURL, apiKey string, headers map[string]string, providerID string) (fantasy.Provider, error) {
	var opts []anthropic.Option

	switch {
	case strings.HasPrefix(apiKey, "Bearer "):
		// NOTE: Prevent the SDK from picking up the API key from env.
		os.Setenv("ANTHROPIC_API_KEY", "")
		headers["Authorization"] = apiKey
	case providerID == string(catwalk.InferenceProviderMiniMax) || providerID == string(catwalk.InferenceProviderMiniMaxChina):
		// NOTE: Prevent the SDK from picking up the API key from env.
		os.Setenv("ANTHROPIC_API_KEY", "")
		headers["Authorization"] = "Bearer " + apiKey
	case apiKey != "":
		// X-Api-Key header
		opts = append(opts, anthropic.WithAPIKey(apiKey))
	}

	if len(headers) > 0 {
		opts = append(opts, anthropic.WithHeaders(headers))
	}

	if baseURL != "" {
		opts = append(opts, anthropic.WithBaseURL(baseURL))
	}

	opts = append(opts, anthropic.WithHTTPClient(c.httpClient()))
	return anthropic.New(opts...)
}

func (c *coordinator) buildOpenaiProvider(baseURL, apiKey string, headers map[string]string) (fantasy.Provider, error) {
	opts := []openai.Option{
		openai.WithAPIKey(apiKey),
		openai.WithUseResponsesAPI(),
	}
	opts = append(opts, openai.WithHTTPClient(c.httpClient()))
	if len(headers) > 0 {
		opts = append(opts, openai.WithHeaders(headers))
	}
	if baseURL != "" {
		opts = append(opts, openai.WithBaseURL(baseURL))
	}
	return openai.New(opts...)
}

func (c *coordinator) buildOpenrouterProvider(_, apiKey string, headers map[string]string) (fantasy.Provider, error) {
	opts := []openrouter.Option{
		openrouter.WithAPIKey(apiKey),
	}
	opts = append(opts, openrouter.WithHTTPClient(c.httpClient()))
	if len(headers) > 0 {
		opts = append(opts, openrouter.WithHeaders(headers))
	}
	return openrouter.New(opts...)
}

func (c *coordinator) buildVercelProvider(_, apiKey string, headers map[string]string) (fantasy.Provider, error) {
	opts := []vercel.Option{
		vercel.WithAPIKey(apiKey),
	}
	opts = append(opts, vercel.WithHTTPClient(c.httpClient()))
	if len(headers) > 0 {
		opts = append(opts, vercel.WithHeaders(headers))
	}
	return vercel.New(opts...)
}

func (c *coordinator) buildOpenaiCompatProvider(baseURL, apiKey string, headers map[string]string, extraBody map[string]any, providerID string, isSubAgent bool) (fantasy.Provider, error) {
	opts := []openaicompat.Option{
		openaicompat.WithBaseURL(baseURL),
		openaicompat.WithAPIKey(apiKey),
	}

	// Set HTTP client based on provider and debug mode.
	var httpClient *http.Client
	if providerID == string(catwalk.InferenceProviderCopilot) {
		opts = append(opts, openaicompat.WithUseResponsesAPI())
		httpClient = copilot.NewClient(isSubAgent, c.cfg.Options.Debug)
	} else {
		httpClient = c.httpClient()
	}
	if httpClient != nil {
		opts = append(opts, openaicompat.WithHTTPClient(httpClient))
	}

	if len(headers) > 0 {
		opts = append(opts, openaicompat.WithHeaders(headers))
	}

	for extraKey, extraValue := range extraBody {
		opts = append(opts, openaicompat.WithSDKOptions(openaisdk.WithJSONSet(extraKey, extraValue)))
	}

	return openaicompat.New(opts...)
}

func (c *coordinator) buildAzureProvider(baseURL, apiKey string, headers map[string]string, options map[string]string) (fantasy.Provider, error) {
	opts := []azure.Option{
		azure.WithBaseURL(baseURL),
		azure.WithAPIKey(apiKey),
		azure.WithUseResponsesAPI(),
	}
	opts = append(opts, azure.WithHTTPClient(c.httpClient()))
	if options == nil {
		options = make(map[string]string)
	}
	if apiVersion, ok := options["apiVersion"]; ok {
		opts = append(opts, azure.WithAPIVersion(apiVersion))
	}
	if len(headers) > 0 {
		opts = append(opts, azure.WithHeaders(headers))
	}

	return azure.New(opts...)
}

func (c *coordinator) buildBedrockProvider(headers map[string]string) (fantasy.Provider, error) {
	var opts []bedrock.Option
	opts = append(opts, bedrock.WithHTTPClient(c.httpClient()))
	if len(headers) > 0 {
		opts = append(opts, bedrock.WithHeaders(headers))
	}
	bearerToken := os.Getenv("AWS_BEARER_TOKEN_BEDROCK")
	if bearerToken != "" {
		opts = append(opts, bedrock.WithAPIKey(bearerToken))
	}
	return bedrock.New(opts...)
}

func (c *coordinator) buildGoogleProvider(baseURL, apiKey string, headers map[string]string) (fantasy.Provider, error) {
	opts := []google.Option{
		google.WithBaseURL(baseURL),
		google.WithGeminiAPIKey(apiKey),
	}
	if c.cfg.Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, google.WithHTTPClient(httpClient))
	}
	if len(headers) > 0 {
		opts = append(opts, google.WithHeaders(headers))
	}
	return google.New(opts...)
}

func (c *coordinator) buildGoogleVertexProvider(headers map[string]string, options map[string]string) (fantasy.Provider, error) {
	opts := []google.Option{}
	if c.cfg.Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, google.WithHTTPClient(httpClient))
	}
	if len(headers) > 0 {
		opts = append(opts, google.WithHeaders(headers))
	}

	project := options["project"]
	location := options["location"]

	opts = append(opts, google.WithVertex(project, location))

	return google.New(opts...)
}

func (c *coordinator) buildHyperProvider(baseURL, apiKey string) (fantasy.Provider, error) {
	opts := []hyper.Option{
		hyper.WithBaseURL(baseURL),
		hyper.WithAPIKey(apiKey),
	}
	opts = append(opts, hyper.WithHTTPClient(c.httpClient()))
	return hyper.New(opts...)
}

func (c *coordinator) httpClient() *http.Client {
	return log.NewHTTPClientWithTimeouts(c.cfg.Options.Debug)
}

func (c *coordinator) isAnthropicThinking(model config.SelectedModel) bool {
	if model.Think {
		return true
	}

	if model.ProviderOptions == nil {
		return false
	}

	opts, err := anthropic.ParseOptions(model.ProviderOptions)
	if err != nil {
		return false
	}
	if opts.Thinking != nil {
		return true
	}
	return false
}

func (c *coordinator) isGeminiThinking(model config.SelectedModel) bool {
	if model.Think {
		return true
	}

	if model.ProviderOptions == nil {
		return false
	}

	// Check for thinkingConfig in provider options
	if _, ok := model.ProviderOptions["thinking_config"]; ok {
		return true
	}
	if _, ok := model.ProviderOptions["thinkingConfig"]; ok {
		return true
	}

	return false
}

func (c *coordinator) buildProvider(providerCfg config.ProviderConfig, model config.SelectedModel, isSubAgent bool) (fantasy.Provider, error) {
	headers := maps.Clone(providerCfg.ExtraHeaders)
	if headers == nil {
		headers = make(map[string]string)
	}

	// handle special headers for anthropic
	if providerCfg.Type == anthropic.Name && c.isAnthropicThinking(model) {
		if v, ok := headers["anthropic-beta"]; ok {
			headers["anthropic-beta"] = v + ",interleaved-thinking-2025-05-14"
		} else {
			headers["anthropic-beta"] = "interleaved-thinking-2025-05-14"
		}
	}

	apiKey, _ := c.cfg.Resolve(providerCfg.APIKey)
	baseURL, _ := c.cfg.Resolve(providerCfg.BaseURL)

	switch providerCfg.Type {
	case openai.Name:
		return c.buildOpenaiProvider(baseURL, apiKey, headers)
	case anthropic.Name:
		return c.buildAnthropicProvider(baseURL, apiKey, headers, providerCfg.ID)
	case openrouter.Name:
		return c.buildOpenrouterProvider(baseURL, apiKey, headers)
	case vercel.Name:
		return c.buildVercelProvider(baseURL, apiKey, headers)
	case azure.Name:
		return c.buildAzureProvider(baseURL, apiKey, headers, providerCfg.ExtraParams)
	case bedrock.Name:
		return c.buildBedrockProvider(headers)
	case google.Name, "gemini":
		return c.buildGoogleProvider(baseURL, apiKey, headers)
	case "google-vertex":
		return c.buildGoogleVertexProvider(headers, providerCfg.ExtraParams)
	case openaicompat.Name:
		if providerCfg.ID == string(catwalk.InferenceProviderZAI) {
			if providerCfg.ExtraBody == nil {
				providerCfg.ExtraBody = map[string]any{}
			}
			providerCfg.ExtraBody["tool_stream"] = true
		}
		return c.buildOpenaiCompatProvider(baseURL, apiKey, headers, providerCfg.ExtraBody, providerCfg.ID, isSubAgent)
	case hyper.Name:
		return c.buildHyperProvider(baseURL, apiKey)
	default:
		return nil, fmt.Errorf("provider type not supported: %q", providerCfg.Type)
	}
}

func isExactoSupported(modelID string) bool {
	supportedModels := []string{
		"moonshotai/kimi-k2-0905",
		"deepseek/deepseek-v3.1-terminus",
		"z-ai/glm-4.6",
		"openai/gpt-oss-120b",
		"qwen/qwen3-coder",
	}
	return slices.Contains(supportedModels, modelID)
}

func (c *coordinator) Cancel(sessionID string) {
	c.currentAgent.Cancel(sessionID)
}

func (c *coordinator) CancelAll() {
	c.currentAgent.CancelAll()
}

func (c *coordinator) ClearQueue(sessionID string) {
	c.currentAgent.ClearQueue(sessionID)
}

func (c *coordinator) IsBusy() bool {
	return c.currentAgent.IsBusy()
}

func (c *coordinator) IsSessionBusy(sessionID string) bool {
	return c.currentAgent.IsSessionBusy(sessionID)
}

func (c *coordinator) Model() Model {
	return c.currentAgent.Model()
}

func (c *coordinator) UpdateModels(ctx context.Context) error {
	// build the models again so we make sure we get the latest config
	large, small, err := c.buildAgentModels(ctx, false)
	if err != nil {
		return err
	}
	c.currentAgent.SetModels(large, small)

	agentCfg, ok := c.cfg.Agents[config.AgentCoder]
	if !ok {
		return errors.New("coder agent not configured")
	}

	toolsResult, err := c.buildTools(ctx, agentCfg)
	if err != nil {
		return err
	}

	// Apply plan mode tool filtering (Codex plan mode architecture)
	// In plan mode, editing and execution tools are FORBIDDEN
	if c.currentAgent != nil {
		sessionID := c.currentAgent.SessionID()
		if sessionID != "" {
			filteredTools, err := tools.PlanModeToolFilter(ctx, c.sessions, sessionID, toolsResult)
			if err != nil {
				slog.Warn("Failed to apply plan mode filter", "error", err)
			} else {
				toolsResult = filteredTools
			}
		}
	}

	c.currentAgent.SetTools(toolsResult)
	c.setToolCache(toolsResult)
	if err := c.refreshSystemPrompt(ctx); err != nil {
		return err
	}
	return nil
}

func (c *coordinator) QueuedPrompts(sessionID string) int {
	return c.currentAgent.QueuedPrompts(sessionID)
}

func (c *coordinator) QueuedPromptsList(sessionID string) []string {
	return c.currentAgent.QueuedPromptsList(sessionID)
}

func (c *coordinator) Summarize(ctx context.Context, sessionID string) error {
	providerCfg, ok := c.cfg.Providers.Get(c.currentAgent.Model().ModelCfg.Provider)
	if !ok {
		return errors.New("model provider not configured")
	}
	return c.currentAgent.Summarize(ctx, sessionID, c.getProviderOptions(c.currentAgent.Model(), providerCfg, c.cfg.Options.GoogleGrounding))
}

func (c *coordinator) isUnauthorized(err error) bool {
	var providerErr *fantasy.ProviderError
	return errors.As(err, &providerErr) && providerErr.StatusCode == http.StatusUnauthorized
}

func (c *coordinator) refreshOAuth2Token(ctx context.Context, providerCfg config.ProviderConfig) error {
	if err := c.cfg.RefreshOAuthToken(ctx, providerCfg.ID); err != nil {
		slog.Error("Failed to refresh OAuth token after 401 error", "provider", providerCfg.ID, "error", err)
		return err
	}
	if err := c.UpdateModels(ctx); err != nil {
		return err
	}
	return nil
}

func (c *coordinator) refreshApiKeyTemplate(ctx context.Context, providerCfg config.ProviderConfig) error {
	newAPIKey, err := c.cfg.Resolve(providerCfg.APIKeyTemplate)
	if err != nil {
		slog.Error("Failed to re-resolve API key after 401 error", "provider", providerCfg.ID, "error", err)
		return err
	}

	providerCfg.APIKey = newAPIKey
	c.cfg.Providers.Set(providerCfg.ID, providerCfg)

	if err := c.UpdateModels(ctx); err != nil {
		return err
	}
	return nil
}

// subAgentParams holds the parameters for running a sub-agent.
type subAgentParams struct {
	Agent          SessionAgent
	SessionID      string
	AgentMessageID string
	ToolCallID     string
	Prompt         string
	SessionTitle   string
	// SessionSetup is an optional callback invoked after session creation
	// but before agent execution, for custom session configuration.
	SessionSetup func(sessionID string)
	AllowNesting bool
}

// runSubAgent runs a sub-agent and handles session management and cost accumulation.
// It creates a sub-session, runs the agent with the given prompt, and propagates
// the cost to the parent session.
func (c *coordinator) runSubAgent(ctx context.Context, params subAgentParams) (fantasy.ToolResponse, error) {
	if !params.AllowNesting {
		parentSession, err := c.sessions.Get(ctx, params.SessionID)
		if err != nil {
			return fantasy.ToolResponse{}, err
		}
		if parentSession.ParentSessionID != "" {
			return fantasy.NewTextErrorResponse("sub-agents cannot spawn sub-agents"), nil
		}
	}

	// Create sub-session
	agentToolSessionID := c.sessions.CreateAgentToolSessionID(params.AgentMessageID, params.ToolCallID)
	createCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	session, err := c.sessions.CreateTaskSession(createCtx, agentToolSessionID, params.SessionID, params.SessionTitle)
	cancel()
	if err != nil {
		return fantasy.ToolResponse{}, fmt.Errorf("create session: %w", err)
	}

	// Call session setup function if provided
	if params.SessionSetup != nil {
		params.SessionSetup(session.ID)
	}

	// Get model configuration
	model := params.Agent.Model()
	maxTokens := model.CatwalkCfg.DefaultMaxTokens
	if model.ModelCfg.MaxTokens != 0 {
		maxTokens = model.ModelCfg.MaxTokens
	}

	providerCfg, ok := c.cfg.Providers.Get(model.ModelCfg.Provider)
	if !ok {
		return fantasy.ToolResponse{}, errors.New("model provider not configured")
	}

	// Run the agent
	result, err := params.Agent.Run(ctx, SessionAgentCall{
		SessionID:        session.ID,
		Prompt:           params.Prompt,
		MaxOutputTokens:  maxTokens,
		ProviderOptions:  c.getProviderOptions(model, providerCfg, c.cfg.Options.GoogleGrounding),
		Temperature:      model.ModelCfg.Temperature,
		TopP:             model.ModelCfg.TopP,
		TopK:             model.ModelCfg.TopK,
		FrequencyPenalty: model.ModelCfg.FrequencyPenalty,
		PresencePenalty:  model.ModelCfg.PresencePenalty,
	})
	if err != nil {
		return fantasy.NewTextErrorResponse("error generating response"), nil
	}

	// Update parent session cost
	if err := c.updateParentSessionCost(ctx, session.ID, params.SessionID); err != nil {
		return fantasy.ToolResponse{}, err
	}

	return fantasy.NewTextResponse(result.Response.Content.Text()), nil
}

// updateParentSessionCost accumulates the cost from a child session to its parent session.
func (c *coordinator) updateParentSessionCost(ctx context.Context, childSessionID, parentSessionID string) error {
	getChildCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	childSession, err := c.sessions.Get(getChildCtx, childSessionID)
	cancel()
	if err != nil {
		return fmt.Errorf("get child session: %w", err)
	}

	getParentCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	parentSession, err := c.sessions.Get(getParentCtx, parentSessionID)
	cancel()
	if err != nil {
		return fmt.Errorf("get parent session: %w", err)
	}

	parentSession.Cost += childSession.Cost

	saveCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	_, err = c.sessions.Save(saveCtx, parentSession)
	cancel()
	if err != nil {
		return fmt.Errorf("save parent session: %w", err)
	}

	return nil
}
