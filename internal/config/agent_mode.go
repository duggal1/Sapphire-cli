package config

type AgentMode string

const (
	AgentModeDefault   AgentMode = "default"
	AgentModePlan      AgentMode = "plan"
	AgentModeDebug     AgentMode = "debug"
	AgentModeSecurity  AgentMode = "security"
	AgentModeArchitect AgentMode = "architect"
	AgentModeReview    AgentMode = "review"
)

func (m AgentMode) Label() string {
	switch m {
	case AgentModePlan:
		return "Plan"
	case AgentModeDebug:
		return "Debug"
	case AgentModeSecurity:
		return "Security"
	case AgentModeArchitect:
		return "Architect"
	case AgentModeReview:
		return "Review"
	default:
		return "Default"
	}
}

func (m AgentMode) IsValid() bool {
	switch m {
	case AgentModeDefault, AgentModePlan, AgentModeDebug, AgentModeSecurity, AgentModeArchitect, AgentModeReview:
		return true
	default:
		return false
	}
}
