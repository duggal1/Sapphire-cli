package agent

import (
	"context"
	"testing"

	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/duggal1/Sapphire-cli/internal/pubsub"
	"github.com/stretchr/testify/require"
)

func TestExtractSingularityResultTextPrefersBestAssistantTextFromCurrentTurn(t *testing.T) {
	t.Parallel()

	const sessionID = "session-1"

	coordinator := &coordinator{
		messages: stubMessageService{
			listMessages: []message.Message{
				{
					SessionID: sessionID,
					Role:      message.User,
					Parts: []message.ContentPart{
						message.TextContent{Text: "Architecture task only."},
					},
				},
				{
					SessionID: sessionID,
					Role:      message.Assistant,
					Parts: []message.ContentPart{
						message.TextContent{Text: "I found the relevant files and I am reading them now."},
					},
				},
				{
					SessionID: sessionID,
					Role:      message.Assistant,
					Parts: []message.ContentPart{
						message.TextContent{Text: "Option A uses the existing RuntimeConfig path. Pros: low blast radius. Cons: limited flexibility. Repo fit is high, migration cost is low, compatibility risk is low, and platform.Load does not exist in the repository."},
					},
				},
				{
					SessionID: sessionID,
					Role:      message.Assistant,
					Parts: []message.ContentPart{
						message.TextContent{Text: "All plan items completed above."},
					},
				},
			},
		},
	}

	result := &fantasy.AgentResult{}
	result.Response.Content = fantasy.ResponseContent{
		fantasy.TextContent{Text: "All plan items completed above."},
	}

	text := coordinator.extractSingularityResultText(context.Background(), sessionID, result)
	require.Contains(t, text, "Repo fit is high")
	require.Contains(t, text, "platform.Load does not exist")
}

type stubMessageService struct {
	listMessages []message.Message
}

func (s stubMessageService) Subscribe(context.Context) <-chan pubsub.Event[message.Message] {
	ch := make(chan pubsub.Event[message.Message])
	close(ch)
	return ch
}

func (s stubMessageService) Create(context.Context, string, message.CreateMessageParams) (message.Message, error) {
	panic("unexpected call")
}

func (s stubMessageService) Update(context.Context, message.Message) error {
	panic("unexpected call")
}

func (s stubMessageService) Get(context.Context, string) (message.Message, error) {
	panic("unexpected call")
}

func (s stubMessageService) List(context.Context, string) ([]message.Message, error) {
	return append([]message.Message{}, s.listMessages...), nil
}

func (s stubMessageService) ListUserMessages(context.Context, string) ([]message.Message, error) {
	panic("unexpected call")
}

func (s stubMessageService) ListAllUserMessages(context.Context) ([]message.Message, error) {
	panic("unexpected call")
}

func (s stubMessageService) Delete(context.Context, string) error {
	panic("unexpected call")
}

func (s stubMessageService) DeleteSessionMessages(context.Context, string) error {
	panic("unexpected call")
}
