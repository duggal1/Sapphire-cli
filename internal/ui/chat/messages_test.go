package chat

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/sapphire/internal/message"
	"github.com/charmbracelet/sapphire/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

func TestAssistantInfoRenderUsesFullWidthSeparator(t *testing.T) {
	sty := styles.DefaultStyles(false)
	item := NewAssistantInfoItem(&sty, &message.Message{
		ID:       "assistant-1",
		Role:     message.Assistant,
		Model:    "gemini-3-flash",
		Provider: "gemini",
		Parts: []message.ContentPart{
			message.Finish{Reason: message.FinishReasonEndTurn},
		},
	}, nil, time.Time{}).(*AssistantInfoItem)

	item.SetRequestTiming(time.Now().Add(-2*time.Second), time.Now())
	rendered := item.RawRender(24)
	lines := strings.Split(rendered, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 footer lines, got %d", len(lines))
	}
	if got := ansi.StringWidth(lines[0]); got != 22 {
		t.Fatalf("expected full-width separator, got width %d: %q", got, lines[0])
	}
}

func TestAssistantInfoRenderUsesRequestLifecycleTimer(t *testing.T) {
	sty := styles.DefaultStyles(false)
	item := NewAssistantInfoItem(&sty, &message.Message{
		ID:    "assistant-2",
		Role:  message.Assistant,
		Model: "gemini-3-flash",
		Parts: []message.ContentPart{
			message.Finish{
				Reason:      message.FinishReasonEndTurn,
				StartTimeMs: 1,
				EndTimeMs:   10,
			},
		},
	}, nil, time.Time{}).(*AssistantInfoItem)

	item.SetRequestTiming(time.Now().Add(-5*time.Second), time.Now())
	rendered := item.RawRender(40)
	if !strings.Contains(rendered, "5.0s") && !strings.Contains(rendered, "4.9s") && !strings.Contains(rendered, "5.1s") {
		t.Fatalf("expected lifecycle duration in footer, got %q", rendered)
	}
}
