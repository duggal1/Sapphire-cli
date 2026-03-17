package memories

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"
	"time"

	"github.com/charmbracelet/sapphire/internal/db"
	"github.com/charmbracelet/sapphire/internal/message"
	"github.com/charmbracelet/sapphire/internal/session"
	"github.com/stretchr/testify/require"
)

type testHarness struct {
	conn     *sql.DB
	q        *db.Queries
	sessions session.Service
	messages message.Service
	service  *Service
	dataDir  string
}

func newTestHarness(t *testing.T) testHarness {
	t.Helper()

	dataDir := t.TempDir()
	conn, err := db.Connect(t.Context(), dataDir)
	require.NoError(t, err)

	q := db.New(conn)
	sessions := session.NewService(q, conn)
	messages := message.NewService(q)
	service := NewService(q, conn, sessions, messages, dataDir)
	service.SetRunners(PhaseRunners{
		Phase1: func(ctx context.Context, invocation Phase1Invocation) (Phase1Output, error) {
			return Phase1Output{
				RawMemory:      "description: session memory\n\n" + invocation.UserPrompt,
				RolloutSummary: "# Rollout Summary\n\n" + invocation.UserPrompt,
				RolloutSlug:    "investigate-terminal-latency",
			}, nil
		},
		Phase2: func(ctx context.Context, invocation Phase2Invocation) error {
			selectedPattern := regexp.MustCompile(`thread_id=([a-zA-Z0-9-]+), rollout_summary_file=(rollout_summaries/[^\s]+\.md)`)
			match := selectedPattern.FindStringSubmatch(invocation.Prompt)
			matches, err := filepath.Glob(filepath.Join(invocation.Root, "rollout_summaries", "*.md"))
			if err != nil {
				return err
			}
			rel := "rollout_summaries/placeholder.md"
			threadID := "unknown-thread"
			if len(matches) > 0 {
				rel = filepath.ToSlash(filepath.Join("rollout_summaries", filepath.Base(matches[0])))
			}
			if len(match) == 3 {
				threadID = match[1]
				rel = match[2]
			}
			rolloutBytes, err := os.ReadFile(filepath.Join(invocation.Root, filepath.FromSlash(rel)))
			if err != nil {
				return err
			}
			knowledge := string(rolloutBytes)
			if err := os.WriteFile(filepath.Join(invocation.Root, "MEMORY.md"), []byte("# Task Group: Investigate terminal latency\n\nscope: persisted memory\napplies_to: cwd=/repo; reuse_rule=reuse in same repo\n\n## Task 1: Investigate latency, success\n\n### rollout_summary_files\n\n- "+rel+" (thread_id="+threadID+")\n\n### keywords\n\n- latency, loader\n\n## Reusable knowledge\n\n- "+knowledge+"\n"), 0o644); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(invocation.Root, "memory_summary.md"), []byte("# Memory Summary\n\n- Investigate terminal latency: see "+rel+"\n- Evidence: "+knowledge+"\n"), 0o644); err != nil {
				return err
			}
			return nil
		},
	})

	t.Cleanup(func() {
		require.NoError(t, conn.Close())
	})

	return testHarness{
		conn:     conn,
		q:        q,
		sessions: sessions,
		messages: messages,
		service:  service,
		dataDir:  dataDir,
	}
}

func createSessionWithMessages(t *testing.T, h testHarness) session.Session {
	t.Helper()

	ctx := t.Context()
	sess, err := h.sessions.Create(ctx, "Investigate terminal latency")
	require.NoError(t, err)

	_, err = h.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "Trace the send path and keep notes."}},
	})
	require.NoError(t, err)

	_, err = h.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: "Loader stalls before the first assistant row appears."},
			message.ToolCall{ID: "tool-1", Name: "view", Input: "{\"path\":\"internal/ui/model/ui.go\"}", Finished: true},
			message.ToolResult{ToolCallID: "tool-1", Name: "view", Content: "critical path confirmed"},
		},
	})
	require.NoError(t, err)

	return sess
}

