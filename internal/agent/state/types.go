package state

type AgentStatus string

const (
	StatusQueued    AgentStatus = "queued"
	StatusRunning   AgentStatus = "running"
	StatusIdle      AgentStatus = "idle"
	StatusCompleted AgentStatus = "completed"
	StatusError     AgentStatus = "error"
	StatusClosed    AgentStatus = "closed"
)
