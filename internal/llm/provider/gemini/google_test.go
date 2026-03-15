package gemini

import (
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

func TestToGooglePromptUsesReasoningThoughtSignatureForFirstFunctionCall(t *testing.T) {
	t.Parallel()

	_, content, warnings := toGooglePrompt(fantasy.Prompt{
		{
			Role: fantasy.MessageRoleAssistant,
			Content: []fantasy.MessagePart{
				fantasy.ReasoningPart{
					Text: "thinking",
					ProviderOptions: fantasy.ProviderOptions{
						Name: &ReasoningMetadata{
							Signature: "sig-123",
							ToolID:    "call-1",
						},
					},
				},
				fantasy.ToolCallPart{
					ToolCallID: "call-1",
					ToolName:   "view",
					Input:      `{"file_path":"a.go"}`,
				},
			},
		},
	})

	require.Empty(t, warnings)
	require.Len(t, content, 1)
	require.Len(t, content[0].Parts, 1)
	require.Equal(t, []byte("sig-123"), content[0].Parts[0].ThoughtSignature)
}

func TestToGooglePromptDoesNotInventFallbackThoughtSignature(t *testing.T) {
	t.Parallel()

	_, content, warnings := toGooglePrompt(fantasy.Prompt{
		{
			Role: fantasy.MessageRoleAssistant,
			Content: []fantasy.MessagePart{
				fantasy.ToolCallPart{
					ToolCallID: "call-1",
					ToolName:   "view",
					Input:      `{"file_path":"a.go"}`,
				},
			},
		},
	})

	require.Empty(t, warnings)
	require.Len(t, content, 1)
	require.Len(t, content[0].Parts, 1)
	require.Nil(t, content[0].Parts[0].ThoughtSignature)
}

func TestToGooglePromptAcceptsLegacyToolCallThoughtSignatureMetadata(t *testing.T) {
	t.Parallel()

	_, content, warnings := toGooglePrompt(fantasy.Prompt{
		{
			Role: fantasy.MessageRoleAssistant,
			Content: []fantasy.MessagePart{
				fantasy.ToolCallPart{
					ToolCallID: "call-1",
					ToolName:   "view",
					Input:      `{"file_path":"a.go"}`,
					ProviderOptions: fantasy.ProviderOptions{
						Name: &ReasoningMetadata{
							Signature: "sig-legacy",
							ToolID:    "call-1",
						},
					},
				},
			},
		},
	})

	require.Empty(t, warnings)
	require.Len(t, content, 1)
	require.Len(t, content[0].Parts, 1)
	require.Equal(t, []byte("sig-legacy"), content[0].Parts[0].ThoughtSignature)
}
