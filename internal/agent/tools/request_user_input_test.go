package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/sapphire/internal/config"
	"github.com/charmbracelet/sapphire/internal/planmode"
	"github.com/stretchr/testify/require"
)

func TestRequestUserInputToolRejectsDefaultMode(t *testing.T) {
	t.Parallel()

	tool := NewRequestUserInputTool()
	ctx := context.WithValue(t.Context(), AgentModeContextKey, config.AgentModeDefault)
	ctx = context.WithValue(ctx, SessionIDContextKey, "session-1")

	_, err := tool.Run(ctx, fantasy.ToolCall{
		ID:    "rui-default",
		Name:  RequestUserInputToolName,
		Input: `{"questions":[{"id":"choice","header":"Choice","question":"Pick one","options":[{"label":"A","description":"first"}]}]}`,
	})
	require.ErrorContains(t, err, "request_user_input is unavailable")
}

func TestRequestUserInputToolReturnsUserResponse(t *testing.T) {
	t.Parallel()

	tool := NewRequestUserInputTool()
	ctx := context.WithValue(t.Context(), AgentModeContextKey, config.AgentModePlan)
	ctx = context.WithValue(ctx, SessionIDContextKey, "session-1")

	done := make(chan struct{})
	responded := make(chan bool, 1)
	go func() {
		defer close(done)
		time.Sleep(20 * time.Millisecond)
		responded <- planmode.Respond("rui-plan", planmode.Response{
			Answers: map[string]planmode.Answer{
				"choice": {Answers: []string{"A (Recommended)"}},
			},
		})
	}()

	resp, err := tool.Run(ctx, fantasy.ToolCall{
		ID:   "rui-plan",
		Name: RequestUserInputToolName,
		Input: `{"questions":[{"id":"choice","header":"Choice","question":"Pick one","options":[` +
			`{"label":"A (Recommended)","description":"first"},` +
			`{"label":"B","description":"second"}]}]}`,
	})
	require.NoError(t, err)
	require.False(t, resp.IsError)

	var parsed planmode.Response
	require.NoError(t, json.Unmarshal([]byte(resp.Content), &parsed))
	require.Equal(t, []string{"A (Recommended)"}, parsed.Answers["choice"].Answers)
	<-done
	require.True(t, <-responded)
}
