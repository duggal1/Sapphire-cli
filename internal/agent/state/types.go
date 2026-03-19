package state

type AgentStatus string

const (
	StatusQueued    AgentStatus = "queued"
	StatusRunning   AgentStatus = "running"
	StatusStuck     AgentStatus = "stuck"
	StatusIdle      AgentStatus = "idle"
	StatusCompleted AgentStatus = "completed"
	StatusError     AgentStatus = "error"
	StatusClosed    AgentStatus = "closed"
)
