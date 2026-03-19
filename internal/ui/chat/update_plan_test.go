package chat

import (
	"strings"
	"testing"

	"github.com/duggal1/Sapphire-cli/internal/agent"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

func TestUpdatePlanRendersTodoTree(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	item := NewUpdatePlanToolMessageItem(&sty, message.ToolCall{
		ID:       "plan-1",
		Name:     tools.UpdatePlanToolName,
		Input:    `{"explanation":"Refined scope","plan":[{"step":"Inspect renderer path","status":"completed"},{"step":"Patch tool rendering","status":"in_progress"},{"step":"Run focused tests","status":"pending"}]}`,
		Finished: true,
	}, &message.ToolResult{
		ToolCallID: "plan-1",
		Name:       tools.UpdatePlanToolName,
		Content:    "Plan updated",
	}, false)

	rendered := ansi.Strip(item.Render(100))
	if !strings.Contains(rendered, "To-Do") || !strings.Contains(rendered, "Inspect renderer path") {
		t.Fatalf("expected todo tree, got %q", rendered)
	}
	if strings.Contains(rendered, "Update Plan") || strings.Contains(rendered, `"plan"`) {
		t.Fatalf("expected raw update_plan payload to stay hidden, got %q", rendered)
	}
}

func TestUpdatePlanInvalidInputRendersNothing(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	item := NewUpdatePlanToolMessageItem(&sty, message.ToolCall{
		ID:       "plan-2",
		Name:     tools.UpdatePlanToolName,
		Input:    `{}`,
		Finished: true,
	}, &message.ToolResult{
		ToolCallID: "plan-2",
		Name:       tools.UpdatePlanToolName,
		Content:    "missing required parameter: plan",
	}, false)

	if rendered := item.Render(100); rendered != "" {
		t.Fatalf("expected invalid update_plan render to be hidden, got %q", rendered)
	}
}

func TestSpawnAgentRejectionRendersHandledLocally(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	item := NewSpawnAgentToolMessageItem(&sty, message.ToolCall{
		ID:       "spawn-1",
		Name:     agent.SpawnAgentToolName,
		Input:    `{"message":"analyze repo"}`,
		Finished: true,
	}, &message.ToolResult{
		ToolCallID: "spawn-1",
		Name:       agent.SpawnAgentToolName,
		Content:    "sub-agent launch rejected: too small for delegation",
	}, false)
	item.SetStatus(ToolStatusError)

	rendered := ansi.Strip(item.Render(100))
	if !strings.Contains(rendered, "handled locally") {
		t.Fatalf("expected structured rejection fallback, got %q", rendered)
	}
	if strings.Contains(strings.ToLower(rendered), "too small for delegation") {
		t.Fatalf("expected raw delegation rejection to stay hidden, got %q", rendered)
	}
}
