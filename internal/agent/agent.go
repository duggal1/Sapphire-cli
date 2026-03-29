// Package agent is the core orchestration layer for Sapphire AI agents.
//
// It provides session-based AI agent functionality for managing
// conversations, tool execution, and message handling. It coordinates
// interactions between language models, messages, sessions, and tools while
// handling features like automatic summarization, queuing, and token
// management.
package agent

import (
	"cmp"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
	"charm.land/fantasy/providers/bedrock"
	"charm.land/fantasy/providers/google"
	"charm.land/fantasy/providers/openai"
	"charm.land/fantasy/providers/openrouter"
	"charm.land/fantasy/providers/vercel"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/exp/charmtone"
	"github.com/duggal1/Sapphire-cli/internal/agent/hyper"
	"github.com/duggal1/Sapphire-cli/internal/agent/longhorizon"
	"github.com/duggal1/Sapphire-cli/internal/agent/memory"
	"github.com/duggal1/Sapphire-cli/internal/agent/planmode"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools/mcp"
	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/duggal1/Sapphire-cli/internal/csync"
	pmem "github.com/duggal1/Sapphire-cli/internal/memory"
	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/duggal1/Sapphire-cli/internal/permission"
	"github.com/duggal1/Sapphire-cli/internal/session"
	"github.com/duggal1/Sapphire-cli/internal/stringext"
)

const (
	defaultSessionName = "Untitled Session"

	// Constants for auto-summarization thresholds
	largeContextWindowThreshold = 200_000
	largeContextWindowBuffer    = 20_000
	smallContextWindowRatio     = 0.2
	smallContextWindowMinBuffer = 3_000

	// Post-compaction injection tuning
	postCompactionInjectionRatio     = 0.2
	postCompactionContextCharsPerTok = 4

	// Message update timeouts to prevent UI stalls on DB contention.
	messageUpdateTimeout      = 750 * time.Millisecond
	messageFinalUpdateTimeout = 5 * time.Second
	messageUpdateMinInterval  = 50 * time.Millisecond

	// Database operation timeouts to avoid stalls.
	dbOpTimeout     = 2 * time.Second
	dbOpLongTimeout = 5 * time.Second

	// Memory injection timeouts (best-effort).
	memoryCallTimeout = 500 * time.Millisecond

	// Stream retry tuning for transient failures.
	maxStreamRetries   = 2
	streamRetryBackoff = 500 * time.Millisecond

	// Codex-style compaction: max tokens for user messages in compacted history.
	// Aligned with Codex's COMPACT_USER_MESSAGE_MAX_TOKENS = 20,000.
	compactUserMessageMaxTokens = 20_000

	// Checklist reconciliation tuning.
	maxTodoReconcileAttempts = 1

	// Structured-mode repair tuning.
	maxStructuredBlockRepairAttempts = 1
)

//go:embed templates/title.md
var titlePrompt []byte

//go:embed templates/summary.md
var summaryPrompt []byte

//go:embed templates/summary_prefix.md
var summaryPrefix []byte

//go:embed templates/python_capabilities.md
var pythonCapabilitiesPrompt []byte

// Used to remove <think> tags from generated titles.
var thinkTagRegex = regexp.MustCompile(`<think>.*?</think>`)

type SessionAgentCall struct {
	SessionID        string
	Prompt           string
	ResumePointID    string
	TodoReconcileTry int
	StructuredTry    int
	SkillContext     string
	ActiveSkills     []string
	ActiveTools      []string
	ProviderOptions  fantasy.ProviderOptions
	Attachments      []message.Attachment
	PrecreatedUser   *message.Message
	SkipUserMessage  bool
	MaxOutputTokens  int64
	Temperature      *float64
	TopP             *float64
	TopK             *int64
	FrequencyPenalty *float64
	PresencePenalty  *float64
}

func buildCompactionContinuationCall(call SessionAgentCall, partialAssistant *message.Message, resumePointID string) SessionAgentCall {
	continuation := call
	continuation.ResumePointID = strings.TrimSpace(resumePointID)
	originalPrompt := strings.TrimSpace(call.Prompt)
	if partialAssistant == nil {
		continuation.Prompt = fmt.Sprintf(
			"The previous turn was compacted because the session got too long. Resume from the durable boot packet and continue the original request without repeating prior work.\n\nOriginal user request:\n%s",
			originalPrompt,
		)
		return continuation
	}

	partialText := strings.TrimSpace(partialAssistant.Content().Text)
	if len(partialText) > 1200 {
		partialText = partialText[len(partialText)-1200:]
	}

	switch {
	case len(partialAssistant.ToolCalls()) > 0:
		continuation.Prompt = fmt.Sprintf(
			"The previous turn was compacted while tools were in flight. Resume from the durable boot packet and continue the original request without repeating completed work.\n\nOriginal user request:\n%s",
			originalPrompt,
		)
	case partialText != "":
		continuation.Prompt = fmt.Sprintf(
			"The previous response was compacted before it finished. Resume from the durable boot packet and continue from where it stopped without repeating prior text.\n\nOriginal user request:\n%s\n\nAlready sent partial response tail:\n%s",
			originalPrompt,
			partialText,
		)
	default:
		continuation.Prompt = fmt.Sprintf(
			"The previous turn was compacted before completion. Resume from the durable boot packet and continue the original request without restarting it.\n\nOriginal user request:\n%s",
			originalPrompt,
		)
	}

	return continuation
}

func buildTodoReconciliationCall(call SessionAgentCall) SessionAgentCall {
	followUp := call
	followUp.SkipUserMessage = true
	followUp.TodoReconcileTry++
	followUp.Prompt = strings.TrimSpace(`Before ending this request, reconcile the live todo list.

Rules:
- The todo list exists to keep your execution structured; do not leave it stale.
- Inspect the current checklist state and act on it now.
- If any listed work is still genuinely required, do that work first.
- Then call update_plan so every retained item ends completed.
- Drop obsolete or superseded items instead of leaving them pending.
- Do not finish this turn with any retained item still pending or in_progress.
- After the update_plan call, return only the minimal final answer.`)
	return followUp
}

func buildStructuredBlockRepairCall(mode planmode.SessionMode, call SessionAgentCall) SessionAgentCall {
	followUp := call
	followUp.SkipUserMessage = true
	followUp.StructuredTry++

	openTag, closeTag, ok := planmode.StructuredBlockTags(mode)
	if !ok {
		return followUp
	}

	followUp.Prompt = strings.TrimSpace(fmt.Sprintf(`You are still in %s mode.

Your previous response is invalid because it did not end with exactly one valid %s ... %s block.

Rules:
- Do not narrate execution, implementation progress, or generic status.
- Do not ask for permission to proceed.
- Use the repository facts and analysis already gathered in this conversation.
- Return only the corrected final deliverable.
- The answer must be a complete Markdown payload wrapped in exactly one %s block.

Return the corrected final deliverable now.`, mode.Title(), openTag, closeTag, openTag))

	return followUp
}

func countIncompleteRenderableTodos(todos []session.Todo) int {
	count := 0
	for _, todo := range todos {
		if !session.IsRenderableTodo(todo) {
			continue
		}
		if session.IsTodoIncompleteStatus(todo.Status) {
			count++
		}
	}
	return count
}

func completeSingleTrailingInProgressTodo(todos []session.Todo) ([]session.Todo, bool) {
	if countIncompleteRenderableTodos(todos) != 1 {
		return todos, false
	}
	updated := slices.Clone(todos)
	for i := range updated {
		if !session.IsRenderableTodo(updated[i]) || updated[i].Status != session.TodoStatusInProgress {
			continue
		}
		updated[i].Status = session.TodoStatusCompleted
		updated[i].ActiveForm = ""
		return updated, true
	}
	return todos, false
}

func completeAllIncompleteTodos(todos []session.Todo) ([]session.Todo, bool) {
	updated := slices.Clone(todos)
	changed := false
	for i := range updated {
		if !session.IsRenderableTodo(updated[i]) || !session.IsTodoIncompleteStatus(updated[i].Status) {
			continue
		}
		updated[i].Status = session.TodoStatusCompleted
		updated[i].ActiveForm = ""
		changed = true
	}
	return updated, changed
}

func buildRuntimeReminder(mode planmode.SessionMode, prompt string) string {
	mode = planmode.NormalizeMode(mode)

	switch mode {
	case planmode.PlanMode:
		return `Plan mode runtime contract:
- If agent.md exists, read it first as a codebase map, then search for the real files that control this task.
- Use single_view for one known file and agentic_view for broader relevant slices.
- Read the relevant implementation files fully, not half, before you finalize the plan.
- Use broad read-only repository inspection when the task spans more than one file.
- Inspect the repository deeply before finalizing the plan; do not stop after a shallow search or one list call.
- Do not use update_plan, do not execute, and do not narrate execution.
- Replace generic explanation with one final decision-complete <proposed_plan>...</proposed_plan> block.
- If the current draft is not yet a real plan block, keep analyzing and then emit the final plan block.
- The final plan must be structured Markdown, neutral in tone, and specific enough to implement without high-impact guesswork.`
	case planmode.ArchitectureMode:
		return `Architect mode runtime contract:
- If agent.md exists, read it first as a system map, then search for the actual files and seams that govern the task.
- Use single_view for one known file and agentic_view for broader relevant slices.
- Read the relevant implementation files fully before finalizing the design.
- Use non-mutating tooling, including shell, Python, tests, and builds, when it improves architectural truth.
- Use read-only inspection and analysis tooling to understand the current structure.
- Do not mutate repository files.
- Return the final answer as exactly one <architecture_spec>...</architecture_spec> block with strongly structured Markdown.`
	case planmode.DebugMode:
		return `Debug mode runtime contract:
- If agent.md exists, read it first as a runtime map, then search for the actual failing path and read the relevant files fully.
- Use single_view for one known file and agentic_view for broader relevant slices.
- Use non-mutating tooling aggressively to reproduce and diagnose before concluding.
- Diagnose from concrete evidence first; do not jump to fixes without tracing the failure.
- You may inspect and run non-mutating diagnostic tooling, but do not mutate repository files in this mode.
- Return the final answer as exactly one <debug_report>...</debug_report> block with structured Markdown and explicit verification.`
	case planmode.SecurityMode:
		return `Security mode runtime contract:
- If agent.md exists, read it first as a system map, then search for and fully read the real files that define exposure and trust boundaries.
- Use single_view for one known file and agentic_view for broader relevant slices.
- Use non-mutating tooling, including shell, Python, tests, and static analysis, when it materially improves confidence.
- Use concrete evidence from code, config, and tooling. Avoid generic security commentary.
- Do not mutate repository files.
- Return the final answer as exactly one <security_report>...</security_report> block with structured Markdown and honest severity.`
	case planmode.ReviewMode:
		return `Review mode runtime contract:
- If agent.md exists, read it first as a codebase map, then fully read the changed files and surrounding implementation before finalizing judgment.
- Use single_view for one known file and agentic_view for broader relevant slices.
- Use non-mutating checks when they materially improve review quality.
- Inspect the real code and behavior; prioritize bugs, regressions, and missing tests.
- Do not mutate repository files.
- Return the final answer as exactly one <review_report>...</review_report> block with structured Markdown and decisive findings.`
	case planmode.OrchestratorMode:
		return `Orchestrator mode runtime contract:
- If agent.md exists, read it first as a system map, then search for and fully read the files that define dependencies, collision risk, and validation paths.
- Use single_view for one known file and agentic_view for broader relevant slices.
- Use non-mutating tooling, including shell, Python, tests, and builds, when it improves dependency and validation truth.
- Reason about agent topology, contracts, blockers, and merge-safe execution from real repository/runtime evidence.
- Do not mutate repository files.
- Return the final answer as exactly one <execution_orchestration>...</execution_orchestration> block with structured Markdown and explicit contracts.`
	default:
		if shouldDelegateToSubAgents(prompt) {
			return `Plan tool protocol for multi-step tasks (Codex update_plan):
1. Use update_plan only for non-trivial multi-step work
2. Call update_plan BEFORE technical work when a plan is warranted
3. Always send the FULL plan on every update; do not omit existing items
4. Keep exactly one step in_progress at a time
5. Before the next command, mark the previous completed step as completed
6. Do not batch-complete items after the fact
7. Do not abandon the plan - complete every step
8. Do NOT repeat the full plan after calling update_plan - the harness already displays it

Skip this only for a single non-destructive read requiring exactly one tool call.`
		}

		return `For multi-step tasks, use update_plan before execution only when the plan is clear and non-trivial. Every plan item must include explicit step text; never send blank or placeholder steps. Keep 5-7 steps max, send the full plan each time, keep one step in_progress, mark completed steps before the next command, use pending -> in_progress -> completed transitions, and finish with all steps completed. Do NOT repeat the plan - the harness displays it.`
	}
}

