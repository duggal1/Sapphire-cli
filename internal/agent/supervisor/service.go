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
	stuckThreshold            = 15 * time.Minute
	stuckEscalationThreshold  = 20 * time.Minute
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
}

type ValidationState struct {
	HasValidOutput bool
	OutputReviewed bool
	ReadyToMerge   bool
	BlockedReason  string
}

type AgentTracker struct {
	AgentID            string
	SessionID          string
	ParentSessionID    string
	WorkItemID         string
	Status             string
	SpawnedAt          time.Time
	LastHeartbeat      time.Time
	LastProgressAt     time.Time
	RetryCount         int
	LastError          string
	LastInterventionAt time.Time
	LastEscalatedAt    time.Time
	CriticalIssue      string
	ValidationState    ValidationState
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
	tracker.SessionID = firstNonEmpty(snapshot.SessionID, tracker.SessionID)
	tracker.ParentSessionID = firstNonEmpty(snapshot.ParentSessionID, tracker.ParentSessionID)
	tracker.WorkItemID = firstNonEmpty(snapshot.WorkItemID, tracker.WorkItemID)
	tracker.Status = normalizeStatus(snapshot.Status)
	if !snapshot.LastHeartbeat.IsZero() {
		tracker.LastHeartbeat = snapshot.LastHeartbeat
	}
	if strings.TrimSpace(snapshot.LastProgress) != "" {
		tracker.LastProgressAt = now
	}
	tracker.LastError = strings.TrimSpace(snapshot.LastError)
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
	s.processCompletions(ctx, trackers)
	s.unblockWaitingAgents(ctx)
	s.reassignRecoverableAgents(ctx, trackers)
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

func (s *Service) superviseAgent(ctx context.Context, tracker *AgentTracker) {
	if tracker == nil {
		return
	}
	currentState, runtime := s.currentSnapshot(ctx, tracker.AgentID)
	if currentState.AgentID != "" {
		s.updateTrackerState(tracker.AgentID, func(existing *AgentTracker) {
			existing.Status = normalizeStatus(firstNonEmpty(currentState.Status, runtime.Status))
			if !currentState.LastHeartbeat.IsZero() {
				existing.LastHeartbeat = currentState.LastHeartbeat
			}
			if strings.TrimSpace(runtime.LastProgress) != "" {
				existing.LastProgressAt = time.Now().UTC()
			}
			existing.LastError = firstNonEmpty(runtime.LastError, existing.LastError)
		})
	}

	heartbeatAge := time.Since(firstNonZeroTime(currentState.LastHeartbeat, runtime.LastHeartbeat, tracker.LastHeartbeat))
	if heartbeatAge > stuckThreshold {
		s.handleStuckAgent(ctx, tracker)
	}
	if s.detectLoop(ctx, tracker.AgentID) {
		s.handleLoopDetection(ctx, tracker)
	}
	if s.detectSilentCompletion(runtime) {
		s.forceCompletionReport(ctx, tracker)
	}
	if s.isBlockedOnDependency(ctx, tracker.WorkItemID) {
		s.updateTrackerState(tracker.AgentID, func(existing *AgentTracker) {
			existing.Status = "blocked"
		})
	}
	s.logActivity(ctx, tracker.AgentID, "supervisor_check", map[string]any{
		"status":        normalizeStatus(firstNonEmpty(currentState.Status, runtime.Status, tracker.Status)),
		"heartbeat_age": heartbeatAge.String(),
	})
}

func (s *Service) handleStuckAgent(ctx context.Context, tracker *AgentTracker) {
	if tracker == nil {
		return
	}
	now := time.Now().UTC()
	shouldNudge := false
	s.updateTrackerState(tracker.AgentID, func(existing *AgentTracker) {
		if existing.Status != "stuck" {
			existing.Status = "stuck"
		}
		if existing.LastInterventionAt.IsZero() || now.Sub(existing.LastInterventionAt) > 5*time.Minute {
			existing.LastInterventionAt = now
			existing.RetryCount++
			shouldNudge = true
		}
		if now.Sub(firstNonZeroTime(existing.LastHeartbeat, tracker.LastHeartbeat, existing.SpawnedAt)) > stuckEscalationThreshold {
			existing.Status = "needs_reassignment"
			existing.CriticalIssue = "sub-agent stuck beyond supervisor threshold"
		}
	})
	if shouldNudge {
		s.nudgeAgent(ctx, tracker, "Progress check — heartbeat is stale. Report status or request help immediately.")
		s.logActivity(ctx, tracker.AgentID, "supervisor_intervention", map[string]any{
			"action": "stuck_nudge",
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
	_, _ = s.mailbox.Send(ctx, tracker.AgentID, "supervisor", "LOOP DETECTED", body, agentmailbox.SendOptions{Priority: 0})
	s.updateTrackerState(tracker.AgentID, func(existing *AgentTracker) {
		existing.Status = "needs_reassignment"
		existing.CriticalIssue = "loop detected"
	})
	s.logActivity(ctx, tracker.AgentID, "supervisor_intervention", map[string]any{
		"action": "loop_break_intervention",
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
	s.nudgeAgent(ctx, tracker, "Completion detected without a usable report. Send a final report immediately.")
	s.logActivity(ctx, tracker.AgentID, "supervisor_intervention", map[string]any{
		"action": "force_completion_report",
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
			s.logActivity(ctx, tracker.AgentID, "supervisor_intervention", map[string]any{
				"action": "invalid_completion",
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
		s.logActivity(ctx, tracker.AgentID, "supervisor_reassigned", map[string]any{
			"work_item_id": workItemID,
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
		_, _ = s.mailbox.Send(ctx, mainMailboxID, "supervisor", "CRITICAL: Sub-agent intervention required", strings.Join(issues, "\n"), agentmailbox.SendOptions{Priority: 0, SkipNudge: true})
	}
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
	_, _ = s.mailbox.Send(ctx, mainMailboxID, "supervisor", "SUBAGENT_VALIDATED", body, agentmailbox.SendOptions{Priority: 1, SkipNudge: true})
}

func (s *Service) hasUnreadMail(ctx context.Context, agentID string) bool {
	if s == nil || s.mailbox == nil {
		return false
	}
	items, err := s.mailbox.Inbox(ctx, agentID, true, 1)
	return err == nil && len(items) > 0
}

func (s *Service) nudgeAgent(ctx context.Context, tracker *AgentTracker, body string) {
	if tracker == nil || s.mailbox == nil {
		return
	}
	_, _ = s.mailbox.Send(ctx, tracker.AgentID, "supervisor", "SUPERVISOR", body, agentmailbox.SendOptions{
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
