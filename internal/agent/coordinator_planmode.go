package agent

import (
	"context"
	_ "embed"
	"fmt"
	"regexp"
	"strings"
	"time"

	"charm.land/fantasy"
	agentbackground "github.com/duggal1/Sapphire-cli/internal/agent/background"
	agentformula "github.com/duggal1/Sapphire-cli/internal/agent/formula"
	"github.com/duggal1/Sapphire-cli/internal/agent/planmode"
	promptpkg "github.com/duggal1/Sapphire-cli/internal/agent/prompt"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/duggal1/Sapphire-cli/internal/session"
)

//go:embed formulas/plan-mode.formula.toml
var planModeFormulaBytes []byte

var nonSlugChars = regexp.MustCompile(`[^a-z0-9]+`)

func (c *coordinator) initPlanMode() error {
	registry, err := tools.NewPlanModeRegistry(tools.PlanModeRegistryOptions{
		WorkingDir:        c.mainWorkingDir(),
		Sessions:          c.sessions,
		OnQuestions:       c.recordPlanQuestions,
		LaunchExploration: c.launchExplorationAgentForPrompt,
	})
	if err != nil {
		return fmt.Errorf("initialize plan mode registry: %w", err)
	}
	parsed, err := agentformula.Parse(planModeFormulaBytes)
	if err != nil {
		return fmt.Errorf("parse embedded plan mode formula: %w", err)
	}
	c.toolRegistry = registry
	c.formulaExecutor = &agentformula.Executor{
		Formula:            parsed,
		ToolRegistry:       registry,
		LLM:                planModeLLMClient{coordinator: c},
		WorkingDir:         c.mainWorkingDir(),
		UpdateProgress:     c.updatePlanModeProgress,
		Approve:            c.awaitPlanModeApproval,
		WaitForExploration: c.WaitForExploration,
	}
	return nil
}

func (c *coordinator) RunPlanMode(ctx context.Context, sessionID, task, taskContext string) (*agentformula.ExecutionState, error) {
	if c == nil || c.formulaExecutor == nil {
		return nil, fmt.Errorf("plan mode executor is not initialized")
	}
	variables := map[string]string{
		"session_id": sessionID,
		"task":       strings.TrimSpace(task),
		"context":    strings.TrimSpace(taskContext),
		"task_slug":  slugifyTask(task),
	}
	return c.formulaExecutor.Execute(ctx, variables)
}

func (c *coordinator) LaunchExplorationAgent(ctx context.Context, workItemID string) (string, error) {
	return c.launchExplorationAgentForPrompt(ctx, tools.ExplorationLaunchRequest{
		Prompt:     strings.TrimSpace(workItemID),
		WorkItemID: strings.TrimSpace(workItemID),
		Title:      "Plan Mode Exploration",
	})
}

func (c *coordinator) launchExplorationAgentForPrompt(ctx context.Context, req tools.ExplorationLaunchRequest) (string, error) {
	if c == nil || c.backgroundDispatcher == nil {
		return "", fmt.Errorf("background dispatcher is not initialized")
	}
	sessionID := tools.GetSessionFromContext(ctx)
	if sessionID == "" {
		return "", fmt.Errorf("session ID is required")
	}
	restrictor := agentbackground.DefaultPlanModeRestrictor()
	spec := agentbackground.TaskSpec{
		SessionID:       sessionID,
		ParentSessionID: sessionID,
		WorkItemID:      req.WorkItemID,
		Name:            "exploration",
		Prompt:          req.Prompt,
		Title:           firstNonEmptyString(req.Title, "Plan Mode Exploration"),
		Worktree:        false,
		AgentID:         config.AgentTask,
		Model:           req.Model,
		ReasoningEffort: req.ReasoningEffort,
		ReadOnly:        true,
		AllowedTools:    append([]string{}, restrictor.AllowedTools...),
	}
	return c.DispatchBackground(ctx, spec)
}

func (c *coordinator) WaitForExploration(ctx context.Context, agentIDs []string) ([]agentformula.ExplorationResult, error) {
	agents, err := c.WaitForCompletion(ctx, agentIDs)
	if err != nil {
		return nil, err
	}
	results := make([]agentformula.ExplorationResult, 0, len(agents))
	for _, agent := range agents {
		results = append(results, agentformula.ExplorationResult{
			AgentID: agent.ID,
			Status:  string(agent.Status),
			Result:  agent.Result,
			Error:   agent.Error,
		})
	}
	return results, nil
}