func buildComplexityModeReminder(mode planmode.SessionMode) string {
	switch planmode.NormalizeMode(mode) {
	case planmode.DefaultSessionMode:
		return `<system_reminder>Complexity mode:
- Initialize plan with update_plan immediately before technical execution.
- Keep the plan tracker synchronized after every state change.
- Read exactly 1 repository file with "single_view". Read 2 or more repository files with "agentic_view". Keep each "agentic_view" batch to 2–30 files and chunk larger reads into multiple batches.
- Edit exactly 1 repository file with "single_edit". Edit 2 or more repository files with "agentic_edit". Keep each "agentic_edit" batch to 2–25 files and chunk larger edits into multiple batches.
- Do not use "bash" for repository discovery, file reads, or temporary prompt/CSV setup when a structured tool exists.
- Never write temporary .txt or .csv payload files just to call spawn_agent or send_input; pass arguments directly in the tool call.
- Use agentic_fetch for current external docs instead of guessing.</system_reminder>`
	default:
		return ""
	}
}

func responseHasRequiredStructuredBlock(mode planmode.SessionMode, assistant *message.Message, result *fantasy.AgentResult) bool {
	mode = planmode.NormalizeMode(mode)
	if mode == planmode.DefaultSessionMode {
		return true
	}

	text := ""
	if assistant != nil {
		text = strings.TrimSpace(assistant.Content().Text)
	}
	if text == "" && result != nil {
		text = strings.TrimSpace(result.Response.Content.Text())
	}
	if text == "" {
		return false
	}

	block, ok := planmode.ExtractStructuredBlockForMode(mode, text)
	return ok && block.IsValid
}

type SessionAgent interface {
	Run(context.Context, SessionAgentCall) (*fantasy.AgentResult, error)
	SetModels(large Model, small Model)
	SetTools(tools []fantasy.AgentTool)
	SetSystemPrompt(systemPrompt string)
	Cancel(sessionID string)
	CancelAll()
	IsSessionBusy(sessionID string) bool
	IsBusy() bool
	QueuedPrompts(sessionID string) int
	QueuedPromptsList(sessionID string) []string
	ClearQueue(sessionID string)
	Enqueue(call SessionAgentCall) error
	Summarize(context.Context, string, fantasy.ProviderOptions) error
	Model() Model
	SetWorkingDir(string)
	SessionID() string // Get the current session ID (for plan mode filtering)
}

type SubmissionStatus string

const (
	SubmissionStatusRunning SubmissionStatus = "running"
	SubmissionStatusQueued  SubmissionStatus = "queued"
)

type SubmissionResult struct {
	Status        SubmissionStatus
	SessionID     string
	UserMessageID string
}

type Model struct {
	Model      fantasy.LanguageModel
	CatwalkCfg catwalk.Model
	ModelCfg   config.SelectedModel
}

// sessionAgent implements the SessionAgent interface.
type sessionAgent struct {
	largeModel         *csync.Value[Model]
	smallModel         *csync.Value[Model]
	systemPromptPrefix *csync.Value[string]
	systemPrompt       *csync.Value[string]
	tools              *csync.Slice[fantasy.AgentTool]

	isSubAgent           bool
	sessions             session.Service
	messages             message.Service
	disableAutoSummarize bool
	isYolo               bool

	messageQueue            *csync.Map[string, []SessionAgentCall]
	activeRequests          *csync.Map[string, context.CancelFunc]
	memory                  memory.MemoryService
	memoryCompiler          *memory.Compiler
	codebaseIndexStatus     func(ctx context.Context, sessionID, workingDir string) string
	pmem                    *pmem.System
	postCompactionInjection *csync.Map[string, bool]
	longHorizon             *longhorizon.Manager
	longHorizonSessions     *csync.Map[string, bool]
	longHorizonInit         *csync.Map[string, bool]
	memoryConsolidator      func(ctx context.Context, sessionID string) error
	waitBackground          func(ctx context.Context, sessionID string) error
	checkpointTurn          func(ctx context.Context, sessionID, prompt, result, status string, force bool)

	// Python tool failure tracking - quit after 3 consecutive failures
	pythonFailures atomic.Int32
	workingDir     *csync.Value[string]
	writeScope     *tools.WriteScope
	sessionID      string // Current session ID (for plan mode filtering)
}

type SessionAgentOptions struct {
	LargeModel           Model
	SmallModel           Model
	SystemPromptPrefix   string
	SystemPrompt         string
	IsSubAgent           bool
	DisableAutoSummarize bool
	IsYolo               bool
	Sessions             session.Service
	Messages             message.Service
	Tools                []fantasy.AgentTool
	WorkingDir           string
	WriteScope           *tools.WriteScope
	Memory               memory.MemoryService
	MemoryCompiler       *memory.Compiler
	CodebaseIndexStatus  func(ctx context.Context, sessionID, workingDir string) string
	Pmem                 *pmem.System
	LongHorizon          *longhorizon.Manager
	MemoryConsolidator   func(ctx context.Context, sessionID string) error
	WaitBackground       func(ctx context.Context, sessionID string) error
	CheckpointTurn       func(ctx context.Context, sessionID, prompt, result, status string, force bool)
}

// NewSessionAgent initializes a new session-based AI agent with the provided configuration options.
func NewSessionAgent(
	opts SessionAgentOptions,
) SessionAgent {
	return &sessionAgent{
		largeModel:              csync.NewValue(opts.LargeModel),
		smallModel:              csync.NewValue(opts.SmallModel),
		systemPromptPrefix:      csync.NewValue(opts.SystemPromptPrefix),
		systemPrompt:            csync.NewValue(opts.SystemPrompt),
		isSubAgent:              opts.IsSubAgent,
		sessions:                opts.Sessions,
		messages:                opts.Messages,
		disableAutoSummarize:    opts.DisableAutoSummarize,
		tools:                   csync.NewSliceFrom(opts.Tools),
		isYolo:                  opts.IsYolo,
		waitBackground:          opts.WaitBackground,
		messageQueue:            csync.NewMap[string, []SessionAgentCall](),
		activeRequests:          csync.NewMap[string, context.CancelFunc](),
		memory:                  opts.Memory,
		memoryCompiler:          opts.MemoryCompiler,
		codebaseIndexStatus:     opts.CodebaseIndexStatus,
		pmem:                    opts.Pmem,
		workingDir:              csync.NewValue(opts.WorkingDir),
		writeScope:              opts.WriteScope,
		postCompactionInjection: csync.NewMap[string, bool](),
		longHorizon:             opts.LongHorizon,
		longHorizonSessions:     csync.NewMap[string, bool](),
		longHorizonInit:         csync.NewMap[string, bool](),
		memoryConsolidator:      opts.MemoryConsolidator,
		checkpointTurn:          opts.CheckpointTurn,
	}
}

func withTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if ctx == nil {
		return context.WithTimeout(context.Background(), timeout)
	}
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining > 0 && remaining < timeout {
			return context.WithCancel(ctx)
		}
	}
	return context.WithTimeout(ctx, timeout)
}

func isDBBusyErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "database is locked") || strings.Contains(msg, "database is busy") || strings.Contains(msg, "sqlite_busy")
}

func retryDB(ctx context.Context, timeout time.Duration, attempts int, fn func(context.Context) error) error {
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	backoff := 100 * time.Millisecond
	for i := 0; i < attempts; i++ {
		opCtx, cancel := withTimeout(ctx, timeout)
		lastErr = fn(opCtx)
		cancel()
		if lastErr == nil {
			return nil
		}
		if errors.Is(lastErr, context.Canceled) || errors.Is(lastErr, context.DeadlineExceeded) || isDBBusyErr(lastErr) {
			if ctx != nil && ctx.Err() != nil {
				return ctx.Err()
			}
			if i < attempts-1 {
				time.Sleep(backoff)
				if backoff < 500*time.Millisecond {
					backoff *= 2
				}
				continue
			}
		}
		return lastErr
	}
	return lastErr
}

func shouldRetryStreamError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var providerErr *fantasy.ProviderError
	if errors.As(err, &providerErr) {
		switch providerErr.StatusCode {
		case 0, 408, 409, 425, 429, 500, 502, 503, 504:
			return true
		}
	}
	return false
}

func (a *sessionAgent) createMessage(ctx context.Context, sessionID string, params message.CreateMessageParams, timeout time.Duration) (message.Message, error) {
	var out message.Message
	err := retryDB(ctx, timeout, 2, func(opCtx context.Context) error {
		msg, err := a.messages.Create(opCtx, sessionID, params)
		if err != nil {
			return err
		}
		out = msg
		return nil
	})
	return out, err
}

func (a *sessionAgent) updateMessage(ctx context.Context, msg message.Message, timeout time.Duration) error {
	return retryDB(ctx, timeout, 2, func(opCtx context.Context) error {
		return a.messages.Update(opCtx, msg)
	})
}

func (a *sessionAgent) getSessionWithTimeout(ctx context.Context, sessionID string) (session.Session, error) {
	var out session.Session
	err := retryDB(ctx, dbOpTimeout, 2, func(opCtx context.Context) error {
		sess, err := a.sessions.Get(opCtx, sessionID)
		if err != nil {
			return err
		}
		out = sess
		return nil
	})
	return out, err
}

func (a *sessionAgent) saveSessionWithTimeout(ctx context.Context, sess session.Session) (session.Session, error) {
	var out session.Session
	err := retryDB(ctx, dbOpLongTimeout, 2, func(opCtx context.Context) error {
		saved, err := a.sessions.Save(opCtx, sess)
		if err != nil {
			return err
		}
		out = saved
		return nil
	})
	return out, err
}

func (a *sessionAgent) finalizeSessionTodos(ctx context.Context, sessionID string, resolver func([]session.Todo) ([]session.Todo, bool)) (bool, error) {
	if resolver == nil {
		return false, nil
	}
	currentSession, err := a.getSessionWithTimeout(ctx, sessionID)
	if err != nil {
		return false, err
	}
	if !session.HasIncompleteTodos(currentSession.Todos) {
		return false, nil
	}
	updatedTodos, changed := resolver(currentSession.Todos)
	if !changed {
		return false, nil
	}
	currentSession.Todos = updatedTodos
	if _, err := a.saveSessionWithTimeout(ctx, currentSession); err != nil {
		return false, err
	}
	return true, nil
}

func (a *sessionAgent) buildTodoReconciliationFollowUp(ctx context.Context, call SessionAgentCall) (*SessionAgentCall, error) {
	currentSession, err := a.getSessionWithTimeout(ctx, call.SessionID)
	if err != nil {
		return nil, err
	}
	if !session.HasIncompleteTodos(currentSession.Todos) {
		return nil, nil
	}
	if resolved, err := a.finalizeSessionTodos(ctx, call.SessionID, completeSingleTrailingInProgressTodo); err != nil {
		return nil, err
	} else if resolved {
		return nil, nil
	}
	if call.TodoReconcileTry < maxTodoReconcileAttempts {
		followUp := buildTodoReconciliationCall(call)
		return &followUp, nil
	}
	if _, err := a.finalizeSessionTodos(ctx, call.SessionID, completeAllIncompleteTodos); err != nil {
		return nil, err
	}
	return nil, nil
}

func isGeminiCodeExecutionModel(model Model) bool {
	if !strings.EqualFold(model.ModelCfg.Provider, google.Name) &&
		!strings.EqualFold(model.ModelCfg.Provider, "gemini") &&
		!strings.EqualFold(model.ModelCfg.Provider, "google-vertex") {
		return false
	}

	modelID := strings.ToLower(strings.TrimSpace(model.CatwalkCfg.ID))
	if modelID == "" {
		modelID = strings.ToLower(strings.TrimSpace(model.ModelCfg.Model))
	}

	return strings.HasPrefix(modelID, "gemini")
}

func (a *sessionAgent) Enqueue(call SessionAgentCall) error {
	if call.Prompt == "" && !message.ContainsTextAttachment(call.Attachments) && call.PrecreatedUser == nil {
		return ErrEmptyPrompt
	}
	if call.SessionID == "" {
		return ErrSessionMissing
	}
	existing, ok := a.messageQueue.Get(call.SessionID)
	if !ok {
		existing = []SessionAgentCall{}
	}
	existing = append(existing, call)
	a.messageQueue.Set(call.SessionID, existing)
	return nil
}

