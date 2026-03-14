package agent

import (
	"context"
	"fmt"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

type fakeModel struct {
	provider     string
	model        string
	responseText string
}

func (m *fakeModel) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
	_ = ctx
	_ = call
	return &fantasy.Response{
		Content:      fantasy.ResponseContent{fantasy.TextContent{Text: m.responseText}},
		FinishReason: fantasy.FinishReasonStop,
	}, nil
}

func (m *fakeModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	_ = ctx
	_ = call
	text := m.responseText
	return func(yield func(fantasy.StreamPart) bool) {
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart}) {
			return
		}
		if text != "" {
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, Delta: text}) {
				return
			}
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd}) {
			return
		}
		_ = yield(fantasy.StreamPart{
			Type:         fantasy.StreamPartTypeFinish,
			FinishReason: fantasy.FinishReasonStop,
		})
	}, nil
}

func (m *fakeModel) GenerateObject(ctx context.Context, call fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	_ = ctx
	_ = call
	return &fantasy.ObjectResponse{
		Object:       map[string]any{},
		FinishReason: fantasy.FinishReasonStop,
	}, nil
}

func (m *fakeModel) StreamObject(ctx context.Context, call fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	_ = ctx
	_ = call
	return func(yield func(fantasy.ObjectStreamPart) bool) {
		_ = yield(fantasy.ObjectStreamPart{Type: fantasy.ObjectStreamPartTypeFinish})
	}, nil
}

func (m *fakeModel) Provider() string { return m.provider }
func (m *fakeModel) Model() string    { return m.model }

func TestSessionAgentSoak50Messages(t *testing.T) {
	env := testEnv(t)
	model := &fakeModel{
		provider:     "gemini",
		model:        "gemini-3.1-flash-lite-preview",
		responseText: "ok",
	}
	agent := testSessionAgent(env, model, model, "")

	ctx := t.Context()
	sess, err := env.sessions.Create(ctx, "Soak Test")
	require.NoError(t, err)

	for i := 1; i <= 50; i++ {
		prompt := fmt.Sprintf("Compute trivial quotient: %d/%d", i, i)
		_, err := agent.Run(ctx, SessionAgentCall{
			SessionID: sess.ID,
			Prompt:    prompt,
		})
		require.NoError(t, err)
	}
}
