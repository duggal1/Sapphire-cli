package tools

import (
	"context"
	"strings"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

func TestPrepareToolCallUnknownToolReturnsStrictGuidance(t *testing.T) {
	t.Parallel()

	call := fantasy.ToolCall{ID: "tc-1", Name: "memory_query", Input: `{"query":"mistake"}`}
	registry := map[string]fantasy.AgentTool{
		"view_memory": fantasy.NewAgentTool("view_memory", "view memory", func(ctx context.Context, input struct{}, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse("ok"), nil
		}),
	}

	_, _, err := PrepareToolCall(context.Background(), call, registry)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Stop inventing tool names")

	metadata := AnnotateToolErrorMetadata(call.Name, err, "")
	parsed, ok := ParseToolErrorMetadata(metadata)
	require.True(t, ok)
	require.Equal(t, "Unknown tool call.", parsed.UIMessage)
}

func TestPrepareToolCallRejectsEmptyViewWithStrictGuidance(t *testing.T) {
	t.Parallel()

	registry := map[string]fantasy.AgentTool{
		ViewToolName: fantasy.NewAgentTool(ViewToolName, "view file", func(ctx context.Context, input ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse("ok"), nil
		}),
	}

	_, _, err := PrepareToolCall(context.Background(), fantasy.ToolCall{
		ID:    "tc-2",
		Name:  ViewToolName,
		Input: `{}`,
	}, registry)
	require.Error(t, err)
	require.Contains(t, err.Error(), "single_view/agentic_view require explicit file path arguments")
}

func TestDeriveToolErrorMetadataForStructuredBashRejection(t *testing.T) {
	t.Parallel()

	meta, ok := DeriveToolErrorMetadata(BashToolName, "do not use bash for repository discovery, file reads, or delegation payload setup when structured tools exist")
	require.True(t, ok)
	require.Equal(t, "Use structured tools instead of bash.", meta.UIMessage)
}

func TestAnnotateToolErrorMetadataPreservesExistingPayload(t *testing.T) {
	t.Parallel()

	err := NewToolGuidanceError(WebSearchToolName, "missing_query", "Missing search query.", "web_search requires query")
	existing := `{"foo":"bar"}`
	metadata := AnnotateToolErrorMetadata(WebSearchToolName, err, existing)
	require.Contains(t, metadata, `"foo":"bar"`)
	require.True(t, strings.Contains(metadata, `"ui_message":"Missing search query."`))
}

func TestDeriveToolErrorMetadataForEditGuardBlock(t *testing.T) {
	t.Parallel()

	meta, ok := DeriveToolErrorMetadata(EditToolName, "edit blocked: fix all current-file errors and warnings in /tmp/first.go before editing /tmp/second.go")
	require.True(t, ok)
	require.Equal(t, "Finish fixing the active file before editing another file.", meta.UIMessage)
}
