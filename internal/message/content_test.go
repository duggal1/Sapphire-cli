package message

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/sapphire/internal/llm/provider/gemini"
	"github.com/stretchr/testify/require"
)

func TestToAIMessageAssistantPreservesToolCallOrderAndThoughtSignature(t *testing.T) {
	t.Parallel()

	msg := &Message{
		Role: Assistant,
		Parts: []ContentPart{
			ReasoningContent{Thinking: "thinking", ThoughtSignature: "sig-123", ToolID: "call-1"},
			ToolCall{ID: "call-1", Name: "list_available_mcps", Input: `{"query":"stripe"}`},
			TextContent{Text: "after tool call"},
		},
	}

	aiMessages := msg.ToAIMessage()
	require.Len(t, aiMessages, 1)
	require.Len(t, aiMessages[0].Content, 3)

	reasoningPart, ok := fantasy.AsMessagePart[fantasy.ReasoningPart](aiMessages[0].Content[0])
	require.True(t, ok)
	require.Equal(t, "thinking", reasoningPart.Text)

	toolCallPart, ok := fantasy.AsMessagePart[fantasy.ToolCallPart](aiMessages[0].Content[1])
	require.True(t, ok)
	require.Equal(t, "call-1", toolCallPart.ToolCallID)

	metadata, ok := toolCallPart.ProviderOptions[gemini.Name]
	require.True(t, ok)
	googleMetadata, ok := metadata.(*gemini.ReasoningMetadata)
	require.True(t, ok)
	require.Equal(t, "sig-123", googleMetadata.Signature)

	textPart, ok := fantasy.AsMessagePart[fantasy.TextPart](aiMessages[0].Content[2])
	require.True(t, ok)
	require.Equal(t, "after tool call", textPart.Text)
}

func TestAppendThoughtSignatureWithoutReasoningBlockPreservesToolID(t *testing.T) {
	t.Parallel()

	msg := &Message{
		Role: Assistant,
		Parts: []ContentPart{
			ToolCall{ID: "call-1", Name: "list_available_mcps", Input: `{"query":"supabase"}`},
		},
	}

	msg.AppendThoughtSignature("sig-456", "call-1")

	aiMessages := msg.ToAIMessage()
	require.Len(t, aiMessages, 1)
	require.Len(t, aiMessages[0].Content, 2)

	toolCallPart, ok := fantasy.AsMessagePart[fantasy.ToolCallPart](aiMessages[0].Content[0])
	require.True(t, ok)
	metadata, ok := toolCallPart.ProviderOptions[gemini.Name]
	require.True(t, ok)
	googleMetadata, ok := metadata.(*gemini.ReasoningMetadata)
	require.True(t, ok)
	require.Equal(t, "sig-456", googleMetadata.Signature)
	require.Equal(t, "call-1", googleMetadata.ToolID)
}

func TestAppendContentOnlyExtendsLatestTextBlock(t *testing.T) {
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

	aiMessages := msg.ToAIMessage()
	require.Len(t, aiMessages, 1)
	require.Len(t, aiMessages[0].Content, 3)

	firstText, ok := fantasy.AsMessagePart[fantasy.TextPart](aiMessages[0].Content[0])
	require.True(t, ok)
	require.Equal(t, "first", firstText.Text)

	toolCall, ok := fantasy.AsMessagePart[fantasy.ToolCallPart](aiMessages[0].Content[1])
	require.True(t, ok)
	require.Equal(t, "call-1", toolCall.ToolCallID)

	secondText, ok := fantasy.AsMessagePart[fantasy.TextPart](aiMessages[0].Content[2])
	require.True(t, ok)
	require.Equal(t, "second + delta", secondText.Text)
}

func TestAppendReasoningContentPreservesLatestReasoningMetadata(t *testing.T) {
	t.Parallel()

	msg := &Message{
		Role: Assistant,
		Parts: []ContentPart{
			ReasoningContent{
				Thinking:         "first",
				ThoughtSignature: "sig-first",
				ToolID:           "call-1",
				FinishedAt:       time.Now().Unix(),
			},
			ToolCall{ID: "call-1", Name: "view", Input: `{"file_path":"a.go"}`},
			ReasoningContent{
				Thinking:         "second",
				ThoughtSignature: "sig-second",
				ToolID:           "call-2",
				Signature:        "anthropic-sig",
			},
		},
	}

	msg.AppendReasoningContent(" + delta")

	lastReasoning := msg.Parts[len(msg.Parts)-1].(ReasoningContent)
	require.Equal(t, "second + delta", lastReasoning.Thinking)
	require.Equal(t, "sig-second", lastReasoning.ThoughtSignature)
	require.Equal(t, "call-2", lastReasoning.ToolID)
	require.Equal(t, "anthropic-sig", lastReasoning.Signature)
}

func TestAppendThoughtSignatureTargetsLatestReasoningBlock(t *testing.T) {
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

	firstReasoning := msg.Parts[0].(ReasoningContent)
	require.Equal(t, "sig-first", firstReasoning.ThoughtSignature)
	require.Equal(t, "call-1", firstReasoning.ToolID)

	lastReasoning := msg.Parts[2].(ReasoningContent)
	require.Equal(t, "sig-second", lastReasoning.ThoughtSignature)
	require.Equal(t, "call-2", lastReasoning.ToolID)
}

func TestToAIMessageAssistantDoesNotInjectGeminiFallbackThoughtSignature(t *testing.T) {
	t.Parallel()

	msg := &Message{
		Role:     Assistant,
		Provider: "google",
		Model:    "gemini-3-flash-preview",
		Parts: []ContentPart{
			ToolCall{ID: "call-1", Name: "todos", Input: `{"tasks":["a"]}`},
			ToolCall{ID: "call-2", Name: "view", Input: `{"file_path":"a.go"}`},
		},
	}

	aiMessages := msg.ToAIMessage()
	require.Len(t, aiMessages, 1)
	require.Len(t, aiMessages[0].Content, 2)

	firstToolCall, ok := fantasy.AsMessagePart[fantasy.ToolCallPart](aiMessages[0].Content[0])
	require.True(t, ok)
	_, ok = firstToolCall.ProviderOptions[gemini.Name]
	require.False(t, ok)
}

func makeTestAttachments(n int, contentSize int) []Attachment {
	attachments := make([]Attachment, n)
	content := []byte(strings.Repeat("x", contentSize))
	for i := range n {
		attachments[i] = Attachment{
			FilePath: fmt.Sprintf("/path/to/file%d.txt", i),
			MimeType: "text/plain",
			Content:  content,
		}
	}
	return attachments
}

func BenchmarkPromptWithTextAttachments(b *testing.B) {
	cases := []struct {
		name        string
		numFiles    int
		contentSize int
	}{
		{"1file_100bytes", 1, 100},
		{"5files_1KB", 5, 1024},
		{"10files_10KB", 10, 10 * 1024},
		{"20files_50KB", 20, 50 * 1024},
	}

	for _, tc := range cases {
		attachments := makeTestAttachments(tc.numFiles, tc.contentSize)
		prompt := "Process these files"

		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_ = PromptWithTextAttachments(prompt, attachments)
			}
		})
	}
}
