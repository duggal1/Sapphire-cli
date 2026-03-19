package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	promptpkg "github.com/charmbracelet/sapphire/internal/agent/prompt"
	"github.com/charmbracelet/sapphire/internal/agent/tools"
	"github.com/charmbracelet/sapphire/internal/config"
	"github.com/charmbracelet/sapphire/internal/message"
	"github.com/charmbracelet/sapphire/internal/pubsub"
	"github.com/charmbracelet/sapphire/internal/session"
)

type subAgentStatus string

const (
	subAgentStatusQueued    subAgentStatus = "queued"
	subAgentStatusRunning   subAgentStatus = "running"
	subAgentStatusCompleted subAgentStatus = "completed"
	subAgentStatusError     subAgentStatus = "error"
	subAgentStatusClosed    subAgentStatus = "closed"

	maxForkedContextMessages     = 40
	subAgentSessionCreateTimeout = 10 * time.Second
	subAgentSessionLoadTimeout   = 10 * time.Second
	subAgentMessageListTimeout   = 5 * time.Second
	subAgentForkContextTimeout   = 10 * time.Second
)

type subAgentSubmission struct {
	ID        string
	Prompt    string
	Status    subAgentStatus
	Result    string
	Err       string
	StartedAt time.Time
	EndedAt   time.Time
}

type subAgentInput struct {
	submissionID string
	prompt       string
	items        []string
}

type subAgentRunner struct {
	id                   string
	sessionID            string
	parentSession        string
	workDir              string
	cleanup              func()
	agent                SessionAgent
	status               subAgentStatus
	lastResult           string
	lastError            string
	lastProgress         string
	lastSubmission       string
	validationPassed     bool
	validationErrors     string
	validationHasChanges bool
	submissions          map[string]*subAgentSubmission
	inputCh              chan subAgentInput
	closed               bool
	pending              int
	cancel               context.CancelFunc
	assignment           subAgentAssignment
	statusBroker         *pubsub.Broker[subAgentStatus]
	mu                   sync.Mutex
}

type subAgentRegistry struct {
	mu     sync.Mutex
	agents map[string]*subAgentRunner
}

func newSubAgentRegistry() *subAgentRegistry {
	return &subAgentRegistry{agents: make(map[string]*subAgentRunner)}
}

func (r *subAgentRegistry) upsert(agentID string, runner *subAgentRunner) {
	if r == nil || agentID == "" || runner == nil {
		return
	}
	r.mu.Lock()
	if r.agents == nil {
		r.agents = make(map[string]*subAgentRunner)
	}
	r.agents[agentID] = runner
	r.mu.Unlock()
}

func (r *subAgentRegistry) get(agentID string) *subAgentRunner {
	if r == nil || agentID == "" {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.agents[agentID]
}

func (r *subAgentRegistry) delete(agentID string) {
	if r == nil || agentID == "" {
		return
	}
	r.mu.Lock()
	delete(r.agents, agentID)
	r.mu.Unlock()
}

func (r *subAgentRegistry) list() []*subAgentRunner {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	runners := make([]*subAgentRunner, 0, len(r.agents))
	for _, runner := range r.agents {
		runners = append(runners, runner)
	}
	return runners
}

func (c *coordinator) ensureSubAgentRegistry() *subAgentRegistry {
	if c.subAgentRegistry != nil {
		return c.subAgentRegistry
	}
	c.subAgentsMu.Lock()
	defer c.subAgentsMu.Unlock()
	if c.subAgentRegistry == nil {
		c.subAgentRegistry = newSubAgentRegistry()
		for agentID, runner := range c.subAgents {
			c.subAgentRegistry.agents[agentID] = runner
		}
	}
	return c.subAgentRegistry
}

func (r *subAgentRunner) snapshot() subAgentSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	return subAgentSnapshot{
		ID:                   r.id,
		Status:               r.status,
		LastResult:           r.lastResult,
		LastError:            r.lastError,
		LastProgress:         r.lastProgress,
		LastSubmission:       r.lastSubmission,
		Pending:              r.pending,
		WorkDir:              r.workDir,
		Branch:               r.assignment.Branch,
		WriteManifest:        append([]string{}, r.assignment.WriteManifest...),
		DefinitionOfDone:     r.assignment.DefinitionOfDone,
		Task:                 r.assignment.Task,
		TaskKey:              r.assignment.TaskKey,
		Domains:              r.assignment.Domains,
		StartedAt:            r.assignment.CreatedAt,
		UpdatedAt:            r.assignment.UpdatedAt,
		ValidationPassed:     r.validationPassed,
		ValidationErrors:     r.validationErrors,
		ValidationHasChanges: r.validationHasChanges,
	}
}

