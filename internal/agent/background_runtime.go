package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/fantasy"
	agentbackground "github.com/duggal1/Sapphire-cli/internal/agent/background"
	"github.com/duggal1/Sapphire-cli/internal/config"
)

func (c *coordinator) backgroundConcurrencyLimit() int {
	const defaultMaxConcurrentBackgroundSubAgents = 10
	if c == nil {
		return defaultMaxConcurrentBackgroundSubAgents
	}
	if limit := cap(c.backgroundSubAgentLimiter); limit > 0 {
		if limit > defaultMaxConcurrentBackgroundSubAgents {
			return defaultMaxConcurrentBackgroundSubAgents
		}
		return limit
	}
	return defaultMaxConcurrentBackgroundSubAgents
}

func (c *coordinator) DispatchBackground(ctx context.Context, spec agentbackground.TaskSpec) (string, error) {
	if c == nil || c.backgroundDispatcher == nil {
		return "", fmt.Errorf("background dispatcher is not initialized")
	}
	if err := c.addBackgroundTasks(context.Background(), spec.SessionID, 1); err != nil {
		return "", err
	}
	agentID, err := c.backgroundDispatcher.Dispatch(ctx, spec)
	if err != nil {
		c.completeBackgroundTasks(context.Background(), spec.SessionID, 1)
		return "", err
	}
	c.recordBackgroundTaskLaunched(context.Background(), spec.SessionID, spec, agentID)
	return agentID, nil
}

func (c *coordinator) GetBackgroundStatus(agentID string) (agentbackground.SubAgent, bool) {
	if c == nil || c.backgroundDispatcher == nil {
		return agentbackground.SubAgent{}, false
	}
	return c.backgroundDispatcher.Get(agentID)
}

func (c *coordinator) ListBackgroundAgents() []agentbackground.SubAgent {
	if c == nil || c.backgroundRegistry == nil {
		return nil
	}
	return c.backgroundRegistry.ListAll()
}

func (c *coordinator) WaitForCompletion(ctx context.Context, agentIDs []string) ([]agentbackground.SubAgent, error) {
	if c == nil || c.backgroundDispatcher == nil {
		return nil, fmt.Errorf("background dispatcher is not initialized")
	}
	return c.backgroundDispatcher.WaitForCompletion(ctx, agentIDs)
}

func (c *coordinator) executeBackgroundSubAgent(ctx context.Context, spec agentbackground.TaskSpec) (agentbackground.ExecutionResult, error) {
	if c == nil {
		return agentbackground.ExecutionResult{}, fmt.Errorf("coordinator is nil")
	}
	parentSessionID := strings.TrimSpace(spec.ParentSessionID)
	if parentSessionID == "" {
		parentSessionID = strings.TrimSpace(spec.SessionID)
	}
	control := c.subAgentControl()
	agentKey := strings.TrimSpace(spec.AgentID)
	if agentKey == "" {
		agentKey = config.AgentTask
	}
	var customTools []fantasy.AgentTool
	if spec.ReadOnly {
		allowed := append([]string{}, spec.AllowedTools...)
		if len(allowed) == 0 {
			allowed = agentbackground.DefaultPlanModeRestrictor().AllowedTools
		}
		workingDir := strings.TrimSpace(spec.WorktreePath)
		if workingDir == "" {
			workingDir = c.mainWorkingDir()
		}
		var err error
		customTools, err = c.buildReadOnlyExplorationTools(ctx, workingDir, allowed)
		if err != nil {
			return agentbackground.ExecutionResult{}, err
		}
	}
	agentID, submissionID, err := control.spawn(ctx, parentSessionID, spawnAgentOptions{
		WorkItemID:       spec.WorkItemID,
		Prompt:           spec.Prompt,
		PromptItems:      append([]string{}, spec.PromptItems...),
		Title:            spec.Title,
		Worktree:         spec.Worktree,
		WorktreePath:     spec.WorktreePath,
		Branch:           spec.Branch,
		WriteManifest:    append([]string{}, spec.WriteManifest...),
		DefinitionOfDone: spec.DefinitionOfDone,
		TestCommand:      spec.TestCommand,
		AgentID:          agentKey,
		Model:            spec.Model,
		ReasoningEffort:  spec.ReasoningEffort,
		ForkContext:      spec.ForkContext,
		CustomTools:      customTools,
	})
	if err != nil {
		return agentbackground.ExecutionResult{}, err
	}
	waitCtx, cancel := context.WithTimeout(ctx, backgroundSubAgentTimeout)
	defer cancel()
	statuses, timedOut := control.wait(waitCtx, []string{agentID}, backgroundSubAgentTimeout)
	results := control.collectResult([]string{agentID})
	defer func() { _ = control.close(agentID) }()
	if timedOut {
		return agentbackground.ExecutionResult{RuntimeAgentID: agentID, SubmissionID: submissionID}, fmt.Errorf("background sub-agent %q timed out after %s", spec.Name, backgroundSubAgentTimeout)
	}
	if len(statuses) == 0 || len(results) == 0 {
		return agentbackground.ExecutionResult{RuntimeAgentID: agentID, SubmissionID: submissionID}, fmt.Errorf("background sub-agent %q did not report a final snapshot", spec.Name)
	}
	result := results[0]
	content, runErr := backgroundCollectedResultContent(spec.Name, result)
	if runErr != nil {
		return agentbackground.ExecutionResult{RuntimeAgentID: agentID, SubmissionID: submissionID}, runErr
	}
	return agentbackground.ExecutionResult{
		RuntimeAgentID: agentID,
		SubmissionID:   submissionID,
		Result:         content,
	}, nil
}

