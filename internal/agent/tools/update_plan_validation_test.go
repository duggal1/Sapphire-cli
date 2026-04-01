package tools

import (
	"context"
	"strings"
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

func TestPrepareToolCallDropsBlankPlanItems(t *testing.T) {
	t.Parallel()

	registry := map[string]fantasy.AgentTool{
		UpdatePlanToolName: stubUpdatePlanTool(),
	}

	call, _, err := PrepareToolCall(context.Background(), fantasy.ToolCall{
		Name:  UpdatePlanToolName,
		Input: `{"plan":[{"step":"","status":"pending"},{"step":"Inspect renderer","status":"in progress"}]}`,
	}, registry)
	if err != nil {
		t.Fatalf("expected normalized update_plan payload, got error: %v", err)
	}

	if strings.Contains(call.Input, `"step":""`) {
		t.Fatalf("expected blank plan step to be removed, got %s", call.Input)
	}
}

func TestPrepareToolCallNormalizesStringifiedPlanArray(t *testing.T) {
	t.Parallel()

	registry := map[string]fantasy.AgentTool{
		UpdatePlanToolName: stubUpdatePlanTool(),
	}

	call, _, err := PrepareToolCall(context.Background(), fantasy.ToolCall{
		Name:  UpdatePlanToolName,
		Input: `{"plan":"[{\"step\":\"Inspect renderer\",\"status\":\"in progress\"}]","explanation":" keep current "}`,
	}, registry)
	if err != nil {
		t.Fatalf("expected stringified update_plan payload to be normalized, got error: %v", err)
	}

	if !strings.Contains(call.Input, `"step":"Inspect renderer"`) {
		t.Fatalf("expected normalized plan step, got %s", call.Input)
	}
	if !strings.Contains(call.Input, `"status":"in_progress"`) {
		t.Fatalf("expected normalized plan status, got %s", call.Input)
	}
	if !strings.Contains(call.Input, `"explanation":"keep current"`) {
		t.Fatalf("expected trimmed explanation, got %s", call.Input)
	}
}

func TestRequirePostWriteVerificationCompletionBlocksCompletedPlanWithoutReadback(t *testing.T) {
	t.Parallel()

	usage := NewToolUsageState()
	usage.MarkArtifactWrite("/repo/AGENTS.md")

	ctx := context.WithValue(context.Background(), ToolUsageStateContextKey, usage)
	err := RequirePostWriteVerificationCompletion(ctx, LearnedToolPolicy{
		TaskFamily:                   "initialize/broad/codebase",
		RequirePostWriteVerification: true,
	})
	if err == nil {
		t.Fatal("expected pending artifact verification to block completion")
	}
	if !strings.Contains(err.Error(), "AGENTS.md") {
		t.Fatalf("expected AGENTS.md in error, got %v", err)
	}
}

func TestRequirePostWriteVerificationCompletionAllowsCompletedPlanAfterReadback(t *testing.T) {
	t.Parallel()

	usage := NewToolUsageState()
	usage.MarkArtifactWrite("/repo/AGENTS.md")
	usage.MarkArtifactVerified("/repo/AGENTS.md")

	ctx := context.WithValue(context.Background(), ToolUsageStateContextKey, usage)
	err := RequirePostWriteVerificationCompletion(ctx, LearnedToolPolicy{
		TaskFamily:                   "initialize/broad/codebase",
		RequirePostWriteVerification: true,
	})
	if err != nil {
		t.Fatalf("expected verified artifact to allow completion, got %v", err)
	}
}

func TestObserveSuccessfulTurnGuardrailResultTracksArtifactWriteAndReadback(t *testing.T) {
	t.Parallel()

	usage := NewToolUsageState()
	ctx := context.WithValue(context.Background(), ToolUsageStateContextKey, usage)
	ctx = context.WithValue(ctx, WorkingDirContextKey, "/repo")

	ObserveSuccessfulTurnGuardrailResult(ctx, WriteToolName, `{"file_path":"AGENTS.md","content":"# guide"}`, false)
	if pending := usage.PendingArtifactVerificationPaths(); len(pending) != 1 {
		t.Fatalf("expected one pending artifact verification, got %v", pending)
	}

	ObserveSuccessfulTurnGuardrailResult(ctx, SingleViewToolName, `{"file_path":"AGENTS.md"}`, false)
	if pending := usage.PendingArtifactVerificationPaths(); len(pending) != 0 {
		t.Fatalf("expected verification to clear pending artifacts, got %v", pending)
	}
}

func TestObserveSuccessfulTurnGuardrailResultTracksGenericFileWriteAndReadback(t *testing.T) {
	t.Parallel()

	usage := NewToolUsageState()
	ctx := context.WithValue(context.Background(), ToolUsageStateContextKey, usage)
	ctx = context.WithValue(ctx, WorkingDirContextKey, "/repo")

	ObserveSuccessfulTurnGuardrailResult(ctx, EditToolName, `{"file_path":"internal/agent/agent.go","old_string":"a","new_string":"b"}`, false)
	pending := usage.PendingArtifactVerificationPaths()
	if len(pending) != 1 || pending[0] != "/repo/internal/agent/agent.go" {
		t.Fatalf("expected generic edited file to be pending verification, got %v", pending)
	}

	ObserveSuccessfulTurnGuardrailResult(ctx, DiagnosticsToolName, `{"path":"internal/agent/agent.go"}`, false)
	if pending := usage.PendingArtifactVerificationPaths(); len(pending) != 0 {
		t.Fatalf("expected diagnostics readback to clear pending artifacts, got %v", pending)
	}
}

func TestObserveSuccessfulTurnGuardrailResultClearsPendingArtifactsAfterVerificationCommand(t *testing.T) {
	t.Parallel()

	usage := NewToolUsageState()
	ctx := context.WithValue(context.Background(), ToolUsageStateContextKey, usage)
	ctx = context.WithValue(ctx, WorkingDirContextKey, "/repo")

	ObserveSuccessfulTurnGuardrailResult(ctx, WriteToolName, `{"file_path":"internal/agent/agent.go","content":"package agent"}`, false)
	if pending := usage.PendingArtifactVerificationPaths(); len(pending) != 1 {
		t.Fatalf("expected one pending artifact before validation, got %v", pending)
	}

	ObserveSuccessfulTurnGuardrailResult(ctx, BashToolName, `{"command":"go test ./internal/agent -count=1","description":"verify agent package"}`, false)
	if pending := usage.PendingArtifactVerificationPaths(); len(pending) != 0 {
		t.Fatalf("expected successful verification command to clear pending artifacts, got %v", pending)
	}
}

func TestObserveSuccessfulTurnGuardrailResultDoesNotClearPendingArtifactsForNonVerificationCommand(t *testing.T) {
	t.Parallel()

	usage := NewToolUsageState()
	ctx := context.WithValue(context.Background(), ToolUsageStateContextKey, usage)
	ctx = context.WithValue(ctx, WorkingDirContextKey, "/repo")

	ObserveSuccessfulTurnGuardrailResult(ctx, WriteToolName, `{"file_path":"internal/agent/agent.go","content":"package agent"}`, false)
	ObserveSuccessfulTurnGuardrailResult(ctx, BashToolName, `{"command":"rg SessionAgent internal/agent","description":"search usage"}`, false)

	pending := usage.PendingArtifactVerificationPaths()
	if len(pending) != 1 || pending[0] != "/repo/internal/agent/agent.go" {
		t.Fatalf("expected non-verification bash command to keep pending artifact, got %v", pending)
	}
}
