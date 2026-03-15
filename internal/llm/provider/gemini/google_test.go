package gemini

import (
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

func TestToGooglePromptUsesToolCallThoughtSignatureMetadata(t *testing.T) {
	t.Parallel()

	_, content, warnings := toGooglePrompt(fantasy.Prompt{
		{
			Role: fantasy.MessageRoleAssistant,
			Content: []fantasy.MessagePart{
				fantasy.TextPart{Text: "preface"},
				fantasy.ToolCallPart{
					ToolCallID: "call-1",
					ToolName:   "list_available_mcps",
					Input:      `{"query":"stripe"}`,
					ProviderOptions: fantasy.ProviderOptions{
						Name: &ReasoningMetadata{
							Signature: "sig-123",
							ToolID:    "call-1",
						},
					},
				},
			},
		},
	})

	require.Empty(t, warnings)
	require.Len(t, content, 1)
	require.Len(t, content[0].Parts, 2)
	require.Equal(t, []byte("sig-123"), content[0].Parts[1].ThoughtSignature)
}

func TestToGooglePromptDoesNotEmitEmptyThoughtSignatureForParallelFollowupCall(t *testing.T) {
	t.Parallel()

	_, content, warnings := toGooglePrompt(fantasy.Prompt{
		{
			Role: fantasy.MessageRoleAssistant,
			Content: []fantasy.MessagePart{
				fantasy.ToolCallPart{
					ToolCallID: "call-1",
					ToolName:   "list_available_mcps",
					Input:      `{"query":"supabase"}`,
					ProviderOptions: fantasy.ProviderOptions{
						Name: &ReasoningMetadata{
							Signature: "sig-123",
							ToolID:    "call-1",
						},
					},
				},
				fantasy.ToolCallPart{
					ToolCallID: "call-2",
					ToolName:   "list_available_mcps",
					Input:      `{"query":"stripe"}`,
				},
			},
		},
	})

	require.Empty(t, warnings)
	require.Len(t, content, 1)
	require.Len(t, content[0].Parts, 2)
	require.Equal(t, []byte("sig-123"), content[0].Parts[0].ThoughtSignature)
	require.Nil(t, content[0].Parts[1].ThoughtSignature)
}
