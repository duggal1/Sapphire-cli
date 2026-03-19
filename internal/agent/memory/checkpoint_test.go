package memory

import (
	"context"
	"testing"
	"time"

	"github.com/duggal1/Sapphire-cli/internal/db"
	"github.com/duggal1/Sapphire-cli/internal/message"
	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
	"github.com/stretchr/testify/require"
)

type checkpointMessageStub struct {
	items []message.Message
}

func (m checkpointMessageStub) List(context.Context, string) ([]message.Message, error) {
	return m.items, nil
}

type checkpointMemoryStub struct {
	summary *StructuredSummaryData
}

func (checkpointMemoryStub) GetProjectConstitution(context.Context, string) (string, error) {
	return "", nil
}
func (checkpointMemoryStub) UpsertProjectConstitution(context.Context, string, string) error {
	return nil
}
func (m checkpointMemoryStub) GetStructuredSummary(context.Context, string) (*StructuredSummaryData, error) {
	return m.summary, nil
}
func (checkpointMemoryStub) CreateStructuredSummary(context.Context, string, StructuredSummaryData) error {
	return nil
}
func (checkpointMemoryStub) GetCodebaseKnowledge(context.Context, string) ([]db.CodebaseKnowledge, error) {
	return nil, nil
}
func (checkpointMemoryStub) UpsertCodebaseKnowledge(context.Context, db.UpsertCodebaseKnowledgeParams) error {
	return nil
}
func (checkpointMemoryStub) ListStructuredSummaries(context.Context, int) ([]db.StructuredSummary, error) {
	return nil, nil
}
func (checkpointMemoryStub) SearchCodebaseKnowledge(context.Context, string, int) ([]db.CodebaseKnowledge, error) {
	return nil, nil
}

func TestCheckpointServiceRecordPersistsPreferencesDecisionsAndResumeData(t *testing.T) {
	ctx := context.Background()
	store, err := orchestrationdb.Open(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })

	msgs := checkpointMessageStub{
		items: []message.Message{
			{SessionID: "session-1", Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "My name is Harshit. I prefer Go over Python. Let's use PostgreSQL."}}},
			{SessionID: "session-1", Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "Understood"}}},
		},
	}
	mem := checkpointMemoryStub{
		summary: &StructuredSummaryData{
			Decisions:   []Decision{{Symbol: "db", Decision: "Use PostgreSQL", Rationale: "Consistency"}},
			FileChanges: []FileChange{{File: "internal/agent/coordinator.go", SemanticChange: "checkpoint service wiring"}},
			TodoStates:  []TodoState{{Content: "Finish checkpoint tests", Status: "pending"}},
		},
	}
	service := NewCheckpointService(store, msgs, mem, nil)

	checkpoint, created, err := service.Record(ctx, CheckpointParams{
		SessionID: "session-1",
		AgentID:   "main:session-1",
		Phase:     "turn",
		Prompt:    "wire persistence",
		Result:    "done",
		Status:    "completed",
		Force:     true,
	})
	require.NoError(t, err)
	require.True(t, created)
	require.NotEmpty(t, checkpoint.ID)
	require.Equal(t, 2, checkpoint.MessageCount)

	pref, err := store.GetUserPreference(ctx, "preference.general")
	require.NoError(t, err)
	require.Equal(t, "Go over Python", pref.Value)

	resume, err := service.Resume(ctx, "session-1", "main:session-1")
	require.NoError(t, err)
	require.Contains(t, resume.PendingTasks, "Finish checkpoint tests")
	require.Contains(t, resume.FilesModified, "internal/agent/coordinator.go")
	require.NotEmpty(t, resume.Decisions)
}

func TestCheckpointServiceRecordSkipsUntilCadenceThreshold(t *testing.T) {
	ctx := context.Background()
	store, err := orchestrationdb.Open(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })

	msgs := checkpointMessageStub{
		items: []message.Message{{SessionID: "session-2", Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "seed"}}}},
	}
	service := NewCheckpointService(store, msgs, checkpointMemoryStub{}, nil)
	base := time.Now().UTC().Add(-10 * time.Minute)
	service.now = func() time.Time { return base }
	_, created, err := service.Record(ctx, CheckpointParams{
		SessionID: "session-2",
		AgentID:   "main:session-2",
		Force:     true,
		Status:    "completed",
	})
	require.NoError(t, err)
	require.True(t, created)

	service.now = func() time.Time { return base.Add(5 * time.Minute) }
	_, created, err = service.Record(ctx, CheckpointParams{
		SessionID: "session-2",
		AgentID:   "main:session-2",
		Status:    "completed",
	})
	require.NoError(t, err)
	require.False(t, created)
}

func TestCheckpointServiceResolvesPreferenceAndDecisionConflicts(t *testing.T) {
	ctx := context.Background()
	store, err := orchestrationdb.Open(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })

	msgs := checkpointMessageStub{
		items: []message.Message{{SessionID: "session-3", Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "I prefer Go over Python. Let's use PostgreSQL."}}}},
	}
	service := NewCheckpointService(store, msgs, checkpointMemoryStub{}, nil)
	now := time.Now().UTC()
	service.now = func() time.Time { return now }
	_, _, err = service.Record(ctx, CheckpointParams{SessionID: "session-3", AgentID: "main:session-3", Force: true, Status: "completed"})
	require.NoError(t, err)

	msgs.items = []message.Message{{SessionID: "session-3", Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "I prefer Rust over Go. Let's use SQLite."}}}}
	service.now = func() time.Time { return now.Add(31 * time.Minute) }
	_, _, err = service.Record(ctx, CheckpointParams{SessionID: "session-3", AgentID: "main:session-3", Status: "completed"})
	require.NoError(t, err)

	pref, err := store.GetUserPreference(ctx, "preference.general")
	require.NoError(t, err)
	require.Equal(t, "Rust over Go", pref.Value)
	require.Equal(t, "conflicted", pref.Confidence)

	resume, err := service.Resume(ctx, "session-3", "main:session-3")
	require.NoError(t, err)
	require.NotEmpty(t, resume.DecisionConflicts)
}

func TestCheckpointServicePrunesOldCheckpoints(t *testing.T) {
	ctx := context.Background()
	store, err := orchestrationdb.Open(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })

	msgs := checkpointMessageStub{
		items: []message.Message{{SessionID: "session-4", Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "seed"}}}},
	}
	service := NewCheckpointService(store, msgs, checkpointMemoryStub{}, nil)
	base := time.Now().UTC().Add(-240 * time.Hour)
	for i := 0; i < 40; i++ {
		idx := i
		service.now = func() time.Time { return base.Add(time.Duration(idx) * time.Hour) }
		status := "completed"
		if idx%10 == 0 {
			status = "compacted"
		}
		_, _, err := service.Record(ctx, CheckpointParams{
			SessionID: "session-4",
			AgentID:   "main:session-4",
			Status:    status,
			Force:     true,
			Summary:   map[string]any{"status": status},
		})
		require.NoError(t, err)
	}

	checkpoints, err := store.ListCheckpoints(ctx, "session-4", "main:session-4", 200)
	require.NoError(t, err)
	require.LessOrEqual(t, len(checkpoints), maxRecentCheckpoints+maxOlderHighValue)
}