func (a *sessionAgent) Run(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
	if call.Prompt == "" && !message.ContainsTextAttachment(call.Attachments) && call.PrecreatedUser == nil {
		return nil, ErrEmptyPrompt
	}
	if call.SessionID == "" {
		return nil, ErrSessionMissing
	}

	// Set current session ID for plan mode filtering
	a.setSessionID(call.SessionID)

	// Reset Python tool failure counter for new run
	a.pythonFailures.Store(0)

	// Queue the message if busy
	if a.IsSessionBusy(call.SessionID) {
		if err := a.Enqueue(call); err != nil {
			return nil, err
		}
		return nil, nil
	}

	// Copy mutable fields under lock to avoid races with SetTools/SetModels.
	agentTools := a.tools.Copy()
	largeModel := a.largeModel.Get()
	systemPrompt := a.systemPrompt.Get()
	promptPrefix := a.systemPromptPrefix.Get()
	var instructions strings.Builder
	activeTools := newActiveToolSet(call.ActiveTools)

	if a.longHorizon != nil {
		if active, ok := a.longHorizonSessions.Get(call.SessionID); ok && active {
			// already active
		} else if a.shouldActivateLongHorizon(call) {
			if pending, ok := a.longHorizonInit.Get(call.SessionID); ok && pending {
				// already initializing
			} else {
				a.longHorizonInit.Set(call.SessionID, true)
				go func(sessionID, prompt string) {
					bgCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
					defer cancel()
					if _, err := a.longHorizon.Ensure(bgCtx, sessionID, prompt); err == nil {
						a.longHorizonSessions.Set(sessionID, true)
						a.longHorizonInit.Del(sessionID)
						a.longHorizon.AppendAudit(bgCtx, sessionID, "Activated long-horizon mode based on task complexity.")
					} else {
						a.longHorizonInit.Del(sessionID)
						slog.Warn("Failed to initialize long-horizon artifacts", "error", err)
					}
				}(call.SessionID, call.Prompt)
			}
		}
	}

	for _, server := range mcp.GetStates() {
		if server.State != mcp.StateConnected {
			continue
		}
		if s := server.Client.InitializeResult().Instructions; s != "" {
			instructions.WriteString(s)
			instructions.WriteString("\n\n")
		}
	}

	if s := instructions.String(); s != "" {
		systemPrompt += "\n\n<mcp-instructions>\n" + s + "\n</mcp-instructions>"
	}
	if capabilityMap := buildMCPCapabilityMap(); capabilityMap != "" {
		systemPrompt += "\n\n" + capabilityMap
	}

	if len(agentTools) > 0 {
		// Add Anthropic caching to the last tool.
		agentTools[len(agentTools)-1].SetProviderOptions(a.getCacheControlOptions())
	}

	agent := fantasy.NewAgent(
		largeModel.Model,
		fantasy.WithSystemPrompt(systemPrompt),
		fantasy.WithTools(agentTools...),
	)

	sessionLock := sync.Mutex{}
	currentSession, err := a.getSessionWithTimeout(ctx, call.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	mode := planmode.NormalizeMode(currentSession.Mode)

	msgs, err := a.getSessionMessages(ctx, currentSession)
	if err != nil {
		return nil, fmt.Errorf("failed to get session messages: %w", err)
	}
	if call.PrecreatedUser != nil {
		msgs = slices.DeleteFunc(msgs, func(msg message.Message) bool {
			return msg.ID == call.PrecreatedUser.ID
		})
	}

	var wg sync.WaitGroup
	// Generate title if first message.
	if len(msgs) == 0 {
		titleCtx := ctx // Copy to avoid race with ctx reassignment below.
		wg.Go(func() {
			a.generateTitle(titleCtx, call.SessionID, call.Prompt)
		})
	}
	defer wg.Wait()

	// Add the user message to the session unless it was created earlier.
	if !call.SkipUserMessage && call.PrecreatedUser == nil {
		created, createErr := a.createUserMessage(ctx, call)
		if createErr != nil {
			return nil, createErr
		}
		call.PrecreatedUser = &created
	}

	// Add the session to the context.
	ctx = context.WithValue(ctx, tools.SessionIDContextKey, call.SessionID)
	ctx = context.WithValue(ctx, tools.SessionModeContextKey, mode)
	runtimeControl := newRuntimeControl()
	ctx = context.WithValue(ctx, tools.RuntimeControlContextKey, runtimeControl)

	genCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	a.activeRequests.Set(call.SessionID, cancel)
	defer a.activeRequests.Del(call.SessionID)

	markActivity := func() {}
	var updateMu sync.Mutex
	var lastUpdate time.Time
	updateAssistant := func(ctx context.Context, msg *message.Message, timeout time.Duration, force bool) error {
		if msg == nil {
			return nil
		}
		if !force && timeout == messageUpdateTimeout {
			updateMu.Lock()
			if !lastUpdate.IsZero() && time.Since(lastUpdate) < messageUpdateMinInterval {
				updateMu.Unlock()
				return nil
			}
			lastUpdate = time.Now()
			updateMu.Unlock()
		}
		updateCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		clone := msg.Clone()
		if err := a.updateMessage(updateCtx, clone, timeout); err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				slog.Debug("Message update timed out", "session_id", msg.SessionID, "message_id", msg.ID, "error", err)
				return nil
			}
			return err
		}
		return nil
	}
	history, files := a.preparePrompt(mode, msgs, call.Prompt, call.Attachments...)
	history = a.injectTieredMemory(ctx, history, call, int(largeModel.CatwalkCfg.ContextWindow))

	startTime := time.Now()
	a.eventPromptSent(call.SessionID)

	var (
		currentAssistant *message.Message
		shouldSummarize  bool
		stepStartTime    time.Time
		firstTokenTime   time.Time
		firstToolName    string
	)
	streamCall := fantasy.AgentStreamCall{
		Prompt:           message.PromptWithTextAttachments(call.Prompt, call.Attachments),
		Files:            files,
		Messages:         history,
		ActiveTools:      activeTools.List(),
		ProviderOptions:  call.ProviderOptions,
		MaxOutputTokens:  &call.MaxOutputTokens,
		TopP:             call.TopP,
		Temperature:      call.Temperature,
		PresencePenalty:  call.PresencePenalty,
		TopK:             call.TopK,
		FrequencyPenalty: call.FrequencyPenalty,
		PrepareStep: func(callContext context.Context, options fantasy.PrepareStepFunctionOptions) (_ context.Context, prepared fantasy.PrepareStepResult, err error) {
			markActivity()
			stepStartTime = time.Now()
			firstTokenTime = time.Time{}
			prepared.Messages = options.Messages
			prepared.ActiveTools = activeTools.List()
			for i := range prepared.Messages {
				prepared.Messages[i].ProviderOptions = nil
			}

			queuedCalls, _ := a.messageQueue.Get(call.SessionID)
			a.messageQueue.Del(call.SessionID)
			for _, queued := range queuedCalls {
				if queued.SkipUserMessage {
					continue
				}
				userMessage, createErr := a.createUserMessage(callContext, queued)
				if createErr != nil {
					return callContext, prepared, createErr
				}
				prepared.Messages = append(prepared.Messages, userMessage.ToAIMessage()...)
			}

			prepared.Messages = a.workaroundProviderMediaLimitations(prepared.Messages, largeModel)

			lastSystemRoleInx := 0
			systemMessageUpdated := false
			for i, msg := range prepared.Messages {
				// Only add cache control to the last message.
				if msg.Role == fantasy.MessageRoleSystem {
					lastSystemRoleInx = i
				} else if !systemMessageUpdated {
					prepared.Messages[lastSystemRoleInx].ProviderOptions = a.getCacheControlOptions()
					systemMessageUpdated = true
				}
				// Than add cache control to the last 2 messages.
				if i > len(prepared.Messages)-3 {
					prepared.Messages[i].ProviderOptions = a.getCacheControlOptions()
				}
			}

			var systemMessages []fantasy.Message
			if systemPrompt != "" {
				systemMessages = append(systemMessages, fantasy.NewSystemMessage(systemPrompt))
			}
			if promptPrefix != "" {
				systemMessages = append(systemMessages, fantasy.NewSystemMessage(promptPrefix))
			}
			if call.SkillContext != "" {
				skillSystemMsg := "<active_skill_context>\n" + call.SkillContext + "\n</active_skill_context>"
				systemMessages = append(systemMessages, fantasy.NewSystemMessage(skillSystemMsg))
			}

			if isGeminiCodeExecutionModel(largeModel) {
				systemMessages = append(systemMessages, fantasy.NewSystemMessage(string(pythonCapabilitiesPrompt)))
			}

			if len(systemMessages) > 0 {
				prepared.Messages = append(systemMessages, prepared.Messages...)
			}

			var assistantMsg message.Message
			assistantMsg, err = a.createMessage(callContext, call.SessionID, message.CreateMessageParams{
				Role:     message.Assistant,
				Parts:    []message.ContentPart{},
				Model:    largeModel.ModelCfg.Model,
				Provider: largeModel.ModelCfg.Provider,
			}, dbOpTimeout)
			if err != nil {
				return callContext, prepared, err
			}
			if len(call.ActiveSkills) > 0 {
				assistantMsg.SetSkillContext(call.ActiveSkills)
				if err := a.updateMessage(callContext, assistantMsg, dbOpTimeout); err != nil {
					return callContext, prepared, err
				}
			}
			callContext = context.WithValue(callContext, tools.MessageIDContextKey, assistantMsg.ID)
			callContext = context.WithValue(callContext, tools.SupportsImagesContextKey, largeModel.CatwalkCfg.SupportsImages)
			callContext = context.WithValue(callContext, tools.ModelNameContextKey, largeModel.CatwalkCfg.Name)
			callContext = context.WithValue(callContext, tools.WorkingDirContextKey, a.workingDir.Get())
			callContext = context.WithValue(callContext, tools.WriteScopeContextKey, a.writeScope)
			currentAssistant = &assistantMsg
			return callContext, prepared, err
		},
		RepairToolCall: func(repairCtx context.Context, options fantasy.ToolCallRepairOptions) (*fantasy.ToolCallContent, error) {
			call := fantasy.ToolCall{
				ID:    options.OriginalToolCall.ToolCallID,
				Name:  options.OriginalToolCall.ToolName,
				Input: options.OriginalToolCall.Input,
			}
			toolMap := make(map[string]fantasy.AgentTool)
			for _, t := range options.AvailableTools {
				if t != nil {
					toolMap[t.Info().Name] = t
				}
			}
			prepared, _, err := tools.PrepareToolCall(repairCtx, call, toolMap)
			if err != nil {
				// We can't fix it. Return nil to let fantasy handle the original validation error.
				return nil, options.ValidationError
			}
			return &fantasy.ToolCallContent{
				ToolCallID: prepared.ID,
				ToolName:   prepared.Name,
				Input:      prepared.Input,
			}, nil
		},
		OnReasoningStart: func(id string, reasoning fantasy.ReasoningContent) error {
			markActivity()
			runtimeControl.NoteReasoning()
			if firstTokenTime.IsZero() {
				firstTokenTime = time.Now()
			}
			currentAssistant.AppendReasoningContent(reasoning.Text)
			return updateAssistant(genCtx, currentAssistant, messageUpdateTimeout, false)
		},
		OnReasoningDelta: func(id string, text string) error {
			markActivity()
			runtimeControl.NoteReasoning()
			if firstTokenTime.IsZero() {
				firstTokenTime = time.Now()
			}
			currentAssistant.AppendReasoningContent(text)
			return updateAssistant(genCtx, currentAssistant, messageUpdateTimeout, false)
		},
		OnReasoningEnd: func(id string, reasoning fantasy.ReasoningContent) error {
			markActivity()
			// handle anthropic signature
			if anthropicData, ok := reasoning.ProviderMetadata[anthropic.Name]; ok {
				if reasoning, ok := anthropicData.(*anthropic.ReasoningOptionMetadata); ok {
					currentAssistant.AppendReasoningSignature(reasoning.Signature)
				}
			}
			if googleData, ok := reasoning.ProviderMetadata[google.Name]; ok {
				if reasoning, ok := googleData.(*google.ReasoningMetadata); ok {
					currentAssistant.AppendThoughtSignature(reasoning.Signature, reasoning.ToolID)
				}
			}
			if openaiData, ok := reasoning.ProviderMetadata[openai.Name]; ok {
				if reasoning, ok := openaiData.(*openai.ResponsesReasoningMetadata); ok {
					currentAssistant.SetReasoningResponsesData(reasoning)
				}
			}
			currentAssistant.FinishThinking()
			return updateAssistant(genCtx, currentAssistant, messageUpdateTimeout, false)
		},
		OnTextDelta: func(id string, text string) error {
			markActivity()
			runtimeControl.NoteReasoning()
			if firstTokenTime.IsZero() {
				firstTokenTime = time.Now()
			}
			// Strip leading newline from initial text content. This is is
			// particularly important in non-interactive mode where leading
			// newlines are very visible.
			if len(currentAssistant.Parts) == 0 {
				text = strings.TrimPrefix(text, "\n")
			}

			currentAssistant.AppendContent(text)
			return updateAssistant(genCtx, currentAssistant, messageUpdateTimeout, false)
		},
		OnToolInputStart: func(id string, toolName string) error {
			markActivity()
			// Fast-path: immediate exit on context cancellation
			if genCtx.Err() != nil {
				return genCtx.Err()
			}
			activeTools.Add(toolName)
			if firstToolName == "" {
				firstToolName = toolName
			}

			toolCall := message.ToolCall{
				ID:               id,
				Name:             toolName,
				ProviderExecuted: false,
				Finished:         false,
			}
			currentAssistant.AddToolCall(toolCall)
			return updateAssistant(genCtx, currentAssistant, messageUpdateTimeout, true)
		},
		OnRetry: func(err *fantasy.ProviderError, delay time.Duration) {
			markActivity()
			if err != nil {
				slog.Warn("Provider retry scheduled", "error", err.Message, "status", err.StatusCode, "delay", delay)
			}
		},
		OnToolCall: func(tc fantasy.ToolCallContent) error {
			markActivity()
			// Fast-path: immediate exit on context cancellation
			if genCtx.Err() != nil {
				return genCtx.Err()
			}
			activeTools.Add(tc.ToolName)
			if firstToolName == "" {
				firstToolName = tc.ToolName
			}

			toolCall := message.ToolCall{
				ID:               tc.ToolCallID,
				Name:             tc.ToolName,
				Input:            tc.Input,
				ProviderExecuted: false,
				Finished:         true,
			}
			currentAssistant.AddToolCall(toolCall)
			if a.pmem != nil {
				a.pmem.RecordToolCall(genCtx, currentAssistant.SessionID, tc.ToolName, tc.Input)
			}
			return updateAssistant(genCtx, currentAssistant, messageUpdateTimeout, true)
		},
		OnToolResult: func(result fantasy.ToolResultContent) error {
			markActivity()
			runtimeControl.FinishToolExecution(result.ToolName)
			// Fast-path: immediate exit on context cancellation
			if genCtx.Err() != nil {
				return genCtx.Err()
			}

			toolResult := a.convertToToolResult(result)

			// Track Python tool failures - quit after 3 consecutive failures
			if result.ToolName == tools.PythonToolName {
				if toolResult.IsError ||
					strings.Contains(strings.ToLower(toolResult.Content), "error") ||
					strings.Contains(strings.ToLower(toolResult.Content), "exception") ||
					strings.Contains(strings.ToLower(toolResult.Content), "traceback") {
					failures := a.pythonFailures.Add(1)
					if failures >= tools.MaxPythonRetries {
						slog.Warn("Python tool failed too many times, quitting", "failures", failures, "max", tools.MaxPythonRetries)
						// Reset counter and return special error to stop agent from retrying
						a.pythonFailures.Store(0)
						return fmt.Errorf("python tool failed %d times consecutively (max: %d). Stopping further Python execution attempts. Please review the task and try a different approach.", failures, tools.MaxPythonRetries)
					}
				} else {
					// Success - reset failure counter
					a.pythonFailures.Store(0)
				}
			}

			if a.pmem != nil {
				var rawInput string
				for _, tc := range currentAssistant.ToolCalls() {
					if tc.ID == result.ToolCallID {
						rawInput = tc.Input
						break
					}
				}
				outStr := toolResult.Content
				if toolResult.IsError {
					outStr = "ERROR: " + outStr
				}
				a.pmem.PushToolResult(currentAssistant.SessionID, len(history), result.ToolName, rawInput, outStr)
				a.pmem.RecordToolResult(genCtx, currentAssistant.SessionID, result.ToolName, outStr, toolResult.IsError)
			}

			_, createMsgErr := a.createMessage(genCtx, currentAssistant.SessionID, message.CreateMessageParams{
				Role: message.Tool,
				Parts: []message.ContentPart{
					toolResult,
				},
			}, dbOpTimeout)
			if createMsgErr != nil {
				if errors.Is(createMsgErr, context.DeadlineExceeded) || errors.Is(createMsgErr, context.Canceled) || isDBBusyErr(createMsgErr) {
					slog.Warn("Skipping tool result persistence due to DB timeout", "error", createMsgErr)
					return nil
				}
				return createMsgErr
			}
			if grounding := buildToolGrounding(result.ToolName, toolResult.Content); grounding != "" {
				_, groundErr := a.createMessage(genCtx, currentAssistant.SessionID, message.CreateMessageParams{
					Role: message.System,
					Parts: []message.ContentPart{
						message.TextContent{Text: grounding},
					},
				}, dbOpTimeout)
				if groundErr != nil {
					if errors.Is(groundErr, context.DeadlineExceeded) || errors.Is(groundErr, context.Canceled) || isDBBusyErr(groundErr) {
						slog.Warn("Skipping tool grounding persistence due to DB timeout", "error", groundErr)
						return nil
					}
					return groundErr
				}
			}
			return nil
		},
		OnStepFinish: func(stepResult fantasy.StepResult) error {
			runtimeControl.ObserveAfterStep()
			finishReason := message.FinishReasonUnknown
			switch stepResult.FinishReason {
			case fantasy.FinishReasonLength:
				finishReason = message.FinishReasonMaxTokens
			case fantasy.FinishReasonStop:
				finishReason = message.FinishReasonEndTurn
			case fantasy.FinishReasonToolCalls:
				finishReason = message.FinishReasonToolUse
			}

			if !a.isSubAgent && a.waitBackground != nil {
				switch stepResult.FinishReason {
				case fantasy.FinishReasonStop, fantasy.FinishReasonLength:
					if err := a.waitBackground(genCtx, call.SessionID); err != nil {
						return err
					}
				}
			}
			if stepResult.FinishReason == fantasy.FinishReasonStop || stepResult.FinishReason == fantasy.FinishReasonLength {
				flushCtx, flushCancel := context.WithTimeout(genCtx, 15*time.Second)
				if err := tools.FlushGitSnapshot(flushCtx, a.workingDir.Get()); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
					slog.Warn("Failed to flush git snapshots at turn end", "working_dir", a.workingDir.Get(), "error", err)
				}
				flushCancel()
			}

			// Capture Gemini-specific usage metadata if available
			sessionLock.Lock()
			defer sessionLock.Unlock()

			// Check if this is a Gemini model by name pattern
			isGemini := strings.Contains(strings.ToLower(largeModel.CatwalkCfg.Name), "gemini")

			if isGemini {
				// Extract Gemini usage metadata
				var promptTokens, completionTokens, totalTokens, thoughtsTokens, cachedTokens int64
				var startTimeMs, endTimeMs int64

				promptTokens = stepResult.Usage.InputTokens
				completionTokens = stepResult.Usage.OutputTokens
				totalTokens = stepResult.Usage.InputTokens + stepResult.Usage.OutputTokens
				// Note: fantasy.Usage may not have all Gemini-specific fields yet
				// Cache tokens would be available when using context caching
				cachedTokens = stepResult.Usage.CacheReadTokens

				// Real timing for Gemini performance metrics
				startTimeMs = stepStartTime.UnixMilli()
				endTimeMs = time.Now().UnixMilli()

				// Fallback: estimate thoughts tokens if reasoning was present but not in usage
				reasoningText := currentAssistant.ReasoningContent().Thinking
				if reasoningText != "" && thoughtsTokens == 0 {
					thoughtsTokens = int64(len(reasoningText) / 4)
				}

				currentAssistant.AddFinishWithGeminiMetadata(
					finishReason, "", "",
					promptTokens, completionTokens, totalTokens, thoughtsTokens, cachedTokens,
					startTimeMs, endTimeMs,
					largeModel.ModelCfg.ReasoningEffort,
				)
			} else {
				currentAssistant.AddFinish(finishReason, "", "")
			}

			if err := updateAssistant(genCtx, currentAssistant, messageFinalUpdateTimeout, true); err != nil {
				return err
			}

			usage := stepResult.Usage
			openrouterCost := a.openrouterCost(stepResult.ProviderMetadata)
			go func(sessionID string, model Model, usage fantasy.Usage, openrouterCost *float64) {
				saveCtx, cancel := context.WithTimeout(context.Background(), dbOpLongTimeout)
				defer cancel()

				updatedSession, getSessionErr := a.getSessionWithTimeout(saveCtx, sessionID)
				if getSessionErr != nil {
					slog.Warn("Failed to load session usage after final render", "session_id", sessionID, "error", getSessionErr)
					return
				}
				a.updateSessionUsage(model, &updatedSession, usage, openrouterCost)
				if _, sessionErr := a.saveSessionWithTimeout(saveCtx, updatedSession); sessionErr != nil {
					slog.Warn("Failed to persist session usage after final render", "session_id", sessionID, "error", sessionErr)
					return
				}
			}(call.SessionID, largeModel, usage, openrouterCost)

			return nil
		},
		StopWhen: []fantasy.StopCondition{
			func(steps []fantasy.StepResult) bool {
				// Priority 1: Context cancellation (Military-grade safeguard)
				if genCtx.Err() != nil {
					return true
				}

				cw := int64(largeModel.CatwalkCfg.ContextWindow)
				tokens := currentSession.CompletionTokens + currentSession.PromptTokens
				remaining := cw - tokens
				var threshold int64
				if cw > largeContextWindowThreshold {
					threshold = largeContextWindowBuffer
				} else {
					threshold = int64(float64(cw) * smallContextWindowRatio)
				}
				if threshold < smallContextWindowMinBuffer {
					threshold = smallContextWindowMinBuffer
				}

				// 75% Context Window Pre-Compaction Checkpoint
				// Rationale: For 1M+ token context windows, models lose >99% accuracy at 90%+ utilization.
				// 75% is the sweet spot: maximizes available context while preserving model accuracy.
				// Codex uses 95% for 256k models; we use 75% for 1M+ models.
				if cw > 0 && float64(tokens) >= float64(cw)*0.75 {
					if a.pmem.ShouldRunCheckpoint() {
						a.pmem.MarkCheckpointDone()
						go func(sessionID string) {
							memCtx, cancel := withTimeout(context.Background(), 2*time.Second)
							defer cancel()
							_ = a.pmem.RunPreCompactionCheckpoint(memCtx, sessionID, "20")
						}(call.SessionID)
					}
				}

				if cw > 0 && (remaining <= threshold) && !a.disableAutoSummarize {
					shouldSummarize = true
					if a.isLongHorizon(call.SessionID) && a.longHorizon != nil {
						a.longHorizon.AppendAudit(ctx, call.SessionID, fmt.Sprintf("Decision: trigger summarization at tokens=%d threshold=%d", tokens, threshold))
					}
					return true
				}
				return false
			},
			func(steps []fantasy.StepResult) bool {
				return hasRepeatedToolCalls(steps, loopDetectionWindowSize, loopDetectionMaxRepeats)
			},
		},
	}

	var result *fantasy.AgentResult
	for attempt := 0; attempt <= maxStreamRetries; attempt++ {
		result, err = agent.Stream(genCtx, streamCall)
		if err == nil || genCtx.Err() != nil {
			break
		}
		if attempt < maxStreamRetries && shouldRetryStreamError(err) && firstTokenTime.IsZero() {
			if currentAssistant != nil && len(currentAssistant.ToolCalls()) == 0 && currentAssistant.Content().Text == "" && currentAssistant.ReasoningContent().Thinking == "" {
				_ = a.messages.Delete(ctx, currentAssistant.ID)
				currentAssistant = nil
			}
			time.Sleep(streamRetryBackoff * time.Duration(attempt+1))
			continue
		}
		break
	}

	a.eventPromptResponded(call.SessionID, time.Since(startTime).Truncate(time.Second))

	if err != nil {
		isCancelErr := errors.Is(err, context.Canceled)
		isPermissionErr := errors.Is(err, permission.ErrorPermissionDenied)
		if currentAssistant == nil {
			return result, err
		}
		// Ensure we finish thinking on error to close the reasoning state.
		currentAssistant.FinishThinking()
		toolCalls := currentAssistant.ToolCalls()
		// INFO: we use the parent context here because the genCtx has been cancelled.
		var msgs []message.Message
		createErr := retryDB(ctx, dbOpTimeout, 2, func(opCtx context.Context) error {
			list, err := a.messages.List(opCtx, currentAssistant.SessionID)
			if err != nil {
				return err
			}
			msgs = list
			return nil
		})
		if createErr != nil {
			return nil, createErr
		}
		for _, tc := range toolCalls {
			if !tc.Finished {
				tc.Finished = true
				tc.Input = "{}"
				currentAssistant.AddToolCall(tc)
				updateErr := a.updateMessage(ctx, *currentAssistant, dbOpTimeout)
				if updateErr != nil {
					return nil, updateErr
				}
			}

			found := false
			for _, msg := range msgs {
				if msg.Role == message.Tool {
					for _, tr := range msg.ToolResults() {
						if tr.ToolCallID == tc.ID {
							found = true
							break
						}
					}
				}
				if found {
					break
				}
			}
			if found {
				continue
			}
			content := "There was an error while executing the tool"
			if isCancelErr {
				content = "Tool execution canceled by user"
			} else if isPermissionErr {
				content = "User denied permission"
			}
			toolResult := message.ToolResult{
				ToolCallID: tc.ID,
				Name:       tc.Name,
				Content:    content,
				IsError:    true,
			}
			_, createErr = a.createMessage(ctx, currentAssistant.SessionID, message.CreateMessageParams{
				Role: message.Tool,
				Parts: []message.ContentPart{
					toolResult,
				},
			}, dbOpTimeout)
			if createErr != nil {
				return nil, createErr
			}
		}
		var fantasyErr *fantasy.Error
		var providerErr *fantasy.ProviderError
		const providerErrorTitle = "Provider Error"
		const requestErrorTitle = "Request Error"
		linkStyle := lipgloss.NewStyle().Foreground(charmtone.Guac).Underline(true)
		if isCancelErr {
			currentAssistant.AddFinish(message.FinishReasonCanceled, "User canceled request", "")
		} else if isPermissionErr {
			currentAssistant.AddFinish(message.FinishReasonPermissionDenied, "User denied permission", "")
		} else if errors.Is(err, hyper.ErrNoCredits) {
			url := hyper.BaseURL()
			link := linkStyle.Hyperlink(url, "id=hyper").Render(url)
			currentAssistant.AddFinish(message.FinishReasonError, "No credits", "You're out of credits. Add more at "+link)
		} else if errors.As(err, &providerErr) {
			if providerErr.Message == "The requested model is not supported." {
				url := "https://github.com/settings/copilot/features"
				link := linkStyle.Hyperlink(url, "id=copilot").Render(url)
				currentAssistant.AddFinish(
					message.FinishReasonError,
					"Copilot model not enabled",
					fmt.Sprintf("%q is not enabled in Copilot. Go to the following page to enable it. Then, wait 5 minutes before trying again. %s", largeModel.CatwalkCfg.Name, link),
				)
			} else {
				currentAssistant.AddFinish(message.FinishReasonError, cmp.Or(stringext.Capitalize(providerErr.Title), providerErrorTitle), providerErr.Message)
			}
		} else if errors.As(err, &fantasyErr) {
			currentAssistant.AddFinish(message.FinishReasonError, cmp.Or(stringext.Capitalize(fantasyErr.Title), requestErrorTitle), fantasyErr.Message)
		} else {
			if title, details, ok := classifyProviderTransportError(err, linkStyle); ok {
				currentAssistant.AddFinish(message.FinishReasonError, title, details)
			} else {
				currentAssistant.AddFinish(message.FinishReasonError, requestErrorTitle, err.Error())
			}
		}
		// Note: we use the parent context here because the genCtx has been
		// cancelled.
		updateErr := updateAssistant(ctx, currentAssistant, messageFinalUpdateTimeout, true)
		if updateErr != nil {
			return nil, updateErr
		}
		a.activeRequests.Del(call.SessionID)
		cancel()
		if a.checkpointTurn != nil {
			a.checkpointTurn(ctx, call.SessionID, call.Prompt, err.Error(), "error", true)
		}
		queuedMessages, ok := a.messageQueue.Get(call.SessionID)
		if ok && len(queuedMessages) > 0 {
			firstQueuedMessage := queuedMessages[0]
			a.messageQueue.Set(call.SessionID, queuedMessages[1:])
			_, _ = a.Run(ctx, firstQueuedMessage)
		}
		return nil, err
	}

	if shouldSummarize {
		if currentAssistant != nil {
			currentAssistant.FinishThinking()
			currentAssistant.AddFinish(message.FinishReasonMaxTokens, "Context compacted", "Continuing after session compaction.")
			if updateErr := updateAssistant(ctx, currentAssistant, messageFinalUpdateTimeout, true); updateErr != nil {
				return nil, updateErr
			}
		}
		a.activeRequests.Del(call.SessionID)
		if a.longHorizon != nil && a.isLongHorizon(call.SessionID) {
			a.longHorizon.AppendAudit(ctx, call.SessionID, "Triggering context compaction/summarization.")
		}
		if summarizeErr := a.Summarize(genCtx, call.SessionID, call.ProviderOptions); summarizeErr != nil {
			return nil, summarizeErr
		}
		if a.longHorizon != nil && a.isLongHorizon(call.SessionID) {
			a.longHorizon.AppendAudit(ctx, call.SessionID, "Summarization complete; triggering memory consolidation.")
			// Trigger async consolidation
			if a.memoryConsolidator != nil {
				go func() {
					_ = a.memoryConsolidator(context.Background(), call.SessionID)
				}()
			}
		}
		if a.checkpointTurn != nil {
			a.checkpointTurn(ctx, call.SessionID, call.Prompt, "context compacted", "compacted", true)
		}
		resumePointID := ""
		if a.memoryCompiler != nil {
			workDir := ""
			if a.workingDir != nil {
				workDir = a.workingDir.Get()
			}
			if memCtx, memCancel := withTimeout(ctx, memoryCallTimeout); memCtx != nil {
				resumePoint, resumeErr := a.memoryCompiler.CreateResumePoint(memCtx, memory.ResumeRequest{
					SessionID:      call.SessionID,
					WorkingDir:     workDir,
					Task:           call.Prompt,
					OriginalPrompt: call.Prompt,
					Reason:         "context_rollover",
				})
				memCancel()
				if resumeErr == nil {
					resumePointID = resumePoint.ID
				}
			}
		}
		if resumePointID == "" {
			a.postCompactionInjection.Set(call.SessionID, true)
		}
		// Queue the message again so it doesn't get dropped.
		existing, ok := a.messageQueue.Get(call.SessionID)
		if !ok {
			existing = []SessionAgentCall{}
		}
		call = buildCompactionContinuationCall(call, currentAssistant, resumePointID)
		existing = append(existing, call)
		a.messageQueue.Set(call.SessionID, existing)
	}

	// Release active request before processing queued messages.
	a.activeRequests.Del(call.SessionID)
	cancel()
	if a.checkpointTurn != nil {
		resultText := ""
		if result != nil {
			resultText = result.Response.Content.Text()
		}
		a.checkpointTurn(ctx, call.SessionID, call.Prompt, resultText, "completed", false)
	}
	if !responseHasRequiredStructuredBlock(mode, currentAssistant, result) {
		if mode != planmode.DefaultSessionMode && call.StructuredTry < maxStructuredBlockRepairAttempts {
			return a.Run(ctx, buildStructuredBlockRepairCall(mode, call))
		}
		if mode != planmode.DefaultSessionMode {
			return nil, fmt.Errorf("%s mode response missing required structured block", mode.Title())
		}
	}
	if followUp, reconcileErr := a.buildTodoReconciliationFollowUp(ctx, call); reconcileErr != nil {
		return nil, reconcileErr
	} else if followUp != nil {
		return a.Run(ctx, *followUp)
	}

	queuedMessages, ok := a.messageQueue.Get(call.SessionID)
	if !ok || len(queuedMessages) == 0 {
		return result, err
	}
	// There are queued messages restart the loop.
	firstQueuedMessage := queuedMessages[0]
	a.messageQueue.Set(call.SessionID, queuedMessages[1:])
	return a.Run(ctx, firstQueuedMessage)
}

