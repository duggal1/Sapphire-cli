package agent

import (
	"testing"

	"charm.land/fantasy"
	toolpkg "github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/stretchr/testify/require"
)

func TestConvertToToolResultAddsUserVisibleErrorMetadata(t *testing.T) {
	t.Parallel()

	a := &sessionAgent{}
	result := fantasy.ToolResultContent{
		ToolCallID: "tc-1",
		ToolName:   toolpkg.WebSearchToolName,
		Result: fantasy.ToolResultOutputContentError{
			Error: toolpkg.NewToolGuidanceError(
				toolpkg.WebSearchToolName,
				"missing_query",
				"Missing search query.",
				"web_search requires query or queries. Do not call it with empty input.",
			),
		},
	}

	converted := a.convertToToolResult(result)
	require.True(t, converted.IsError)
	require.Equal(t, "web_search requires query or queries. Do not call it with empty input.", converted.Content)

	meta, ok := toolpkg.ParseToolErrorMetadata(converted.Metadata)
	require.True(t, ok)
	require.Equal(t, "Missing search query.", meta.UIMessage)
}

func TestConvertToToolResultUsesExecutedToolMetadataForRewrites(t *testing.T) {
	t.Parallel()

	a := &sessionAgent{}
	result := fantasy.ToolResultContent{
		ToolCallID: "tc-2",
		ToolName:   toolpkg.LSToolName,
		Result: fantasy.ToolResultOutputContentError{
			Error: toolpkg.WrapRuntimeExecutionError(
				toolpkg.NewToolGuidanceError(
					toolpkg.ToolSearchToolName,
					"missing_query",
					"Missing search query.",
					"tool_search requires a query.",
				),
				fantasy.ToolCall{ID: "tc-2", Name: toolpkg.LSToolName, Input: `{"path":"."}`},
				fantasy.ToolCall{ID: "tc-2", Name: toolpkg.ToolSearchToolName, Input: `{"query":"initialize repo"}`},
			),
		},
	}

	converted := a.convertToToolResult(result)
	require.True(t, converted.IsError)
	require.Equal(t, toolpkg.ToolSearchToolName, converted.Name)

	runtimeMeta, ok := toolpkg.ParseRuntimeExecutionMetadata(converted.Metadata)
	require.True(t, ok)
	require.Equal(t, toolpkg.LSToolName, runtimeMeta.RequestedToolName)
	require.Equal(t, toolpkg.ToolSearchToolName, runtimeMeta.ExecutedToolName)

	errMeta, ok := toolpkg.ParseToolErrorMetadata(converted.Metadata)
	require.True(t, ok)
	require.Equal(t, toolpkg.ToolSearchToolName, errMeta.ToolName)
	require.Equal(t, "Missing search query.", errMeta.UIMessage)
}
