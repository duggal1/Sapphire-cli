package agent

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

type capturedCallModel struct {
	mu           sync.Mutex
	calls        []fantasy.Call
	responseText string
	titleDelay   time.Duration
}

func (m *capturedCallModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return nil, nil
}

func (m *capturedCallModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.mu.Lock()
	m.calls = append(m.calls, call)
	m.mu.Unlock()

	if m.titleDelay > 0 && promptText(call.Prompt) != "" && strings.Contains(promptText(call.Prompt), "Generate a concise title for the following content:") {
		select {
		case <-time.After(m.titleDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

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
		_ = yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
	}, nil
}

func (m *capturedCallModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, nil
}

func (m *capturedCallModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, nil
}

func (m *capturedCallModel) Provider() string { return "test-provider" }
func (m *capturedCallModel) Model() string    { return "test-model" }

func (m *capturedCallModel) snapshotCalls() []fantasy.Call {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]fantasy.Call(nil), m.calls...)
}

func promptText(prompt fantasy.Prompt) string {
	var sb strings.Builder
	for _, msg := range prompt {
		for _, part := range msg.Content {
			if text, ok := fantasy.AsMessagePart[fantasy.TextPart](part); ok {
				if sb.Len() > 0 {
					sb.WriteString("\n")
				}
				sb.WriteString(text.Text)
			}
		}
	}
	return sb.String()
}

func TestSessionAgentDirectReplyUsesLeanPromptAndNoTools(t *testing.T) {
	t.Setenv("SAPPHIRE_NON_INTERACTIVE", "1")

	env := testEnv(t)
	model := &capturedCallModel{responseText: "Hey"}
	dummyTool := fantasy.NewAgentTool(
		"dummy_tool",
		"dummy tool",
		func(ctx context.Context, input struct{}, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse("ok"), nil
		},
	)
	agent := testSessionAgent(env, model, model, "heavy system prompt", dummyTool)

	sess, err := env.sessions.Create(t.Context(), "direct reply")
	require.NoError(t, err)

	_, err = agent.Run(t.Context(), SessionAgentCall{
		SessionID:    sess.ID,
		Prompt:       "hi",
		SkillContext: "inventory and orchestration state that must not leak into trivial turns",
		ActiveSkills: []string{"debug"},
	})
	require.NoError(t, err)

	calls := model.snapshotCalls()
	require.Len(t, calls, 1)
	require.Empty(t, calls[0].Tools)

	text := promptText(calls[0].Prompt)
	require.Contains(t, text, "Reply naturally and briefly to the user.")
	require.NotContains(t, text, "Complexity mode:")
	require.NotContains(t, text, "update_plan checklist")
	require.NotContains(t, text, "active_skill_context")
	require.NotContains(t, text, "inventory and orchestration state")
}

func TestSessionAgentFirstTurnDoesNotWaitForTitleGeneration(t *testing.T) {
	t.Setenv("SAPPHIRE_NON_INTERACTIVE", "")

	env := testEnv(t)
	model := &capturedCallModel{
		responseText: "Hello",
		titleDelay:   350 * time.Millisecond,
	}
	agent := testSessionAgent(env, model, model, "")

	sess, err := env.sessions.Create(t.Context(), "title")
	require.NoError(t, err)

	start := time.Now()
	_, err = agent.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "hello",
	})
	elapsed := time.Since(start)
	require.NoError(t, err)
	require.Less(t, elapsed, 250*time.Millisecond)

	require.Eventually(t, func() bool {
		for _, call := range model.snapshotCalls() {
			if strings.Contains(promptText(call.Prompt), "Generate a concise title for the following content:") {
				return true
			}
		}
		return false
	}, time.Second, 10*time.Millisecond)
}
