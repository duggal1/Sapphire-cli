package tools

import (
	"errors"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

func TestAnnotateRuntimeExecutionMetadataTracksRewrite(t *testing.T) {
	t.Parallel()

	metadata := AnnotateRuntimeExecutionMetadata("", fantasy.ToolCall{
		ID:    "call-1",
		Name:  LSToolName,
		Input: `{"path":"."}`,
	}, fantasy.ToolCall{
		ID:    "call-1",
		Name:  ToolSearchToolName,
		Input: `{"query":"Initialize the repo"}`,
	})

	parsed, ok := ParseRuntimeExecutionMetadata(metadata)
	require.True(t, ok)
	require.Equal(t, LSToolName, parsed.RequestedToolName)
	require.Equal(t, ToolSearchToolName, parsed.ExecutedToolName)
	require.True(t, parsed.Rewritten)

	observedTool, observedInput := ResolveObservedToolExecution(LSToolName, `{"path":"."}`, metadata)
	require.Equal(t, ToolSearchToolName, observedTool)
	require.Equal(t, `{"query":"Initialize the repo"}`, observedInput)
}

func TestAnnotateRuntimeExecutionErrorMetadataTracksRewrite(t *testing.T) {
	t.Parallel()

	err := WrapRuntimeExecutionError(errors.New("tool search failed"), fantasy.ToolCall{
		ID:    "call-2",
		Name:  LSToolName,
		Input: `{"path":"."}`,
	}, fantasy.ToolCall{
		ID:    "call-2",
		Name:  ToolSearchToolName,
		Input: `{"query":"broad init"}`,
	})

	metadata := AnnotateRuntimeExecutionErrorMetadata("", err)
	parsed, ok := ParseRuntimeExecutionMetadata(metadata)
	require.True(t, ok)
	require.Equal(t, LSToolName, parsed.RequestedToolName)
	require.Equal(t, ToolSearchToolName, parsed.ExecutedToolName)
	require.Equal(t, `{"query":"broad init"}`, parsed.ExecutedInput)
}
