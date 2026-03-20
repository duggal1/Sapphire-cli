package orchestrationdb

import "time"

type AgentMail struct {
	ID        string    `json:"id"`
	ToAgent   string    `json:"to_agent"`
	FromAgent string    `json:"from_agent"`
	Subject   string    `json:"subject"`
	Body      string    `json:"body"`
	Priority  int       `json:"priority"`
	ThreadID  string    `json:"thread_id"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"created_at"`
	ReadAt    time.Time `json:"read_at,omitempty"`
}

type AgentState struct {
	AgentID       string    `json:"agent_id"`
	Role          string    `json:"role"`
	Status        string    `json:"status"`
	SessionID     string    `json:"session_id"`
	WorktreePath  string    `json:"worktree_path"`
	Branch        string    `json:"branch"`
	HookBeadID    string    `json:"hook_bead_id"`
	ParentAgentID string    `json:"parent_agent_id"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type AgentActivity struct {
	ID          string    `json:"id"`
	AgentID     string    `json:"agent_id"`
	EventType   string    `json:"event_type"`
	DetailsJSON string    `json:"details_json"`
	CreatedAt   time.Time `json:"created_at"`
}

type WorkItem struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Status       string    `json:"status"`
	Assignee     string    `json:"assignee"`
	ParentID     string    `json:"parent_id"`
	ConvoyID     string    `json:"convoy_id"`
	Dependencies string    `json:"dependencies"`
	CreatedAt    time.Time `json:"created_at"`
	ClosedAt     time.Time `json:"closed_at,omitempty"`
}

type Convoy struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Owner         string    `json:"owner"`
	Notify        string    `json:"notify"`
	MergeStrategy string    `json:"merge_strategy"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	ClosedAt      time.Time `json:"closed_at,omitempty"`
}

type ConvoyTrack struct {
	ConvoyID   string    `json:"convoy_id"`
	WorkItemID string    `json:"work_item_id"`
	AddedAt    time.Time `json:"added_at"`
}

type AgentHook struct {
	AgentID    string    `json:"agent_id"`
	HookBeadID string    `json:"hook_bead_id"`
	HookedAt   time.Time `json:"hooked_at"`
	Status     string    `json:"status"`
}

type DispatchQueueItem struct {
	ID              string    `json:"id"`
	SessionID       string    `json:"session_id"`
	WorkItemID      string    `json:"work_item_id"`
	TargetScope     string    `json:"target_scope"`
	Status          string    `json:"status"`
	Priority        int       `json:"priority"`
	PayloadJSON     string    `json:"payload_json"`
	RetryCount      int       `json:"retry_count"`
	LastError       string    `json:"last_error"`
	AvailableAt     time.Time `json:"available_at"`
	LeasedBy        string    `json:"leased_by"`
	LeasedAt        time.Time `json:"leased_at,omitempty"`
	AssignedAgentID string    `json:"assigned_agent_id,omitempty"`
	SubmissionID    string    `json:"submission_id,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type SessionCheckpoint struct {
	ID                 string    `json:"id"`
	SessionID          string    `json:"session_id"`
	AgentID            string    `json:"agent_id"`
	WorkItemID         string    `json:"work_item_id"`
	ParentCheckpointID string    `json:"parent_checkpoint_id"`
	MessageCount       int       `json:"message_count"`
	SummaryJSON        string    `json:"summary_json"`
	AuditTail          string    `json:"audit_tail"`
	PendingTasksJSON   string    `json:"pending_tasks_json"`
	FilesModifiedJSON  string    `json:"files_modified_json"`
	MailCursor         int64     `json:"mail_cursor"`
	ActivityCursor     int64     `json:"activity_cursor"`
	CreatedAt          time.Time `json:"created_at"`
}

type WorktreeRun struct {
	ID            string    `json:"id"`
	SessionID     string    `json:"session_id"`
	AgentID       string    `json:"agent_id"`
	ParentAgentID string    `json:"parent_agent_id"`
	Kind          string    `json:"kind"`
	Policy        string    `json:"policy"`
	Status        string    `json:"status"`
	RepoRoot      string    `json:"repo_root"`
	WorktreePath  string    `json:"worktree_path"`
	Branch        string    `json:"branch"`
	BaseRef       string    `json:"base_ref"`
	TaskKey       string    `json:"task_key"`
	Title         string    `json:"title"`
	MetadataJSON  string    `json:"metadata_json"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	LandedAt      time.Time `json:"landed_at,omitempty"`
	RemovedAt     time.Time `json:"removed_at,omitempty"`
}

type DecisionRecord struct {
	ID                 string    `json:"id"`
	SessionID          string    `json:"session_id"`
	Category           string    `json:"category"`
	Key                string    `json:"key"`
	Value              string    `json:"value"`
	Confidence         string    `json:"confidence"`
	SourceCheckpointID string    `json:"source_checkpoint_id"`
	CreatedAt          time.Time `json:"created_at"`
}

type UserPreference struct {
	Key             string    `json:"key"`
	Value           string    `json:"value"`
	Confidence      string    `json:"confidence"`
	SourceSessionID string    `json:"source_session_id"`
	UpdatedAt       time.Time `json:"updated_at"`
}