func (r *subAgentRunner) enqueue(prompt string, items []string) string {
	submissionID := uuid.New().String()
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return ""
	}
	r.submissions[submissionID] = &subAgentSubmission{
		ID:     submissionID,
		Prompt: prompt,
		Status: subAgentStatusQueued,
	}
	r.pending++
	r.status = subAgentStatusQueued
	r.lastResult = ""
	r.lastError = ""
	r.lastProgress = ""
	r.assignment.UpdatedAt = time.Now()
	broker := r.statusBroker
	r.mu.Unlock()
	publishSubAgentStatus(broker, subAgentStatusQueued)
	if !r.sendInput(subAgentInput{submissionID: submissionID, prompt: prompt, items: append([]string{}, items...)}) {
		r.mu.Lock()
		delete(r.submissions, submissionID)
		if r.pending > 0 {
			r.pending--
		}
		broker = nil
		if r.pending == 0 && r.status == subAgentStatusQueued {
			r.status = subAgentStatusCompleted
			broker = r.statusBroker
		}
		r.mu.Unlock()
		publishSubAgentStatus(broker, subAgentStatusCompleted)
		return ""
	}
	return submissionID
}

func (r *subAgentRunner) sendInput(input subAgentInput) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	r.inputCh <- input
	return true
}

func (r *subAgentRunner) interrupt() {
	r.mu.Lock()
	cancel := r.cancel
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (r *subAgentRunner) close() {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	r.closed = true
	cancel := r.cancel
	r.status = subAgentStatusClosed
	now := time.Now()
	for _, submission := range r.submissions {
		if submission == nil || isSubAgentFinalStatus(submission.Status) {
			continue
		}
		submission.Status = subAgentStatusClosed
		submission.Err = "agent closed"
		submission.EndedAt = now
	}
	broker := r.statusBroker
	r.mu.Unlock()
	publishSubAgentStatus(broker, subAgentStatusClosed)
	if cancel != nil {
		cancel()
	}
	close(r.inputCh)
	if broker != nil {
		broker.Shutdown()
	}
	if r.cleanup != nil {
		r.cleanup()
	}
}

type subAgentSnapshot struct {
	ID                   string         `json:"id"`
	Status               subAgentStatus `json:"status"`
	LastResult           string         `json:"last_result,omitempty"`
	LastError            string         `json:"last_error,omitempty"`
	LastProgress         string         `json:"last_progress,omitempty"`
	LastSubmission       string         `json:"last_submission,omitempty"`
	Pending              int            `json:"pending"`
	WorkDir              string         `json:"work_dir,omitempty"`
	Branch               string         `json:"branch,omitempty"`
	WriteManifest        []string       `json:"write_manifest,omitempty"`
	DefinitionOfDone     string         `json:"definition_of_done,omitempty"`
	Task                 string         `json:"task,omitempty"`
	TaskKey              string         `json:"task_key,omitempty"`
	Domains              []string       `json:"domains,omitempty"`
	StartedAt            time.Time      `json:"started_at,omitempty"`
	UpdatedAt            time.Time      `json:"updated_at,omitempty"`
	ValidationPassed     bool           `json:"validation_passed,omitempty"`
	ValidationErrors     string         `json:"validation_errors,omitempty"`
	ValidationHasChanges bool           `json:"validation_has_changes,omitempty"`
}

type subAgentStatusEntry struct {
	ID     string         `json:"id"`
	Status subAgentStatus `json:"status"`
}

type subAgentCollectedResult struct {
	ID                   string         `json:"id"`
	SubmissionID         string         `json:"submission_id,omitempty"`
	Status               subAgentStatus `json:"status"`
	Result               string         `json:"result,omitempty"`
	Error                string         `json:"error,omitempty"`
	Progress             string         `json:"progress,omitempty"`
	WorkDir              string         `json:"work_dir,omitempty"`
	Branch               string         `json:"branch,omitempty"`
	ValidationPassed     bool           `json:"validation_passed,omitempty"`
	ValidationErrors     string         `json:"validation_errors,omitempty"`
	ValidationHasChanges bool           `json:"validation_has_changes,omitempty"`
}

type spawnAgentOptions struct {
	Prompt           string
	PromptItems      []string
	Title            string
	Worktree         bool
	ReuseWorktree    bool
	AllowReuse       bool
	WorktreePath     string
	Branch           string
	WriteManifest    []string
	DefinitionOfDone string
	TestCommand      string
	AgentID          string
	Model            string
	ReasoningEffort  string
	ForkContext      bool
}

func (c *coordinator) spawnSubAgent(ctx context.Context, parentSessionID string, opts spawnAgentOptions) (string, string, error) {
	if opts.Prompt == "" && len(opts.PromptItems) == 0 {
		return "", "", errors.New("prompt is required")
	}
	promptText := opts.Prompt
	if promptText == "" && len(opts.PromptItems) > 0 {
		promptText = strings.Join(opts.PromptItems, "\n")
	}
	if parentSessionID != "" {
		if _, err := c.validateSubAgentLaunch(ctx, parentSessionID, promptText); err != nil {
			return "", "", err
		}
	}
	decision := evaluateSubAgentLaunch(promptText)
	assignmentID := fmt.Sprintf("subagent-%d", time.Now().UnixNano())

	agentID := "agent-" + uuid.New().String()
	workDir := c.cfg.WorkingDir()
	cleanup := func() {}
	branch := strings.TrimSpace(opts.Branch)

	if opts.Worktree {
		wtDir, wtBranch, wtCleanup, err := c.prepareSubAgentWorktree(ctx, parentSessionID, agentID, subAgentWorktreeSpec{
			WorktreePath: opts.WorktreePath,
			Branch:       branch,
			Reuse:        opts.ReuseWorktree,
			AllowReuse:   opts.AllowReuse,
			TaskKey:      decision.TaskKey,
			AssignmentID: assignmentID,
		})
		if err != nil {
			return "", "", err
		}
		workDir = wtDir
		branch = wtBranch
		cleanup = wtCleanup

	}

	writeManifest := opts.WriteManifest
	if writeManifest == nil {
		writeManifest = []string{}
	}
	normalizedManifest := normalizeWriteManifest(c.cfg.WorkingDir(), workDir, writeManifest)
	assignment, assignmentPrompt := buildSubAgentAssignment(assignmentID, parentSessionID, opts.Title, promptText, workDir, decision, normalizedManifest, branch, opts.DefinitionOfDone, opts.TestCommand, c.GetLongHorizonState(parentSessionID))

	if opts.Worktree {
		// Write TASK.md into the worktree
		if err := writeSubAgentTaskContext(workDir, assignment); err != nil {
			slog.Warn("Failed to write sub-agent task context", "workdir", workDir, "error", err)
		}
	}

	agentKey := config.AgentCoder
	if opts.AgentID != "" {
		agentKey = opts.AgentID
	}
	agentCfg, ok := c.cfg.Agents[agentKey]
	if !ok {
		cleanup()
		return "", "", fmt.Errorf("agent %q not configured", agentKey)
	}

	writeScope := tools.NewWriteScope(workDir, normalizedManifest)

	promptTemplate, err := coderPrompt(promptpkg.WithWorkingDir(workDir))
	if err != nil {
		cleanup()
		return "", "", err
	}

	override, err := c.resolveSubAgentModelOverride(opts.Model, opts.ReasoningEffort)
	if err != nil {
		cleanup()
		return "", "", err
	}
	agent, err := c.buildAgentWithWorkingDirOverrides(ctx, promptTemplate, agentCfg, true, workDir, override, writeScope)
	if err != nil {
		cleanup()
		return "", "", err
	}

	title := opts.Title
	if title == "" {
		title = "Sub-Agent Session"
	}

	createCtx, cancel := context.WithTimeout(ctx, subAgentSessionCreateTimeout)
	session, err := c.sessions.CreateTaskSession(createCtx, agentID, parentSessionID, title)
	cancel()
	if err != nil {
		cleanup()
		return "", "", fmt.Errorf("create sub-agent session: %w", err)
	}
	if err := c.recordSubAgentMetadata(ctx, session.ID, subAgentMetadata{
		AssignmentID:     assignment.ID,
		WorktreePath:     workDir,
		Branch:           branch,
		WriteManifest:    normalizedManifest,
		DefinitionOfDone: opts.DefinitionOfDone,
		TestCommand:      opts.TestCommand,
	}); err != nil {
		cleanup()
		return "", "", err
	}
	if opts.ForkContext && parentSessionID != "" {
		if err := c.forkSubAgentContext(ctx, parentSessionID, session.ID); err != nil {
			cleanup()
			return "", "", fmt.Errorf("fork context: %w", err)
		}
	}

	runner := &subAgentRunner{
		id:            agentID,
		sessionID:     session.ID,
		parentSession: parentSessionID,
		workDir:       workDir,
		cleanup:       cleanup,
		agent:         agent,
		status:        subAgentStatusQueued,
		submissions:   make(map[string]*subAgentSubmission),
		inputCh:       make(chan subAgentInput, 16),
		assignment:    assignment,
		statusBroker:  pubsub.NewBroker[subAgentStatus](),
	}

	c.ensureSubAgentRegistry().upsert(agentID, runner)

	c.publishSubAgentEvent(SubAgentSpawnedEvent, runner, "", SubAgentStageSpawned, "")

	go c.runSubAgentLoop(runner)

	submissionID := runner.enqueue(assignmentPrompt, opts.PromptItems)
	if submissionID != "" {
		c.publishSubAgentEvent(SubAgentWaitingEvent, runner, submissionID, SubAgentStageWaiting, "")
		c.startSubAgentCompletionWatcher(runner, submissionID)
	}
	return agentID, submissionID, nil
}

func (c *coordinator) runSubAgentLoop(runner *subAgentRunner) {
	for input := range runner.inputCh {
		runner.mu.Lock()
		if runner.closed {
			runner.mu.Unlock()
			continue
		}
		submission := runner.submissions[input.submissionID]
		if submission == nil {
			submission = &subAgentSubmission{ID: input.submissionID, Prompt: input.prompt}
			runner.submissions[input.submissionID] = submission
		}
		if submission.Status == subAgentStatusRunning || submission.Status == subAgentStatusCompleted || submission.Status == subAgentStatusError {
			runner.mu.Unlock()
			continue
		}
		submission.Status = subAgentStatusRunning
		submission.StartedAt = time.Now()
		runner.status = subAgentStatusRunning
		if runner.pending > 0 {
			runner.pending--
		}
		runner.lastSubmission = input.submissionID
		runner.lastError = ""
		runner.lastProgress = ""
		runner.assignment.UpdatedAt = time.Now()
		broker := runner.statusBroker
		runner.mu.Unlock()
		publishSubAgentStatus(broker, subAgentStatusRunning)

		c.publishSubAgentEvent(SubAgentRunningEvent, runner, submission.ID, SubAgentStageRunning, "")

		runCtx, cancel := context.WithCancel(context.Background())
		runner.mu.Lock()
		runner.cancel = cancel
		runner.mu.Unlock()

		result, err := c.runSubAgentTurn(runCtx, runner.agent, runner.sessionID, runner.parentSession, input.prompt)
		cancel()

		// Run validation gate on the worktree if it exists
		runner.mu.Lock()
		workDir := runner.workDir
		branch := runner.assignment.Branch
		taskSlug := runner.assignment.TaskKey
		testCommand := runner.assignment.TestCommand
		taskTitle := runner.assignment.Title
		runner.mu.Unlock()
		var validationReport string
		if workDir != "" && isSubAgentWorktree(workDir) {
			vCtx, vCancel := context.WithTimeout(context.Background(), validationBuildTimeout+validationTestTimeout+validationLintTimeout+validationSecurityTimeout+validationGateTimeout)
			vResult := validateWorktreeResult(vCtx, workDir, branch, testCommand)
			vCancel()
			validationReport = formatValidationReport(vResult)
			runner.mu.Lock()
			runner.validationPassed = vResult.Passed
			runner.validationErrors = vResult.Errors
			runner.validationHasChanges = vResult.HasChanges
			runner.mu.Unlock()

			// If validation failed and there was no run error, note it
			if !vResult.Passed && err == nil {
				result += validationReport
			}

			// Zero-change auto-delete: worktrees with no changes are dead weight
			if !vResult.HasChanges {
				slog.Info("Worktree has zero changes, auto-deleting immediately", "workdir", workDir)
				if root := c.cfg.WorkingDir(); root != "" {
					_ = removeWorktree(root, workDir)
				}
			}

			// Auto-commit on success with changes
			if err == nil && vResult.Passed && vResult.HasChanges {
				commitCtx, commitCancel := context.WithTimeout(context.Background(), validationGateTimeout)
				commitErr := autoCommitWorktree(commitCtx, workDir, defaultSubAgentCommitMessage(taskTitle, taskSlug))
				commitCancel()
				if commitErr != nil {
					err = fmt.Errorf("auto-commit failed: %w", commitErr)
				}
			}

			// Failed-test archival: archive to review/ branch namespace, then quarantine
			if !vResult.Passed && vResult.HasChanges {
				if root := c.cfg.WorkingDir(); root != "" {
					archiveCtx, archiveCancel := context.WithTimeout(context.Background(), validationGateTimeout)
					archiveWorktreeToReviewBranch(archiveCtx, workDir, branch, taskSlug)
					archiveCancel()
					_ = c.quarantineWorktree(root, workDir, taskSlug)
				}
			}

			// Crash preservation: on error with changes, keep worktree alive as crash dump
			if err != nil && vResult.HasChanges {
				slog.Warn("Agent errored — preserving worktree as crash dump", "workdir", workDir, "error", err)
				runner.mu.Lock()
				runner.cleanup = func() {} // noop: never auto-clean on crash
				runner.mu.Unlock()
			}
		}

		eventType := SubAgentCompletedEvent
		stage := SubAgentStageCompleted
		errMsg := ""
		var payload SubAgentLifecycleEvent
		runner.mu.Lock()
		runner.cancel = nil
		submission.EndedAt = time.Now()
		if err != nil {
			submission.Status = subAgentStatusError
			submission.Err = err.Error()
			runner.status = subAgentStatusError
			runner.lastError = err.Error()
			runner.lastProgress = ""
			eventType = SubAgentFailedEvent
			stage = SubAgentStageFailed
			errMsg = err.Error()
		} else {
			submission.Status = subAgentStatusCompleted
			// Append validation report to result
			finalResult := result
			if validationReport != "" && !strings.Contains(result, "VALIDATION GATE") {
				finalResult += validationReport
			}
			submission.Result = finalResult
			runner.status = subAgentStatusCompleted
			report := parseSubAgentReport(finalResult)
			if report.Summary != "" {
				runner.lastResult = report.Summary
			} else {
				runner.lastResult = finalResult
			}
			if report.Progress != "" {
				runner.lastProgress = report.Progress
			}
			runner.lastError = ""

			// Trigger async memory extraction on successful completion
			if c.memoryPipe != nil {
				c.memoryPipe.TriggerPostCompletion(runner.sessionID, finalResult)
			}
		}
		runner.assignment.UpdatedAt = time.Now()
		broker = runner.statusBroker
		payload = runner.lifecycleEventLocked(submission.ID, stage, errMsg)
		runner.mu.Unlock()
		publishSubAgentStatus(broker, payload.Status)
		publishSubAgentLifecycleEvent(eventType, payload)
	}
}

func (c *coordinator) runSubAgentTurn(ctx context.Context, agent SessionAgent, sessionID, parentSessionID, prompt string) (string, error) {
	model := agent.Model()
	maxTokens := model.CatwalkCfg.DefaultMaxTokens
	if model.ModelCfg.MaxTokens != 0 {
		maxTokens = model.ModelCfg.MaxTokens
	}

	providerCfg, ok := c.cfg.Providers.Get(model.ModelCfg.Provider)
	if !ok {
		return "", errors.New("model provider not configured")
	}

	result, err := agent.Run(ctx, SessionAgentCall{
		SessionID:        sessionID,
		Prompt:           prompt,
		MaxOutputTokens:  maxTokens,
		ProviderOptions:  c.getProviderOptions(model, providerCfg, c.cfg.Options.GoogleGrounding),
		Temperature:      model.ModelCfg.Temperature,
		TopP:             model.ModelCfg.TopP,
		TopK:             model.ModelCfg.TopK,
		FrequencyPenalty: model.ModelCfg.FrequencyPenalty,
		PresencePenalty:  model.ModelCfg.PresencePenalty,
	})
	if err != nil {
		return "", err
	}

	if err := c.updateParentSessionCost(ctx, sessionID, parentSessionID); err != nil {
		return "", err
	}

	return result.Response.Content.Text(), nil
}

func (c *coordinator) resumeSubAgent(ctx context.Context, parentSessionID, agentID, message string) (string, subAgentStatus, error) {
	if agentID == "" {
		return "", subAgentStatusError, errors.New("agent id is required")
	}

	runner := c.ensureSubAgentRegistry().get(agentID)
	if runner != nil {
		runner.mu.Lock()
		status := runner.status
		runner.mu.Unlock()
		if message == "" {
			return "", status, nil
		}
		submissionID, err := c.sendSubAgentInput(ctx, agentID, message, nil, false)
		if err != nil {
			return "", status, err
		}
		return submissionID, runner.snapshot().Status, nil
	}

	loadCtx, cancel := context.WithTimeout(ctx, subAgentSessionLoadTimeout)
	sess, err := c.sessions.Get(loadCtx, agentID)
	cancel()
	if err != nil {
		return "", subAgentStatusError, fmt.Errorf("resume sub-agent: %w", err)
	}
	effectiveParent := parentSessionID
	if effectiveParent == "" {
		effectiveParent = sess.ParentSessionID
	}
	if err := c.validateSubAgentResume(ctx, effectiveParent, sess); err != nil {
		return "", subAgentStatusError, err
	}

	workDir := c.cfg.WorkingDir()
	cleanup := func() {}
	meta, _ := c.loadSubAgentMetadata(ctx, sess.ID)
	branch := strings.TrimSpace(meta.Branch)
	assignmentID := strings.TrimSpace(meta.AssignmentID)
	if assignmentID == "" {
		assignmentID = fmt.Sprintf("subagent-resume-%d", time.Now().UnixNano())
	}
	if meta.WorktreePath != "" {
		wtDir, wtBranch, wtCleanup, err := c.prepareSubAgentWorktree(ctx, effectiveParent, agentID, subAgentWorktreeSpec{
			WorktreePath: meta.WorktreePath,
			Branch:       branch,
			Reuse:        true,
			AllowReuse:   true,
			TaskKey:      "",
			AssignmentID: assignmentID,
		})
		if err != nil {
			slog.Warn("Failed to restore sub-agent worktree; falling back to repo root", "error", err)
		} else {
			workDir = wtDir
			branch = wtBranch
			cleanup = wtCleanup
		}
	}

	agentCfg, ok := c.cfg.Agents[config.AgentCoder]
	if !ok {
		cleanup()
		return "", subAgentStatusError, fmt.Errorf("agent %q not configured", config.AgentCoder)
	}

	promptTemplate, err := coderPrompt(promptpkg.WithWorkingDir(workDir))
	if err != nil {
		cleanup()
		return "", subAgentStatusError, err
	}

	normalizedManifest := normalizeWriteManifest(c.cfg.WorkingDir(), workDir, meta.WriteManifest)
	writeScope := tools.NewWriteScope(workDir, normalizedManifest)
	agent, err := c.buildAgentWithWorkingDirOverrides(ctx, promptTemplate, agentCfg, true, workDir, nil, writeScope)
	if err != nil {
		cleanup()
		return "", subAgentStatusError, err
	}

	task := strings.TrimSpace(message)
	if task == "" {
		task = c.resumeSubAgentTask(ctx, sess)
	}
	decision := evaluateSubAgentLaunch(task)
	assignment, _ := buildSubAgentAssignment(assignmentID, effectiveParent, sess.Title, task, workDir, decision, normalizedManifest, branch, meta.DefinitionOfDone, meta.TestCommand, c.GetLongHorizonState(effectiveParent))

	runner = &subAgentRunner{
		id:            agentID,
		sessionID:     sess.ID,
		parentSession: effectiveParent,
		workDir:       workDir,
		cleanup:       cleanup,
		agent:         agent,
		status:        subAgentStatusCompleted,
		submissions:   make(map[string]*subAgentSubmission),
		inputCh:       make(chan subAgentInput, 16),
		assignment:    assignment,
		statusBroker:  pubsub.NewBroker[subAgentStatus](),
	}

	c.ensureSubAgentRegistry().upsert(agentID, runner)

	c.publishSubAgentEvent(SubAgentSpawnedEvent, runner, "", SubAgentStageSpawned, "")

	go c.runSubAgentLoop(runner)

	if message == "" {
		return "", runner.status, nil
	}
	submissionID, err := c.sendSubAgentInput(ctx, agentID, message, nil, false)
	if err != nil {
		return "", runner.status, err
	}
	return submissionID, runner.snapshot().Status, nil
}

func (c *coordinator) sendSubAgentInput(ctx context.Context, agentID, prompt string, items []string, interrupt bool) (string, error) {
	runner, err := c.getSubAgent(agentID)
	if err != nil {
		return "", err
	}
	if interrupt {
		runner.interrupt()
	}
	runner.mu.Lock()
	assignment := runner.assignment
	runner.mu.Unlock()
	if assignment.Task != "" {
		prompt = buildSubAgentFollowupPrompt(assignment, prompt, items)
	}
	submissionID := runner.enqueue(prompt, items)
	if submissionID == "" {
		return "", errors.New("agent is closed")
	}
	c.publishSubAgentEvent(SubAgentWaitingEvent, runner, submissionID, SubAgentStageWaiting, "")
	c.startSubAgentCompletionWatcher(runner, submissionID)
	return submissionID, nil
}

func (c *coordinator) waitSubAgents(ctx context.Context, ids []string, timeout time.Duration) ([]subAgentSnapshot, bool) {
	if len(ids) == 0 {
		return nil, false
	}
	waitCtx := ctx
	cancel := func() {}
	if timeout > 0 {
		waitCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	snapshots, _ := c.snapshotSubAgentsByID(ids)
	for _, snap := range snapshots {
		if isSubAgentFinalStatus(snap.Status) {
			return snapshots, false
		}
	}

	completed := make(chan struct{}, len(ids))
	for _, id := range ids {
		runner, err := c.getSubAgent(id)
		if err != nil {
			continue
		}
		initialStatus, updates := runner.subscribeStatus(waitCtx)
		if isSubAgentFinalStatus(initialStatus) {
			return c.snapshotSubAgentsByID(ids)
		}
		go func(ch <-chan pubsub.Event[subAgentStatus], runner *subAgentRunner) {
			for {
				select {
				case <-waitCtx.Done():
					return
				case _, ok := <-ch:
					if !ok {
						runner.mu.Lock()
						status := runner.status
						runner.mu.Unlock()
						if isSubAgentFinalStatus(status) {
							select {
							case completed <- struct{}{}:
							default:
							}
						}
						return
					}
					runner.mu.Lock()
					status := runner.status
					runner.mu.Unlock()
					if !isSubAgentFinalStatus(status) {
						continue
					}
					select {
					case completed <- struct{}{}:
					default:
					}
					return
				}
			}
		}(updates, runner)
	}

	select {
	case <-waitCtx.Done():
		snapshots, _ := c.snapshotSubAgentsByID(ids)
		return snapshots, true
	case <-completed:
		snapshots, _ := c.snapshotSubAgentsByID(ids)
		return snapshots, false
	}
}

func (c *coordinator) snapshotSubAgentsByID(ids []string) ([]subAgentSnapshot, bool) {
	snapshots := make([]subAgentSnapshot, 0, len(ids))
	allFinal := true
	for _, id := range ids {
		runner, err := c.getSubAgent(id)
		if err != nil {
			snapshots = append(snapshots, subAgentSnapshot{ID: id, Status: subAgentStatusClosed})
			continue
		}
		snap := runner.snapshot()
		snapshots = append(snapshots, snap)
		if !isSubAgentFinalStatus(snap.Status) {
			allFinal = false
		}
	}
	return snapshots, allFinal
}

func (r *subAgentRunner) latestCollectedResult() subAgentCollectedResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := subAgentCollectedResult{
		ID:                   r.id,
		SubmissionID:         r.lastSubmission,
		Status:               r.status,
		Result:               r.lastResult,
		Error:                r.lastError,
		Progress:             r.lastProgress,
		WorkDir:              r.workDir,
		Branch:               r.assignment.Branch,
		ValidationPassed:     r.validationPassed,
		ValidationErrors:     r.validationErrors,
		ValidationHasChanges: r.validationHasChanges,
	}
	if submission := r.submissions[r.lastSubmission]; submission != nil {
		if submission.Status != "" {
			result.Status = submission.Status
		}
		if submission.Result != "" {
			result.Result = submission.Result
		}
		if submission.Err != "" {
			result.Error = submission.Err
		}
	}
	return result
}

func (c *coordinator) waitSubAgentStatuses(ctx context.Context, ids []string, timeout time.Duration) ([]subAgentStatusEntry, bool) {
	snapshots, timedOut := c.waitSubAgents(ctx, ids, timeout)
	statuses := make([]subAgentStatusEntry, 0, len(snapshots))
	for _, snap := range snapshots {
		statuses = append(statuses, subAgentStatusEntry{
			ID:     snap.ID,
			Status: snap.Status,
		})
	}
	return statuses, timedOut
}

func (c *coordinator) collectSubAgentResults(ids []string) []subAgentCollectedResult {
	results := make([]subAgentCollectedResult, 0, len(ids))
	for _, id := range ids {
		runner, err := c.getSubAgent(id)
		if err != nil {
			results = append(results, subAgentCollectedResult{
				ID:     id,
				Status: subAgentStatusClosed,
				Error:  err.Error(),
			})
			continue
		}
		results = append(results, runner.latestCollectedResult())
	}
	return results
}

func (c *coordinator) closeSubAgent(agentID string) error {
	runner, err := c.getSubAgent(agentID)
	if err != nil {
		return err
	}

	// Quarantine logic: if failed/closed with changes, preserve.
	runner.mu.Lock()
	workDir := runner.workDir
	taskSlug := runner.assignment.TaskKey
	status := runner.status
	runner.mu.Unlock()

	if workDir != "" && isSubAgentWorktree(workDir) {
		// If the agent is in a non-success state, consider quarantine
		if status == subAgentStatusError || status == subAgentStatusClosed || status == subAgentStatusQueued {
			if root := c.cfg.WorkingDir(); root != "" {
				// quarantineWorktree helper checks for changes internally;
				// if changes exist, it moves to quarantine and we clear cleanup.
				if qErr := c.quarantineWorktree(root, workDir, taskSlug); qErr == nil {
					runner.mu.Lock()
					runner.cleanup = nil // Prevent runner.close() from deleting it
					runner.mu.Unlock()
				}
			}
		}
	}

	runner.close()
	c.publishSubAgentEvent(SubAgentClosedEvent, runner, "", SubAgentStageClosed, "")
	c.ensureSubAgentRegistry().delete(agentID)
	return nil
}

func (c *coordinator) getSubAgent(agentID string) (*subAgentRunner, error) {
	runner := c.ensureSubAgentRegistry().get(agentID)
	if runner == nil {
		return nil, fmt.Errorf("agent %s not found", agentID)
	}
	return runner, nil
}

func (c *coordinator) resumeSubAgentTask(ctx context.Context, sess session.Session) string {
	listCtx, cancel := context.WithTimeout(ctx, subAgentMessageListTimeout)
	defer cancel()
	messages, err := c.messages.ListUserMessages(listCtx, sess.ID)
	if err == nil && len(messages) > 0 {
		last := messages[len(messages)-1]
		if text := strings.TrimSpace(last.Content().Text); text != "" {
			return text
		}
	}
	if title := strings.TrimSpace(sess.Title); title != "" {
		return title
	}
	return "Resumed sub-agent session"
}

func (c *coordinator) forkSubAgentContext(ctx context.Context, parentSessionID, childSessionID string) error {
	if parentSessionID == "" || childSessionID == "" {
		return nil
	}
	forkCtx, cancel := context.WithTimeout(ctx, subAgentForkContextTimeout)
	defer cancel()

	parentSession, err := c.sessions.Get(forkCtx, parentSessionID)
	if err != nil {
		return err
	}

	_, _ = c.messages.Create(forkCtx, childSessionID, message.CreateMessageParams{
		Role: message.System,
		Parts: []message.ContentPart{
			message.TextContent{Text: "Forked context snapshot from parent session. Tool history and reasoning state are intentionally excluded to keep the fork isolated."},
		},
	})

	var preload []message.Message
	if parentSession.SummaryMessageID != "" {
		if summary, err := c.messages.Get(forkCtx, parentSession.SummaryMessageID); err == nil {
			preload = append(preload, summary)
		}
	}

	parentMessages, err := c.messages.List(forkCtx, parentSessionID)
	if err != nil {
		return err
	}

	filtered := make([]message.Message, 0, len(parentMessages))
	for _, msg := range parentMessages {
		if msg.Role != message.User && msg.Role != message.Assistant {
			continue
		}
		if msg.IsSummaryMessage {
			continue
		}
		if len(extractMessageTextParts(msg)) == 0 {
			continue
		}
		filtered = append(filtered, msg)
	}
	if len(filtered) > maxForkedContextMessages {
		filtered = filtered[len(filtered)-maxForkedContextMessages:]
	}

	for _, msg := range preload {
		if err := c.copyMessageTextParts(forkCtx, childSessionID, msg); err != nil {
			return err
		}
	}
	for _, msg := range filtered {
		if err := c.copyMessageTextParts(forkCtx, childSessionID, msg); err != nil {
			return err
		}
	}
	return nil
}

func (c *coordinator) copyMessageTextParts(ctx context.Context, sessionID string, msg message.Message) error {
	parts := extractMessageTextParts(msg)
	if len(parts) == 0 {
		return nil
	}
	_, err := c.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role:             msg.Role,
		Parts:            parts,
		Model:            msg.Model,
		Provider:         msg.Provider,
		IsSummaryMessage: msg.IsSummaryMessage,
	})
	return err
}

func extractMessageTextParts(msg message.Message) []message.ContentPart {
	if len(msg.Parts) == 0 {
		return nil
	}
	parts := make([]message.ContentPart, 0, len(msg.Parts))
	for _, part := range msg.Parts {
		text, ok := part.(message.TextContent)
		if !ok {
			continue
		}
		if strings.TrimSpace(text.Text) == "" {
			continue
		}
		parts = append(parts, text)
	}
	return parts
}
