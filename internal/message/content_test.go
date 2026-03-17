package message

import (
	"testing"
	"time"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/google"
	"github.com/stretchr/testify/require"
)

func TestToAIMessageAssistantUsesSingleReasoningBlockForGemini(t *testing.T) {
	t.Parallel()

	msg := &Message{
		Role: Assistant,
		Parts: []ContentPart{
			ReasoningContent{Thinking: "thinking", ThoughtSignature: "sig-123", ToolID: "call-1"},
			ToolCall{ID: "call-1", Name: "view", Input: `{"file_path":"a.go"}`},
			TextContent{Text: "after tool call"},
		},
	}

	aiMessages := msg.ToAIMessage()
	require.Len(t, aiMessages, 1)
	require.Len(t, aiMessages[0].Content, 3)

	textPart, ok := fantasy.AsMessagePart[fantasy.TextPart](aiMessages[0].Content[0])
	require.True(t, ok)
	require.Equal(t, "after tool call", textPart.Text)

	reasoningPart, ok := fantasy.AsMessagePart[fantasy.ReasoningPart](aiMessages[0].Content[1])
	require.True(t, ok)
	metadata, ok := reasoningPart.ProviderOptions[google.Name]
	require.True(t, ok)
	googleMetadata, ok := metadata.(*google.ReasoningMetadata)
	require.True(t, ok)
	require.Equal(t, "sig-123", googleMetadata.Signature)
	require.Equal(t, "call-1", googleMetadata.ToolID)

	toolCallPart, ok := fantasy.AsMessagePart[fantasy.ToolCallPart](aiMessages[0].Content[2])
	require.True(t, ok)
	require.Equal(t, "call-1", toolCallPart.ToolCallID)
	_, ok = toolCallPart.ProviderOptions[google.Name]
	require.False(t, ok)
}

func TestAppendThoughtSignatureStoresGeminiMetadataOnReasoningBlock(t *testing.T) {
	t.Parallel()

	msg := &Message{
		Role: Assistant,
		Parts: []ContentPart{
			ToolCall{ID: "call-1", Name: "view", Input: `{"file_path":"a.go"}`},
		},
	}

	msg.AppendThoughtSignature("sig-456", "call-1")

	reasoning := msg.ReasoningContent()
	require.Equal(t, "sig-456", reasoning.ThoughtSignature)
	require.Equal(t, "call-1", reasoning.ToolID)
}

func TestAppendContentExtendsFirstTextBlock(t *testing.T) {
	t.Parallel()

	msg := &Message{
		Role: Assistant,
		Parts: []ContentPart{
			TextContent{Text: "first"},
			ToolCall{ID: "call-1", Name: "view", Input: `{"file_path":"a.go"}`},
			TextContent{Text: "second"},
		},
	}

	msg.AppendContent(" + delta")

	require.Equal(t, "first + delta", msg.Content().Text)
}

func TestAppendReasoningContentPreservesExistingGeminiMetadata(t *testing.T) {
	t.Parallel()

	msg := &Message{
		Role: Assistant,
		Parts: []ContentPart{
			ReasoningContent{
				Thinking:         "first",
				ThoughtSignature: "sig-first",
				ToolID:           "call-1",
				Signature:        "anthropic-sig",
				FinishedAt:       time.Now().Unix(),
			},
			ToolCall{ID: "call-1", Name: "view", Input: `{"file_path":"a.go"}`},
		},
	}

	msg.AppendReasoningContent(" + delta")

	reasoning := msg.ReasoningContent()
	require.Equal(t, "first + delta", reasoning.Thinking)
	require.Equal(t, "sig-first", reasoning.ThoughtSignature)
	require.Equal(t, "call-1", reasoning.ToolID)
	require.Equal(t, "anthropic-sig", reasoning.Signature)
}

func TestAppendThoughtSignatureTargetsFirstReasoningBlock(t *testing.T) {
	t.Parallel()

	msg := &Message{
		Role: Assistant,
		Parts: []ContentPart{
			ReasoningContent{Thinking: "first", ThoughtSignature: "sig-first", ToolID: "call-1"},
			TextContent{Text: "separator"},
			ReasoningContent{Thinking: "second"},
		},
	}

	msg.AppendThoughtSignature("sig-second", "call-2")

	reasoning := msg.ReasoningContent()
	require.Equal(t, "sig-firstsig-second", reasoning.ThoughtSignature)
	require.Equal(t, "call-2", reasoning.ToolID)
}

func TestToAIMessageAssistantDoesNotAttachGeminiMetadataToToolCall(t *testing.T) {
	t.Parallel()

	msg := &Message{
		Role:     Assistant,
		Provider: "google",
		Model:    "gemini-3-flash-preview",
		Parts: []ContentPart{
			ReasoningContent{Thinking: "thinking", ThoughtSignature: "sig-123", ToolID: "call-1"},
			ToolCall{ID: "call-1", Name: "todos", Input: `{"tasks":["a"]}`},
		},
	}

	aiMessages := msg.ToAIMessage()
	require.Len(t, aiMessages, 1)
	require.Len(t, aiMessages[0].Content, 2)

	toolCall, ok := fantasy.AsMessagePart[fantasy.ToolCallPart](aiMessages[0].Content[1])
	require.True(t, ok)
	_, ok = toolCall.ProviderOptions[google.Name]
	require.False(t, ok)
}