func classifyProviderTransportError(err error, linkStyle lipgloss.Style) (string, string, bool) {
	if err == nil {
		return "", "", false
	}
	raw := strings.TrimSpace(err.Error())
	lower := strings.ToLower(raw)

	if strings.Contains(lower, "openrouter.ai/api/v1/chat/completions") &&
		strings.Contains(lower, "no endpoints available matching your guardrail restrictions and data policy") {
		url := "https://openrouter.ai/settings/privacy"
		link := linkStyle.Hyperlink(url, "id=openrouter-privacy").Render(url)
		return "OpenRouter privacy settings blocked this model",
			"No OpenRouter provider endpoint currently matches your account privacy and guardrail settings for this model. Review your provider/privacy policy here: " + link,
			true
	}

	if strings.Contains(lower, "openrouter.ai/api/v1/chat/completions") &&
		(strings.Contains(lower, "connection reset by peer") || strings.Contains(lower, "eof")) {
		return "OpenRouter connection reset",
			"The selected OpenRouter endpoint dropped the network connection. This is usually a transient provider routing failure; retry the request or switch to another model/provider.",
			true
	}

	if strings.Contains(lower, "no such host") || (strings.Contains(lower, "dial tcp") && strings.Contains(lower, "lookup ")) {
		return "Model provider DNS lookup failed",
			"Sapphire could not resolve the selected model provider host. Check internet connectivity, DNS, VPN/proxy settings, or firewall rules, then retry the request.",
			true
	}

	if strings.Contains(lower, "i/o timeout") || strings.Contains(lower, "context deadline exceeded") {
		return "Model provider network timeout",
			"Sapphire reached the model provider but the network request timed out before a response arrived. Retry the request, or check connectivity and provider availability.",
			true
	}

	return "", "", false
}

