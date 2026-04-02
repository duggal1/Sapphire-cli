package deepplanning

import (
	"regexp"
	"strings"
)

const (
	PlanningStatusText      = "Planning..."
	PlanCompletedStatusText = "Planned successfully"
	pendingAssistantPrefix  = "local:pending-assistant:"
	planningAssistantPrefix = "local:pending-assistant:deep-planning:"
)

var nonAlphaNumericRegex = regexp.MustCompile(`[^a-z0-9]+`)

var triggerPhrases = []string{
	"create a plan mode",
	"make a plan mode",
	"deep planning",
	"deep plan",
	"plan mode",
	"think extremely long",
	"think for a long time",
	"think very long",
	"think longer",
	"think deeper",
	"plan this deeply",
}

func normalize(input string) string {
	lower := strings.ToLower(strings.TrimSpace(input))
	if lower == "" {
		return ""
	}
	return strings.TrimSpace(nonAlphaNumericRegex.ReplaceAllString(lower, " "))
}

func hasStandaloneToken(normalized, target string) bool {
	for _, token := range strings.Fields(normalized) {
		if token == target {
			return true
		}
	}
	return false
}

// IsRequested returns true when the user's prompt explicitly asks for a
// deeper planning pass. This intentionally errs on the side of activation.
func IsRequested(input string) bool {
	normalized := normalize(input)
	if normalized == "" {
		return false
	}

	for _, phrase := range triggerPhrases {
		if strings.Contains(normalized, phrase) {
			return true
		}
	}

	return hasStandaloneToken(normalized, "plan") || hasStandaloneToken(normalized, "planning")
}

func PendingAssistantPlaceholderID(sessionID string, planning bool) string {
	if planning {
		return planningAssistantPrefix + sessionID
	}
	return pendingAssistantPrefix + sessionID
}

func IsPlanningAssistantPlaceholderID(messageID string) bool {
	return strings.HasPrefix(strings.TrimSpace(messageID), planningAssistantPrefix)
}
