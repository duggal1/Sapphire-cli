package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/sapphire/internal/agent/prompt"
	"github.com/charmbracelet/sapphire/internal/config"
	"github.com/charmbracelet/sapphire/internal/message"
	"github.com/google/uuid"
	"log/slog"
)

const BackgroundSubAgentsToolName = "background_subagents"

func shouldRunBackgroundSubAgents(userPrompt string, tasks []autonomousSubAgentTask) bool {
	hasIndependentWorkstreams := len(tasks) >= 3
	sharedState := false
	significantTime := len(tasks) >= 3
	mainHasConcurrentWork := strings.TrimSpace(userPrompt) != ""
	resultsNotBlocking := true
	return hasIndependentWorkstreams && !sharedState && significantTime && mainHasConcurrentWork && resultsNotBlocking
}

func (c *coordinator) autonomousSubAgentContextMaybeBackground(ctx context.Context, sessionID, userPrompt string) (string, error) {
	tasks := buildAutonomousSubAgentTasks(userPrompt)
	if !shouldRunBackgroundSubAgents(userPrompt, tasks) {
		return c.autonomousSubAgentContext(ctx, sessionID, userPrompt)
	}

	if err := c.launchBackgroundAutonomousSubAgents(ctx, sessionID, userPrompt, tasks); err != nil {
		return "", err
	}
	return "", nil
}

func (c *coordinator) launchBackgroundAutonomousSubAgents(ctx context.Context, sessionID, userPrompt string, tasks []autonomousSubAgentTask) error {
	agentCfg, ok := c.cfg.Agents[config.AgentTask]
	if !ok {
		return fmt.Errorf("task agent not configured")
	}

	prompt, err := taskPrompt(prompt.WithWorkingDir(c.cfg.WorkingDir()))
	if err != nil {
		return err
	}

	agent, err := c.buildAgent(ctx, prompt, agentCfg, true)
	if err != nil {
		return err
	}

	if err := c.publishBackgroundSubAgentIndicator(ctx, sessionID, len(tasks)); err != nil {
		return err
	}

	for _, task := range tasks {
		task := task
		go func() {
			resp, runErr := c.runSubAgent(ctx, subAgentParams{
				Agent:          agent,
				SessionID:      sessionID,
				AgentMessageID: "background-sub-agent",
				ToolCallID:     fmt.Sprintf("background-%s-%s", task.Name, uuid.New().String()),
				Prompt:         task.Prompt,
				SessionTitle:   task.SessionTitle,
			})
			c.publishBackgroundSubAgentResult(ctx, sessionID, task, resp, runErr)
		}()
	}

	return nil
}

func (c *coordinator) publishBackgroundSubAgentIndicator(ctx context.Context, sessionID string, taskCount int) error {
	input, _ := json.Marshal(map[string]int{"count": taskCount})
	toolCallID := "background-indicator-" + uuid.New().String()

	_, err := c.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{
				ID:       toolCallID,
				Name:     BackgroundSubAgentsToolName,
				Input:    string(input),
				Finished: true,
			},
		},
	})
	if err != nil {
		return err
	}

	_, err = c.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role: message.Tool,
		Parts: []message.ContentPart{
			message.ToolResult{
				ToolCallID: toolCallID,
				Name:       BackgroundSubAgentsToolName,
				Content:    "running agents in the background",
				IsError:    false,
			},
		},
	})
	return err
}

func (c *coordinator) publishBackgroundSubAgentResult(
	ctx context.Context,
	sessionID string,
	task autonomousSubAgentTask,
	resp fantasy.ToolResponse,
	runErr error,
) {
	input, _ := json.Marshal(map[string]string{"prompt": task.Prompt})
	toolCallID := "background-result-" + uuid.New().String()

	_, err := c.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{
				ID:       toolCallID,
				Name:     AgentToolName,
				Input:    string(input),
				Finished: true,
			},
		},
	})
	if err != nil {
		slog.Error("Failed to publish background sub-agent tool call", "error", err)
		return
	}

	content := resp.Content
	isError := resp.IsError
	if runErr != nil {
		content = runErr.Error()
		isError = true
	}
	if task.Name != "" {
		content = fmt.Sprintf("Background sub-agent (%s):\n\n%s", task.Name, content)
	}

	_, err = c.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role: message.Tool,
		Parts: []message.ContentPart{
			message.ToolResult{
				ToolCallID: toolCallID,
				Name:       AgentToolName,
				Content:    content,
				IsError:    isError,
			},
		},
	})
	if err != nil {
		slog.Error("Failed to publish background sub-agent result", "error", err)
	}
}
