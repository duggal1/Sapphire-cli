package chat

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func TestWebSearchToolRenderContextRendersResultTree(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	renderer := &WebSearchToolRenderContext{}
	opts := &ToolRenderOpts{
		ToolCall: message.ToolCall{
			Finished: true,
			Input:    `{"query":"Google Search"}`,
		},
		Result: &message.ToolResult{
			Content: "Found 2 search results:\n\n1. Example Title\n   URL: https://example.com\n   Summary: Example summary\n\n2. Another Title\n   URL: https://example.org\n   Summary: Another summary\n",
		},
		Status: ToolStatusSuccess,
	}

	rendered := renderer.RenderTool(&sty, 120, opts)

	require.Contains(t, rendered, "Search")
	require.Contains(t, rendered, "Results · 2")
	require.Contains(t, rendered, "Example Title")
	require.Contains(t, rendered, "Another Title")
	require.Contains(t, rendered, "URL")
	require.Contains(t, rendered, "Summary")
	require.Contains(t, rendered, "├")
	require.Contains(t, rendered, "└")
}

func TestWebSearchToolRenderContextRendersParallelTree(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	renderer := &WebSearchToolRenderContext{}
	opts := &ToolRenderOpts{
		ToolCall: message.ToolCall{
			Finished: true,
			Input:    `{"queries":["latest go version march 2026","goland 2026.1 release"],"max_results":3}`,
		},
		Result: &message.ToolResult{
			Content: "Searched 2 queries in parallel.\n\nQuery: latest go version march 2026\nFound 1 search results:\n\n1. Go 1.26 is released\n   URL: https://go.dev/blog/go1.26\n   Summary: New language release.\n\nQuery: goland 2026.1 release\nFound 1 search results:\n\n1. GoLand 2026.1 Is Released\n   URL: https://blog.jetbrains.com/go/2026/03/26/goland-2026-1-is-released/\n   Summary: IDE release notes.\n",
		},
		Status: ToolStatusSuccess,
	}

	rendered := renderer.RenderTool(&sty, 120, opts)

	require.Contains(t, rendered, "Results · 2 queries")
	require.Contains(t, rendered, "latest go version march 2026")
	require.Contains(t, rendered, "Go 1.26 is released")
	require.Contains(t, rendered, "GoLand 2026.1 Is Released")
}

func TestWebSearchToolRenderContextRendersPendingLoader(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	renderer := &WebSearchToolRenderContext{}
	opts := &ToolRenderOpts{
		ToolCall: message.ToolCall{
			Finished: false,
			Input:    `{"query":"latest go version"}`,
		},
		Status: ToolStatusRunning,
	}

	rendered := ansi.Strip(renderer.RenderTool(&sty, 120, opts))

	require.Contains(t, rendered, "Search")
	require.Contains(t, rendered, "Searching the web")
}

func TestGoogleSearchToolRenderContextRendersGroundingTree(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	renderer := &GoogleSearchToolRenderContext{}
	opts := &ToolRenderOpts{
		ToolCall: message.ToolCall{
			Finished: true,
			Input:    `{"query":"latest go version march 2026"}`,
		},
		Result: &message.ToolResult{
			Content: "Answer:\nGo 1.26.1 is the latest stable release.\n\nGoogle search queries:\n- latest go version march 2026\n\nGrounded web sources (1):\n1. Go 1.26 is released\n   URL: https://go.dev/blog/go1.26\n\nURL context retrieval:\n- https://go.dev/blog/go1.26 [URL_RETRIEVAL_STATUS_SUCCESS]\n",
		},
		Status: ToolStatusSuccess,
	}

	rendered := renderer.RenderTool(&sty, 120, opts)

	require.Contains(t, rendered, "Google Grounding")
	require.Contains(t, rendered, "Grounding")
	require.Contains(t, rendered, "Answer")
	require.Contains(t, rendered, "Queries · 1")
	require.Contains(t, rendered, "Sources · 1")
	require.Contains(t, rendered, "URL Context · 1")
	require.Contains(t, rendered, "Go 1.26 is released")
}
