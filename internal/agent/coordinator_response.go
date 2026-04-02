package agent

import (
	"context"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/message"
)

func (c *coordinator) respondWithText(ctx context.Context, sessionID, text string) (*fantasy.AgentResult, error) {
	_, err := c.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: text},
			message.Finish{
				Reason:  message.FinishReasonEndTurn,
				Time:    time.Now().Unix(),
				Message: "Direct reply",
			},
		},
	})
	if err != nil {
		return nil, err
	}
	return &fantasy.AgentResult{
		Response: fantasy.Response{
			Content: fantasy.ResponseContent{
				fantasy.TextContent{Text: text},
			},
		},
	}, nil
}

func directReplyOnlyResponse(prompt string, attachments []message.Attachment) (string, bool) {
	if len(attachments) > 0 || !isDirectReplyOnlyPrompt(prompt, attachments) {
		return "", false
	}

	switch normalizePromptForPolicy(prompt) {
	case "hi", "hello", "hello there":
		return "Hi.", true
	case "hey", "hey there":
		return "Hey.", true
	case "good morning":
		return "Good morning.", true
	case "good afternoon":
		return "Good afternoon.", true
	case "good evening":
		return "Good evening.", true
	case "thanks", "thank you", "thank you so much", "thx":
		return "You're welcome.", true
	case "ok", "okay", "cool", "nice", "sounds good", "got it", "understood":
		return "Understood.", true
	case "how are you":
		return "Doing well. What do you need?", true
	case "what s up":
		return "Ready. What do you need?", true
	default:
		if strings.TrimSpace(prompt) == "" {
			return "", false
		}
		return "Understood.", true
	}
}

func (c *coordinator) respondDirectlyToUser(ctx context.Context, sessionID, prompt string, attachments []message.Attachment) (*fantasy.AgentResult, string, error, bool) {
	text, ok := directReplyOnlyResponse(prompt, attachments)
	if !ok {
		return nil, "", nil, false
	}

	userMessage, err := c.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: prompt},
		},
	})
	if err != nil {
		return nil, "", err, true
	}

	result, err := c.respondWithText(ctx, sessionID, text)
	if err != nil {
		return nil, userMessage.ID, err, true
	}
	return result, userMessage.ID, nil, true
}
