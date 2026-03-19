package agent

import (
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
	publishSubAgentLifecycleEvent(eventType, payload)
}

func (r *subAgentRunner) lifecycleEventLocked(submissionID string, stage SubAgentLifecycleStage, errMsg string) SubAgentLifecycleEvent {
	payload := SubAgentLifecycleEvent{
		AgentID:         r.id,
		SessionID:       r.sessionID,
		ParentSessionID: r.parentSession,
		SubmissionID:    submissionID,
		Stage:           stage,
		Status:          r.status,
		Task:            r.assignment.Task,
		TaskKey:         r.assignment.TaskKey,
		Domains:         append([]string{}, r.assignment.Domains...),
		Result:          r.lastResult,
		Progress:        r.lastProgress,
		Error:           r.lastError,
		Pending:         r.pending,
		Timestamp:       time.Now(),
	}
	if errMsg != "" {
		payload.Error = errMsg
	}
	return payload
}
