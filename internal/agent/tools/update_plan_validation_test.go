package tools

import (
	"context"
	"testing"

	"charm.land/fantasy"
)

func stubUpdatePlanTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(
		UpdatePlanToolName,
		"stub",
		func(context.Context, UpdatePlanArgs, fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse("ok"), nil
		},
	)
}

func TestPrepareToolCallRejectsEmptyPlanStepStatus(t *testing.T) {
	t.Parallel()

	registry := map[string]fantasy.AgentTool{
		UpdatePlanToolName: stubUpdatePlanTool(),
	}

	_, _, err := PrepareToolCall(context.Background(), fantasy.ToolCall{
		Name:  UpdatePlanToolName,
		Input: `{"plan":[{"step":"","status":""}]}`,
	}, registry)
	if err == nil {
		t.Fatal("expected invalid update_plan payload to be rejected")
	}
}

func TestPrepareToolCallNormalizesPlanStatus(t *testing.T) {
	t.Parallel()

	registry := map[string]fantasy.AgentTool{
		UpdatePlanToolName: stubUpdatePlanTool(),
	}

	call, _, err := PrepareToolCall(context.Background(), fantasy.ToolCall{
		Name:  UpdatePlanToolName,
		Input: `{"plan":[{"step":"Inspect renderer","status":"in progress"}]}`,
	}, registry)
	if err != nil {
		t.Fatalf("expected normalized update_plan payload, got error: %v", err)
	}

	if call.Input != `{"plan":[{"status":"in_progress","step":"Inspect renderer"}]}` &&
		call.Input != `{"plan":[{"step":"Inspect renderer","status":"in_progress"}]}` {
		t.Fatalf("unexpected normalized payload: %s", call.Input)
	}
}
