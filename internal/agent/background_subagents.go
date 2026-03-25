package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	agentbackground "github.com/duggal1/Sapphire-cli/internal/agent/background"
	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/google/uuid"
)

const BackgroundSubAgentsToolName = "background_subagents"
const maxBackgroundSubAgents = 100

const backgroundSubAgentTimeout = 10 * time.Minute

type backgroundIndicatorState struct {
	count               int
	total               int
	toolCallMessageID   string
	toolResultMessageID string
	done                chan struct{}
	title               string
	agents              map[string]BackgroundSubAgentView
	order               []string
	lastToolCallInput   string
	lastToolResult      string
	lastFinished        bool
	lastErrorState      bool
}

func (c *coordinator) acquireBackgroundSubAgentSlot() {
	c.backgroundSubAgentLimiter <- struct{}{}
}

func (c *coordinator) releaseBackgroundSubAgentSlot() {
	<-c.backgroundSubAgentLimiter
}

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

	go func() {
		bgCtx := context.Background()

		if err := c.addBackgroundTasks(bgCtx, sessionID, len(tasks)); err != nil {
			slog.Error("Failed to publish background sub-agent indicator", "error", err)
		}

		for _, task := range tasks {
			task := task
			go func() {
				if c.backgroundDispatcher == nil {
					c.publishBackgroundSubAgentResult(bgCtx, sessionID, task.Name, "", fmt.Errorf("background dispatcher is not initialized"))
					c.completeBackgroundTasks(bgCtx, sessionID, 1)
					return
				}
				if _, err := c.backgroundDispatcher.Dispatch(bgCtx, backgroundTaskSpecFromAutonomousTask(sessionID, task)); err != nil {
					slog.Error("Failed to dispatch background sub-agent", "error", err)
					c.publishBackgroundSubAgentResult(bgCtx, sessionID, task.Name, "", err)
					c.completeBackgroundTasks(bgCtx, sessionID, 1)
				}
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
		state.total += taskCount
		if err := c.updateBackgroundIndicatorMessagesLocked(ctx, sessionID, state, false); err != nil {
			slog.Error("Failed to update background sub-agent indicator", "error", err)
		}
		c.backgroundIndicatorMu.Unlock()
		return nil
	}
	c.backgroundIndicatorMu.Unlock()

	input, _ := json.Marshal(BackgroundSubAgentsToolInput{
		Count: taskCount,
		Title: "Background Sub-Agents",
	})
	toolCallID := "background-indicator-" + uuid.New().String()

	toolCallMsg, err := c.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{
				ID:       toolCallID,
				Name:     BackgroundSubAgentsToolName,
				Input:    string(input),
				Finished: false,
			},
		},
	})
	if err != nil {
		return err
	}

	initialPayload, _ := json.Marshal(BackgroundSubAgentsToolPayload{
		Status: "launching",
		Title:  "Background Sub-Agents",
		Count:  taskCount,
		Active: taskCount,
	})
	toolResultMsg, err := c.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role: message.Tool,
		Parts: []message.ContentPart{
			message.ToolResult{
				ToolCallID: toolCallID,
				Name:       BackgroundSubAgentsToolName,
				Content:    string(initialPayload),
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
		total:               taskCount,
		toolCallMessageID:   toolCallMsg.ID,
		toolResultMessageID: toolResultMsg.ID,
		done:                make(chan struct{}),
		title:               "Background Sub-Agents",
		agents:              make(map[string]BackgroundSubAgentView),
	}
	c.backgroundIndicatorMu.Unlock()
	return nil
}