func (c *coordinator) handleBackgroundCompletion(ctx context.Context, agent agentbackground.SubAgent) {
	if c == nil {
		return
	}
	c.recordBackgroundTaskCompleted(ctx, agent.SessionID, agent)
	c.completeBackgroundTasks(context.Background(), agent.SessionID, 1)
}

func backgroundCollectedResultContent(taskName string, result subAgentCollectedResult) (string, error) {
	switch {
	case strings.TrimSpace(result.Error) != "":
		return "", fmt.Errorf("%s", result.Error)
	case strings.TrimSpace(result.Result) != "":
		return result.Result, nil
	case strings.TrimSpace(result.Progress) != "":
		return result.Progress, nil
	default:
		return "completed without a summary", nil
	}
}

func backgroundTaskSpecFromAgentParams(sessionID string, params AgentParams) agentbackground.TaskSpec {
	useWorktree := true
	if params.Worktree != nil {
		useWorktree = *params.Worktree
	}
	return agentbackground.TaskSpec{
		SessionID:        sessionID,
		ParentSessionID:  sessionID,
		Name:             "agent",
		Prompt:           params.Prompt,
		Title:            "Background Agent Session",
		Worktree:         useWorktree,
		WorktreePath:     params.WorktreePath,
		Branch:           params.Branch,
		WriteManifest:    append([]string{}, params.WriteManifest...),
		DefinitionOfDone: params.DefinitionOfDone,
		AgentID:          config.AgentTask,
	}
}

func backgroundTaskSpecFromAutonomousTask(sessionID string, task autonomousSubAgentTask) agentbackground.TaskSpec {
	return agentbackground.TaskSpec{
		SessionID:       sessionID,
		ParentSessionID: sessionID,
		Name:            task.Name,
		Prompt:          task.Prompt,
		Title:           task.SessionTitle,
		Worktree:        true,
		AgentID:         config.AgentTask,
	}
}

func (c *coordinator) monitorBackgroundSubAgent(ctx context.Context, sessionID, taskName, agentID string) {
	if c == nil || c.backgroundMonitor == nil || c.backgroundDispatcher == nil {
		return
	}
	waitCtx, cancel := context.WithTimeout(ctx, backgroundSubAgentTimeout+(30*time.Second))
	defer cancel()
	_, _ = c.backgroundDispatcher.WaitForCompletion(waitCtx, []string{agentID})
}
