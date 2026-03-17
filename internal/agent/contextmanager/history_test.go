package contextmanager

import (
	"testing"

	"github.com/charmbracelet/sapphire/internal/message"
	"github.com/stretchr/testify/require"
)

func TestSliceFromSummaryCheckpointPreservesCheckpointForward(t *testing.T) {
	msgs := []message.Message{
		{ID: "m1", Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "first"}}},
		{ID: "m2", Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "summary checkpoint"}}},
		{ID: "m3", Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "recent assistant"}}},
	}

	sliced := SliceFromSummaryCheckpoint(msgs, "m2")
	require.Len(t, sliced, 2)
	require.Equal(t, "m2", sliced[0].ID)
	require.Equal(t, message.User, sliced[0].Role)
	require.Equal(t, "m3", sliced[1].ID)
}

func TestCompactionThresholdTriggersAtEightyFivePercentUsage(t *testing.T) {
	require.True(t, ShouldCompact(100_000, 70_000, 15_500))
	require.False(t, ShouldCompact(100_000, 70_000, 14_000))
	require.EqualValues(t, 30_000, CompactionThreshold(200_001))
}
