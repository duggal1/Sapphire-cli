package background

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type ExecutionResult struct {
	RuntimeAgentID string
	SubmissionID   string
	Result         string
}

type PlanModeRestrictor struct {
	AllowedTools []string
}

func DefaultPlanModeRestrictor() PlanModeRestrictor {
	return PlanModeRestrictor{
		AllowedTools: []string{
			"read_file",
			"search_codebase",
			"list_directory",
			"run_command",
			"launch_exploration_agent",
			"request_user_input",
			"update_plan",
		},
	}
}

type Hooks struct {
	Execute       func(context.Context, TaskSpec) (ExecutionResult, error)
	DefaultCtx    func() context.Context
	MaxConcurrent func() int
}

type Dispatcher struct {
	registry *Registry
	capacity *CapacityController
	hooks    Hooks
}

func NewDispatcher(registry *Registry, hooks Hooks) *Dispatcher {
	maxConcurrent := 5
	if hooks.MaxConcurrent != nil && hooks.MaxConcurrent() > 0 {
		maxConcurrent = hooks.MaxConcurrent()
	}
	return &Dispatcher{
		registry: registry,
		capacity: NewCapacityController(maxConcurrent),
		hooks:    hooks,
	}
}

func (d *Dispatcher) Dispatch(ctx context.Context, spec TaskSpec) (string, error) {
	if d == nil || d.registry == nil || d.hooks.Execute == nil {
		return "", fmt.Errorf("background dispatcher is not initialized")
	}
	if spec.SessionID == "" {
		return "", fmt.Errorf("session id is required")
	}
	agentID := spec.ID
	if agentID == "" {
		agentID = "bg-" + uuid.NewString()
	}
	spec.ID = agentID
	d.registry.Register(SubAgent{
		ID:              agentID,
		SessionID:       spec.SessionID,
		ParentSessionID: spec.ParentSessionID,
		WorkItemID:      spec.WorkItemID,
		Name:            spec.Name,
		Status:          StatusQueued,
		Task:            spec,
	})
	go d.runBackgroundWorker(spec)
	return agentID, nil
}

func (d *Dispatcher) runBackgroundWorker(spec TaskSpec) {
	ctx := context.Background()
	if d.hooks.DefaultCtx != nil {
		ctx = d.hooks.DefaultCtx()
	}
	if err := d.capacity.Acquire(ctx); err != nil {
		d.registry.SetError(spec.ID, err.Error())
		d.registry.UpdateStatus(spec.ID, StatusFailed)
		return
	}
	defer d.capacity.Release()

	d.registry.UpdateStatus(spec.ID, StatusRunning)
	result, err := d.hooks.Execute(ctx, spec)
	if err != nil {
		d.registry.SetError(spec.ID, err.Error())
		d.registry.UpdateStatus(spec.ID, StatusFailed)
		return
	}
	d.registry.SetRuntime(spec.ID, result.RuntimeAgentID, result.SubmissionID)
	d.registry.SetResult(spec.ID, result.Result)
	d.registry.UpdateStatus(spec.ID, StatusCompleted)
}

func (d *Dispatcher) Get(agentID string) (SubAgent, bool) {
	if d == nil || d.registry == nil {
		return SubAgent{}, false
	}
	return d.registry.Get(agentID)
}

func (d *Dispatcher) ListActive() []SubAgent {
	if d == nil || d.registry == nil {
		return nil
	}
	return d.registry.ListActive()
}

func (d *Dispatcher) WaitForCompletion(ctx context.Context, agentIDs []string) ([]SubAgent, error) {
	pending := make(map[string]struct{}, len(agentIDs))
	for _, id := range agentIDs {
		if id != "" {
			pending[id] = struct{}{}
		}
	}
	if len(pending) == 0 {
		return nil, nil
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			allDone := true
			for id := range pending {
				agent, ok := d.registry.Get(id)
				if !ok {
					allDone = false
					continue
				}
				if agent.Status == StatusQueued || agent.Status == StatusRunning {
					allDone = false
				}
			}
			if allDone {
				items := make([]SubAgent, 0, len(pending))
				for id := range pending {
					if agent, ok := d.registry.Get(id); ok {
						items = append(items, agent)
					}
				}
				return items, nil
			}
		}
	}
}
