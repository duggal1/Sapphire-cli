package agent

import (
	"context"
	"time"

	"github.com/duggal1/Sapphire-cli/internal/pubsub"
)

type SubAgentLifecycleStage string

const (
	SubAgentStageSpawned   SubAgentLifecycleStage = "spawned"
	SubAgentStageRunning   SubAgentLifecycleStage = "running"
	SubAgentStageHeartbeat SubAgentLifecycleStage = "heartbeat"
	SubAgentStageWaiting   SubAgentLifecycleStage = "waiting"
	SubAgentStageStuck     SubAgentLifecycleStage = "stuck"
	SubAgentStageCompleted SubAgentLifecycleStage = "completed"
	SubAgentStageFailed    SubAgentLifecycleStage = "failed"
	SubAgentStageClosed    SubAgentLifecycleStage = "closed"
)

const (
	SubAgentSpawnedEvent   pubsub.EventType = "subagent_spawned"
	SubAgentRunningEvent   pubsub.EventType = "subagent_running"
	SubAgentHeartbeatEvent pubsub.EventType = "subagent_heartbeat"
	SubAgentWaitingEvent   pubsub.EventType = "subagent_waiting"
	SubAgentStuckEvent     pubsub.EventType = "subagent_stuck"
	SubAgentCompletedEvent pubsub.EventType = "subagent_completed"
	SubAgentFailedEvent    pubsub.EventType = "subagent_failed"
	SubAgentClosedEvent    pubsub.EventType = "subagent_closed"
)

type SubAgentLifecycleEvent struct {
	AgentID         string                 `json:"agent_id"`
	SessionID       string                 `json:"session_id"`
	ParentSessionID string                 `json:"parent_session_id,omitempty"`
	SubmissionID    string                 `json:"submission_id,omitempty"`
	Stage           SubAgentLifecycleStage `json:"stage"`
	Status          subAgentStatus         `json:"status"`
	Task            string                 `json:"task,omitempty"`
	TaskKey         string                 `json:"task_key,omitempty"`
	Domains         []string               `json:"domains,omitempty"`
	Result          string                 `json:"result,omitempty"`
	Progress        string                 `json:"progress,omitempty"`
	Error           string                 `json:"error,omitempty"`
	Pending         int                    `json:"pending,omitempty"`
	Timestamp       time.Time              `json:"timestamp"`
}

var subAgentEventBroker = pubsub.NewBroker[SubAgentLifecycleEvent]()

func SubscribeSubAgentEvents(ctx context.Context) <-chan pubsub.Event[SubAgentLifecycleEvent] {
	return subAgentEventBroker.Subscribe(ctx)
}

func publishSubAgentLifecycleEvent(eventType pubsub.EventType, payload SubAgentLifecycleEvent) {
	subAgentEventBroker.Publish(eventType, payload)
}
