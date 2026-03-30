package agent

import (
	"strings"
	"time"

	"github.com/duggal1/Sapphire-cli/internal/pubsub"
)

func (c *coordinator) publishSubAgentEvent(eventType pubsub.EventType, runner *subAgentRunner, submissionID string, stage SubAgentLifecycleStage, errMsg string) {
	if runner == nil {
		return
	}
	runner.mu.Lock()
	payload := runner.lifecycleEventLocked(submissionID, stage, errMsg)
	runner.mu.Unlock()
	c.countSubAgentLaunchMetric("event.subagent_lifecycle_publish", 1)
	publishSubAgentLifecycleEvent(eventType, payload)
}

func (r *subAgentRunner) lifecycleEventLocked(submissionID string, stage SubAgentLifecycleStage, errMsg string) SubAgentLifecycleEvent {
	payload := SubAgentLifecycleEvent{
		AgentID:          r.id,
		SessionID:        r.sessionID,
		ParentSessionID:  r.parentSession,
		SubmissionID:     submissionID,
		Stage:            stage,
		Status:           r.status,
		Title:            firstNonEmptyString(strings.TrimSpace(r.assignment.Title), strings.TrimSpace(r.assignment.TaskKey), strings.TrimSpace(r.assignment.Task)),
		WorkDir:          r.workDir,
		StartedAt:        r.assignment.CreatedAt,
		HeartbeatContext: r.heartbeatContext,
		CurrentTool:      r.currentTool,
		LastTool:         r.lastTool,
		ToolCallCount:    r.toolCallCount,
		Task:             r.assignment.Task,
		TaskKey:          r.assignment.TaskKey,
		Domains:          append([]string{}, r.assignment.Domains...),
		Result:           r.lastResult,
		Progress:         r.lastProgress,
		Error:            r.lastError,
		Pending:          r.pending,
		Timestamp:        time.Now(),
	}
	if submission := r.submissions[submissionID]; submission != nil && !submission.StartedAt.IsZero() {
		payload.StartedAt = submission.StartedAt
	}
	if errMsg != "" {
		payload.Error = errMsg
	}
	return payload
}
