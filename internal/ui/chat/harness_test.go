package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
)

func TestHarnessToolRenderUsesConcisePlanView(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	item := NewHarnessToolMessageItem(&sty, message.ToolCall{
		ID:       "harness-1",
		Name:     tools.RunHarnessToolName,
		Input:    `{"task":"demo of harness tool","goal_type":"implementation","mode":"execute"}`,
		Finished: true,
	}, &message.ToolResult{
		ToolCallID: "harness-1",
		Name:       tools.RunHarnessToolName,
		Content:    `{"required":false,"reason":"simple task","complexity_score":0,"mode":"execute","goal_type":"implementation","working_dir":"/Users/harshitduggal/workspace/Sapphire-cli","execution_mode":"single_agent","pattern":"single_track","agents":[{"name":"planner","role":"task decomposition"},{"name":"implementer","role":"implementation execution"},{"name":"reviewer","role":"verification"}],"required_skills":["harness"],"skill_policy":{"mode":"local_required_then_extended_if_missing","load_immediately":["harness"],"extended_allowed":false},"phases":["classify","load_skills","execute","verify"],"artifacts":["execution_contract","working_notes","verification_report","change_summary"],"verification_plan":["load required local skills before implementation","run the narrowest relevant verification first","check diagnostics after each edit batch","confirm the result matches the requested scope only"],"next_action":"load_skills_then_execute","source_skill":"internal/skills/bundled/harness/SKILL.md"}`,
	}, false)

	rendered := ansi.Strip(item.Render(120))
	if !strings.Contains(rendered, "Planned") {
		t.Fatalf("expected clean plan presentation, got %q", rendered)
	}
	if !strings.Contains(rendered, "Task") || !strings.Contains(rendered, "demo of harness tool") {
		t.Fatalf("expected task section, got %q", rendered)
	}
	if !strings.Contains(rendered, "Route") || !strings.Contains(rendered, "Single agent") {
		t.Fatalf("expected route summary, got %q", rendered)
	}
	if !strings.Contains(rendered, "Flow") || !strings.Contains(rendered, "classify -> load skills -> execute -> verify") {
		t.Fatalf("expected concise flow summary, got %q", rendered)
	}
	if !strings.Contains(rendered, "Next") || !strings.Contains(rendered, "Load skills, then execute") {
		t.Fatalf("expected next action summary, got %q", rendered)
	}
	if strings.Contains(rendered, "Execution") || strings.Contains(rendered, "├") || strings.Contains(rendered, "└") {
		t.Fatalf("expected tree structure to be removed, got %q", rendered)
	}
	if strings.Contains(rendered, "run_harness") ||
		strings.Contains(rendered, "goal_type") ||
		strings.Contains(rendered, "required_skills") ||
		strings.Contains(rendered, "execution_mode") ||
		strings.Contains(rendered, "result.json") {
		t.Fatalf("expected developer-facing JSON to be hidden, got %q", rendered)
	}
}

func TestHarnessToolPendingUsesPlanningLabel(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	item := NewHarnessToolMessageItem(&sty, message.ToolCall{
		ID:    "harness-pending",
		Name:  tools.RunHarnessToolName,
		Input: `{"task":"repo audit"}`,
	}, nil, false)

	rendered := ansi.Strip(item.Render(100))
	if !strings.Contains(rendered, "Planing..") {
		t.Fatalf("expected planning label, got %q", rendered)
	}
	if strings.Contains(rendered, "Run Harness") {
		t.Fatalf("expected developer-facing tool name to be hidden, got %q", rendered)
	}
}
