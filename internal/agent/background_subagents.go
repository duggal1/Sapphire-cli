package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/charmbracelet/sapphire/internal/config"
	"github.com/charmbracelet/sapphire/internal/message"
	"github.com/google/uuid"
)

const BackgroundSubAgentsToolName = "background_subagents"
const maxBackgroundSubAgents = 100

// Maximum time a single background sub-agent can run before being cancelled.
const backgroundSubAgentTimeout = 10 * time.Minute

type backgroundIndicatorState struct {
	count               int
	toolCallMessageID   string
	toolResultMessageID string
	done                chan struct{}
}

func (c *coordinator) acquireBackgroundSubAgentSlot() {
	c.backgroundSubAgentLimiter <- struct{}{}
}

func (c *coordinator) releaseBackgroundSubAgentSlot() {
	<-c.backgroundSubAgentLimiter
}

// autonomousSubAgentContextMaybeBackground launches sub-agents entirely in the
// background. Nothing in this function blocks coordinator.Run(). The agent
// build, system prompt construction, and sub-agent execution all happen in a
// detached goroutine. Results are delivered asynchronously as tool-call/result
// message pairs into the session.
func (c *coordinator) autonomousSubAgentContextMaybeBackground(ctx context.Context, sessionID, userPrompt string) (string, error) {
	tasks := buildAutonomousSubAgentTasks(userPrompt)
	if len(tasks) == 0 {
		return "", nil
	}

	if err := c.launchBackgroundAutonomousSubAgents(ctx, sessionID, userPrompt, tasks); err != nil {
		return "", err
	}
	return "", nil
}

func (c *coordinator) launchBackgroundAutonomousSubAgents(ctx context.Context, sessionID, _ string, tasks []autonomousSubAgentTask) error {
	if _, ok := c.cfg.Agents[config.AgentTask]; !ok {
		return fmt.Errorf("task agent not configured")
	}

	// Run detached so coordinator.Run() is not blocked.
	go func() {
		bgCtx := context.Background()

		if err := c.addBackgroundTasks(bgCtx, sessionID, len(tasks)); err != nil {
			slog.Error("Failed to publish background sub-agent indicator", "error", err)
		}

		for _, task := range tasks {
			task := task
			go func() {
				agentID, _, err := c.spawnSubAgent(bgCtx, sessionID, spawnAgentOptions{
					Prompt:   task.Prompt,
					Title:    task.SessionTitle,
					Worktree: true,
					AgentID:  config.AgentTask,
				})
				if err != nil {
					slog.Error("Failed to spawn background sub-agent", "error", err)
					c.publishBackgroundSubAgentResult(bgCtx, sessionID, task.Name, "", err)
					c.completeBackgroundTasks(bgCtx, sessionID, 1)
					return
				}
				c.monitorBackgroundSubAgent(bgCtx, sessionID, task.Name, agentID)
			}()
		}
	}()

	return nil
}

