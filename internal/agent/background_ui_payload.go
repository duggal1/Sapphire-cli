package agent

import "time"

type BackgroundSubAgentsToolInput struct {
	Count int    `json:"count"`
	Title string `json:"title,omitempty"`
}

type BackgroundSubAgentView struct {
	ID        string    `json:"id,omitempty"`
	Name      string    `json:"name,omitempty"`
	Title     string    `json:"title,omitempty"`
	LegType   string    `json:"leg_type,omitempty"`
	Status    string    `json:"status,omitempty"`
	WorkDir   string    `json:"work_dir,omitempty"`
	Branch    string    `json:"branch,omitempty"`
	StartedAt time.Time `json:"started_at,omitempty"`
	Summary   string    `json:"summary,omitempty"`
	Preview   string    `json:"preview,omitempty"`
	Result    string    `json:"result,omitempty"`
	Error     string    `json:"error,omitempty"`
}

type BackgroundSubAgentsToolPayload struct {
	Status    string                   `json:"status"`
	Title     string                   `json:"title,omitempty"`
	Count     int                      `json:"count"`
	Active    int                      `json:"active"`
	Completed int                      `json:"completed"`
	Failed    int                      `json:"failed,omitempty"`
	Agents    []BackgroundSubAgentView `json:"agents,omitempty"`
}
