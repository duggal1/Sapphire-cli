package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
	"github.com/duggal1/Sapphire-cli/internal/ui/util"
)

func TestListToolOmitsRootDotPath(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	item := NewLSToolMessageItem(&sty, message.ToolCall{
		ID:       "ls-1",
		Name:     tools.LSToolName,
		Input:    `{"path":"."}`,
		Finished: true,
	}, &message.ToolResult{
		ToolCallID: "ls-1",
		Name:       tools.LSToolName,
		Content:    "- internal\n- README.md",
	}, false)

	rendered := ansi.Strip(item.Render(100))
	if strings.Contains(rendered, "List · .") {
		t.Fatalf("expected root list path to be omitted, got %q", rendered)
	}
	if strings.Contains(rendered, "Path: .") {
		t.Fatalf("expected root path field to be omitted, got %q", rendered)
	}
}

func TestToolCopyContentIncludesInputAndResult(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	item := NewBashToolMessageItem(&sty, message.ToolCall{
		ID:       "bash-1",
		Name:     tools.BashToolName,
		Input:    `{"command":"go test ./..."}`,
		Finished: true,
	}, &message.ToolResult{
		ToolCallID: "bash-1",
		Name:       tools.BashToolName,
		Content:    "line one\nline two",
		Metadata:   `{"output":"line one\nline two"}`,
	}, false)

	copyable, ok := item.(interface{ CopyContent() string })
	if !ok {
		t.Fatal("expected bash tool item to expose CopyContent")
	}

	content := copyable.CopyContent()
	for _, expected := range []string{
		"## Bash Tool Call",
		"**Command:** go test ./...",
		"### Result:",
		"line two",
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("expected copy payload to contain %q, got %q", expected, content)
		}
	}
}

func TestNewCopyMsgUsesCopyToastType(t *testing.T) {
	t.Parallel()

	msg := util.NewCopyMsg("Copied to clipboard")
	if msg.Type != util.InfoTypeCopy {
		t.Fatalf("expected copy toast type, got %v", msg.Type)
	}
	if msg.Msg != "Copied to clipboard" {
		t.Fatalf("expected copy toast message, got %q", msg.Msg)
	}
}
