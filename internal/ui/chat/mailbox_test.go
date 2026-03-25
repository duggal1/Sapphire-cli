package chat

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/duggal1/Sapphire-cli/internal/agent"
	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
)

func TestMailboxPresentationHumanizesTransportPayload(t *testing.T) {
	t.Parallel()

	item := presentMailboxItem(mailboxToolMessage{
		From:      "agent-123",
		Subject:   "SUBAGENT_DONE",
		Body:      "Agent: agent-123\nAssignment: Orchestration Analyst\nSummary: Found 3 orchestration weaknesses\nNext: Patch supervisor loop",
		CreatedAt: time.Now().Add(-2 * time.Minute),
	})

	if item.Kind != "Completed" {
		t.Fatalf("expected completed kind, got %q", item.Kind)
	}
	if item.Source != "Orchestration Analyst" {
		t.Fatalf("expected assignment-based source, got %q", item.Source)
	}
	if !strings.Contains(item.Summary, "Found 3 orchestration weaknesses") {
		t.Fatalf("expected human summary, got %q", item.Summary)
	}
}

func TestMailboxToolRendererAvoidsRawInboxDump(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	payload, err := json.Marshal([]mailboxToolMessage{{
		ID:        "msg-1",
		From:      "supervisor",
		Subject:   "CRITICAL: Sub-agent intervention required",
		Body:      "Summary: Timeout monitor needs review\nThread: subagent-1",
		CreatedAt: time.Now().Add(-time.Minute),
	}})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	item := NewAgentMailInboxToolMessageItem(&sty, message.ToolCall{
		ID:       "tool-1",
		Name:     agent.AgentMailInboxToolName,
		Finished: true,
	}, &message.ToolResult{
		ToolCallID: "tool-1",
		Name:       agent.AgentMailInboxToolName,
		Content:    string(payload),
	}, false)

	rendered := ansi.Strip(item.Render(120))
	if !strings.Contains(rendered, "Mailbox") {
		t.Fatalf("expected mailbox heading, got %q", rendered)
	}
	if !strings.Contains(rendered, "Action Required") {
		t.Fatalf("expected humanized category, got %q", rendered)
	}
	if strings.Contains(rendered, "\"subject\"") || strings.Contains(rendered, "\"thread_id\"") {
		t.Fatalf("expected raw JSON fields to stay hidden, got %q", rendered)
	}
}
