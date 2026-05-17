package agent

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

const (
	parallelExecutionToolCalls = 12
	parallelExecutionSleep     = 300 * time.Millisecond
)

type parallelExecutionInput struct {
	Value string `json:"value"`
}

type mockParallelExecutionModel struct {
	streamCount atomic.Int32
}

func (m *mockParallelExecutionModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockParallelExecutionModel) Stream(_ context.Context, _ fantasy.Call) (fantasy.StreamResponse, error) {
	turn := m.streamCount.Add(1)
	return func(yield func(fantasy.StreamPart) bool) {
		if turn == 1 {
			for i := range parallelExecutionToolCalls {
				if !yield(fantasy.StreamPart{
					Type:          fantasy.StreamPartTypeToolCall,
					ID:            fmt.Sprintf("call-%02d", i),
					ToolCallName:  "sleep",
					ToolCallInput: fmt.Sprintf(`{"value":"%02d"}`, i),
				}) {
					return
				}
			}
			yield(fantasy.StreamPart{
				Type:         fantasy.StreamPartTypeFinish,
				FinishReason: fantasy.FinishReasonToolCalls,
			})
			return
		}

		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "text-1"}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "text-1", Delta: "done"}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "text-1"}) {
			return
		}
		yield(fantasy.StreamPart{
			Type:         fantasy.StreamPartTypeFinish,
			Usage:        fantasy.Usage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2},
			FinishReason: fantasy.FinishReasonStop,
		})
	}, nil
}

func (m *mockParallelExecutionModel) Provider() string { return "mock" }
func (m *mockParallelExecutionModel) Model() string    { return "mock" }

func (m *mockParallelExecutionModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockParallelExecutionModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func TestFantasyExecutesParallelToolBatchesWithBoundedConcurrency(t *testing.T) {
	var (
		active        atomic.Int32
		maxConcurrent atomic.Int32
	)

	updateMax := func(next int32) {
		for {
			current := maxConcurrent.Load()
			if next <= current {
				return
			}
			if maxConcurrent.CompareAndSwap(current, next) {
				return
			}
		}
	}

	tool := fantasy.NewParallelAgentTool(
		"sleep",
		"sleep in parallel",
		func(ctx context.Context, input parallelExecutionInput, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			current := active.Add(1)
			updateMax(current)
			defer active.Add(-1)

			select {
			case <-ctx.Done():
				return fantasy.ToolResponse{}, ctx.Err()
			case <-time.After(parallelExecutionSleep):
			}

			return fantasy.NewTextResponse(input.Value), nil
		},
	)

	agent := fantasy.NewAgent(&mockParallelExecutionModel{}, fantasy.WithTools(tool))

	start := time.Now()
	result, err := agent.Stream(context.Background(), fantasy.AgentStreamCall{Prompt: "run the tools"})
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "done", result.Response.Content.Text())
	require.Equal(t, int32(5), maxConcurrent.Load())
	require.Less(t, elapsed, 1500*time.Millisecond)
}