func (a *sessionAgent) Summarize(ctx context.Context, sessionID string, opts fantasy.ProviderOptions) error {
	if a.IsSessionBusy(sessionID) {
		return ErrSessionBusy
	}

	largeModel := a.largeModel.Get()
	systemPromptPrefix := a.systemPromptPrefix.Get()

	currentSession, err := a.getSessionWithTimeout(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}
	msgs, err := a.getSessionMessages(ctx, currentSession)
	if err != nil {
		return err
	}
	if len(msgs) == 0 {
		return nil
	}

	// Codex-style compaction: build history and retry with trimming on context exceeded
	aiMsgs, _ := a.preparePrompt(planmode.DefaultSessionMode, msgs, "")

	genCtx, cancel := context.WithCancel(ctx)
	a.activeRequests.Set(sessionID, cancel)
	defer a.activeRequests.Del(sessionID)
	defer cancel()

	// Create summary message placeholder
	summaryMessage, err := a.createMessage(ctx, sessionID, message.CreateMessageParams{
		Role:             message.Assistant,
		Model:            largeModel.Model.Model(),
		Provider:         largeModel.Model.Provider(),
		IsSummaryMessage: true,
	}, dbOpTimeout)
	if err != nil {
		return err
	}

	// Codex-style trim-and-retry loop
	var truncatedCount int
	maxRetries := 3
	var finalNarrativeResp *fantasy.AgentResult

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Prepare prompt with current history
		summaryPromptText := buildSummaryPrompt()

		// Stream summary
		narrativeAgent := fantasy.NewAgent(largeModel.Model,
			fantasy.WithSystemPrompt(string(summaryPrompt)),
		)

		narrativeResp, streamErr := narrativeAgent.Stream(genCtx, fantasy.AgentStreamCall{
			Prompt:          summaryPromptText,
			Messages:        aiMsgs,
			ProviderOptions: opts,
			PrepareStep: func(callContext context.Context, options fantasy.PrepareStepFunctionOptions) (_ context.Context, prepared fantasy.PrepareStepResult, err error) {
				prepared.Messages = options.Messages
				if systemPromptPrefix != "" {
					prepared.Messages = append([]fantasy.Message{fantasy.NewSystemMessage(systemPromptPrefix)}, prepared.Messages...)
				}
				return callContext, prepared, nil
			},
			OnReasoningDelta: func(id string, text string) error {
				summaryMessage.AppendReasoningContent(text)
				if err := a.updateMessage(genCtx, summaryMessage, messageUpdateTimeout); err != nil {
					if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || isDBBusyErr(err) {
						return nil
					}
					return err
				}
				return nil
			},
			OnTextDelta: func(id, text string) error {
				summaryMessage.AppendContent(text)
				if err := a.updateMessage(genCtx, summaryMessage, messageUpdateTimeout); err != nil {
					if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || isDBBusyErr(err) {
						return nil
					}
					return err
				}
				return nil
			},
		})

		// Handle stream errors
		if streamErr != nil {
			if errors.Is(streamErr, context.Canceled) {
				_ = a.messages.Delete(ctx, summaryMessage.ID)
				return streamErr
			}

			// Codex-style: on context exceeded, trim oldest message and retry
			if isContextWindowExceeded(streamErr) && len(aiMsgs) > 1 {
				slog.Warn("Context window exceeded during compaction; removing oldest history item",
					"attempt", attempt+1, "error", streamErr)
				aiMsgs = aiMsgs[1:] // Trim oldest message
				truncatedCount++
				continue
			}

			// Max retries exceeded or non-retryable error
			if attempt < maxRetries {
				time.Sleep(streamRetryBackoff * time.Duration(attempt+1))
				continue
			}
			return streamErr
		}

		// Success - exit retry loop
		finalNarrativeResp = narrativeResp
		break
	}

	// Codex-style: build compacted history with token-budgeted user messages
	summaryMessage.AddFinish(message.FinishReasonEndTurn, "", "")

	// Collect user messages (excluding summaries) - Codex's collect_user_messages
	userMessages := collectUserMessages(msgs)

	// Build compacted history with token limit - Codex's build_compacted_history_with_limit
	compactedHistory := buildCompactedHistory(userMessages, summaryMessage.Content().Text, compactUserMessageMaxTokens)

	// Codex-style: summary_prefix + summary content
	summaryText := string(summaryPrefix) + "\n" + compactedHistory
	summaryMessage.Parts = []message.ContentPart{message.TextContent{Text: summaryText}}
	_ = a.updateMessage(genCtx, summaryMessage, dbOpLongTimeout)

	// Persist structured summary
	a.persistStructuredSummary(genCtx, sessionID, aiMsgs, nil, opts, systemPromptPrefix, largeModel.Model)

	// Update session usage
	if finalNarrativeResp != nil {
		a.updateSessionUsage(largeModel, &currentSession, finalNarrativeResp.TotalUsage, nil)
	}
	currentSession.SummaryMessageID = summaryMessage.ID

	// Reset checkpoint state for next compaction cycle
	if a.pmem != nil {
		a.pmem.ResetCheckpointState()
	}

	// Audit log for long-horizon tasks
	if a.isLongHorizon(sessionID) && a.longHorizon != nil {
		a.longHorizon.AppendAudit(ctx, sessionID, "Completed summarization checkpoint and structured extraction.")
	}

	// Save session
	_, err = a.saveSessionWithTimeout(genCtx, currentSession)
	return err
}

