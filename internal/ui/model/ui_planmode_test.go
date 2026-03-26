package model

import (
	"testing"

	"github.com/duggal1/Sapphire-cli/internal/message"
)

func TestLatestPlanProposalFromMessagesUsesLatestAssistantPlan(t *testing.T) {
	t.Parallel()

	msgs := []message.Message{
		{
			ID:   "user-1",
			Role: message.User,
			Parts: []message.ContentPart{
				message.TextContent{Text: "make a plan"},
			},
		},
		{
			ID:   "assistant-1",
			Role: message.Assistant,
			Parts: []message.ContentPart{
				message.TextContent{Text: "<proposed_plan>\n# First\n\n- one\n</proposed_plan>"},
			},
		},
		{
			ID:   "assistant-2",
			Role: message.Assistant,
			Parts: []message.ContentPart{
				message.TextContent{Text: "intermediate explanation only"},
			},
		},
		{
			ID:   "assistant-3",
			Role: message.Assistant,
			Parts: []message.ContentPart{
				message.TextContent{Text: "<proposed_plan>\n# Final\n\n- two\n</proposed_plan>"},
			},
		},
	}

	got := latestPlanProposalFromMessages(msgs)
	if got != "# Final\n\n- two" {
		t.Fatalf("expected latest plan block content, got %q", got)
	}
}

func TestLatestPlanProposalFromMessagesIgnoresInvalidAssistantText(t *testing.T) {
	t.Parallel()

	msgs := []message.Message{
		{
			ID:   "assistant-1",
			Role: message.Assistant,
			Parts: []message.ContentPart{
				message.TextContent{Text: "plain explanation with no plan block"},
			},
		},
	}

	if got := latestPlanProposalFromMessages(msgs); got != "" {
		t.Fatalf("expected no plan content, got %q", got)
	}
}
