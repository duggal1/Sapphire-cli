package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/duggal1/Sapphire-cli/internal/agent"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
)

func TestUpdatePlanRenderShowsChecklist(t *testing.T) {
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
	if !strings.Contains(rendered, "Inspect renderer path") {
		t.Fatalf("expected update_plan render to show plan step, got %q", rendered)
	}
	if !strings.Contains(rendered, "├──") && !strings.Contains(rendered, "└──") {
		t.Fatalf("expected update_plan render to restore tree branches, got %q", rendered)
	}
}

func TestUpdatePlanInvalidInputRendersError(t *testing.T) {
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

	rendered := ansi.Strip(item.Render(100))
	if !strings.Contains(rendered, "To-Do") {
		t.Fatalf("expected invalid update_plan render to remain visible, got %q", rendered)
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
