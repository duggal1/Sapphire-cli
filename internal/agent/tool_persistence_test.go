package agent

import (
	"strings"
	"testing"

	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/stretchr/testify/require"
)

func TestCompactToolResultForPersistenceShrinksHeavyReadTools(t *testing.T) {
	t.Parallel()

	result := compactToolResultForPersistence(tools.AgenticViewToolName, message.ToolResult{
		Name:     tools.AgenticViewToolName,
		Content:  strings.Repeat("x", compactPersistedToolResultLimit+500),
		Data:     strings.Repeat("y", 2048),
		Metadata: strings.Repeat("z", 2048),
	})

	require.Contains(t, result.Content, "persisted summary")
	require.LessOrEqual(t, len(result.Content), compactPersistedToolResultLimit+128)
	require.Empty(t, result.Data)
	require.LessOrEqual(t, len(result.Metadata), 1027)
}

func TestCompactToolResultForPersistenceLeavesSmallNonReadToolsAlone(t *testing.T) {
	t.Parallel()

	original := message.ToolResult{
		Name:    "bash",
		Content: "ok",
	}
	require.Equal(t, original, compactToolResultForPersistence("bash", original))
}
