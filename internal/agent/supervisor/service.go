package supervisor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	agentactivity "github.com/duggal1/Sapphire-cli/internal/agent/activity"
	agentmailbox "github.com/duggal1/Sapphire-cli/internal/agent/mailbox"
	agentstate "github.com/duggal1/Sapphire-cli/internal/agent/state"
	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
)

const (
	defaultPatrolInterval     = 2 * time.Minute
	startupFailureThreshold   = 45 * time.Second
	slowThreshold             = 5 * time.Minute
	degradedThreshold         = 10 * time.Minute
	stuckThreshold            = 15 * time.Minute
	stuckEscalationThreshold  = 20 * time.Minute
	statusRequestSpacing      = 6 * time.Minute
	recoveryInterventionGap   = 8 * time.Minute
	recoveryHistoryWindow     = 30 * time.Minute
	maxRecoveryNudges         = 3
	reassignHistoryWindow     = 45 * time.Minute
	maxReassignments          = 2
	loopDetectionWindowSize   = 10
	loopDetectionRepeatCount  = 5
	criticalEscalationSpacing = 10 * time.Minute
)

type AgentRuntimeSnapshot struct {
	AgentID              string
	SessionID            string
	ParentSessionID      string
	WorkItemID           string
	Status               string
	DefinitionOfDone     string
	LastResult           string
	LastError            string
	LastProgress         string
	Branch               string
	ValidationPassed     bool
	ValidationErrors     string
	ValidationHasChanges bool
	LastHeartbeat        time.Time
	HeartbeatContext     string
	RunnerPresent        bool
	Busy                 bool
	QueuedPrompts        int
	PendingSubmissions   int
	HasOutstandingWork   bool
}

type ValidationState struct {
	HasValidOutput bool
	OutputReviewed bool
	ReadyToMerge   bool
	BlockedReason  string
}

type AgentTracker struct {
	AgentID             string
	SessionID           string
	ParentSessionID     string
	WorkItemID          string
	Status              string
	SpawnedAt           time.Time
	LastHeartbeat       time.Time
	LastProgressAt      time.Time
	RetryCount          int
	StatusRequests      int
	RecoveryAttempts    int
	LastError           string
	LastInterventionAt  time.Time
	LastStatusRequestAt time.Time
	LastRecoveryAt      time.Time
	LastEscalatedAt     time.Time
	CriticalIssue       string
	ValidationState     ValidationState
}

type Hooks struct {
	GetRuntimeSnapshot        func(agentID string) (AgentRuntimeSnapshot, bool)
	ResolveMainMailboxID      func(sessionID string) string
	EnsureDispatchForWorkItem func(ctx context.Context, item orchestrationdb.WorkItem) (string, error)
}

type Service struct {
	store           *orchestrationdb.Store
	stateService    *agentstate.Service
	activityService *agentactivity.Service
	mailbox         *agentmailbox.Service
	hooks           Hooks

	mu             sync.Mutex
	agents         map[string]*AgentTracker
	patrolInterval time.Duration
	cancel         context.CancelFunc
}

func NewService(store *orchestrationdb.Store, stateService *agentstate.Service, activityService *agentactivity.Service, mailbox *agentmailbox.Service, hooks Hooks) *Service {
	return &Service{
		store:           store,
		stateService:    stateService,
		activityService: activityService,
		mailbox:         mailbox,
		hooks:           hooks,
		agents:          make(map[string]*AgentTracker),
		patrolInterval:  defaultPatrolInterval,
	}
}

