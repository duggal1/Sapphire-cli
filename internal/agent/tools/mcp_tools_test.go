package tools

import (
	"testing"

	mcptool "github.com/duggal1/Sapphire-cli/internal/agent/tools/mcp"
	"github.com/stretchr/testify/require"
)

func TestDynamicMCPToolInfoIsParallel(t *testing.T) {
	t.Parallel()

	tool := &Tool{
		mcpName: "example",
		tool: &mcptool.Tool{
			Name:        "search",
			Description: "search the remote system",
			InputSchema: map[string]any{
				"properties": map[string]any{
					"query": map[string]any{"type": "string"},
				},
				"required": []string{"query"},
			},
		},
	}

	info := tool.Info()
	require.True(t, info.Parallel)
	require.Equal(t, []string{"query"}, info.Required)
}
