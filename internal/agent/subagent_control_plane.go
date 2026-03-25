package agent

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/duggal1/Sapphire-cli/internal/pubsub"
)

const subAgentStatusUpdatedEvent pubsub.EventType = "subagent_status_updated"

type subAgentControlPlane struct {
	coordinator *coordinator
}

func (c *coordinator) subAgentControl() subAgentControlPlane {
	return subAgentControlPlane{coordinator: c}
}

func (p subAgentControlPlane) spawn(ctx context.Context, parentSessionID string, opts spawnAgentOptions) (string, string, error) {
	return p.coordinator.spawnSubAgent(ctx, parentSessionID, opts)
}

func (p subAgentControlPlane) resume(ctx context.Context, parentSessionID, agentID, prompt string) (string, subAgentStatus, error) {
	return p.coordinator.resumeSubAgent(ctx, parentSessionID, agentID, prompt)
}

func (p subAgentControlPlane) sendInput(ctx context.Context, agentID, prompt string, items []string, interrupt bool) (string, error) {
	return p.coordinator.sendSubAgentInput(ctx, agentID, prompt, items, interrupt)
}

func (p subAgentControlPlane) wait(ctx context.Context, ids []string, timeout time.Duration) ([]subAgentStatusEntry, bool) {
	return p.coordinator.waitSubAgentStatuses(ctx, ids, timeout)
}

func (p subAgentControlPlane) collectResult(ids []string) []subAgentCollectedResult {
	return p.coordinator.collectSubAgentResults(ids)
}

func (p subAgentControlPlane) close(agentID string) error {
	return p.coordinator.closeSubAgent(agentID)
}

func isSubAgentFinalStatus(status subAgentStatus) bool {
	return !isSubAgentActiveStatus(status)
}

func isSubAgentActiveStatus(status subAgentStatus) bool {
	switch status {
	case subAgentStatusQueued, subAgentStatusStarting, subAgentStatusReady, subAgentStatusWaitingOnMail, subAgentStatusRetrying, subAgentStatusRunning, subAgentStatusDegraded:
		return true
	default:
		return false
	}
}

func publishSubAgentStatus(broker *pubsub.Broker[subAgentStatus], status subAgentStatus) {
	if broker == nil {
		return
	}
	broker.Publish(subAgentStatusUpdatedEvent, status)
}

func (r *subAgentRunner) subscribeStatus(ctx context.Context) (subAgentStatus, <-chan pubsub.Event[subAgentStatus]) {
	r.mu.Lock()
	status := r.effectiveStatusLocked()
	broker := r.statusBroker
	r.mu.Unlock()
	if broker == nil {
		ch := make(chan pubsub.Event[subAgentStatus])
		close(ch)
		return status, ch
	}
	return status, broker.Subscribe(ctx)
}

func (r *subAgentRunner) submissionIsFinal(submissionID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	submission := r.submissions[submissionID]
	if submission == nil {
		return false
	}
	return isSubAgentFinalStatus(submission.Status)
}

func (c *coordinator) startSubAgentCompletionWatcher(runner *subAgentRunner, submissionID string) {
	if runner == nil || submissionID == "" || runner.parentSession == "" {
		return
	}

	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		initialStatus, updates := runner.subscribeStatus(ctx)
		if isSubAgentFinalStatus(initialStatus) && runner.submissionIsFinal(submissionID) {
			c.publishSubAgentCompletionNotification(context.Background(), runner, submissionID)
			return
		}

		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-updates:
				if !ok {
					if runner.submissionIsFinal(submissionID) {
						c.publishSubAgentCompletionNotification(context.Background(), runner, submissionID)
					}
					return
				}
				if runner.submissionIsFinal(submissionID) {
					c.publishSubAgentCompletionNotification(context.Background(), runner, submissionID)
					return
				}
			}
		}
	}()
}

func (c *coordinator) publishSubAgentCompletionNotification(ctx context.Context, runner *subAgentRunner, submissionID string) {
	if c == nil || c.messages == nil || runner == nil {
		return
	}

	runner.mu.Lock()
	parentSessionID := runner.parentSession
	submission := runner.submissions[submissionID]
	submissionStatus := subAgentStatus("")
	if submission != nil {
		submissionStatus = submission.Status
	}
	if parentSessionID == "" || submission == nil || !isSubAgentFinalStatus(submissionStatus) || submission.Notified {
		runner.mu.Unlock()
		return
	}
	submission.Notified = true
	runner.mu.Unlock()

	payload, err := json.Marshal(map[string]string{
		"agent_id":      runner.id,
		"submission_id": submissionID,
		"status":        string(submissionStatus),
	})
	if err != nil {
		slog.Warn("Failed to encode sub-agent notification", "agent_id", runner.id, "error", err)
		return
	}

	_, err = c.messages.Create(ctx, parentSessionID, message.CreateMessageParams{
		Role: message.System,
		Parts: []message.ContentPart{
			message.TextContent{Text: "<subagent_notification>" + string(payload) + "</subagent_notification>"},
		},
	})
	if err != nil {
		slog.Warn("Failed to publish sub-agent notification", "agent_id", runner.id, "parent_session_id", parentSessionID, "error", err)
	}
}
