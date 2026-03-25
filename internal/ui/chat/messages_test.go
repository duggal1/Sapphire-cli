package chat

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
)

func TestAssistantThinkingRenderUsesThinkingBoxPresentation(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	item := NewAssistantMessageItem(&sty, &message.Message{
		ID:   "assistant-thinking",
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.ReasoningContent{
				Thinking: "## Analyzing\n\n- first\n- second",
			},
		},
	})

	rendered := ansi.Strip(item.Render(100))
	if strings.Count(rendered, "Thinking") != 1 {
		t.Fatalf("expected exactly one thinking label, got %q", rendered)
	}
	if !strings.Contains(rendered, "Analyzing") || !strings.Contains(rendered, "first") {
		t.Fatalf("expected rendered thinking markdown, got %q", rendered)
	}
}

func TestUserMessageUsesChevronPrefix(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	item := NewUserMessageItem(&sty, &message.Message{
		ID:   "user-prefix",
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: "hello"},
		},
	}, nil)

	rendered := ansi.Strip(item.Render(80))
	if !strings.HasPrefix(rendered, "> hello") {
		t.Fatalf("expected chevron prefix, got %q", rendered)
	}
}

func TestAssistantInfoItemRendersLiveFooterBeforeFinish(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	msg := &message.Message{
		ID:        "assistant-footer",
		Role:      message.Assistant,
		Model:     "gpt-test",
		Parts:     []message.ContentPart{},
		CreatedAt: time.Now().Unix(),
	}

	item, ok := NewAssistantInfoItem(&sty, msg, nil, time.Now()).(*AssistantInfoItem)
	if !ok {
		t.Fatal("expected assistant info item")
	}
	item.SetRequestTiming(time.Now().Add(-2*time.Second), time.Time{})

	rendered := ansi.Strip(item.RawRender(100))
	if !strings.Contains(rendered, "[⏱") {
		t.Fatalf("expected live footer timer, got %q", rendered)
	}
	if strings.Contains(rendered, " in") || strings.Contains(rendered, " out") || strings.Contains(rendered, " cached") || strings.Contains(rendered, " thoughts") {
		t.Fatalf("expected running footer to hide token stats, got %q", rendered)
	}
	if !strings.Contains(rendered, "gpt-test") {
		t.Fatalf("expected model name in footer, got %q", rendered)
	}
	if strings.Contains(rendered, "Thinking") {
		t.Fatalf("expected live shimmer label to render in the assistant message, got %q", rendered)
	}
}

func TestAssistantInfoItemHidesZeroUsageStatsAfterFinish(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	msg := &message.Message{
		ID:    "assistant-finished-footer",
		Role:  message.Assistant,
		Model: "gpt-test",
		Parts: []message.ContentPart{
			message.Finish{
				Reason:           message.FinishReasonEndTurn,
				Message:          "done",
				PromptTokens:     0,
				CompletionTokens: 0,
				CachedTokens:     0,
				ThoughtsTokens:   0,
			},
		},
		CreatedAt: time.Now().Add(-3 * time.Second).Unix(),
	}

	item, ok := NewAssistantInfoItem(&sty, msg, nil, time.Now().Add(-4*time.Second)).(*AssistantInfoItem)
	if !ok {
		t.Fatal("expected assistant info item")
	}
	item.SetRequestTiming(time.Now().Add(-4*time.Second), time.Now())

	rendered := ansi.Strip(item.RawRender(100))
	if strings.Contains(rendered, " 0 in") || strings.Contains(rendered, " 0 out") || strings.Contains(rendered, " 0 cached") || strings.Contains(rendered, " 0 thoughts") {
		t.Fatalf("expected finished footer to hide zero-value stats, got %q", rendered)
	}
}

func TestAssistantMessageItemRendersInlineLoaderWithVerticalSpacing(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	item := NewAssistantMessageItem(&sty, &message.Message{
		ID:   "assistant-inline-loader",
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.ReasoningContent{Thinking: ""},
		},
	})

	rendered := ansi.Strip(item.Render(80))
	label := loadingPhraseForMessage("assistant-inline-loader")
	if !strings.Contains(rendered, label) {
		t.Fatalf("expected inline loader label, got %q", rendered)
	}
	lines := strings.Split(rendered, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "" || strings.TrimSpace(lines[len(lines)-1]) != "" {
		t.Fatalf("expected top and bottom padding around inline loader, got %q", rendered)
	}
	if !strings.Contains(lines[1], label) {
		t.Fatalf("expected vertical padding around inline loader, got %q", rendered)
	}
}
