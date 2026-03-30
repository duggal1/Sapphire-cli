package agent

import (
	"context"
	"time"

	"github.com/duggal1/Sapphire-cli/internal/pubsub"
)

type SubAgentLifecycleStage string

const (
	SubAgentStageSpawned       SubAgentLifecycleStage = "spawned"
	SubAgentStageStarting      SubAgentLifecycleStage = "starting"
	SubAgentStageReady         SubAgentLifecycleStage = "ready"
	SubAgentStageRunning       SubAgentLifecycleStage = "running"
	SubAgentStageTooling       SubAgentLifecycleStage = "tooling"
	SubAgentStageDegraded      SubAgentLifecycleStage = "degraded"
	SubAgentStageHeartbeat     SubAgentLifecycleStage = "heartbeat"
	SubAgentStageWaiting       SubAgentLifecycleStage = "waiting"
	SubAgentStageWaitingOnMail SubAgentLifecycleStage = "waiting_on_mail"
	SubAgentStageStuck         SubAgentLifecycleStage = "stuck"
	SubAgentStageTimedOut      SubAgentLifecycleStage = "timed_out"
	SubAgentStageBlocked       SubAgentLifecycleStage = "blocked"
	SubAgentStageCompleted     SubAgentLifecycleStage = "completed"
	SubAgentStageFailed        SubAgentLifecycleStage = "failed"
	SubAgentStageClosed        SubAgentLifecycleStage = "closed"
)

const (
	SubAgentSpawnedEvent       pubsub.EventType = "subagent_spawned"
	SubAgentStartingEvent      pubsub.EventType = "subagent_starting"
	SubAgentReadyEvent         pubsub.EventType = "subagent_ready"
	SubAgentRunningEvent       pubsub.EventType = "subagent_running"
	SubAgentToolingEvent       pubsub.EventType = "subagent_tooling"
	SubAgentDegradedEvent      pubsub.EventType = "subagent_degraded"
	SubAgentHeartbeatEvent     pubsub.EventType = "subagent_heartbeat"
	SubAgentWaitingEvent       pubsub.EventType = "subagent_waiting"
	SubAgentWaitingOnMailEvent pubsub.EventType = "subagent_waiting_on_mail"
	SubAgentStuckEvent         pubsub.EventType = "subagent_stuck"
	SubAgentTimedOutEvent      pubsub.EventType = "subagent_timed_out"
	SubAgentBlockedEvent       pubsub.EventType = "subagent_blocked"
	SubAgentCompletedEvent     pubsub.EventType = "subagent_completed"
	SubAgentFailedEvent        pubsub.EventType = "subagent_failed"
	SubAgentClosedEvent        pubsub.EventType = "subagent_closed"
)

type SubAgentLifecycleEvent struct {
	AgentID          string                 `json:"agent_id"`
	SessionID        string                 `json:"session_id"`
	ParentSessionID  string                 `json:"parent_session_id,omitempty"`
	SubmissionID     string                 `json:"submission_id,omitempty"`
	Stage            SubAgentLifecycleStage `json:"stage"`
	Status           subAgentStatus         `json:"status"`
	Title            string                 `json:"title,omitempty"`
	WorkDir          string                 `json:"work_dir,omitempty"`
	StartedAt        time.Time              `json:"started_at,omitempty"`
	HeartbeatContext string                 `json:"heartbeat_context,omitempty"`
	CurrentTool      string                 `json:"current_tool,omitempty"`
	LastTool         string                 `json:"last_tool,omitempty"`
	ToolCallCount    int                    `json:"tool_call_count,omitempty"`
	Task             string                 `json:"task,omitempty"`
	TaskKey          string                 `json:"task_key,omitempty"`
	Domains          []string               `json:"domains,omitempty"`
	Result           string                 `json:"result,omitempty"`
	Progress         string                 `json:"progress,omitempty"`
	Error            string                 `json:"error,omitempty"`
	Pending          int                    `json:"pending,omitempty"`
	Timestamp        time.Time              `json:"timestamp"`
}

var subAgentEventBroker = pubsub.NewBroker[SubAgentLifecycleEvent]()

func SubscribeSubAgentEvents(ctx context.Context) <-chan pubsub.Event[SubAgentLifecycleEvent] {
	return subAgentEventBroker.Subscribe(ctx)
}

func publishSubAgentLifecycleEvent(eventType pubsub.EventType, payload SubAgentLifecycleEvent) {
	subAgentEventBroker.Publish(eventType, payload)
}
