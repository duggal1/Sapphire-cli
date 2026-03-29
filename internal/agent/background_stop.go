package agent

import (
	"context"
	"errors"

	agentbackground "github.com/duggal1/Sapphire-cli/internal/agent/background"
)

type BackgroundStopSummary struct {
	ClosedSubAgents            int `json:"closed_sub_agents"`
	StoppedBackgroundTasks     int `json:"stopped_background_tasks"`
	StoppedDispatches          int `json:"stopped_dispatches"`
	BlockedWorkItems           int `json:"blocked_work_items"`
	BlockedAgentStates         int `json:"blocked_agent_states"`
	DeadLetteredMail           int `json:"dead_lettered_mail"`
	KilledBackgroundShells     int `json:"killed_background_shells"`
	KilledFastBackgroundShells int `json:"killed_fast_background_shells"`
	CancelledCodebaseIndexes   int `json:"cancelled_codebase_indexes"`
}

func (s BackgroundStopSummary) ToMap() map[string]int {
	return map[string]int{
		"closed_sub_agents":             s.ClosedSubAgents,
		"stopped_background_tasks":      s.StoppedBackgroundTasks,
		"stopped_dispatches":            s.StoppedDispatches,
		"blocked_work_items":            s.BlockedWorkItems,
		"blocked_agent_states":          s.BlockedAgentStates,
		"dead_lettered_mail":            s.DeadLetteredMail,
		"killed_background_shells":      s.KilledBackgroundShells,
		"killed_fast_background_shells": s.KilledFastBackgroundShells,
		"cancelled_codebase_indexes":    s.CancelledCodebaseIndexes,
	}
}

func (c *coordinator) StopBackgroundActivity(ctx context.Context) (BackgroundStopSummary, error) {
	if c == nil {
		return BackgroundStopSummary{}, nil
	}

	const stopReason = "background activity stopped by user"

	var (
		summary BackgroundStopSummary
		errs    []error
	)

	servicesRunning := c.orchestrationSvcCancel != nil
	if servicesRunning {
		c.stopOrchestrationServices()
	}
	if c.stopActiveCodebaseIndexing() {
		summary.CancelledCodebaseIndexes++
	}

	for _, runner := range c.ensureSubAgentRegistry().list() {
		if runner == nil {
			continue
		}
		runner.mu.Lock()
		agentID := runner.id
		status := runner.effectiveStatusLocked()
		runner.mu.Unlock()
		if !isSubAgentActiveStatus(status) {
			continue
		}
		if err := c.closeSubAgent(agentID); err != nil {
			errs = append(errs, err)
			continue
		}
		summary.ClosedSubAgents++
	}

	if c.backgroundRegistry != nil {
		for _, item := range c.backgroundRegistry.ListActive() {
			c.backgroundRegistry.SetError(item.ID, stopReason)
			c.backgroundRegistry.UpdateStatus(item.ID, agentbackground.StatusFailed)
			summary.StoppedBackgroundTasks++
		}
	}

	if c.orchestrationStore != nil {
		storeSummary, err := c.orchestrationStore.StopBackgroundActivity(ctx, stopReason)
		if err != nil {
			errs = append(errs, err)
		} else {
			summary.StoppedDispatches += storeSummary.StoppedDispatches
			summary.BlockedWorkItems += storeSummary.BlockedWorkItems
			summary.BlockedAgentStates += storeSummary.BlockedAgentState
			summary.DeadLetteredMail += storeSummary.DeadLetteredMail
		}
	}

	return summary, errors.Join(errs...)
}