func TestRunStartupMaterializesCanonicalArtifacts(t *testing.T) {
	h := newTestHarness(t)
	sess := createSessionWithMessages(t, h)

	err := h.service.runStartup(t.Context(), StartOptions{
		SessionID:  sess.ID,
		WorkingDir: "/repo",
	})
	require.NoError(t, err)

	stage1, err := h.q.GetStage1OutputBySessionID(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Contains(t, stage1.RawMemory, "Loader stalls before the first assistant row appears.")
	require.NotEmpty(t, stage1.RolloutSummaryFile)

	summaryFile, err := os.ReadFile(filepath.Join(h.service.Root(), "memory_summary.md"))
	require.NoError(t, err)
	require.Contains(t, string(summaryFile), stage1.RolloutSummaryFile)

	registryFile, err := os.ReadFile(filepath.Join(h.service.Root(), "MEMORY.md"))
	require.NoError(t, err)
	require.Contains(t, string(registryFile), sess.ID)

	rolloutFile, err := os.ReadFile(filepath.Join(h.service.Root(), stage1.RolloutSummaryFile))
	require.NoError(t, err)
	require.Contains(t, string(rolloutFile), "Analyze this rollout and produce JSON")

	summaryMat, err := h.q.GetMemorySummaryMaterialization(t.Context())
	require.NoError(t, err)
	require.Contains(t, summaryMat.Content, stage1.RolloutSummaryFile)

	registryMat, err := h.q.GetMemoryRegistryMaterialization(t.Context())
	require.NoError(t, err)
	require.Contains(t, registryMat.Content, sess.ID)
}

func TestSyncMaterializationsRebuildsFilesFromSQL(t *testing.T) {
	h := newTestHarness(t)
	sess := createSessionWithMessages(t, h)

	err := h.service.runStartup(t.Context(), StartOptions{
		SessionID:  sess.ID,
		WorkingDir: "/repo",
	})
	require.NoError(t, err)

	stage1, err := h.q.GetStage1OutputBySessionID(t.Context(), sess.ID)
	require.NoError(t, err)

	require.NoError(t, h.service.Clear())

	svc2 := NewService(h.q, h.conn, h.sessions, h.messages, h.dataDir)
	require.NoError(t, svc2.SyncMaterializations(t.Context()))

	summaryMat, err := h.q.GetMemorySummaryMaterialization(t.Context())
	require.NoError(t, err)
	summaryFile, err := os.ReadFile(filepath.Join(svc2.Root(), "memory_summary.md"))
	require.NoError(t, err)
	require.Equal(t, summaryMat.Content+"\n", string(summaryFile))

	registryMat, err := h.q.GetMemoryRegistryMaterialization(t.Context())
	require.NoError(t, err)
	registryFile, err := os.ReadFile(filepath.Join(svc2.Root(), "MEMORY.md"))
	require.NoError(t, err)
	require.Equal(t, registryMat.Content+"\n", string(registryFile))

	rolloutFile, err := os.ReadFile(filepath.Join(svc2.Root(), stage1.RolloutSummaryFile))
	require.NoError(t, err)
	require.Contains(t, string(rolloutFile), "Analyze this rollout and produce JSON")
}

func TestApplyStaleCorrectionUpdatesCanonicalSQLBeforeRematerialization(t *testing.T) {
	h := newTestHarness(t)
	sess := createSessionWithMessages(t, h)

	err := h.service.runStartup(t.Context(), StartOptions{
		SessionID:  sess.ID,
		WorkingDir: "/repo",
	})
	require.NoError(t, err)

	stage1Before, err := h.q.GetStage1OutputBySessionID(t.Context(), sess.ID)
	require.NoError(t, err)

	err = h.service.ApplyStaleCorrection(
		t.Context(),
		sess.ID,
		"Corrected terminal latency memory",
		"Verified current evidence: the dead time comes from the pre-dispatch submit path, not provider latency.",
		sess.ID,
		stage1Before.RolloutSummaryFile,
		[]string{sess.ID},
	)
	require.NoError(t, err)

	stage1After, err := h.q.GetStage1OutputBySessionID(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Contains(t, stage1After.RolloutSummary, "pre-dispatch submit path")

	entries, err := h.q.ListMemoryRegistryEntries(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, entries)
	require.Equal(t, "Corrected terminal latency memory", entries[0].Title)
	require.Contains(t, entries[0].Body, "pre-dispatch submit path")

	citations, err := h.q.ListMemoryRegistryCitationsByEntry(t.Context(), entries[0].ID)
	require.NoError(t, err)
	require.Len(t, citations, 1)
	require.Equal(t, sess.ID, citations[0].SessionID)

	registryFile, err := os.ReadFile(filepath.Join(h.service.Root(), "MEMORY.md"))
	require.NoError(t, err)
	require.Contains(t, string(registryFile), "pre-dispatch submit path")

	summaryPrompt := h.service.RuntimePrompt(t.Context())
	require.Contains(t, summaryPrompt, "memory_summary.md")
	require.Contains(t, summaryPrompt, "pre-dispatch submit path")

	usage := h.service.UsageSnapshot()
	require.Equal(t, 1, usage.ReadsByKind["memory_summary"])
}

func TestGetPhase2SelectionReportsAddedRetainedAndRemovedRows(t *testing.T) {
	h := newTestHarness(t)
	ctx := t.Context()
	cutoff := nowUnix() - int64(retentionWindow.Seconds())

	oldSession := createSessionWithMessages(t, h)
	midSession := createSessionWithMessages(t, h)
	newSession := createSessionWithMessages(t, h)

	require.NoError(t, h.q.UpsertStage1Output(ctx, db.UpsertStage1OutputParams{
		SessionID:          oldSession.ID,
		SourceUpdatedAt:    cutoff - 10,
		RawMemory:          "raw-old",
		RolloutSummary:     "summary-old",
		RolloutSlug:        "old",
		RolloutSummaryFile: "rollout_summaries/old.md",
	}))
	require.NoError(t, h.q.UpsertStage1Output(ctx, db.UpsertStage1OutputParams{
		SessionID:          midSession.ID,
		SourceUpdatedAt:    cutoff + 10,
		RawMemory:          "raw-mid",
		RolloutSummary:     "summary-mid",
		RolloutSlug:        "mid",
		RolloutSummaryFile: "rollout_summaries/mid.md",
	}))
	require.NoError(t, h.q.UpsertStage1Output(ctx, db.UpsertStage1OutputParams{
		SessionID:          newSession.ID,
		SourceUpdatedAt:    cutoff + 20,
		RawMemory:          "raw-new",
		RolloutSummary:     "summary-new",
		RolloutSlug:        "new",
		RolloutSummaryFile: "rollout_summaries/new.md",
	}))
	_, err := h.q.MarkStage1OutputSelectedForPhase2(ctx, db.MarkStage1OutputSelectedForPhase2Params{
		SessionID:       oldSession.ID,
		SourceUpdatedAt: cutoff - 10,
	})
	require.NoError(t, err)
	_, err = h.q.MarkStage1OutputSelectedForPhase2(ctx, db.MarkStage1OutputSelectedForPhase2Params{
		SessionID:       newSession.ID,
		SourceUpdatedAt: cutoff + 20,
	})
	require.NoError(t, err)

	selection, err := h.service.getPhase2Selection(ctx)
	require.NoError(t, err)

	require.Len(t, selection.Selected, 2)
	require.Equal(t, newSession.ID, selection.Selected[0].SessionID)
	require.Equal(t, midSession.ID, selection.Selected[1].SessionID)

	require.Len(t, selection.PreviousSelected, 2)
	previousIDs := []string{selection.PreviousSelected[0].SessionID, selection.PreviousSelected[1].SessionID}
	sort.Strings(previousIDs)
	expectedPrevious := []string{newSession.ID, oldSession.ID}
	sort.Strings(expectedPrevious)
	require.Equal(t, expectedPrevious, previousIDs)

	require.Contains(t, selection.RetainedIDs, newSession.ID)
	require.NotContains(t, selection.RetainedIDs, midSession.ID)

	require.Len(t, selection.Removed, 1)
	require.Equal(t, oldSession.ID, selection.Removed[0].SessionID)
	require.Equal(t, "rollout_summaries/old.md", selection.Removed[0].RolloutSummaryFile)
}

func TestGetPhase2SelectionTreatsRegeneratedSelectedRowsAsAdded(t *testing.T) {
	h := newTestHarness(t)
	ctx := t.Context()
	sess := createSessionWithMessages(t, h)
	base := nowUnix()

	require.NoError(t, h.q.UpsertStage1Output(ctx, db.UpsertStage1OutputParams{
		SessionID:          sess.ID,
		SourceUpdatedAt:    base,
		RawMemory:          "raw-1",
		RolloutSummary:     "summary-1",
		RolloutSlug:        "regen-1",
		RolloutSummaryFile: "rollout_summaries/regen-1.md",
	}))
	_, err := h.q.MarkStage1OutputSelectedForPhase2(ctx, db.MarkStage1OutputSelectedForPhase2Params{
		SessionID:       sess.ID,
		SourceUpdatedAt: base,
	})
	require.NoError(t, err)

	require.NoError(t, h.q.UpsertStage1Output(ctx, db.UpsertStage1OutputParams{
		SessionID:          sess.ID,
		SourceUpdatedAt:    base + 1,
		RawMemory:          "raw-2",
		RolloutSummary:     "summary-2",
		RolloutSlug:        "regen-2",
		RolloutSummaryFile: "rollout_summaries/regen-2.md",
	}))

	selection, err := h.service.getPhase2Selection(ctx)
	require.NoError(t, err)

	require.Len(t, selection.Selected, 1)
	require.Equal(t, sess.ID, selection.Selected[0].SessionID)
	require.Equal(t, base+1, selection.Selected[0].SourceUpdatedAt)
	require.Len(t, selection.PreviousSelected, 1)
	require.Equal(t, sess.ID, selection.PreviousSelected[0].SessionID)
	require.Empty(t, selection.RetainedIDs)
	require.Empty(t, selection.Removed)

	stage1, err := h.q.GetStage1OutputBySessionID(ctx, sess.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1), stage1.SelectedForPhase2)
	require.Equal(t, base, stage1.SelectedForPhase2SourceUpdatedAt.Int64)
}

func TestStartReturnsImmediatelyAndRunsMemoryPipelineInBackground(t *testing.T) {
	h := newTestHarness(t)
	sess := createSessionWithMessages(t, h)

	started := make(chan struct{})
	release := make(chan struct{})
	h.service.SetRunners(PhaseRunners{
		Phase1: func(ctx context.Context, invocation Phase1Invocation) (Phase1Output, error) {
			close(started)
			<-release
			return Phase1Output{
				RawMemory:      "raw",
				RolloutSummary: "summary",
				RolloutSlug:    "background",
			}, nil
		},
		Phase2: func(ctx context.Context, invocation Phase2Invocation) error {
			return nil
		},
	})

	begin := time.Now()
	h.service.Start(t.Context(), StartOptions{
		SessionID:  sess.ID,
		WorkingDir: "/repo",
	})
	require.Less(t, time.Since(begin), 100*time.Millisecond)

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("background memory pipeline did not start")
	}

	select {
	case <-release:
		t.Fatal("background memory runner should still be blocked")
	default:
	}

	close(release)
	require.Eventually(t, func() bool {
		_, err := h.q.GetStage1OutputBySessionID(t.Context(), sess.ID)
		return err == nil
	}, 2*time.Second, 25*time.Millisecond)
}
