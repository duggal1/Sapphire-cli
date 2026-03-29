package supervisor

import (
	"context"
	"testing"
	"time"

	agentactivity "github.com/duggal1/Sapphire-cli/internal/agent/activity"
	agentmailbox "github.com/duggal1/Sapphire-cli/internal/agent/mailbox"
	agentstate "github.com/duggal1/Sapphire-cli/internal/agent/state"
	persistmemory "github.com/duggal1/Sapphire-cli/internal/memory"
	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSupervisorEnforcesMistakeLoggingForUnloggedFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repoRoot := t.TempDir()
	store, err := orchestrationdb.Open(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	mailbox := agentmailbox.NewService(store, nil)
	service := NewService(
		store,
		agentstate.NewService(store),
		agentactivity.NewService(store),
		mailbox,
		Hooks{},
	)

	failureID := uuid.NewString()
	require.NoError(t, store.RecordActivity(ctx, orchestrationdb.AgentActivity{
		ID:          failureID,
		AgentID:     "agent-1",
		EventType:   "failed",
		CreatedAt:   time.Now().UTC(),
		DetailsJSON: `{"status":"error","error":"validation gate failed"}`,
	}))

	tracker := &AgentTracker{AgentID: "agent-1", SessionID: "session-1"}
	runtime := AgentRuntimeSnapshot{
		AgentID:    "agent-1",
		SessionID:  "session-1",
		RepoRoot:   repoRoot,
		WorkingDir: repoRoot,
		LastError:  "validation gate failed",
	}
	service.enforceMistakeLogging(ctx, tracker, runtime)

	inbox, err := store.ListInbox(ctx, "agent-1", false, 10)
	require.NoError(t, err)
	require.Len(t, inbox, 1)
	require.Equal(t, subjectMistakeLogRequired, inbox[0].Subject)
	require.Contains(t, inbox[0].Body, "mistake_fingerprint: activity:"+failureID)
	require.Contains(t, inbox[0].Body, ".sapphire/mistake.md")
}

func TestSupervisorSkipsAlreadyLoggedFailureFingerprint(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repoRoot := t.TempDir()
	store, err := orchestrationdb.Open(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	mailbox := agentmailbox.NewService(store, nil)
	service := NewService(
		store,
		agentstate.NewService(store),
		agentactivity.NewService(store),
		mailbox,
		Hooks{},
	)

	failureID := uuid.NewString()
	require.NoError(t, store.RecordActivity(ctx, orchestrationdb.AgentActivity{
		ID:          failureID,
		AgentID:     "agent-1",
		EventType:   "failed",
		CreatedAt:   time.Now().UTC(),
		DetailsJSON: `{"status":"error","error":"validation gate failed"}`,
	}))
	_, appended, err := persistmemory.AppendMistake(repoRoot, persistmemory.MistakeLogInput{
		Fingerprint:    "activity:" + failureID,
		Date:           time.Now().UTC(),
		Task:           "Validation failure",
		TaskDomain:     "build",
		Agent:          "agent-1",
		Model:          "test-model",
		Worktree:       "shared",
		WhatHappened:   "Validation gate failed",
		RootCauseClass: persistmemory.MistakeRootCauseWrongAssumption,
		RootCause:      "Assumed the branch was already green.",
		Severity:       persistmemory.MistakeSeverityHigh,
		PreventionRule: "Always run the validation gate before reporting completion.",
		Resolved:       true,
		StatusNote:     "Prevention rule persisted to durable memory.",
	})
	require.NoError(t, err)
	require.True(t, appended)

	tracker := &AgentTracker{AgentID: "agent-1", SessionID: "session-1"}
	runtime := AgentRuntimeSnapshot{
		AgentID:    "agent-1",
		SessionID:  "session-1",
		RepoRoot:   repoRoot,
		WorkingDir: repoRoot,
		LastError:  "validation gate failed",
	}
	service.enforceMistakeLogging(ctx, tracker, runtime)

	inbox, err := store.ListInbox(ctx, "agent-1", false, 10)
	require.NoError(t, err)
	require.Empty(t, inbox)
}