func (a *sessionAgent) shouldActivateLongHorizon(call SessionAgentCall) bool {
	wordCount := len(strings.Fields(call.Prompt))
	if wordCount >= 80 {
		return true
	}
	if shouldDelegateToSubAgents(call.Prompt) {
		return true
	}
	if len(call.Attachments) > 2 {
		return true
	}
	return false
}

func (a *sessionAgent) isLongHorizon(sessionID string) bool {
	if a.longHorizonSessions == nil {
		return false
	}
	val, ok := a.longHorizonSessions.Get(sessionID)
	return ok && val
}

func (a *sessionAgent) getCacheControlOptions() fantasy.ProviderOptions {
	if t, _ := strconv.ParseBool(os.Getenv("CRUSH_DISABLE_ANTHROPIC_CACHE")); t {
		return fantasy.ProviderOptions{}
	}
	return fantasy.ProviderOptions{
		anthropic.Name: &anthropic.ProviderCacheControlOptions{
			CacheControl: anthropic.CacheControl{Type: "ephemeral"},
		},
		bedrock.Name: &anthropic.ProviderCacheControlOptions{
			CacheControl: anthropic.CacheControl{Type: "ephemeral"},
		},
		vercel.Name: &anthropic.ProviderCacheControlOptions{
			CacheControl: anthropic.CacheControl{Type: "ephemeral"},
		},
	}
}

func (a *sessionAgent) createUserMessage(ctx context.Context, call SessionAgentCall) (message.Message, error) {
	parts := []message.ContentPart{message.TextContent{Text: call.Prompt}}
	var attachmentParts []message.ContentPart
	for _, attachment := range call.Attachments {
		attachmentParts = append(attachmentParts, message.BinaryContent{Path: attachment.FilePath, MIMEType: attachment.MimeType, Data: attachment.Content})
	}
	parts = append(parts, attachmentParts...)
	msg, err := a.createMessage(ctx, call.SessionID, message.CreateMessageParams{
		Role:  message.User,
		Parts: parts,
	}, dbOpTimeout)
	if err != nil {
		return message.Message{}, fmt.Errorf("failed to create user message: %w", err)
	}
	return msg, nil
}

func shouldDelegateToSubAgents(prompt string) bool {
	normalized := strings.ToLower(strings.TrimSpace(prompt))
	if normalized == "" {
		return false
	}

	complexitySignals := []string{
		"codebase",
		"entire repo",
		"whole repo",
		"whole project",
		"entire project",
		"across the project",
		"across the codebase",
		"multiple files",
		"multiple packages",
		"multi-package",
		"cross-cutting",
		"large refactor",
		"architecture",
		"architect",
		"migration",
		"comprehensive",
		"integration",
	}

	signalCount := 0
	for _, signal := range complexitySignals {
		if strings.Contains(normalized, signal) {
			signalCount++
		}
	}

	if signalCount >= 3 {
		return true
	}

	if len(strings.Fields(normalized)) >= 60 {
		return true
	}

	return false
}

func isMultiStepPrompt(prompt string) bool {
	normalized := strings.TrimSpace(prompt)
	if normalized == "" {
		return false
	}
	if shouldDelegateToSubAgents(normalized) {
		return true
	}
	words := strings.Fields(normalized)
	if len(words) >= 40 {
		return true
	}
	lines := strings.Split(normalized, "\n")
	bullets := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "• ") {
			bullets++
			continue
		}
		if len(line) >= 2 && line[0] >= '0' && line[0] <= '9' {
			for i := 1; i < len(line) && i < 4; i++ {
				if line[i] == '.' || line[i] == ')' {
					bullets++
					break
				}
				if line[i] < '0' || line[i] > '9' {
					break
				}
			}
		}
	}
	if bullets >= 2 {
		return true
	}
	if len(words) >= 12 && strings.Contains(normalized, " and ") {
		return true
	}
	return false
}

func appendSkillContext(existing, extra string) string {
	extra = strings.TrimSpace(extra)
	if extra == "" {
		return existing
	}
	if strings.TrimSpace(existing) == "" {
		return extra
	}
	return existing + "\n\n" + extra
}

func (a *sessionAgent) preparePrompt(mode planmode.SessionMode, msgs []message.Message, prompt string, attachments ...message.Attachment) ([]fantasy.Message, []fantasy.FilePart) {
	var history []fantasy.Message
	if !a.isSubAgent {
		if reminder := strings.TrimSpace(buildRuntimeReminder(mode, prompt)); reminder != "" {
			history = append(history, fantasy.NewUserMessage(
				fmt.Sprintf("<system_reminder>%s</system_reminder>", reminder),
			))
		}
		if extra := buildComplexityModeReminder(mode); extra != "" {
			history = append(history, fantasy.NewUserMessage(extra))
		}
	} else {
		history = append(history, fantasy.NewUserMessage(
			`<system_reminder>Sub-agent Directive:
Execute your assigned chunk of the tasks autonomously and efficiently.
- Read one known repository file with "single_view". Read any multi-file target set or broad repository slice with "agentic_view". Use "agentic_view" comprehensively: read broad relevant slices in each sweep instead of minimal batches.
- Edit exactly 1 repository file with "single_edit". Edit 2 or more repository files with "agentic_edit". Keep each "agentic_edit" batch to 2–25 files and chunk larger edits into multiple batches.
- Do not use "bash" for repository discovery, file reads, or temporary prompt/CSV setup when a structured tool exists.
- Never write temporary .txt or .csv payload files just to call spawn_agent or send_input; pass arguments directly in the tool call.
- External facts: Use "agentic_fetch" (retrieve documentation immediately; do not guess).
- Code Execution: Use "python" tool for complex computations, data processing, or verification.
- Shell: Use "bash" for terminal commands and background jobs.
Resolve your assigned scope independently and return only verified, concise objective results back to the main agent.</system_reminder>`,
		))
	}
	for _, m := range msgs {
		if len(m.Parts) == 0 {
			continue
		}
		// Assistant message without content or tool calls (cancelled before it
		// returned anything).
		if m.Role == message.Assistant && len(m.ToolCalls()) == 0 && m.Content().Text == "" && m.ReasoningContent().String() == "" {
			continue
		}
		history = append(history, m.ToAIMessage()...)
	}

	var files []fantasy.FilePart
	for _, attachment := range attachments {
		if attachment.IsText() {
			continue
		}
		files = append(files, fantasy.FilePart{
			Filename:  attachment.FileName,
			Data:      attachment.Content,
			MediaType: attachment.MimeType,
		})
	}

	return history, files
}

func (a *sessionAgent) injectTieredMemory(ctx context.Context, history []fantasy.Message, call SessionAgentCall, contextWindow int) (retHistory []fantasy.Message) {
	// Add permanent recovery for any internal panics here.
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Caught panic in injectTieredMemory", "error", r)
			retHistory = history // Safely fallback to original history
		}
	}()

	retHistory = history
	sessionID := call.SessionID

	if a == nil {
		return history
	}
	if a.memory == nil && a.memoryCompiler == nil {
		return history
	}
	if a.memory != nil {
		if val := reflect.ValueOf(a.memory); val.Kind() == reflect.Ptr && val.IsNil() && a.memoryCompiler == nil {
			return history
		}
	}

	// Determine whether we're on the first turn after a compaction summary.
	postCompaction := false
	charBudget := 0
	if a.postCompactionInjection != nil {
		if pending, ok := a.postCompactionInjection.Get(sessionID); ok && pending {
			postCompaction = true
			a.postCompactionInjection.Del(sessionID)
		}
	}
	if postCompaction && contextWindow > 0 {
		charBudget = int(float64(contextWindow) * postCompactionInjectionRatio * postCompactionContextCharsPerTok)
	}
	contextStage := pmem.ContextLoadStageCold
	if a.sessions != nil && strings.TrimSpace(sessionID) != "" {
		if sessionState, err := a.getSessionWithTimeout(ctx, sessionID); err == nil {
			contextStage = determineContextLoadStage(sessionState.PromptTokens+sessionState.CompletionTokens, contextWindow, postCompaction)
		} else if postCompaction {
			contextStage = pmem.ContextLoadStage50
		}
	} else if postCompaction {
		contextStage = pmem.ContextLoadStage50
	}

	constitution := ""
	if a.memory != nil {
		if memCtx, cancel := withTimeout(ctx, memoryCallTimeout); memCtx != nil {
			constitution, _ = a.memory.GetProjectConstitution(memCtx, "default")
			cancel()
		}
	}

	longHorizonContext := ""
	if a.longHorizon != nil && a.isLongHorizon(sessionID) {
		longHorizonContext = a.longHorizon.BuildInjection(sessionID)
	}

	historicalContext := ""
	if a.pmem != nil && contextStage >= pmem.ContextLoadStage10 {
		if memCtx, cancel := withTimeout(ctx, memoryCallTimeout); memCtx != nil {
			if charBudget > 0 {
				historicalContext = a.pmem.BuildContextInjectionForSessionAtStage(memCtx, sessionID, charBudget/postCompactionContextCharsPerTok, contextStage)
			} else {
				historicalContext = a.pmem.BuildContextInjectionForSessionAtStage(memCtx, sessionID, contextWindow, contextStage)
			}
			cancel()
		}
	}

	compiledInjection := ""
	workDir := ""
	if a.workingDir != nil {
		workDir = a.workingDir.Get()
	}
	if a.memoryCompiler != nil {
		if memCtx, cancel := withTimeout(ctx, memoryCallTimeout); memCtx != nil {
			if strings.TrimSpace(call.ResumePointID) != "" {
				compiledInjection = a.memoryCompiler.RenderResumePointInjection(memCtx, call.ResumePointID)
			}
			if compiledInjection == "" && strings.TrimSpace(call.Prompt) != "" {
				if resumePoint, ok := a.memoryCompiler.MatchPendingResumePoint(memCtx, sessionID, call.Prompt); ok {
					compiledInjection = a.memoryCompiler.RenderResumePointInjection(memCtx, resumePoint.ID)
				}
			}
			cancel()
		}
	}
	if compiledInjection == "" && a.memoryCompiler != nil && shouldInjectCompiledCodebase(contextStage) {
		if memCtx, cancel := withTimeout(ctx, memoryCallTimeout); memCtx != nil {
			compiledInjection = a.memoryCompiler.RenderPromptInjection(memCtx, memory.CompileRequest{
				SessionID:           sessionID,
				WorkingDir:          workDir,
				Task:                call.Prompt,
				ProjectConstitution: constitution,
				LongHorizonContext:  longHorizonContext,
				HistoricalContext:   historicalContext,
			})
			cancel()
		}
	}
	codebaseIndexStatus := ""
	if a.codebaseIndexStatus != nil && shouldInjectCompiledCodebase(contextStage) {
		if memCtx, cancel := withTimeout(ctx, memoryCallTimeout); memCtx != nil {
			codebaseIndexStatus = a.codebaseIndexStatus(memCtx, sessionID, workDir)
			cancel()
		}
	}

	if compiledInjection != "" {
		retHistory = append([]fantasy.Message{
			fantasy.NewSystemMessage(compiledInjection),
		}, retHistory...)
	} else {
		if constitution != "" {
			retHistory = append([]fantasy.Message{
				fantasy.NewSystemMessage("## PROJECT CONSTITUTION (Tier 1 Hot Memory)\n" + constitution),
			}, retHistory...)
		}
		if longHorizonContext != "" {
			retHistory = append([]fantasy.Message{
				fantasy.NewSystemMessage(longHorizonContext),
			}, retHistory...)
		}
		if historicalContext != "" {
			retHistory = append([]fantasy.Message{
				fantasy.NewSystemMessage(historicalContext),
			}, retHistory...)
		}
	}
	if codebaseIndexStatus != "" {
		retHistory = append([]fantasy.Message{
			fantasy.NewSystemMessage(codebaseIndexStatus),
		}, retHistory...)
	}

	if a.pmem != nil {
		if notice := a.pmem.BackpressureNotice(); notice != "" {
			retHistory = append([]fantasy.Message{
				fantasy.NewSystemMessage(notice),
			}, retHistory...)
		}
	}

	return retHistory
}

