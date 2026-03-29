package supervisor

import "time"

type PatrolVerdict string

const (
	PatrolVerdictHealthy             PatrolVerdict = "healthy"
	PatrolVerdictSlow                PatrolVerdict = "slow"
	PatrolVerdictStale               PatrolVerdict = "stale"
	PatrolVerdictOrphaned            PatrolVerdict = "orphaned"
	PatrolVerdictCrashed             PatrolVerdict = "crashed"
	PatrolVerdictLooping             PatrolVerdict = "looping"
	PatrolVerdictWaitingOnDependency PatrolVerdict = "waiting_on_dependency"
	PatrolVerdictBlocked             PatrolVerdict = "blocked"
	PatrolVerdictCompleted           PatrolVerdict = "completed"
	PatrolVerdictCompletionInvalid   PatrolVerdict = "completion_invalid"
	PatrolVerdictNeedsReassignment   PatrolVerdict = "needs_reassignment"
)

type PatrolAction string

const (
	PatrolActionNone             PatrolAction = "none"
	PatrolActionRequestStatus    PatrolAction = "request_status"
	PatrolActionRecoveryNudge    PatrolAction = "recovery_nudge"
	PatrolActionLoopBreak        PatrolAction = "loop_break"
	PatrolActionLogMistake       PatrolAction = "log_mistake"
	PatrolActionCompletionReview PatrolAction = "completion_review"
	PatrolActionReassign         PatrolAction = "reassign"
	PatrolActionEscalate         PatrolAction = "escalate"
	PatrolActionWait             PatrolAction = "wait"
)

type PatrolEvidence struct {
	TrackerStatus       string    `json:"tracker_status,omitempty"`
	StateStatus         string    `json:"state_status,omitempty"`
	RuntimeStatus       string    `json:"runtime_status,omitempty"`
	HeartbeatAt         time.Time `json:"heartbeat_at,omitempty"`
	HeartbeatAge        string    `json:"heartbeat_age,omitempty"`
	LastProgressAt      time.Time `json:"last_progress_at,omitempty"`
	LastProgressAge     string    `json:"last_progress_age,omitempty"`
	HeartbeatContext    string    `json:"heartbeat_context,omitempty"`
	LastError           string    `json:"last_error,omitempty"`
	RunnerPresent       bool      `json:"runner_present,omitempty"`
	Busy                bool      `json:"busy,omitempty"`
	QueuedPrompts       int       `json:"queued_prompts,omitempty"`
	PendingSubmissions  int       `json:"pending_submissions,omitempty"`
	HasOutstandingWork  bool      `json:"has_outstanding_work,omitempty"`
	ValidationPassed    bool      `json:"validation_passed,omitempty"`
	ValidationErrors    string    `json:"validation_errors,omitempty"`
	ActionableMail      bool      `json:"actionable_mail,omitempty"`
	DependencyBlocked   bool      `json:"dependency_blocked,omitempty"`
	CriticalIssue       string    `json:"critical_issue,omitempty"`
	RecoveryAttempts    int       `json:"recovery_attempts,omitempty"`
	RecentRecoveryNudge int       `json:"recent_recovery_nudges,omitempty"`
	RecentReassignments int       `json:"recent_reassignments,omitempty"`
}

type PatrolReceipt struct {
	AgentID           string         `json:"agent_id"`
	SessionID         string         `json:"session_id,omitempty"`
	WorkItemID        string         `json:"work_item_id,omitempty"`
	Verdict           PatrolVerdict  `json:"verdict"`
	RecommendedAction PatrolAction   `json:"recommended_action"`
	Summary           string         `json:"summary,omitempty"`
	Evidence          PatrolEvidence `json:"evidence,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
}
