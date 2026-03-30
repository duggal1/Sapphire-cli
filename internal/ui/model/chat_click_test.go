package model

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/duggal1/Sapphire-cli/internal/ui/chat"
	"github.com/duggal1/Sapphire-cli/internal/ui/common"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
)

func TestToolSingleClickExpandsImmediately(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	item := chat.NewBashToolMessageItem(&sty, message.ToolCall{
		ID:       "bash-2",
		Name:     tools.BashToolName,
		Input:    `{"command":"go test ./..."}`,
		Finished: true,
	}, &message.ToolResult{
		ToolCallID: "bash-2",
		Name:       tools.BashToolName,
		Content:    "line one\nline two",
		Metadata:   `{"output":"line one\nline two"}`,
	}, false)

	before := ansi.Strip(item.Render(100))
	if !strings.Contains(before, "Output hidden by default") {
		t.Fatalf("expected collapsed tool output summary before click, got %q", before)
	}
	if strings.Contains(before, "line two") {
		t.Fatalf("expected collapsed tool output to hide full content, got %q", before)
	}

	chatModel := NewChat(common.DefaultCommon(nil))
	chatModel.SetSize(100, 20)
	chatModel.SetMessages(item)

	handled, cmd := chatModel.HandleMouseDown(0, 0)
	if !handled {
		t.Fatal("expected click to be handled")
	}
	if cmd != nil {
		t.Fatal("expected immediate expansion to avoid delayed click command")
	}

	after := ansi.Strip(item.Render(100))
	if !strings.Contains(after, "line two") {
		t.Fatalf("expected tool output to expand immediately after click, got %q", after)
	}
}