func (a *sessionAgent) getSessionMessages(ctx context.Context, session session.Session) ([]message.Message, error) {
	var msgs []message.Message
	err := retryDB(ctx, dbOpTimeout, 2, func(opCtx context.Context) error {
		list, err := a.messages.List(opCtx, session.ID)
		if err != nil {
			return err
		}
		msgs = list
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list messages: %w", err)
	}

	if session.SummaryMessageID != "" {
		summaryMsgIndex := -1
		for i, msg := range msgs {
			if msg.ID == session.SummaryMessageID {
				summaryMsgIndex = i
				break
			}
		}
		if summaryMsgIndex != -1 {
			msgs = msgs[summaryMsgIndex:]
			msgs[0].Role = message.User
		}
	}
	return msgs, nil
}

// isSummaryMessage checks if a message is a compaction summary message.
// Aligned with Codex's is_summary_message function.
func isSummaryMessage(message string) bool {
	return strings.HasPrefix(message, string(summaryPrefix))
}

func determineContextLoadStage(tokens int64, contextWindow int, postCompaction bool) pmem.ContextLoadStage {
	if postCompaction {
		return pmem.ContextLoadStage50
	}
	if contextWindow <= 0 || tokens <= 0 {
		return pmem.ContextLoadStageCold
	}
	percentUsed := int((tokens * 100) / int64(contextWindow))
	switch {
	case percentUsed >= 50:
		return pmem.ContextLoadStage50
	case percentUsed >= 40:
		return pmem.ContextLoadStage40
	case percentUsed >= 30:
		return pmem.ContextLoadStage30
	case percentUsed >= 20:
		return pmem.ContextLoadStage20
	case percentUsed >= 10:
		return pmem.ContextLoadStage10
	default:
		return pmem.ContextLoadStageCold
	}
}

func shouldInjectCompiledCodebase(stage pmem.ContextLoadStage) bool {
	return stage >= pmem.ContextLoadStage50
}

// isContextWindowExceeded checks if an error is a context window exceeded error.
// Aligned with Codex's context window exceeded handling.
func isContextWindowExceeded(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "context") &&
		(strings.Contains(errStr, "exceed") || strings.Contains(errStr, "limit") || strings.Contains(errStr, "maximum"))
}

// collectUserMessages extracts user messages from history, excluding summary messages.
// Aligned with Codex's collect_user_messages function.
func collectUserMessages(msgs []message.Message) []string {
	var userMessages []string
	for _, msg := range msgs {
		if msg.Role == message.User {
			content := msg.Content().Text
			if content != "" && !isSummaryMessage(content) {
				userMessages = append(userMessages, content)
			}
		}
	}
	return userMessages
}

// buildCompactedHistory builds compacted history with token-budgeted user messages.
// Aligned with Codex's build_compacted_history_with_limit function.
func buildCompactedHistory(userMessages []string, summaryText string, maxTokens int) string {
	if len(userMessages) == 0 {
		if summaryText == "" {
			return "(no summary available)"
		}
		return summaryText
	}

	// Select user messages within token budget (reverse order to keep recent)
	var selectedMessages []string
	remaining := maxTokens

	for i := len(userMessages) - 1; i >= 0 && remaining > 0; i-- {
		msg := userMessages[i]
		tokens := estimateTokens(msg)

		if tokens <= remaining {
			selectedMessages = append(selectedMessages, msg)
			remaining -= tokens
		} else {
			// Truncate message to fit remaining budget
			truncated := truncateToTokenBudget(msg, remaining)
			if truncated != "" {
				selectedMessages = append(selectedMessages, truncated)
			}
			break
		}
	}

	// Reverse to restore original order
	for i, j := 0, len(selectedMessages)-1; i < j; i, j = i+1, j-1 {
		selectedMessages[i], selectedMessages[j] = selectedMessages[j], selectedMessages[i]
	}

	// Build compacted history
	var sb strings.Builder
	for _, msg := range selectedMessages {
		sb.WriteString(msg)
		sb.WriteString("\n")
	}

	if summaryText != "" {
		sb.WriteString(summaryText)
	}

	return sb.String()
}

// truncateToTokenBudget truncates text to fit within token budget.
func truncateToTokenBudget(text string, maxTokens int) string {
	if maxTokens <= 0 || text == "" {
		return ""
	}
	maxChars := maxTokens * 4 // Approximate chars per token
	if len(text) <= maxChars {
		return text
	}
	if maxChars <= 3 {
		return text[:maxChars]
	}
	return text[:maxChars-3] + "..."
}

// estimateTokens estimates token count from text length.
// Uses 4 chars per token as a rough approximation.
func estimateTokens(text string) int {
	if text == "" {
		return 0
	}
	return (len(text) + 3) / 4
}

// generateTitle generates a session titled based on the initial prompt.
func (a *sessionAgent) generateTitle(ctx context.Context, sessionID string, userPrompt string) {
	if userPrompt == "" {
		return
	}

	smallModel := a.smallModel.Get()
	largeModel := a.largeModel.Get()
	systemPromptPrefix := a.systemPromptPrefix.Get()

	var maxOutputTokens int64 = 40
	if smallModel.CatwalkCfg.CanReason {
		maxOutputTokens = smallModel.CatwalkCfg.DefaultMaxTokens
	}

	newAgent := func(m fantasy.LanguageModel, p []byte, tok int64) fantasy.Agent {
		return fantasy.NewAgent(m,
			fantasy.WithSystemPrompt(string(p)+"\n /no_think"),
			fantasy.WithMaxOutputTokens(tok),
		)
	}

	streamCall := fantasy.AgentStreamCall{
		Prompt: fmt.Sprintf("Generate a concise title for the following content:\n\n%s\n <think>\n\n</think>", userPrompt),
		PrepareStep: func(callCtx context.Context, opts fantasy.PrepareStepFunctionOptions) (_ context.Context, prepared fantasy.PrepareStepResult, err error) {
			prepared.Messages = opts.Messages
			if systemPromptPrefix != "" {
				prepared.Messages = append([]fantasy.Message{
					fantasy.NewSystemMessage(systemPromptPrefix),
				}, prepared.Messages...)
			}
			return callCtx, prepared, nil
		},
	}

	// Use the small model to generate the title.
	model := smallModel
	agent := newAgent(model.Model, titlePrompt, maxOutputTokens)
	resp, err := agent.Stream(ctx, streamCall)
	if err == nil {
		// We successfully generated a title with the small model.
		slog.Debug("Generated title with small model")
	} else {
		// It didn't work. Let's try with the big model.
		slog.Error("Error generating title with small model; trying big model", "err", err)
		model = largeModel
		agent = newAgent(model.Model, titlePrompt, maxOutputTokens)
		resp, err = agent.Stream(ctx, streamCall)
		if err == nil {
			slog.Debug("Generated title with large model")
		} else {
			// Welp, the large model didn't work either. Use the default
			// session name and return.
			slog.Error("Error generating title with large model", "err", err)
			saveErr := retryDB(ctx, dbOpTimeout, 2, func(opCtx context.Context) error {
				return a.sessions.UpdateTitleAndUsage(opCtx, sessionID, defaultSessionName, 0, 0, 0)
			})
			if saveErr != nil {
				slog.Error("Failed to save session title and usage", "error", saveErr)
			}
			return
		}
	}

	if resp == nil {
		// Actually, we didn't get a response so we can't. Use the default
		// session name and return.
		slog.Error("Response is nil; can't generate title")
		saveErr := retryDB(ctx, dbOpTimeout, 2, func(opCtx context.Context) error {
			return a.sessions.UpdateTitleAndUsage(opCtx, sessionID, defaultSessionName, 0, 0, 0)
		})
		if saveErr != nil {
			slog.Error("Failed to save session title and usage", "error", saveErr)
		}
		return
	}

	// Clean up title.
	var title string
	title = strings.ReplaceAll(resp.Response.Content.Text(), "\n", " ")

	// Remove thinking tags if present.
	title = thinkTagRegex.ReplaceAllString(title, "")

	title = strings.TrimSpace(title)
	title = cmp.Or(title, defaultSessionName)

	// Calculate usage and cost.
	var openrouterCost *float64
	for _, step := range resp.Steps {
		stepCost := a.openrouterCost(step.ProviderMetadata)
		if stepCost != nil {
			newCost := *stepCost
			if openrouterCost != nil {
				newCost += *openrouterCost
			}
			openrouterCost = &newCost
		}
	}

	modelConfig := model.CatwalkCfg
	cost := modelConfig.CostPer1MInCached/1e6*float64(resp.TotalUsage.CacheCreationTokens) +
		modelConfig.CostPer1MOutCached/1e6*float64(resp.TotalUsage.CacheReadTokens) +
		modelConfig.CostPer1MIn/1e6*float64(resp.TotalUsage.InputTokens) +
		modelConfig.CostPer1MOut/1e6*float64(resp.TotalUsage.OutputTokens)

	// Use override cost if available (e.g., from OpenRouter).
	if openrouterCost != nil {
		cost = *openrouterCost
	}

	promptTokens := resp.TotalUsage.InputTokens + resp.TotalUsage.CacheCreationTokens
	completionTokens := resp.TotalUsage.OutputTokens

	// Atomically update only title and usage fields to avoid overriding other
	// concurrent session updates.
	saveErr := retryDB(ctx, dbOpTimeout, 2, func(opCtx context.Context) error {
		return a.sessions.UpdateTitleAndUsage(opCtx, sessionID, title, promptTokens, completionTokens, cost)
	})
	if saveErr != nil {
		slog.Error("Failed to save session title and usage", "error", saveErr)
		return
	}
}

func (a *sessionAgent) openrouterCost(metadata fantasy.ProviderMetadata) *float64 {
	openrouterMetadata, ok := metadata[openrouter.Name]
	if !ok {
		return nil
	}

	opts, ok := openrouterMetadata.(*openrouter.ProviderMetadata)
	if !ok {
		return nil
	}
	return &opts.Usage.Cost
}

func (a *sessionAgent) updateSessionUsage(model Model, session *session.Session, usage fantasy.Usage, overrideCost *float64) {
	modelConfig := model.CatwalkCfg
	cost := modelConfig.CostPer1MInCached/1e6*float64(usage.CacheCreationTokens) +
		modelConfig.CostPer1MOutCached/1e6*float64(usage.CacheReadTokens) +
		modelConfig.CostPer1MIn/1e6*float64(usage.InputTokens) +
		modelConfig.CostPer1MOut/1e6*float64(usage.OutputTokens)

	a.eventTokensUsed(session.ID, model, usage, cost)

	if overrideCost != nil {
		session.Cost += *overrideCost
	} else {
		session.Cost += cost
	}

	session.CompletionTokens = usage.OutputTokens
	session.PromptTokens = usage.InputTokens + usage.CacheReadTokens
}

func (a *sessionAgent) Cancel(sessionID string) {
	// Cancel regular requests. Don't use Take() here - we need the entry to
	// remain in activeRequests so IsBusy() returns true until the goroutine
	// fully completes (including error handling that may access the DB).
	// The defer in processRequest will clean up the entry.
	if cancel, ok := a.activeRequests.Get(sessionID); ok && cancel != nil {
		slog.Debug("Request cancellation initiated", "session_id", sessionID)
		cancel()
	}

	// Also check for summarize requests.
	if cancel, ok := a.activeRequests.Get(sessionID + "-summarize"); ok && cancel != nil {
		slog.Debug("Summarize cancellation initiated", "session_id", sessionID)
		cancel()
	}

	if a.QueuedPrompts(sessionID) > 0 {
		slog.Debug("Clearing queued prompts", "session_id", sessionID)
		a.messageQueue.Del(sessionID)
	}
}

func (a *sessionAgent) ClearQueue(sessionID string) {
	if a.QueuedPrompts(sessionID) > 0 {
		slog.Debug("Clearing queued prompts", "session_id", sessionID)
		a.messageQueue.Del(sessionID)
	}
}

func (a *sessionAgent) CancelAll() {
	if !a.IsBusy() {
		return
	}
	for key := range a.activeRequests.Seq2() {
		a.Cancel(key) // key is sessionID
	}

	timeout := time.After(5 * time.Second)
	for a.IsBusy() {
		select {
		case <-timeout:
			return
		default:
			time.Sleep(200 * time.Millisecond)
		}
	}
}

func (a *sessionAgent) IsBusy() bool {
	var busy bool
	for cancelFunc := range a.activeRequests.Seq() {
		if cancelFunc != nil {
			busy = true
			break
		}
	}
	return busy
}

func (a *sessionAgent) IsSessionBusy(sessionID string) bool {
	_, busy := a.activeRequests.Get(sessionID)
	return busy
}

func (a *sessionAgent) QueuedPrompts(sessionID string) int {
	l, ok := a.messageQueue.Get(sessionID)
	if !ok {
		return 0
	}
	return len(l)
}

func (a *sessionAgent) QueuedPromptsList(sessionID string) []string {
	l, ok := a.messageQueue.Get(sessionID)
	if !ok {
		return nil
	}
	prompts := make([]string, len(l))
	for i, call := range l {
		prompts[i] = call.Prompt
	}
	return prompts
}

func (a *sessionAgent) SetModels(large Model, small Model) {
	a.largeModel.Set(large)
	a.smallModel.Set(small)
}

func (a *sessionAgent) SetTools(tools []fantasy.AgentTool) {
	a.tools.SetSlice(tools)
}

func (a *sessionAgent) SetWorkingDir(workingDir string) {
	a.workingDir.Set(workingDir)
}

func (a *sessionAgent) SetSystemPrompt(systemPrompt string) {
	a.systemPrompt.Set(systemPrompt)
}

func (a *sessionAgent) Model() Model {
	return a.largeModel.Get()
}