func (c *coordinator) completeBackgroundTasks(ctx context.Context, sessionID string, taskCount int) {
	if taskCount <= 0 {
		return
	}

	c.backgroundIndicatorMu.Lock()
	state, ok := c.backgroundIndicators[sessionID]
	if !ok {
		c.backgroundIndicatorMu.Unlock()
		return
	}
	state.count -= taskCount
	finished := state.count <= 0
	if finished {
		state.count = 0
		delete(c.backgroundIndicators, sessionID)
	}
	if err := c.updateBackgroundIndicatorMessagesLocked(ctx, sessionID, state, finished); err != nil {
		slog.Error("Failed to finalize background sub-agent indicator", "error", err)
	}
	done := state.done
	c.backgroundIndicatorMu.Unlock()

	if finished && done != nil {
		close(done)
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

func (c *coordinator) publishBackgroundSubAgentResult(ctx context.Context, sessionID, taskName, content string, runErr error) {
	c.backgroundIndicatorMu.Lock()
	defer c.backgroundIndicatorMu.Unlock()

	state, ok := c.backgroundIndicators[sessionID]
	if !ok {
		return
	}

	entryID := taskName
	if entryID == "" {
		entryID = fmt.Sprintf("background-%d", len(state.order)+1)
	}
	entry, exists := state.agents[entryID]
	if !exists {
		entry = BackgroundSubAgentView{
			ID:     entryID,
			Name:   taskName,
			Title:  taskName,
			Status: "failed",
		}
		state.order = append(state.order, entryID)
	}
	entry.Name = firstNonEmptyBackgroundValue(taskName, entry.Name)
	entry.Title = firstNonEmptyBackgroundValue(taskName, entry.Title)
	entry.Result = strings.TrimSpace(content)
	entry.Preview = strings.TrimSpace(content)
	entry.Status = "completed"
	if runErr != nil {
		entry.Error = runErr.Error()
		entry.Status = "failed"
	}
	state.agents[entryID] = entry
	if err := c.updateBackgroundIndicatorMessagesLocked(ctx, sessionID, state, false); err != nil {
		slog.Error("Failed to update background sub-agent result", "error", err)
	}
}

func (c *coordinator) recordBackgroundTaskLaunched(ctx context.Context, sessionID string, spec agentbackground.TaskSpec, agentID string) {
	c.backgroundIndicatorMu.Lock()
	defer c.backgroundIndicatorMu.Unlock()

	state, ok := c.backgroundIndicators[sessionID]
	if !ok {
		return
	}
	entry, exists := state.agents[agentID]
	if !exists {
		state.order = append(state.order, agentID)
	}
	entry.ID = agentID
	entry.Name = firstNonEmptyBackgroundValue(spec.Name, entry.Name)
	entry.Title = firstNonEmptyBackgroundValue(spec.Title, entry.Title, spec.Name)
	entry.LegType = firstNonEmptyBackgroundValue(string(spec.LegType), entry.LegType)
	entry.WorkDir = firstNonEmptyBackgroundValue(spec.WorktreePath, entry.WorkDir)
	entry.Branch = firstNonEmptyBackgroundValue(spec.Branch, entry.Branch)
	if entry.StartedAt.IsZero() {
		entry.StartedAt = time.Now().UTC()
	}
	entry.Status = "queued"
	state.agents[agentID] = entry
	if err := c.updateBackgroundIndicatorMessagesLocked(ctx, sessionID, state, false); err != nil {
		slog.Error("Failed to record background sub-agent launch", "error", err)
	}
}

func (c *coordinator) recordBackgroundTaskCompleted(ctx context.Context, sessionID string, agent agentbackground.SubAgent) {
	c.backgroundIndicatorMu.Lock()
	defer c.backgroundIndicatorMu.Unlock()

	state, ok := c.backgroundIndicators[sessionID]
	if !ok {
		return
	}

	entry, exists := state.agents[agent.ID]
	if !exists {
		state.order = append(state.order, agent.ID)
	}
	entry.ID = agent.ID
	entry.Name = firstNonEmptyBackgroundValue(agent.Name, agent.Task.Name, entry.Name)
	entry.Title = firstNonEmptyBackgroundValue(agent.Task.Title, agent.Name, entry.Title)
	entry.LegType = firstNonEmptyBackgroundValue(string(agent.Task.LegType), entry.LegType)
	entry.WorkDir = firstNonEmptyBackgroundValue(agent.Task.WorktreePath, entry.WorkDir)
	entry.Branch = firstNonEmptyBackgroundValue(agent.Task.Branch, entry.Branch)
	entry.StartedAt = firstNonZeroTime(agent.StartedAt, entry.StartedAt)
	entry.Status = strings.TrimSpace(string(agent.Status))
	entry.Result = strings.TrimSpace(agent.Result)
	entry.Preview = strings.TrimSpace(agent.Result)
	entry.Error = strings.TrimSpace(agent.Error)
	state.agents[agent.ID] = entry
	if err := c.updateBackgroundIndicatorMessagesLocked(ctx, sessionID, state, false); err != nil {
		slog.Error("Failed to record background sub-agent completion", "error", err)
	}
}

func (c *coordinator) updateBackgroundIndicatorMessagesLocked(ctx context.Context, sessionID string, state *backgroundIndicatorState, finished bool) error {
	if c == nil || c.messages == nil || state == nil {
		return nil
	}
	payload := c.buildBackgroundIndicatorPayloadLocked(state, finished)
	resultJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	if state.toolResultMessageID != "" {
		resultContent := string(resultJSON)
		resultIsError := payload.Failed > 0 && finished
		if state.lastToolResult == resultContent && state.lastErrorState == resultIsError {
			goto update_tool_call
		}
		resultMsg, err := c.messages.Get(ctx, state.toolResultMessageID)
		if err == nil {
			for i, part := range resultMsg.Parts {
				res, ok := part.(message.ToolResult)
				if !ok || res.Name != BackgroundSubAgentsToolName {
					continue
				}
				res.Content = resultContent
				res.IsError = resultIsError
				resultMsg.Parts[i] = res
				break
			}
			if err := c.messages.Update(ctx, resultMsg); err != nil {
				return err
			}
			state.lastToolResult = resultContent
			state.lastErrorState = resultIsError
		}
	}

update_tool_call:
	if state.toolCallMessageID != "" {
		inputJSON, _ := json.Marshal(BackgroundSubAgentsToolInput{
			Count: state.total,
			Title: firstNonEmptyBackgroundValue(state.title, payload.Title),
		})
		inputContent := string(inputJSON)
		if state.lastToolCallInput == inputContent && state.lastFinished == finished {
			return nil
		}
		callMsg, err := c.messages.Get(ctx, state.toolCallMessageID)
		if err == nil {
			for i, part := range callMsg.Parts {
				call, ok := part.(message.ToolCall)
				if !ok || call.Name != BackgroundSubAgentsToolName {
					continue
				}
				call.Input = inputContent
				call.Finished = finished
				callMsg.Parts[i] = call
				break
			}
			if err := c.messages.Update(ctx, callMsg); err != nil {
				return err
			}
			state.lastToolCallInput = inputContent
			state.lastFinished = finished
		}
	}

	return nil
}

func (c *coordinator) buildBackgroundIndicatorPayloadLocked(state *backgroundIndicatorState, finished bool) BackgroundSubAgentsToolPayload {
	payload := BackgroundSubAgentsToolPayload{
		Status: "running",
		Title:  firstNonEmptyBackgroundValue(state.title, "Background Sub-Agents"),
		Count:  state.total,
		Active: state.count,
	}
	if state.total == 0 {
		payload.Count = len(state.agents)
	}
	ordered := make([]BackgroundSubAgentView, 0, len(state.agents))
	for _, id := range state.order {
		entry, ok := state.agents[id]
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(entry.Status)) {
		case "completed":
			payload.Completed++
		case "failed", "error":
			payload.Completed++
			payload.Failed++
		}
		ordered = append(ordered, entry)
	}
	if len(ordered) < len(state.agents) {
		keys := make([]string, 0, len(state.agents))
		for id := range state.agents {
			if !containsBackgroundID(state.order, id) {
				keys = append(keys, id)
			}
		}
		sort.Strings(keys)
		for _, id := range keys {
			ordered = append(ordered, state.agents[id])
		}
	}
	payload.Agents = ordered
	switch {
	case finished && payload.Failed > 0:
		payload.Status = "failed"
	case finished:
		payload.Status = "completed"
	case payload.Completed > 0:
		payload.Status = "running"
	default:
		payload.Status = "launching"
	}
	return payload
}

func containsBackgroundID(ids []string, target string) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

func firstNonEmptyBackgroundValue(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func firstNonZeroTime(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value
		}
	}
	return time.Time{}
}
