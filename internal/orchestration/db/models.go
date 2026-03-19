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
	ParentAgentID string    `json:"parent_agent_id"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type AgentActivity struct {
	ID          string    `json:"id"`
	AgentID     string    `json:"agent_id"`
	EventType   string    `json:"event_type"`
	DetailsJSON string    `json:"details_json"`
	CreatedAt   time.Time `json:"created_at"`
}
