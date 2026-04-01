package agent

import (
	"strings"
	"time"

	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
)

func (c *coordinator) OnToolInputStart(sessionID, toolName string) {
	c.recordSubAgentToolTelemetry(sessionID, toolName, false, false)
}

func (c *coordinator) OnToolCall(sessionID, toolName, rawInput string) {
	if c != nil && c.singularity != nil {
		c.singularity.RecordToolCall(sessionID, toolName, rawInput)
	}
	c.recordSubAgentToolTelemetry(sessionID, toolName, true, false)
}

func (c *coordinator) OnToolResult(sessionID, toolName, content, metadata string, isError bool) {
	observedToolName, _ := tools.ResolveObservedToolExecution(toolName, "", metadata)
	if c != nil && c.singularity != nil {
		if execution, ok := tools.ParseRuntimeExecutionMetadata(metadata); ok {
			c.singularity.ReconcileToolExecution(
				sessionID,
				execution.RequestedToolName,
				execution.RequestedInput,
				execution.ExecutedToolName,
				execution.ExecutedInput,
			)
			observedToolName = strings.TrimSpace(execution.ExecutedToolName)
		}
		c.singularity.RecordToolResult(sessionID, observedToolName, content, metadata, isError)
	}
	c.recordSubAgentToolTelemetry(sessionID, observedToolName, false, true)
}

func (c *coordinator) recordSubAgentToolTelemetry(sessionID, toolName string, incrementCount bool, finished bool) {
	if c == nil {
		return
	}
	runner := c.runnerBySessionID(strings.TrimSpace(sessionID))
	if runner == nil {
		return
	}

	toolName = strings.TrimSpace(toolName)
	now := time.Now().UTC()

	runner.mu.Lock()
	if runner.closed {
		runner.mu.Unlock()
		return
	}
	if incrementCount {
		runner.toolCallCount++
	}
	if toolName != "" {
		if finished {
			runner.currentTool = ""
			runner.lastTool = toolName
			runner.heartbeatContext = "completed tool " + toolName
		} else {
			runner.currentTool = toolName
			runner.heartbeatContext = "running tool " + toolName
		}
	}
	runner.lastHeartbeat = now
	runner.assignment.UpdatedAt = now
	if submission := runner.submissions[runner.lastSubmission]; submission != nil && !isSubAgentFinalStatus(submission.Status) {
		submission.HeartbeatAt = now
	}
	payload := runner.lifecycleEventLocked(runner.lastSubmission, SubAgentStageTooling, "")
	runner.mu.Unlock()

	c.countSubAgentLaunchMetric("event.subagent_lifecycle_publish", 1)
	publishSubAgentLifecycleEvent(SubAgentToolingEvent, payload)
}