// convertToToolResult converts a fantasy tool result to a message tool result.
func (a *sessionAgent) convertToToolResult(result fantasy.ToolResultContent) message.ToolResult {
	baseResult := message.ToolResult{
		ToolCallID: result.ToolCallID,
		Name:       result.ToolName,
		Metadata:   result.ClientMetadata,
	}

	switch result.Result.GetType() {
	case fantasy.ToolResultContentTypeText:
		if r, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentText](result.Result); ok {
			baseResult.Content = r.Text
		}
	case fantasy.ToolResultContentTypeError:
		if r, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentError](result.Result); ok {
			baseResult.Content = r.Error.Error()
			baseResult.IsError = true
		}
	case fantasy.ToolResultContentTypeMedia:
		if r, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentMedia](result.Result); ok {
			content := r.Text
			if content == "" {
				content = fmt.Sprintf("Loaded %s content", r.MediaType)
			}
			baseResult.Content = content
			baseResult.Data = r.Data
			baseResult.MIMEType = r.MediaType
		}
	}

	return baseResult
}

// workaroundProviderMediaLimitations converts media content in tool results to
// user messages for providers that don't natively support images in tool results.
//
// Problem: OpenAI, Google, OpenRouter, and other OpenAI-compatible providers
// don't support sending images/media in tool result messages - they only accept
// text in tool results. However, they DO support images in user messages.
//
// If we send media in tool results to these providers, the API returns an error.
//
// Solution: For these providers, we:
//  1. Replace the media in the tool result with a text placeholder
//  2. Inject a user message immediately after with the image as a file attachment
//  3. This maintains the tool execution flow while working around API limitations
//
// Anthropic and Bedrock support images natively in tool results, so we skip
// this workaround for them.
//
// Example transformation:
//
//	BEFORE: [tool result: image data]
//	AFTER:  [tool result: "Image loaded - see attached"], [user: image attachment]
func (a *sessionAgent) workaroundProviderMediaLimitations(messages []fantasy.Message, largeModel Model) []fantasy.Message {
	providerSupportsMedia := largeModel.ModelCfg.Provider == string(catwalk.InferenceProviderAnthropic) ||
		largeModel.ModelCfg.Provider == string(catwalk.InferenceProviderBedrock)

	if providerSupportsMedia {
		return messages
	}

	convertedMessages := make([]fantasy.Message, 0, len(messages))

	for _, msg := range messages {
		if msg.Role != fantasy.MessageRoleTool {
			convertedMessages = append(convertedMessages, msg)
			continue
		}

		textParts := make([]fantasy.MessagePart, 0, len(msg.Content))
		var mediaFiles []fantasy.FilePart

		for _, part := range msg.Content {
			toolResult, ok := fantasy.AsMessagePart[fantasy.ToolResultPart](part)
			if !ok {
				textParts = append(textParts, part)
				continue
			}

			if media, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentMedia](toolResult.Output); ok {
				decoded, err := base64.StdEncoding.DecodeString(media.Data)
				if err != nil {
					slog.Warn("Failed to decode media data", "error", err)
					textParts = append(textParts, part)
					continue
				}

				mediaFiles = append(mediaFiles, fantasy.FilePart{
					Data:      decoded,
					MediaType: media.MediaType,
					Filename:  fmt.Sprintf("tool-result-%s", toolResult.ToolCallID),
				})

				textParts = append(textParts, fantasy.ToolResultPart{
					ToolCallID: toolResult.ToolCallID,
					Output: fantasy.ToolResultOutputContentText{
						Text: "[Image/media content loaded - see attached file]",
					},
					ProviderOptions: toolResult.ProviderOptions,
				})
			} else {
				textParts = append(textParts, part)
			}
		}

		convertedMessages = append(convertedMessages, fantasy.Message{
			Role:    fantasy.MessageRoleTool,
			Content: textParts,
		})

		if len(mediaFiles) > 0 {
			convertedMessages = append(convertedMessages, fantasy.NewUserMessage(
				"Here is the media content from the tool result:",
				mediaFiles...,
			))
		}
	}

	return convertedMessages
}

// buildSummaryPrompt constructs the prompt text for session summarization.
// COMMENTED OUT - TODO LIST DISABLED
/*
func buildSummaryPrompt(todos []session.Todo) string {
	var sb strings.Builder
	sb.WriteString("Provide a detailed summary of our conversation above.")
	if len(todos) > 0 {
		sb.WriteString("\n\n## Current Todo List\n\n")
		for _, t := range todos {
			if t.ID != "" {
				fmt.Fprintf(&sb, "- [%s] (%s) %s", t.Status, t.ID, t.Content)
			} else {
				fmt.Fprintf(&sb, "- [%s] %s", t.Status, t.Content)
			}
			if t.ActiveForm != "" {
				fmt.Fprintf(&sb, " — %s", t.ActiveForm)
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\nInclude these tasks and their statuses in your summary. ")
		sb.WriteString("Instruct the resuming assistant to use the `todos` tool to continue tracking progress on these tasks.")
	}
	return sb.String()
}
*/

// Replacement function without todos
// Aligned with Codex's CONTEXT CHECKPOINT COMPACTION prompt.
func buildSummaryPrompt() string {
	return "You are performing a CONTEXT CHECKPOINT COMPACTION. Create a handoff summary for another LLM that will resume the task.\n\nInclude:\n- Current progress and key decisions made\n- Important context, constraints, or user preferences\n- What remains to be done (clear next steps)\n- Any critical data, examples, or references needed to continue\n\nBe concise, structured, and focused on helping the next LLM seamlessly continue the work."
}

// COMMENTED OUT - TODO LIST DISABLED
/*
func buildStructuredSummaryPrompt(todos []session.Todo) string {
	var sb strings.Builder
	sb.WriteString("Extract the current session state from the conversation above as structured JSON.")
	if len(todos) > 0 {
		sb.WriteString("\n\n## Current Todo List\n\n")
		for _, t := range todos {
			if !session.IsRenderableTodo(t) {
				continue
			}
			fmt.Fprintf(&sb, "- [%s] %s", t.Status, cmp.Or(strings.TrimSpace(t.ActiveForm), strings.TrimSpace(t.Content)))
			if t.ID != "" {
				fmt.Fprintf(&sb, " (%s)", t.ID)
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\nUse the todo list above as the source of truth for current task state.")
	}
	return sb.String()
}
*/

// Replacement function without todos
func buildStructuredSummaryPrompt() string {
	return "Extract the current session state from the conversation above as structured JSON."
}

func (a *sessionAgent) persistStructuredSummary(
	ctx context.Context,
	sessionID string,
	aiMsgs []fantasy.Message,
	todos interface{}, // COMMENTED OUT - was []session.Todo
	opts fantasy.ProviderOptions,
	systemPromptPrefix string,
	model fantasy.LanguageModel,
) {
	if a.memory == nil {
		return
	}

	structuredAgent := fantasy.NewAgent(model,
		fantasy.WithSystemPrompt(string(structuredSummaryPromptTmpl)),
	)
	resp, err := structuredAgent.Generate(ctx, fantasy.AgentCall{
		Prompt:          buildStructuredSummaryPrompt(), // COMMENTED OUT todos
		Messages:        aiMsgs,
		ProviderOptions: opts,
		PrepareStep: func(callContext context.Context, options fantasy.PrepareStepFunctionOptions) (_ context.Context, prepared fantasy.PrepareStepResult, err error) {
			prepared.Messages = options.Messages
			if systemPromptPrefix != "" {
				prepared.Messages = append([]fantasy.Message{fantasy.NewSystemMessage(systemPromptPrefix)}, prepared.Messages...)
			}
			return callContext, prepared, nil
		},
	})
	if err != nil {
		slog.Warn("Failed to generate structured session summary", "session_id", sessionID, "error", err)
		return
	}

	data, err := parseStructuredSummaryData(resp.Response.Content.Text(), todos) // COMMENTED OUT todos
	if err != nil {
		slog.Warn("Failed to parse structured session summary", "session_id", sessionID, "error", err)
		return
	}

	memCtx, cancel := withTimeout(ctx, memoryCallTimeout)
	if memCtx == nil {
		return
	}
	defer cancel()

	if err := a.memory.CreateStructuredSummary(memCtx, sessionID, data); err != nil {
		slog.Warn("Failed to persist structured session summary", "session_id", sessionID, "error", err)
	}
}

// COMMENTED OUT - TODO LIST DISABLED
/*
func parseStructuredSummaryData(raw string, todos []session.Todo) (memory.StructuredSummaryData, error) {
	trimmed := strings.TrimSpace(raw)
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start {
		trimmed = trimmed[start : end+1]
	}

	var data memory.StructuredSummaryData
	if err := json.Unmarshal([]byte(trimmed), &data); err != nil {
		return memory.StructuredSummaryData{}, err
	}

	data.TodoStates = todoStatesFromSessionTodos(todos)
	return data, nil
}

func todoStatesFromSessionTodos(todos []session.Todo) []memory.TodoState {
	states := make([]memory.TodoState, 0, len(todos))
	for _, todo := range todos {
		if !session.IsRenderableTodo(todo) {
			continue
		}
		states = append(states, memory.TodoState{
			Content:      cmp.Or(strings.TrimSpace(todo.Content), strings.TrimSpace(todo.ActiveForm)),
			Status:       string(todo.Status),
			Dependencies: nil,
		})
	}
	return states
}
*/

func parseStructuredSummaryData(raw string, todos interface{}) (memory.StructuredSummaryData, error) {
	trimmed := strings.TrimSpace(raw)
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start {
		trimmed = trimmed[start : end+1]
	}

	var data memory.StructuredSummaryData
	if err := json.Unmarshal([]byte(trimmed), &data); err != nil {
		return memory.StructuredSummaryData{}, err
	}

	if sessionTodos, ok := todos.([]session.Todo); ok && len(sessionTodos) > 0 {
		data.TodoStates = todoStatesFromSessionTodos(sessionTodos)
	}
	return data, nil
}

func todoStatesFromSessionTodos(todos []session.Todo) []memory.TodoState {
	states := make([]memory.TodoState, 0, len(todos))
	for _, todo := range todos {
		if !session.IsRenderableTodo(todo) {
			continue
		}
		states = append(states, memory.TodoState{
			Content:      cmp.Or(strings.TrimSpace(todo.Content), strings.TrimSpace(todo.ActiveForm)),
			Status:       string(todo.Status),
			Dependencies: nil,
		})
	}
	return states
}

func buildSessionContinuityInjection(structured *memory.StructuredSummaryData, todos interface{}) string {
	if structured == nil {
		structured = &memory.StructuredSummaryData{}
	}

	todoStates := structured.TodoStates
	if len(todoStates) == 0 {
		if sessionTodos, ok := todos.([]session.Todo); ok && len(sessionTodos) > 0 {
			todoStates = todoStatesFromSessionTodos(sessionTodos)
		}
	}

	var sb strings.Builder
	if len(structured.Decisions) > 0 || len(structured.FileChanges) > 0 || len(structured.FailureModes) > 0 || len(todoStates) > 0 {
		sb.WriteString("## SESSION CONTINUITY\n")
		sb.WriteString("Use this as the stable handoff state for provider/model switches and post-compaction recovery.\n")
	}

	if len(todoStates) > 0 {
		sb.WriteString("\n### Active Todo State\n")
		for _, todo := range todoStates {
			if strings.TrimSpace(todo.Content) == "" {
				continue
			}
			fmt.Fprintf(&sb, "- [%s] %s\n", todo.Status, todo.Content)
		}
	}

	if len(structured.Decisions) > 0 {
		sb.WriteString("\n### Key Decisions\n")
		for _, decision := range structured.Decisions {
			if strings.TrimSpace(decision.Decision) == "" {
				continue
			}
			fmt.Fprintf(&sb, "- %s", decision.Decision)
			if decision.File != "" {
				fmt.Fprintf(&sb, " (%s)", decision.File)
			}
			if decision.Rationale != "" {
				fmt.Fprintf(&sb, " — %s", decision.Rationale)
			}
			sb.WriteString("\n")
		}
	}

	if len(structured.FileChanges) > 0 {
		sb.WriteString("\n### Recent File Changes\n")
		for _, change := range structured.FileChanges {
			if strings.TrimSpace(change.File) == "" || strings.TrimSpace(change.SemanticChange) == "" {
				continue
			}
			fmt.Fprintf(&sb, "- %s: %s\n", change.File, change.SemanticChange)
		}
	}

	if len(structured.FailureModes) > 0 {
		sb.WriteString("\n### Known Failure Modes\n")
		for _, failure := range structured.FailureModes {
			if strings.TrimSpace(failure.Issue) == "" {
				continue
			}
			fmt.Fprintf(&sb, "- %s", failure.Issue)
			if failure.Resolution != "" {
				fmt.Fprintf(&sb, " — %s", failure.Resolution)
			}
			sb.WriteString("\n")
		}
	}

	return strings.TrimSpace(sb.String())
}

// SessionID returns the current session ID (for plan mode filtering)
func (a *sessionAgent) SessionID() string {
	return a.sessionID
}

// setSessionID sets the current session ID (for plan mode filtering)
func (a *sessionAgent) setSessionID(sessionID string) {
	a.sessionID = sessionID
}
