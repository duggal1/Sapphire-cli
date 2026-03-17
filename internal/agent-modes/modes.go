package agentmodes

import (
	_ "embed"
	"strings"

	"github.com/charmbracelet/sapphire/internal/config"
)

//go:embed template/modes/plan.md
var planModePrompt string

//go:embed template/modes/debug.md
var debugModePrompt string

//go:embed template/modes/security.md
var securityModePrompt string

//go:embed template/modes/architect.md
var architectModePrompt string

//go:embed template/modes/review.md
var reviewModePrompt string

func Prompt(mode config.AgentMode) string {
	switch mode {
	case config.AgentModePlan:
		return strings.TrimSpace(planModePrompt)
	case config.AgentModeDebug:
		return strings.TrimSpace(debugModePrompt)
	case config.AgentModeSecurity:
		return strings.TrimSpace(securityModePrompt)
	case config.AgentModeArchitect:
		return strings.TrimSpace(architectModePrompt)
	case config.AgentModeReview:
		return strings.TrimSpace(reviewModePrompt)
	default:
		return ""
	}
}

func VisibleModes() []config.AgentMode {
	return []config.AgentMode{
		config.AgentModePlan,
		config.AgentModeDebug,
		config.AgentModeSecurity,
		config.AgentModeArchitect,
		config.AgentModeReview,
	}
}
