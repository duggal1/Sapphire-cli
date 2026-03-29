package memory

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteRecordExplicitSavesDedupByContentWhenTurnIndexZero(t *testing.T) {
	t.Parallel()

	store, err := NewStore(t.TempDir(), "session-1", t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })

	firstID, err := store.WriteRecord(context.Background(), MemoryRecord{
		SessionID:               "session-1",
		EventType:               "architectural_decision",
		Timestamp:               1,
		TurnIndex:               0,
		Salience:                1.0,
		ContentJSON:             `{"decision":"Rule one"}`,
		IsArchitecturalDecision: true,
	})
	require.NoError(t, err)

	secondID, err := store.WriteRecord(context.Background(), MemoryRecord{
		SessionID:               "session-1",
		EventType:               "architectural_decision",
		Timestamp:               2,
		TurnIndex:               0,
		Salience:                1.0,
		ContentJSON:             `{"decision":"Rule two"}`,
		IsArchitecturalDecision: true,
	})
	require.NoError(t, err)
	require.NotEqual(t, firstID, secondID)

	duplicateID, err := store.WriteRecord(context.Background(), MemoryRecord{
		SessionID:               "session-1",
		EventType:               "architectural_decision",
		Timestamp:               3,
		TurnIndex:               0,
		Salience:                1.0,
		ContentJSON:             `{"decision":"Rule one"}`,
		IsArchitecturalDecision: true,
	})
	require.NoError(t, err)
	require.Equal(t, firstID, duplicateID)

	count, err := store.CountRecords(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 2, count)
}
