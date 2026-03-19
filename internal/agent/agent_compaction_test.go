package agent

import (
	"strings"
	"testing"

	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/stretchr/testify/require"
)

func TestBuildCompactionContinuationCallUsesContinuationPromptForPartialText(t *testing.T) {
	t.Parallel()

	call := SessionAgentCall{Prompt: "Fix the failing request"}
	assistant := &message.Message{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: "I found the root cause and started applying the fix"},
		},
	}

	continued := buildCompactionContinuationCall(call, assistant)
	require.Contains(t, continued.Prompt, "Continue from where it stopped")
	require.Contains(t, continued.Prompt, "Fix the failing request")
	require.Contains(t, continued.Prompt, "I found the root cause")
}

func TestBuildCompactionContinuationCallTrimsPartialTail(t *testing.T) {
	t.Parallel()

	call := SessionAgentCall{Prompt: "Continue"}
	assistant := &message.Message{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: strings.Repeat("x", 1500)},
		},
	}

	continued := buildCompactionContinuationCall(call, assistant)
	require.LessOrEqual(t, len(continued.Prompt), len(call.Prompt)+1600)
	require.NotContains(t, continued.Prompt, strings.Repeat("x", 1300))
}