func (s *Service) Start(ctx context.Context) {
	if s == nil || s.cancel != nil {
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	ticker := time.NewTicker(s.patrolInterval)
	go func() {
		defer ticker.Stop()
		for {
			s.runPatrolCycle(runCtx)
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (s *Service) Stop() {
	if s == nil || s.cancel == nil {
		return
	}
	s.cancel()
	s.cancel = nil
}

func (s *Service) TrackAgent(snapshot AgentRuntimeSnapshot) {
	if s == nil || strings.TrimSpace(snapshot.AgentID) == "" {
		return
	}
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	tracker := s.agents[snapshot.AgentID]
	if tracker == nil {
		tracker = &AgentTracker{
			AgentID:         snapshot.AgentID,
			SessionID:       snapshot.SessionID,
			ParentSessionID: snapshot.ParentSessionID,
			WorkItemID:      snapshot.WorkItemID,
			SpawnedAt:       now,
		}
		s.agents[snapshot.AgentID] = tracker
	}
	previousStatus := tracker.Status
	tracker.SessionID = firstNonEmpty(snapshot.SessionID, tracker.SessionID)
	tracker.ParentSessionID = firstNonEmpty(snapshot.ParentSessionID, tracker.ParentSessionID)
	tracker.WorkItemID = firstNonEmpty(snapshot.WorkItemID, tracker.WorkItemID)
	tracker.Status = normalizeStatus(snapshot.Status)
	if !snapshot.LastHeartbeat.IsZero() {
		tracker.LastHeartbeat = snapshot.LastHeartbeat
	}
	if strings.TrimSpace(snapshot.LastProgress) != "" {
		tracker.LastProgressAt = now
		tracker.StatusRequests = 0
		tracker.RecoveryAttempts = 0
		tracker.CriticalIssue = ""
	}
	tracker.LastError = strings.TrimSpace(snapshot.LastError)
	if previousStatus != "" && tracker.Status != previousStatus && (tracker.Status == "running" || tracker.Status == "ready" || tracker.Status == "starting" || tracker.Status == "waiting_on_mail" || tracker.Status == "completed") {
		tracker.StatusRequests = 0
		tracker.RecoveryAttempts = 0
	}
}

func (s *Service) NotifyCompletion(snapshot AgentRuntimeSnapshot) {
	if s == nil || strings.TrimSpace(snapshot.AgentID) == "" {
		return
	}
	s.TrackAgent(snapshot)
	s.mu.Lock()
	if tracker := s.agents[snapshot.AgentID]; tracker != nil {
		tracker.Status = normalizeStatus(snapshot.Status)
		tracker.LastError = strings.TrimSpace(snapshot.LastError)
	}
	s.mu.Unlock()
}

func (s *Service) runPatrolCycle(ctx context.Context) {
	if s == nil {
		return
	}
	trackers := s.snapshotTrackers()
	for _, tracker := range trackers {
		s.superviseAgent(ctx, tracker)
	}
	trackers = s.snapshotTrackers()
	s.processCompletions(ctx, trackers)
	s.unblockWaitingAgents(ctx)
	trackers = s.snapshotTrackers()
	s.reassignRecoverableAgents(ctx, trackers)
	trackers = s.snapshotTrackers()
	s.escalateCriticalIssues(ctx, trackers)
}

func (s *Service) snapshotTrackers() []*AgentTracker {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]*AgentTracker, 0, len(s.agents))
	for _, tracker := range s.agents {
		copyTracker := *tracker
		items = append(items, &copyTracker)
	}
	return items
}

func (s *Service) buildPatrolReceipt(ctx context.Context, tracker *AgentTracker, currentState orchestrationdb.AgentState, runtime AgentRuntimeSnapshot) PatrolReceipt {
	now := time.Now().UTC()
	status := normalizeStatus(firstNonEmpty(currentState.Status, runtime.Status, tracker.Status))
	heartbeatAt := firstNonZeroTime(currentState.LastHeartbeat, runtime.LastHeartbeat, tracker.LastHeartbeat, tracker.SpawnedAt)
	progressAt := firstNonZeroTime(tracker.LastProgressAt, tracker.SpawnedAt)
	dependencyBlocked := s.isBlockedOnDependency(ctx, tracker.WorkItemID)
	actionableMail := s.hasUnreadMail(ctx, tracker.AgentID)
	recoveryCount := s.countRecentInterventions(ctx, tracker.AgentID, string(PatrolActionRecoveryNudge), recoveryHistoryWindow)
	reassignCount := s.countRecentInterventions(ctx, tracker.AgentID, string(PatrolActionReassign), reassignHistoryWindow)

	receipt := PatrolReceipt{
		AgentID:   tracker.AgentID,
		SessionID: firstNonEmpty(tracker.SessionID, runtime.SessionID),
		WorkItemID: firstNonEmpty(
			tracker.WorkItemID,
			runtime.WorkItemID,
		),
		CreatedAt: now,
		Evidence: PatrolEvidence{
			TrackerStatus:       tracker.Status,
			StateStatus:         normalizeStatus(currentState.Status),
			RuntimeStatus:       normalizeStatus(runtime.Status),
			HeartbeatAt:         heartbeatAt,
			HeartbeatAge:        ageString(now, heartbeatAt),
			LastProgressAt:      progressAt,
			LastProgressAge:     ageString(now, progressAt),
			HeartbeatContext:    firstNonEmpty(runtime.HeartbeatContext, tracker.CriticalIssue),
			LastError:           firstNonEmpty(runtime.LastError, tracker.LastError),
			RunnerPresent:       runtime.RunnerPresent,
			Busy:                runtime.Busy,
			QueuedPrompts:       runtime.QueuedPrompts,
			PendingSubmissions:  runtime.PendingSubmissions,
			HasOutstandingWork:  runtime.HasOutstandingWork,
			ValidationPassed:    runtime.ValidationPassed,
			ValidationErrors:    strings.TrimSpace(runtime.ValidationErrors),
			ActionableMail:      actionableMail,
			DependencyBlocked:   dependencyBlocked,
			CriticalIssue:       tracker.CriticalIssue,
			RecoveryAttempts:    tracker.RecoveryAttempts,
			RecentRecoveryNudge: recoveryCount,
			RecentReassignments: reassignCount,
		},
	}

	switch {
	case s.detectLoop(ctx, tracker.AgentID):
		receipt.Verdict = PatrolVerdictLooping
		receipt.RecommendedAction = PatrolActionLoopBreak
		receipt.Summary = "repeated supervisor-visible activity pattern detected"
		return receipt
	case status == "needs_reassignment":
		receipt.Verdict = PatrolVerdictNeedsReassignment
		receipt.RecommendedAction = PatrolActionReassign
		receipt.Summary = firstNonEmpty(tracker.CriticalIssue, "sub-agent requires reassignment")
		return receipt
	case status == "completed" || status == "closed":
		if tracker.ValidationState.OutputReviewed && tracker.ValidationState.HasValidOutput {
			receipt.Verdict = PatrolVerdictCompleted
			receipt.RecommendedAction = PatrolActionNone
			receipt.Summary = "completed and validated"
			return receipt
		}
		receipt.Verdict = PatrolVerdictCompleted
		receipt.RecommendedAction = PatrolActionCompletionReview
		receipt.Summary = "completed and awaiting supervisor validation"
		return receipt
	case status == "blocked":
		receipt.Verdict = PatrolVerdictBlocked
		if dependencyBlocked {
			receipt.RecommendedAction = PatrolActionWait
			receipt.Summary = "blocked on dependency"
		} else {
			receipt.RecommendedAction = PatrolActionReassign
			receipt.Summary = "blocked without unresolved dependency"
		}
		return receipt
	case dependencyBlocked:
		receipt.Verdict = PatrolVerdictWaitingOnDependency
		receipt.RecommendedAction = PatrolActionWait
		receipt.Summary = "waiting on dependency completion"
		return receipt
	case activeRuntimeStatus(status) && !runtime.RunnerPresent:
		receipt.RecommendedAction = PatrolActionEscalate
		if status == "error" || status == "timed_out" || status == "stuck" || strings.TrimSpace(receipt.Evidence.LastError) != "" {
			receipt.Verdict = PatrolVerdictCrashed
			receipt.Summary = firstNonEmpty(receipt.Evidence.LastError, "runner disappeared after runtime failure")
		} else {
			receipt.Verdict = PatrolVerdictOrphaned
			receipt.Summary = "agent state is active but the live runner is missing"
		}
		if heartbeatContext := strings.TrimSpace(receipt.Evidence.HeartbeatContext); heartbeatContext != "" && !strings.Contains(receipt.Summary, heartbeatContext) {
			receipt.Summary += " | last heartbeat: " + heartbeatContext
		}
		return receipt
	}

	if heartbeatAt.IsZero() {
		receipt.Verdict = PatrolVerdictCrashed
		receipt.RecommendedAction = PatrolActionEscalate
		receipt.Summary = "no usable heartbeat was recorded"
		return receipt
	}

	heartbeatAge := now.Sub(heartbeatAt)
	if status == "starting" || status == "ready" {
		if heartbeatAge >= startupFailureThreshold {
			receipt.Verdict = PatrolVerdictCrashed
			receipt.RecommendedAction = PatrolActionEscalate
			receipt.Summary = "sub-agent failed to become healthy during startup"
			return receipt
		}
		receipt.Verdict = PatrolVerdictHealthy
		receipt.RecommendedAction = PatrolActionNone
		receipt.Summary = "startup still within allowed threshold"
		return receipt
	}

	switch {
	case heartbeatAge >= stuckEscalationThreshold:
		receipt.Verdict = PatrolVerdictStale
		if reassignCount >= maxReassignments {
			receipt.RecommendedAction = PatrolActionEscalate
			receipt.Summary = "stale agent exceeded reassignment budget"
		} else {
			receipt.RecommendedAction = PatrolActionReassign
			receipt.Summary = "stale agent exceeded recovery threshold"
		}
	case heartbeatAge >= stuckThreshold:
		receipt.Verdict = PatrolVerdictStale
		if recoveryCount >= maxRecoveryNudges || tracker.RecoveryAttempts >= maxRecoveryNudges {
			receipt.RecommendedAction = PatrolActionReassign
			receipt.Summary = "stale agent exhausted recovery nudges"
		} else {
			receipt.RecommendedAction = PatrolActionRecoveryNudge
			receipt.Summary = "heartbeat is stale and recovery is required"
		}
	case heartbeatAge >= degradedThreshold:
		receipt.Verdict = PatrolVerdictStale
		receipt.RecommendedAction = PatrolActionRecoveryNudge
		receipt.Summary = "heartbeat is degraded and should be refreshed"
	case heartbeatAge >= slowThreshold:
		receipt.Verdict = PatrolVerdictSlow
		receipt.RecommendedAction = PatrolActionRequestStatus
		receipt.Summary = "agent is progressing slowly"
	default:
		receipt.Verdict = PatrolVerdictHealthy
		receipt.RecommendedAction = PatrolActionNone
		receipt.Summary = "agent is healthy"
	}

	if receipt.Verdict == PatrolVerdictHealthy && actionableMail {
		receipt.Summary = "agent is healthy with actionable coordination mail"
	}
	return receipt
}

func (s *Service) maybeRequestStatus(ctx context.Context, tracker *AgentTracker, body string) bool {
	if tracker == nil {
		return false
	}
	now := time.Now().UTC()
	shouldSend := false
	s.updateTrackerState(tracker.AgentID, func(existing *AgentTracker) {
		if existing.StatusRequests > 0 && now.Sub(existing.LastStatusRequestAt) < statusRequestSpacing {
			return
		}
		existing.StatusRequests++
		existing.LastStatusRequestAt = now
		existing.LastInterventionAt = now
		shouldSend = true
	})
	if shouldSend {
		s.nudgeAgent(ctx, tracker, subjectStatusRequest, body)
	}
	return shouldSend
}

func (s *Service) superviseAgent(ctx context.Context, tracker *AgentTracker) {
	if tracker == nil {
		return
	}
	currentState, runtime := s.currentSnapshot(ctx, tracker.AgentID)
	currentStatus := normalizeStatus(firstNonEmpty(currentState.Status, runtime.Status, tracker.Status))
	if currentState.AgentID != "" {
		s.updateTrackerState(tracker.AgentID, func(existing *AgentTracker) {
			existing.Status = currentStatus
			if !currentState.LastHeartbeat.IsZero() {
				existing.LastHeartbeat = currentState.LastHeartbeat
			}
			if strings.TrimSpace(runtime.LastProgress) != "" {
				existing.LastProgressAt = time.Now().UTC()
			}
			existing.LastError = firstNonEmpty(runtime.LastError, existing.LastError)
		})
	}

	receipt := s.buildPatrolReceipt(ctx, tracker, currentState, runtime)
	s.recordPatrolReceipt(ctx, receipt)
	s.applyPatrolReceipt(ctx, tracker, receipt)
	s.logActivity(ctx, tracker.AgentID, "supervisor_check", map[string]any{
		"status":        currentStatus,
		"verdict":       receipt.Verdict,
		"action":        receipt.RecommendedAction,
		"heartbeat_age": receipt.Evidence.HeartbeatAge,
	})
}

func (s *Service) applyPatrolReceipt(ctx context.Context, tracker *AgentTracker, receipt PatrolReceipt) {
	if tracker == nil {
		return
	}
	switch receipt.Verdict {
	case PatrolVerdictWaitingOnDependency:
		s.updateTrackerState(tracker.AgentID, func(existing *AgentTracker) {
			existing.Status = "waiting_on_dependency"
			existing.CriticalIssue = ""
		})
	case PatrolVerdictLooping:
		s.handleLoopDetection(ctx, tracker)
	case PatrolVerdictCompleted:
		if receipt.RecommendedAction == PatrolActionCompletionReview {
			s.forceCompletionReport(ctx, tracker)
		}
	case PatrolVerdictSlow:
		if receipt.RecommendedAction == PatrolActionRequestStatus {
			if s.maybeRequestStatus(ctx, tracker, "Status check. Briefly report concrete progress or the blocker, then continue.") {
				s.logActivity(ctx, tracker.AgentID, "supervisor_intervention", map[string]any{
					"action": string(PatrolActionRequestStatus),
				})
			}
		}
	case PatrolVerdictStale:
		switch receipt.RecommendedAction {
		case PatrolActionRecoveryNudge:
			s.handleStuckAgent(ctx, tracker)
		case PatrolActionReassign:
			issue := firstNonEmpty(receipt.Summary, "sub-agent exceeded recovery threshold")
			if heartbeatContext := strings.TrimSpace(receipt.Evidence.HeartbeatContext); heartbeatContext != "" && !strings.Contains(issue, heartbeatContext) {
				issue += " | last heartbeat: " + heartbeatContext
			}
			s.updateTrackerState(tracker.AgentID, func(existing *AgentTracker) {
				existing.Status = "needs_reassignment"
				existing.CriticalIssue = issue
			})
			s.escalateCriticalIssueNow(ctx, tracker, issue)
		case PatrolActionEscalate:
			s.updateTrackerState(tracker.AgentID, func(existing *AgentTracker) {
				existing.Status = "needs_reassignment"
				existing.CriticalIssue = firstNonEmpty(receipt.Summary, "sub-agent exceeded reassignment budget")
			})
			s.escalateCriticalIssueNow(ctx, tracker, receipt.Summary)
		}
	case PatrolVerdictOrphaned, PatrolVerdictCrashed:
		issue := firstNonEmpty(receipt.Summary, "sub-agent runtime disappeared")
		s.updateTrackerState(tracker.AgentID, func(existing *AgentTracker) {
			existing.Status = "needs_reassignment"
			existing.CriticalIssue = issue
		})
		s.escalateCriticalIssueNow(ctx, tracker, issue)
	case PatrolVerdictNeedsReassignment, PatrolVerdictCompletionInvalid:
		s.updateTrackerState(tracker.AgentID, func(existing *AgentTracker) {
			existing.Status = "needs_reassignment"
			existing.CriticalIssue = firstNonEmpty(receipt.Summary, existing.CriticalIssue, "sub-agent requires reassignment")
		})
	}
}

func (s *Service) handleStuckAgent(ctx context.Context, tracker *AgentTracker) {
	if tracker == nil {
		return
	}
	now := time.Now().UTC()
	shouldNudge := false
	recentRecoveryNudges := s.countRecentInterventions(ctx, tracker.AgentID, string(PatrolActionRecoveryNudge), recoveryHistoryWindow)
	s.updateTrackerState(tracker.AgentID, func(existing *AgentTracker) {
		if existing.Status != "degraded" && existing.Status != "stalled" {
			existing.Status = "degraded"
		}
		if existing.RecoveryAttempts >= maxRecoveryNudges || recentRecoveryNudges >= maxRecoveryNudges {
			existing.Status = "needs_reassignment"
			existing.CriticalIssue = "recovery attempts exhausted"
			return
		}
		if existing.RecoveryAttempts == 0 || now.Sub(existing.LastRecoveryAt) > recoveryInterventionGap {
			existing.RecoveryAttempts++
			existing.LastRecoveryAt = now
			existing.LastInterventionAt = now
			existing.RetryCount++
			shouldNudge = true
		}
		if now.Sub(firstNonZeroTime(existing.LastHeartbeat, tracker.LastHeartbeat, existing.SpawnedAt)) >= stuckEscalationThreshold {
			existing.Status = "needs_reassignment"
			existing.CriticalIssue = "sub-agent stuck beyond supervisor threshold"
		}
	})
	if shouldNudge {
		s.nudgeAgent(ctx, tracker, subjectRecoveryNudge, "Recovery required. Heartbeat is stale. Report current state, blocker, or next action immediately.")
		s.logActivity(ctx, tracker.AgentID, "supervisor_intervention", map[string]any{
			"action": string(PatrolActionRecoveryNudge),
		})
	}
}

func (s *Service) detectLoop(ctx context.Context, agentID string) bool {
	if s == nil || s.activityService == nil || strings.TrimSpace(agentID) == "" {
		return false
	}
	items, err := s.activityService.Recent(ctx, agentID, loopDetectionWindowSize)
	if err != nil || len(items) == 0 {
		return false
	}
	patterns := map[string]int{}
	for _, item := range items {
		key := item.EventType + ":" + strings.TrimSpace(item.DetailsJSON)
		patterns[key]++
	}
	for _, count := range patterns {
		if count >= loopDetectionRepeatCount {
			return true
		}
	}
	return false
}

func (s *Service) handleLoopDetection(ctx context.Context, tracker *AgentTracker) {
	if tracker == nil || s.mailbox == nil {
		return
	}
	body := "Loop detected.\n\nBreak the repetition immediately.\n1. Stop the current repeated action.\n2. Report current state.\n3. Wait for updated instructions if blocked."
	s.nudgeAgent(ctx, tracker, subjectLoopBreak, body)
	s.updateTrackerState(tracker.AgentID, func(existing *AgentTracker) {
		existing.Status = "needs_reassignment"
		existing.CriticalIssue = "loop detected"
	})
	s.logActivity(ctx, tracker.AgentID, "supervisor_intervention", map[string]any{
		"action": string(PatrolActionLoopBreak),
	})
}

func (s *Service) detectSilentCompletion(runtime AgentRuntimeSnapshot) bool {
	status := normalizeStatus(runtime.Status)
	if status != "completed" {
		return false
	}
	return strings.TrimSpace(runtime.LastResult) == "" && strings.TrimSpace(runtime.LastProgress) == ""
}

func (s *Service) forceCompletionReport(ctx context.Context, tracker *AgentTracker) {
	if tracker == nil {
		return
	}
	s.nudgeAgent(ctx, tracker, subjectCompletionReview, "Completion detected without a usable report. Send a final report immediately.")
	s.logActivity(ctx, tracker.AgentID, "supervisor_intervention", map[string]any{
		"action": string(PatrolActionCompletionReview),
	})
}

func (s *Service) processCompletions(ctx context.Context, trackers []*AgentTracker) {
	for _, tracker := range trackers {
		if tracker == nil {
			continue
		}
		if normalizeStatus(tracker.Status) != "completed" {
			continue
		}
		if tracker.ValidationState.OutputReviewed {
			continue
		}
		validation := s.validateCompletion(ctx, tracker)
		s.updateTrackerState(tracker.AgentID, func(existing *AgentTracker) {
			existing.ValidationState = validation
			if validation.HasValidOutput {
				existing.Status = "completed"
				existing.CriticalIssue = ""
			} else {
				existing.Status = "needs_reassignment"
				existing.CriticalIssue = validation.BlockedReason
			}
		})
		if validation.HasValidOutput {
			s.unblockDependents(ctx, tracker.WorkItemID)
			s.reportToMainAgent(ctx, tracker, validation)
		} else {
			s.recordPatrolReceipt(ctx, PatrolReceipt{
				AgentID:           tracker.AgentID,
				SessionID:         tracker.SessionID,
				WorkItemID:        tracker.WorkItemID,
				Verdict:           PatrolVerdictCompletionInvalid,
				RecommendedAction: PatrolActionReassign,
				Summary:           validation.BlockedReason,
				Evidence: PatrolEvidence{
					TrackerStatus: tracker.Status,
					CriticalIssue: validation.BlockedReason,
				},
				CreatedAt: time.Now().UTC(),
			})
			s.logActivity(ctx, tracker.AgentID, "supervisor_intervention", map[string]any{
				"action": string(PatrolActionCompletionReview),
				"reason": validation.BlockedReason,
			})
		}
	}
}

func (s *Service) validateCompletion(ctx context.Context, tracker *AgentTracker) ValidationState {
	_, runtime := s.currentSnapshot(ctx, tracker.AgentID)
	if normalizeStatus(runtime.Status) != "completed" {
		return ValidationState{HasValidOutput: false, OutputReviewed: true, BlockedReason: "runtime status is not completed"}
	}
	if !runtime.ValidationPassed || strings.TrimSpace(runtime.ValidationErrors) != "" {
		return ValidationState{HasValidOutput: false, OutputReviewed: true, BlockedReason: "validation gate failed"}
	}
	if strings.TrimSpace(runtime.LastResult) == "" && strings.TrimSpace(runtime.LastProgress) == "" {
		return ValidationState{HasValidOutput: false, OutputReviewed: true, BlockedReason: "no completion report content"}
	}
	if strings.TrimSpace(runtime.DefinitionOfDone) != "" && strings.TrimSpace(runtime.LastResult) == "" {
		return ValidationState{HasValidOutput: false, OutputReviewed: true, BlockedReason: "definition of done exists but no final result was reported"}
	}
	return ValidationState{
		HasValidOutput: true,
		OutputReviewed: true,
		ReadyToMerge:   true,
	}
}

func (s *Service) unblockWaitingAgents(ctx context.Context) {
	if s == nil || s.store == nil {
		return
	}
	items, err := s.store.ListWorkItemsByStatus(ctx, []string{"blocked"}, 200)
	if err != nil {
		return
	}
	for _, item := range items {
		if !s.areAllDependenciesComplete(ctx, item) {
			continue
		}
		if s.hasBlockedDispatchBarrier(ctx, item.ID) {
			continue
		}
		item.Status = "open"
		item.ClosedAt = time.Time{}
		_ = s.store.UpsertWorkItem(ctx, item)
		if s.hooks.EnsureDispatchForWorkItem != nil {
			_, _ = s.hooks.EnsureDispatchForWorkItem(ctx, item)
		}
		s.logActivity(ctx, item.Assignee, "supervisor_unblocked", map[string]any{
			"work_item_id": item.ID,
		})
	}
}

func (s *Service) hasBlockedDispatchBarrier(ctx context.Context, workItemID string) bool {
	if s == nil || s.store == nil || strings.TrimSpace(workItemID) == "" {
		return false
	}
	dispatches, err := s.store.ListDispatchesByWorkItem(ctx, workItemID, []string{"blocked"}, 1)
	return err == nil && len(dispatches) > 0
}

func (s *Service) unblockDependents(ctx context.Context, completedWorkItemID string) {
	if strings.TrimSpace(completedWorkItemID) == "" {
		return
	}
	s.unblockWaitingAgents(ctx)
}

func (s *Service) reassignRecoverableAgents(ctx context.Context, trackers []*AgentTracker) {
	if s == nil || s.store == nil || s.hooks.EnsureDispatchForWorkItem == nil {
		return
	}
	for _, tracker := range trackers {
		if tracker == nil {
			continue
		}
		if normalizeStatus(tracker.Status) != "needs_reassignment" {
			continue
		}
		workItemID := strings.TrimSpace(tracker.WorkItemID)
		if workItemID == "" {
			continue
		}
		item, err := s.store.GetWorkItem(ctx, workItemID)
		if err != nil {
			continue
		}
		if !s.areAllDependenciesComplete(ctx, item) {
			continue
		}
		item.Status = "open"
		item.ClosedAt = time.Time{}
		if err := s.store.UpsertWorkItem(ctx, item); err != nil {
			continue
		}
		if _, err := s.hooks.EnsureDispatchForWorkItem(ctx, item); err != nil {
			s.logActivity(ctx, tracker.AgentID, "supervisor_reassign_failed", map[string]any{
				"work_item_id": workItemID,
				"error":        err.Error(),
			})
			continue
		}
		s.updateTrackerState(tracker.AgentID, func(existing *AgentTracker) {
			existing.Status = "reassigned"
			existing.CriticalIssue = ""
			existing.LastInterventionAt = time.Now().UTC()
		})
		s.notifyMainAgent(ctx, tracker.SessionID, subjectReassignmentNotice, fmt.Sprintf("- %s | reassigned | %s", tracker.AgentID, workItemID), 0)
		s.logActivity(ctx, tracker.AgentID, "supervisor_reassigned", map[string]any{
			"work_item_id": workItemID,
		})
		s.logActivity(ctx, tracker.AgentID, "supervisor_intervention", map[string]any{
			"action": string(PatrolActionReassign),
		})
	}
}

func (s *Service) escalateCriticalIssues(ctx context.Context, trackers []*AgentTracker) {
	sessionIssues := map[string][]string{}
	for _, tracker := range trackers {
		if tracker == nil || strings.TrimSpace(tracker.CriticalIssue) == "" {
			continue
		}
		if !tracker.LastEscalatedAt.IsZero() && time.Since(tracker.LastEscalatedAt) < criticalEscalationSpacing {
			continue
		}
		sessionIssues[tracker.SessionID] = append(sessionIssues[tracker.SessionID], fmt.Sprintf("- %s | %s | %s", tracker.AgentID, tracker.Status, tracker.CriticalIssue))
		s.updateTrackerState(tracker.AgentID, func(existing *AgentTracker) {
			existing.LastEscalatedAt = time.Now().UTC()
		})
	}
	for sessionID, issues := range sessionIssues {
		mainMailboxID := ""
		if s.hooks.ResolveMainMailboxID != nil {
			mainMailboxID = s.hooks.ResolveMainMailboxID(sessionID)
		}
		if mainMailboxID == "" || s.mailbox == nil {
			continue
		}
		_, _ = s.mailbox.Send(ctx, mainMailboxID, "supervisor", subjectEscalation, strings.Join(issues, "\n"), agentmailbox.SendOptions{Priority: 0, SkipNudge: true})
	}
}

func (s *Service) escalateCriticalIssueNow(ctx context.Context, tracker *AgentTracker, issue string) {
	if tracker == nil || strings.TrimSpace(issue) == "" || s.mailbox == nil || s.hooks.ResolveMainMailboxID == nil {
		return
	}
	now := time.Now().UTC()
	shouldSend := false
	sessionID := tracker.SessionID
	agentID := tracker.AgentID
	status := tracker.Status
	s.updateTrackerState(tracker.AgentID, func(existing *AgentTracker) {
		if !existing.LastEscalatedAt.IsZero() && now.Sub(existing.LastEscalatedAt) < criticalEscalationSpacing {
			return
		}
		existing.LastEscalatedAt = now
		sessionID = firstNonEmpty(existing.SessionID, sessionID)
		status = firstNonEmpty(existing.Status, status)
		shouldSend = true
	})
	if !shouldSend {
		return
	}
	mainMailboxID := s.hooks.ResolveMainMailboxID(sessionID)
	if mainMailboxID == "" {
		return
	}
	body := fmt.Sprintf("- %s | %s | %s", agentID, status, issue)
	_, _ = s.mailbox.Send(ctx, mainMailboxID, "supervisor", subjectEscalation, body, agentmailbox.SendOptions{Priority: 0, SkipNudge: true})
}

func (s *Service) reportToMainAgent(ctx context.Context, tracker *AgentTracker, validation ValidationState) {
	if tracker == nil || s.mailbox == nil || s.hooks.ResolveMainMailboxID == nil {
		return
	}
	mainMailboxID := s.hooks.ResolveMainMailboxID(tracker.SessionID)
	if mainMailboxID == "" {
		return
	}
	body := fmt.Sprintf("Validated completion.\n- Agent: %s\n- Work item: %s\n- Ready to merge: %t", tracker.AgentID, tracker.WorkItemID, validation.ReadyToMerge)
	_, _ = s.mailbox.Send(ctx, mainMailboxID, "supervisor", subjectCompletionValidated, body, agentmailbox.SendOptions{Priority: 1, SkipNudge: true})
}

func (s *Service) hasUnreadMail(ctx context.Context, agentID string) bool {
	if s == nil || s.mailbox == nil {
		return false
	}
	items, err := s.mailbox.Actionable(ctx, agentID, 1)
	return err == nil && len(items) > 0
}

func (s *Service) nudgeAgent(ctx context.Context, tracker *AgentTracker, subject, body string) {
	if tracker == nil || s.mailbox == nil {
		return
	}
	if strings.TrimSpace(subject) == "" {
		subject = subjectRecoveryNudge
	}
	_, _ = s.mailbox.Send(ctx, tracker.AgentID, "supervisor", subject, body, agentmailbox.SendOptions{
		Priority:  0,
		SkipNudge: true,
	})
}

func (s *Service) isBlockedOnDependency(ctx context.Context, workItemID string) bool {
	if s == nil || s.store == nil || strings.TrimSpace(workItemID) == "" {
		return false
	}
	item, err := s.store.GetWorkItem(ctx, workItemID)
	if err != nil {
		return false
	}
	deps := parseDependencies(item.Dependencies)
	return len(deps) > 0 && !s.areAllDependenciesComplete(ctx, item)
}

func (s *Service) areAllDependenciesComplete(ctx context.Context, item orchestrationdb.WorkItem) bool {
	deps := parseDependencies(item.Dependencies)
	for _, depID := range deps {
		dep, err := s.store.GetWorkItem(ctx, depID)
		if err != nil {
			return false
		}
		if normalizeStatus(dep.Status) != "closed" && normalizeStatus(dep.Status) != "completed" {
			return false
		}
	}
	return true
}

func (s *Service) currentSnapshot(ctx context.Context, agentID string) (orchestrationdb.AgentState, AgentRuntimeSnapshot) {
	var state orchestrationdb.AgentState
	if s.stateService != nil {
		state, _ = s.stateService.Status(ctx, agentID)
	}
	var runtime AgentRuntimeSnapshot
	if s.hooks.GetRuntimeSnapshot != nil {
		runtime, _ = s.hooks.GetRuntimeSnapshot(agentID)
	}
	return state, runtime
}

func (s *Service) recordPatrolReceipt(ctx context.Context, receipt PatrolReceipt) {
	if s == nil || strings.TrimSpace(receipt.AgentID) == "" {
		return
	}
	s.logActivity(ctx, receipt.AgentID, "supervisor_patrol_receipt", map[string]any{
		"session_id":         receipt.SessionID,
		"work_item_id":       receipt.WorkItemID,
		"verdict":            receipt.Verdict,
		"recommended_action": receipt.RecommendedAction,
		"summary":            receipt.Summary,
		"evidence":           receipt.Evidence,
		"created_at":         receipt.CreatedAt.UTC().Format(time.RFC3339Nano),
	})
}

func (s *Service) countRecentInterventions(ctx context.Context, agentID, action string, window time.Duration) int {
	if s == nil || s.activityService == nil || strings.TrimSpace(agentID) == "" || strings.TrimSpace(action) == "" {
		return 0
	}
	if window <= 0 {
		return 0
	}
	items, err := s.activityService.Recent(ctx, agentID, 64)
	if err != nil {
		return 0
	}
	since := time.Now().UTC().Add(-window)
	count := 0
	for _, item := range items {
		if item.EventType != "supervisor_intervention" {
			continue
		}
		if !item.CreatedAt.IsZero() && item.CreatedAt.Before(since) {
			continue
		}
		var details map[string]any
		if err := json.Unmarshal([]byte(item.DetailsJSON), &details); err != nil {
			continue
		}
		if strings.TrimSpace(stringValue(details["action"])) == action {
			count++
		}
	}
	return count
}

func (s *Service) notifyMainAgent(ctx context.Context, sessionID, subject, body string, priority int) {
	if s == nil || s.mailbox == nil || s.hooks.ResolveMainMailboxID == nil {
		return
	}
	mainMailboxID := s.hooks.ResolveMainMailboxID(sessionID)
	if strings.TrimSpace(mainMailboxID) == "" {
		return
	}
	_, _ = s.mailbox.Send(ctx, mainMailboxID, "supervisor", subject, body, agentmailbox.SendOptions{Priority: priority, SkipNudge: true})
}

func (s *Service) updateTrackerState(agentID string, fn func(*AgentTracker)) {
	if s == nil || strings.TrimSpace(agentID) == "" || fn == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tracker := s.agents[agentID]
	if tracker == nil {
		tracker = &AgentTracker{AgentID: agentID, SpawnedAt: time.Now().UTC()}
		s.agents[agentID] = tracker
	}
	fn(tracker)
}

func (s *Service) logActivity(ctx context.Context, agentID, eventType string, details map[string]any) {
	if s == nil || s.activityService == nil || strings.TrimSpace(agentID) == "" || strings.TrimSpace(eventType) == "" {
		return
	}
	data := "{}"
	if details != nil {
		if encoded, err := json.Marshal(details); err == nil {
			data = string(encoded)
		}
	}
	_ = s.activityService.Log(ctx, agentID, agentactivity.EventType(eventType), data)
}

func parseDependencies(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var deps []string
	if err := json.Unmarshal([]byte(raw), &deps); err != nil {
		return nil
	}
	out := make([]string, 0, len(deps))
	for _, dep := range deps {
		if dep = strings.TrimSpace(dep); dep != "" {
			out = append(out, dep)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
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

func ageString(now, at time.Time) string {
	if at.IsZero() {
		return ""
	}
	if now.Before(at) {
		return "0s"
	}
	return now.Sub(at).Truncate(time.Second).String()
}

func activeRuntimeStatus(status string) bool {
	switch normalizeStatus(status) {
	case "starting", "ready", "waiting_on_mail", "retrying", "running", "degraded", "stalled", "timed_out", "stuck", "error":
		return true
	default:
		return false
	}
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprint(value)
	}
}

func normalizeStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "waiting":
		return strings.TrimSpace(status)
	case "closed":
		return "completed"
	default:
		return strings.ToLower(strings.TrimSpace(status))
	}
}
