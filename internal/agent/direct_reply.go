package agent

import (
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/message"
)

// DirectReplyForPrompt returns a local conversational response for extremely
// trivial social turns that should never pay the full agent/model path.
func DirectReplyForPrompt(prompt string, attachments []message.Attachment) (string, bool) {
	if !isDirectReplyOnlyPrompt(prompt, attachments) {
		return "", false
	}

	switch normalizePromptForPolicy(prompt) {
	case "thanks", "thank you", "thank you so much", "thx":
		return "You’re welcome.", true
	case "ok", "okay", "cool", "nice", "sounds good", "got it", "understood":
		return "Understood.", true
	case "how are you":
		return "Doing well. What are we working on?", true
	default:
		return "Hey. What are we working on today?", true
	}
}

func directReplyAgentResult(reply string) *fantasy.AgentResult {
	return &fantasy.AgentResult{
		Response: fantasy.Response{
			Content: fantasy.ResponseContent{
				fantasy.TextContent{Text: reply},
			},
		},
	}
}

func directReplyAssistantParts(reply string) []message.ContentPart {
	reply = strings.TrimSpace(reply)
	parts := make([]message.ContentPart, 0, 2)
	if reply != "" {
		parts = append(parts, message.TextContent{Text: reply})
	}
	parts = append(parts, message.Finish{
		Reason: message.FinishReasonEndTurn,
		Time:   time.Now().Unix(),
	})
	return parts
}