func (c *coordinator) recordPlanQuestions(ctx context.Context, questions []tools.PlanQuestion) error {
	if c == nil || c.messages == nil {
		return nil
	}
	sessionID := tools.GetSessionFromContext(ctx)
	if sessionID == "" {
		return nil
	}
	var builder strings.Builder
	builder.WriteString("Plan mode needs more input:\n\n")
	for _, question := range questions {
		builder.WriteString(question.Question)
		builder.WriteString("\n")
		for _, option := range question.Options {
			builder.WriteString("- ")
			builder.WriteString(option.Label)
			builder.WriteString(": ")
			builder.WriteString(option.Description)
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}
	_, err := c.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: strings.TrimSpace(builder.String())},
		},
	})
	return err
}

func (c *coordinator) updatePlanModeProgress(ctx context.Context, entries []agentformula.ProgressEntry) error {
	if c == nil || c.sessions == nil {
		return nil
	}
	sessionID := tools.GetSessionFromContext(ctx)
	if sessionID == "" {
		return nil
	}
	currentSession, err := c.sessions.Get(ctx, sessionID)
	if err != nil {
		return err
	}
	currentSession.Todos = make([]session.Todo, 0, len(entries))
	for i, entry := range entries {
		currentSession.Todos = append(currentSession.Todos, session.Todo{
			ID:         fmt.Sprintf("formula-step-%d", i),
			Content:    entry.Step,
			Status:     session.TodoStatus(entry.Status),
			ActiveForm: entry.Step,
		})
	}
	_, err = c.sessions.Save(ctx, currentSession)
	return err
}

func (c *coordinator) awaitPlanModeApproval(ctx context.Context, state *agentformula.ExecutionState) (bool, error) {
	if c == nil || c.sessions == nil {
		return false, fmt.Errorf("session service is not initialized")
	}
	if isNonInteractiveMode() {
		return false, fmt.Errorf("plan mode approval requires interactive mode")
	}
	if err := c.sessions.SetMode(ctx, state.SessionID, planmode.PlanMode); err != nil {
		return false, err
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-ticker.C:
			mode, err := c.sessions.GetMode(ctx, state.SessionID)
			if err != nil {
				return false, err
			}
			if mode.IsExecutionMode() {
				return true, nil
			}
		}
	}
}

type planModeLLMClient struct {
	coordinator *coordinator
}

func (c planModeLLMClient) ExecuteStep(ctx context.Context, req agentformula.LLMRequest) (agentformula.LLMResult, error) {
	if c.coordinator == nil {
		return agentformula.LLMResult{}, fmt.Errorf("coordinator is nil")
	}
	agentCfg, ok := c.coordinator.cfg.Agents[config.AgentCoder]
	if !ok {
		return agentformula.LLMResult{}, fmt.Errorf("coder agent is not configured")
	}
	prompt, err := coderPrompt(promptpkg.WithWorkingDir(c.coordinator.mainWorkingDir()))
	if err != nil {
		return agentformula.LLMResult{}, err
	}

	var override *agentModelOverride
	if strings.TrimSpace(req.ReasoningEffort) != "" {
		override, err = c.coordinator.resolveSubAgentModelOverride("", req.ReasoningEffort)
		if err != nil {
			return agentformula.LLMResult{}, err
		}
	}

	agent, err := c.coordinator.buildAgentWithWorkingDirOverrides(ctx, prompt, agentCfg, false, c.coordinator.mainWorkingDir(), override, nil)
	if err != nil {
		return agentformula.LLMResult{}, err
	}
	if !req.UseDefaultTools && c.coordinator.toolRegistry != nil {
		agent.SetTools(c.coordinator.toolRegistry.AgentTools(req.ToolNames...))
	}

	result, err := agent.Run(ctx, SessionAgentCall{
		SessionID: req.SessionID,
		Prompt:    req.Prompt,
	})
	if err != nil {
		return agentformula.LLMResult{}, err
	}
	return agentformula.LLMResult{Output: extractAgentResultText(result)}, nil
}

func extractAgentResultText(result *fantasy.AgentResult) string {
	if result == nil {
		return ""
	}
	return strings.TrimSpace(result.Response.Content.Text())
}

func slugifyTask(task string) string {
	slug := strings.ToLower(strings.TrimSpace(task))
	slug = nonSlugChars.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return "plan-mode-task"
	}
	return slug
}