func (c *coordinator) addBackgroundTasks(ctx context.Context, sessionID string, taskCount int) error {
	if taskCount <= 0 {
		return nil
	}

	c.backgroundIndicatorMu.Lock()
	state, ok := c.backgroundIndicators[sessionID]
	if ok {
		state.count += taskCount
		c.backgroundIndicatorMu.Unlock()
		return nil
	}
	c.backgroundIndicatorMu.Unlock()

	input, _ := json.Marshal(map[string]int{"count": taskCount})
	toolCallID := "background-indicator-" + uuid.New().String()

	toolCallMsg, err := c.messages.Create(ctx, sessionID, message.CreateMessageParams{
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

	toolResultMsg, err := c.messages.Create(ctx, sessionID, message.CreateMessageParams{
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
	if err != nil {
		_ = c.messages.Delete(ctx, toolCallMsg.ID)
		return err
	}

	c.backgroundIndicatorMu.Lock()
	if state, ok := c.backgroundIndicators[sessionID]; ok {
		state.count += taskCount
		c.backgroundIndicatorMu.Unlock()
		_ = c.messages.Delete(ctx, toolCallMsg.ID)
		_ = c.messages.Delete(ctx, toolResultMsg.ID)
		return nil
	}
	c.backgroundIndicators[sessionID] = &backgroundIndicatorState{
		count:               taskCount,
		toolCallMessageID:   toolCallMsg.ID,
		toolResultMessageID: toolResultMsg.ID,
		done:                make(chan struct{}),
	}
	c.backgroundIndicatorMu.Unlock()
	return nil
}

func (c *coordinator) completeBackgroundTasks(ctx context.Context, sessionID string, taskCount int) {
	if taskCount <= 0 {
		return
	}

	var (
		toDelete []string
		done     chan struct{}
	)
	c.backgroundIndicatorMu.Lock()
	state, ok := c.backgroundIndicators[sessionID]
	if !ok {
		c.backgroundIndicatorMu.Unlock()
		return
	}
	state.count -= taskCount
	if state.count > 0 {
		c.backgroundIndicatorMu.Unlock()
		return
	}
	delete(c.backgroundIndicators, sessionID)
	done = state.done
	toDelete = []string{state.toolCallMessageID, state.toolResultMessageID}
	c.backgroundIndicatorMu.Unlock()

	if done != nil {
		close(done)
	}
	for _, id := range toDelete {
		if id == "" {
			continue
		}
		if err := c.messages.Delete(ctx, id); err != nil {
			slog.Error("Failed to delete background sub-agent indicator", "error", err)
		}
	}
}

func (c *coordinator) waitForBackgroundSubAgents(ctx context.Context, sessionID string) {
	c.backgroundIndicatorMu.Lock()
	state, ok := c.backgroundIndicators[sessionID]
	if !ok {
		c.backgroundIndicatorMu.Unlock()
		return
	}
	done := state.done
	c.backgroundIndicatorMu.Unlock()
	if done == nil {
		return
	}
	select {
	case <-done:
	case <-ctx.Done():
	}
}

func (c *coordinator) pollBackgroundSubAgents(sessionID string) {
	c.backgroundIndicatorMu.Lock()
	state, ok := c.backgroundIndicators[sessionID]
	if !ok {
		c.backgroundIndicatorMu.Unlock()
		return
	}
	done := state.done
	c.backgroundIndicatorMu.Unlock()
	if done == nil {
		return
	}
	select {
	case <-done:
	default:
	}
}

func (c *coordinator) monitorBackgroundSubAgent(ctx context.Context, sessionID, taskName, agentID string) {
	waitCtx, cancel := context.WithTimeout(ctx, backgroundSubAgentTimeout)
	defer cancel()
	snapshots, timedOut := c.waitSubAgents(waitCtx, []string{agentID}, backgroundSubAgentTimeout)
	var (
		content string
		runErr  error
	)
	if timedOut {
		runErr = fmt.Errorf("background sub-agent %q timed out after %s", taskName, backgroundSubAgentTimeout)
	} else if len(snapshots) == 0 {
		runErr = fmt.Errorf("background sub-agent %q did not report a final snapshot", taskName)
	} else {
		snap := snapshots[0]
		if snap.LastError != "" {
			runErr = fmt.Errorf("%s", snap.LastError)
		} else if snap.LastResult != "" {
			content = snap.LastResult
		} else if snap.LastProgress != "" {
			content = snap.LastProgress
		} else {
			content = "completed without a summary"
		}
	}
	c.publishBackgroundSubAgentResult(ctx, sessionID, taskName, content, runErr)
	_ = c.closeSubAgent(agentID)
	c.completeBackgroundTasks(ctx, sessionID, 1)
}

func (c *coordinator) publishBackgroundSubAgentResult(ctx context.Context, sessionID, taskName, content string, runErr error) {
	isError := false
	if runErr != nil {
		content = runErr.Error()
		isError = true
	}
	if taskName != "" {
		content = fmt.Sprintf("Background sub-agent (%s):\n\n%s", taskName, content)
	}
	if isError {
		content = fmt.Sprintf("Background sub-agent error:\n\n%s", content)
	}

	_, err := c.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: content},
		},
	})
	if err != nil {
		slog.Error("Failed to publish background sub-agent result", "error", err)
	}
}
