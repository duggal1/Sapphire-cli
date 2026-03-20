package background

import (
	"sync"
	"time"
)

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

type TaskSpec struct {
	ID               string
	SessionID        string
	ParentSessionID  string
	WorkItemID       string
	Name             string
	Prompt           string
	PromptItems      []string
	Title            string
	Worktree         bool
	WorktreePath     string
	Branch           string
	WriteManifest    []string
	DefinitionOfDone string
	TestCommand      string
	AgentID          string
	Model            string
	ReasoningEffort  string
	ForkContext      bool
	ReadOnly         bool
	AllowedTools     []string
}

type SubAgent struct {
	ID              string
	SessionID       string
	ParentSessionID string
	WorkItemID      string
	Name            string
	Status          Status
	StartedAt       time.Time
	CompletedAt     time.Time
	RuntimeAgentID  string
	SubmissionID    string
	Result          string
	Error           string
	Notified        bool
	Task            TaskSpec
}

type Registry struct {
	mu     sync.RWMutex
	agents map[string]*SubAgent
}

func NewRegistry() *Registry {
	return &Registry{agents: make(map[string]*SubAgent)}
}

func (r *Registry) Register(agent SubAgent) {
	if r == nil || agent.ID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.agents == nil {
		r.agents = make(map[string]*SubAgent)
	}
	copy := agent
	if copy.Status == "" {
		copy.Status = StatusQueued
	}
	r.agents[agent.ID] = &copy
}

func (r *Registry) UpdateStatus(agentID string, status Status) {
	r.update(agentID, func(agent *SubAgent) {
		agent.Status = status
		now := time.Now().UTC()
		if status == StatusRunning && agent.StartedAt.IsZero() {
			agent.StartedAt = now
		}
		if status == StatusCompleted || status == StatusFailed {
			agent.CompletedAt = now
		}
	})
}

func (r *Registry) SetRuntime(agentID, runtimeAgentID, submissionID string) {
	r.update(agentID, func(agent *SubAgent) {
		agent.RuntimeAgentID = runtimeAgentID
		agent.SubmissionID = submissionID
	})
}

func (r *Registry) SetResult(agentID, result string) {
	r.update(agentID, func(agent *SubAgent) {
		agent.Result = result
	})
}

func (r *Registry) SetError(agentID, err string) {
	r.update(agentID, func(agent *SubAgent) {
		agent.Error = err
	})
}

func (r *Registry) Get(agentID string) (SubAgent, bool) {
	if r == nil || agentID == "" {
		return SubAgent{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	agent, ok := r.agents[agentID]
	if !ok || agent == nil {
		return SubAgent{}, false
	}
	return *agent, true
}

func (r *Registry) ListByStatus(status Status) []SubAgent {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]SubAgent, 0, len(r.agents))
	for _, agent := range r.agents {
		if agent == nil || agent.Status != status {
			continue
		}
		items = append(items, *agent)
	}
	return items
}

func (r *Registry) ListActive() []SubAgent {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]SubAgent, 0, len(r.agents))
	for _, agent := range r.agents {
		if agent == nil {
			continue
		}
		if agent.Status == StatusQueued || agent.Status == StatusRunning {
			items = append(items, *agent)
		}
	}
	return items
}

func (r *Registry) ListAll() []SubAgent {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]SubAgent, 0, len(r.agents))
	for _, agent := range r.agents {
		if agent == nil {
			continue
		}
		items = append(items, *agent)
	}
	return items
}

func (r *Registry) MarkNotified(agentID string) bool {
	if r == nil || agentID == "" {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	agent, ok := r.agents[agentID]
	if !ok || agent == nil || agent.Notified {
		return false
	}
	agent.Notified = true
	return true
}

func (r *Registry) update(agentID string, fn func(*SubAgent)) {
	if r == nil || agentID == "" || fn == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	agent, ok := r.agents[agentID]
	if !ok || agent == nil {
		return
	}
	fn(agent)
}
