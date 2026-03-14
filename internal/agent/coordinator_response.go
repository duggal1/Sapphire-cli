package agent

import (
	"context"

	"charm.land/fantasy"
	"github.com/charmbracelet/sapphire/internal/message"
)

func (c *coordinator) respondWithText(ctx context.Context, sessionID, text string) (*fantasy.AgentResult, error) {
	_, err := c.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: text},
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
