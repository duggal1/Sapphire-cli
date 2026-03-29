package memory

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	appdb "github.com/duggal1/Sapphire-cli/internal/db"
	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCompilerRolloverResumeUsesDurableBootPacket(t *testing.T) {
	ctx := context.Background()
	repoRoot := seedMemoryTestRepo(t)
	conn := openMemoryRuntimeTestDB(t, repoRoot)
	compiler := NewCompiler(conn, nil)

	resumePoint, err := compiler.CreateResumePoint(ctx, ResumeRequest{
		SessionID:          "session-rollover",
		AgentID:            "main:session-rollover",
		WorkingDir:         repoRoot,
		Task:               "Fix Foo in foo.go",
		OriginalPrompt:     "Fix Foo in foo.go",
		ContinuationPrompt: "Continue fixing Foo after compaction",
		Reason:             "context_rollover",
	})
	require.NoError(t, err)
	require.FileExists(t, resumePoint.BootPacketArtifactPath)
	require.FileExists(t, resumePoint.HandoffArtifactPath)

	matched, ok := compiler.MatchPendingResumePoint(ctx, "session-rollover", "  Fix Foo in foo.go ")
	require.True(t, ok)
	require.Equal(t, resumePoint.ID, matched.ID)

	injection := compiler.RenderResumePointInjection(ctx, resumePoint.ID)
	require.Contains(t, injection, "## DURABLE RESUME BOOT PACKET")
	require.Contains(t, injection, "Fix Foo in foo.go")

	item, err := appdb.New(conn).GetMemoryResumePoint(ctx, resumePoint.ID)
	require.NoError(t, err)
	require.Equal(t, "resumed", item.Status)
	require.NotZero(t, item.ResumedAt)
}

func TestCompilerCrashRestartResumePointRecovery(t *testing.T) {
	ctx := context.Background()
	repoRoot := seedMemoryTestRepo(t)
	dataDir := filepath.Join(repoRoot, ".sapphire")
	require.NoError(t, os.MkdirAll(dataDir, 0o755))

	conn, err := appdb.Connect(ctx, dataDir)
	require.NoError(t, err)
	compiler := NewCompiler(conn, nil)
	resumePoint, err := compiler.CreateResumePoint(ctx, ResumeRequest{
		SessionID:          "session-restart",
		AgentID:            "main:session-restart",
		WorkingDir:         repoRoot,
		Task:               "Refactor Foo in foo.go",
		OriginalPrompt:     "Refactor Foo in foo.go",
		ContinuationPrompt: "Resume the refactor",
		Reason:             "restart_recovery",
	})
	require.NoError(t, err)
	require.NoError(t, conn.Close())

	conn2, err := appdb.Connect(ctx, dataDir)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn2.Close()
	})
	compiler2 := NewCompiler(conn2, nil)

	matched, ok := compiler2.MatchPendingResumePoint(ctx, "session-restart", "Refactor Foo in foo.go")
	require.True(t, ok)
	require.Equal(t, resumePoint.ID, matched.ID)

	injection := compiler2.RenderResumePointInjection(ctx, resumePoint.ID)
	require.Contains(t, injection, "Continue the interrupted task from this durable state")
	require.Contains(t, injection, "Refactor Foo in foo.go")

	item, err := appdb.New(conn2).GetMemoryResumePoint(ctx, resumePoint.ID)
	require.NoError(t, err)
	require.Equal(t, "resumed", item.Status)
}

func TestCompilerPersistsStructuredSubAgentMemory(t *testing.T) {
	ctx := context.Background()
	repoRoot := seedMemoryTestRepo(t)
	conn := openMemoryRuntimeTestDB(t, repoRoot)
	store := openMemoryTestStore(t, repoRoot)
	compiler := NewCompiler(conn, store)

	_, err := store.SaveDecision(ctx, orchestrationdb.DecisionRecord{
		SessionID:  "sub-session",
		Category:   "orchestration",
		Key:        "mailbox",
		Value:      "Use durable inbox routing",
		Confidence: "accepted",
		CreatedAt:  time.Now().UTC(),
	})
	require.NoError(t, err)

	err = compiler.PersistSubAgentOutcome(ctx, SubAgentOutcomeInput{
		SessionID:       "sub-session",
		ParentSessionID: "parent-session",
		AgentID:         "agent-timeout-monitor",
		AssignmentID:    "assignment-1",
		SubmissionID:    "submission-1",
		WorkingDir:      repoRoot,
		Status:          "blocked",
		Summary:         "Investigated Foo and isolated the blocking path.",
		Progress:        "Found issue in Foo and ConfigLoader.",
		Risks:           "Config loading may regress if the fallback path changes.",
		Blockers:        "Need approval to change config defaults.",
		NextAction:      "Inspect config.yaml and update the fallback path.",
		Files:           []string{filepath.Join(repoRoot, "foo.go"), "config.yaml"},
		Commands:        []string{"rg Foo foo.go", "rg ConfigLoader ."},
		RawResult:       "Evidence: Foo fails when ConfigLoader is missing. Symbol candidates: Foo ConfigLoader.",
	})
	require.NoError(t, err)

	q := appdb.New(conn)
	reports, err := q.ListMemorySubAgentReportsBySession(ctx, "parent-session", 10)
	require.NoError(t, err)
	require.Len(t, reports, 1)
	require.Equal(t, "blocked", reports[0].Status)
	require.Equal(t, []string{"config.yaml", "foo.go"}, reports[0].Files)
	require.Contains(t, reports[0].TouchedSymbols, "Foo")
	require.FileExists(t, reports[0].ArtifactPath)

	findings, err := q.ListMemoryFindingsBySession(ctx, "sub-session", 20)
	require.NoError(t, err)
	byKind := make(map[string][]appdb.MemoryFinding)
	for _, item := range findings {
		byKind[item.Kind] = append(byKind[item.Kind], item)
	}
	require.NotEmpty(t, byKind["finding"])
	require.NotEmpty(t, byKind["uncertainty"])
	require.NotEmpty(t, byKind["next_action"])
	require.NotEmpty(t, byKind["decision"])

	var provenanceCount int
	err = conn.QueryRowContext(ctx, `SELECT COUNT(1) FROM memory_provenance WHERE subagent_report_id = ?`, reports[0].ID).Scan(&provenanceCount)
	require.NoError(t, err)
	require.Equal(t, 1, provenanceCount)

	var factLinkCount int
	err = conn.QueryRowContext(ctx, `SELECT COUNT(1) FROM memory_fact_provenance WHERE fact_kind = 'subagent_report' AND fact_id = ?`, reports[0].ID).Scan(&factLinkCount)
	require.NoError(t, err)
	require.Equal(t, 1, factLinkCount)
}

func TestCompilerIncrementalIndexPreservesUnchangedFileRows(t *testing.T) {
	ctx := context.Background()
	repoRoot := seedMemoryTestRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(repoRoot, "bar.go"), []byte(`package sample

func Bar() string {
	return Foo()
}
`), 0o644))

	conn := openMemoryRuntimeTestDB(t, repoRoot)
	compiler := NewCompiler(conn, nil)

	scope, err := compiler.ensureIndexedScope(ctx, repoRoot)
	require.NoError(t, err)

	q := appdb.New(conn)
	filesBefore, err := q.ListMemoryRepoFilesByScope(ctx, scope.ID)
	require.NoError(t, err)
	barBefore := memoryFileByPath(t, filesBefore, "bar.go")

	symbolsBefore, err := q.ListMemoryRepoSymbolsByScope(ctx, scope.ID)
	require.NoError(t, err)
	barSymbolBefore := memorySymbolByName(t, symbolsBefore, barBefore.ID, "Bar")

	fooPath := filepath.Join(repoRoot, "foo.go")
	require.NoError(t, os.WriteFile(fooPath, []byte(`package sample

func Foo() string {
	return helper()
}

func helper() string {
	return "updated"
}
`), 0o644))
	future := time.Now().Add(3 * time.Second)
	require.NoError(t, os.Chtimes(fooPath, future, future))

	_, err = compiler.Compile(ctx, CompileRequest{
		SessionID:  "session-index",
		AgentID:    "main:session-index",
		WorkingDir: repoRoot,
		Task:       "Update Foo in foo.go",
	})
	require.NoError(t, err)

	filesAfter, err := q.ListMemoryRepoFilesByScope(ctx, scope.ID)
	require.NoError(t, err)
	barAfter := memoryFileByPath(t, filesAfter, "bar.go")
	require.Equal(t, barBefore.ID, barAfter.ID)
	require.Equal(t, barBefore.CreatedAt, barAfter.CreatedAt)
	require.Equal(t, barBefore.UpdatedAt, barAfter.UpdatedAt)

	symbolsAfter, err := q.ListMemoryRepoSymbolsByScope(ctx, scope.ID)
	require.NoError(t, err)
	barSymbolAfter := memorySymbolByName(t, symbolsAfter, barAfter.ID, "Bar")
	require.Equal(t, barSymbolBefore.ID, barSymbolAfter.ID)
	require.Equal(t, barSymbolBefore.CreatedAt, barSymbolAfter.CreatedAt)
	require.Equal(t, barSymbolBefore.UpdatedAt, barSymbolAfter.UpdatedAt)

	epochs, err := q.ListMemoryIndexEpochsByScope(ctx, scope.ID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(epochs), 2)
}

func TestCompilerIndexesDistinctGenericReceiverMethods(t *testing.T) {
	ctx := context.Background()
	repoRoot := seedMemoryTestRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(repoRoot, "generic.go"), []byte(`package sample

type Foo[T any] struct{}
type Bar[T any] struct{}

func (f *Foo[T]) Run() string {
	return "foo"
}

func (b *Bar[T]) Run() string {
	return "bar"
}
`), 0o644))

	conn := openMemoryRuntimeTestDB(t, repoRoot)
	compiler := NewCompiler(conn, nil)

	scope, err := compiler.ensureIndexedScope(ctx, repoRoot)
	require.NoError(t, err)

	q := appdb.New(conn)
	files, err := q.ListMemoryRepoFilesByScope(ctx, scope.ID)
	require.NoError(t, err)
	genericFile := memoryFileByPath(t, files, "generic.go")

	symbols, err := q.ListMemoryRepoSymbolsByScope(ctx, scope.ID)
	require.NoError(t, err)

	runKeys := make([]string, 0, 2)
	for _, item := range symbols {
		if item.FileID != genericFile.ID || item.Kind != "method" || item.Name != "Run" {
			continue
		}
		runKeys = append(runKeys, item.StableKey)
	}

	require.Len(t, runKeys, 2)
	require.NotEqual(t, runKeys[0], runKeys[1])
	require.Contains(t, runKeys, "generic.go::method::Foo::Run")
	require.Contains(t, runKeys, "generic.go::method::Bar::Run")
	require.NotContains(t, genericFile.Facts, "duplicate_symbol_keys")
}

func TestCompilerPrunesDurableMemoryArtifactsAndHistory(t *testing.T) {
	ctx := context.Background()
	repoRoot := seedMemoryTestRepo(t)
	conn := openMemoryRuntimeTestDB(t, repoRoot)
	compiler := NewCompiler(conn, nil)
	scope, err := compiler.ensureIndexedScope(ctx, repoRoot)
	require.NoError(t, err)

	q := appdb.New(conn)
	sessionID := "session-prune"
	subSessionID := "subagent-prune"
	base := time.Now().UTC().Add(-2 * time.Hour).Unix()

	var oldestHandoffArtifact, newestHandoffArtifact string
	var oldestBootArtifact, newestBootArtifact string
	var oldestResumeBootArtifact, newestResumeBootArtifact string

	handoffIDs := make([]string, 0, maxRetainedResumePoints+8)
	for i := 0; i < maxRetainedHandoffs+8; i++ {
		artifact := writeMemoryTestArtifact(t, repoRoot, "handoffs", i)
		if i == 0 {
			oldestHandoffArtifact = artifact
		}
		newestHandoffArtifact = artifact
		id := uuid.NewString()
		handoffIDs = append(handoffIDs, id)
		require.NoError(t, q.InsertMemoryHandoff(ctx, appdb.InsertMemoryHandoffParams{
			ID:           id,
			SessionID:    sessionID,
			AgentID:      "main:prune",
			RepoScopeID:  scope.ID,
			Status:       "active",
			Objective:    fmt.Sprintf("objective-%d", i),
			ArtifactPath: artifact,
			CreatedAt:    base + int64(i),
		}))
	}

	for i := 0; i < maxRetainedBootPackets+8; i++ {
		artifact := writeMemoryTestArtifact(t, repoRoot, "boot_packets", i)
		if i == 0 {
			oldestBootArtifact = artifact
		}
		newestBootArtifact = artifact
		require.NoError(t, q.InsertMemoryBootPacket(ctx, appdb.InsertMemoryBootPacketParams{
			ID:            uuid.NewString(),
			SessionID:     sessionID,
			AgentID:       "main:prune",
			RepoScopeID:   scope.ID,
			TaskHash:      fmt.Sprintf("task-%d", i),
			ArtifactPath:  artifact,
			RequiredReads: []byte(`[]`),
			CreatedAt:     base + int64(i),
		}))
	}

	for i := 0; i < maxRetainedResumePoints+8; i++ {
		bootArtifact := writeMemoryTestArtifact(t, repoRoot, "boot_packets", 1000+i)
		handoffArtifact := writeMemoryTestArtifact(t, repoRoot, "handoffs", 1000+i)
		if i == 0 {
			oldestResumeBootArtifact = bootArtifact
		}
		newestResumeBootArtifact = bootArtifact
		handoffID := handoffIDs[minInt(i, len(handoffIDs)-1)]
		require.NoError(t, q.InsertMemoryResumePoint(ctx, appdb.InsertMemoryResumePointParams{
			ID:                     uuid.NewString(),
			SessionID:              sessionID,
			AgentID:                "main:prune",
			RepoScopeID:            scope.ID,
			HandoffID:              handoffID,
			BootPacketArtifactPath: bootArtifact,
			HandoffArtifactPath:    handoffArtifact,
			ContinuationPrompt:     fmt.Sprintf("continue-%d", i),
			OriginalPrompt:         fmt.Sprintf("original-%d", i),
			ResumeReason:           "context_rollover",
			Status:                 "pending",
			CreatedAt:              base + int64(i),
			ResumedAt:              0,
		}))
	}

	for i := 0; i < maxRetainedFindings+8; i++ {
		require.NoError(t, q.InsertMemoryFinding(ctx, appdb.InsertMemoryFindingParams{
			ID:          uuid.NewString(),
			SessionID:   sessionID,
			AgentID:     "main:prune",
			RepoScopeID: scope.ID,
			Kind:        "finding",
			Title:       fmt.Sprintf("finding-%d", i),
			Content:     "retained test finding",
			Status:      "active",
			CreatedAt:   base + int64(i),
			UpdatedAt:   base + int64(i),
		}))
	}

	for i := 0; i < maxRetainedIndexEpochs+8; i++ {
		require.NoError(t, q.InsertMemoryIndexEpoch(ctx, appdb.InsertMemoryIndexEpochParams{
			ID:           uuid.NewString(),
			ScopeID:      scope.ID,
			Epoch:        scope.LatestEpoch + int64(i) + 1,
			HeadCommit:   scope.HeadCommit,
			ChangedFiles: []string{"foo.go"},
			RemovedFiles: nil,
			FileCount:    3,
			Status:       "ready",
			CreatedAt:    base + int64(i),
			CompletedAt:  base + int64(i),
		}))
	}

	for i := 0; i < maxRetainedSubAgentRows+8; i++ {
		artifact := writeMemoryTestArtifact(t, repoRoot, "subagent_reports", i)
		require.NoError(t, q.InsertMemorySubAgentReport(ctx, appdb.InsertMemorySubAgentReportParams{
			ID:              uuid.NewString(),
			SessionID:       subSessionID,
			ParentSessionID: sessionID,
			AgentID:         fmt.Sprintf("agent-%d", i),
			AssignmentID:    fmt.Sprintf("assignment-%d", i),
			SubmissionID:    fmt.Sprintf("submission-%d", i),
			RepoScopeID:     scope.ID,
			Status:          "completed",
			Summary:         "summary",
			Progress:        "progress",
			Files:           []string{"foo.go"},
			Commands:        []string{"rg Foo foo.go"},
			RawResult:       "done",
			ArtifactPath:    artifact,
			CreatedAt:       base + int64(i),
			UpdatedAt:       base + int64(i),
		}))
	}

	require.NoError(t, compiler.pruneDurableMemory(ctx, sessionID, scope.ID))
	require.NoError(t, compiler.pruneSubAgentReports(ctx, subSessionID))

	handoffs, err := q.ListMemoryHandoffsBySession(ctx, sessionID)
	require.NoError(t, err)
	require.Len(t, handoffs, maxRetainedHandoffs)

	bootPackets, err := q.ListMemoryBootPacketsBySession(ctx, sessionID)
	require.NoError(t, err)
	require.Len(t, bootPackets, maxRetainedBootPackets)

	resumePoints, err := q.ListMemoryResumePointsBySession(ctx, sessionID)
	require.NoError(t, err)
	require.Len(t, resumePoints, maxRetainedResumePoints)

	findings, err := q.ListMemoryFindingsBySession(ctx, sessionID, maxRetainedFindings+32)
	require.NoError(t, err)
	require.Len(t, findings, maxRetainedFindings)

	epochs, err := q.ListMemoryIndexEpochsByScope(ctx, scope.ID)
	require.NoError(t, err)
	require.Len(t, epochs, maxRetainedIndexEpochs)

	reports, err := q.ListMemorySubAgentReportsBySession(ctx, subSessionID, maxRetainedSubAgentRows+32)
	require.NoError(t, err)
	require.Len(t, reports, maxRetainedSubAgentRows)

	assertPathMissing(t, oldestHandoffArtifact)
	assertPathMissing(t, oldestBootArtifact)
	assertPathMissing(t, oldestResumeBootArtifact)
	require.FileExists(t, newestHandoffArtifact)
	require.FileExists(t, newestBootArtifact)
	require.FileExists(t, newestResumeBootArtifact)
}

func TestCompilerCompileSchedulesDurableMemoryPruneOffCriticalPath(t *testing.T) {
	ctx := context.Background()
	repoRoot := seedMemoryTestRepo(t)
	conn := openMemoryRuntimeTestDB(t, repoRoot)
	compiler := NewCompiler(conn, nil)
	compiler.pruneDelay = 150 * time.Millisecond
	compiler.pruneInterval = time.Millisecond
	compiler.pruneTimeout = 5 * time.Second

	scope, err := compiler.ensureIndexedScope(ctx, repoRoot)
	require.NoError(t, err)

	q := appdb.New(conn)
	sessionID := "session-async-prune"
	base := time.Now().UTC().Add(-time.Hour).Unix()
	for i := 0; i < maxRetainedBootPackets+8; i++ {
		require.NoError(t, q.InsertMemoryBootPacket(ctx, appdb.InsertMemoryBootPacketParams{
			ID:            uuid.NewString(),
			SessionID:     sessionID,
			AgentID:       "main:async-prune",
			RepoScopeID:   scope.ID,
			TaskHash:      fmt.Sprintf("task-%d", i),
			ArtifactPath:  writeMemoryTestArtifact(t, repoRoot, "boot_packets", 2000+i),
			RequiredReads: []byte(`[]`),
			CreatedAt:     base + int64(i),
		}))
	}

	_, err = compiler.Compile(ctx, CompileRequest{
		SessionID:  sessionID,
		AgentID:    "main:async-prune",
		WorkingDir: repoRoot,
		Task:       "Inspect Foo in foo.go",
	})
	require.NoError(t, err)

	packets, err := q.ListMemoryBootPacketsBySession(ctx, sessionID)
	require.NoError(t, err)
	require.Greater(t, len(packets), maxRetainedBootPackets)

	require.Eventually(t, func() bool {
		packets, err := q.ListMemoryBootPacketsBySession(ctx, sessionID)
		return err == nil && len(packets) == maxRetainedBootPackets
	}, 5*time.Second, 50*time.Millisecond)
}

func openMemoryRuntimeTestDB(t *testing.T, repoRoot string) *sql.DB {
	t.Helper()

	dataDir := filepath.Join(repoRoot, ".sapphire")
	require.NoError(t, os.MkdirAll(dataDir, 0o755))
	conn, err := appdb.Connect(context.Background(), dataDir)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
	})
	return conn
}

func openMemoryTestStore(t *testing.T, repoRoot string) *orchestrationdb.Store {
	t.Helper()

	dataDir := filepath.Join(repoRoot, ".sapphire")
	require.NoError(t, os.MkdirAll(dataDir, 0o755))
	store, err := orchestrationdb.Open(context.Background(), dataDir)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store
}

func memoryFileByPath(t *testing.T, items []appdb.MemoryRepoFile, path string) appdb.MemoryRepoFile {
	t.Helper()
	for _, item := range items {
		if item.Path == path {
			return item
		}
	}
	t.Fatalf("file %q not found", path)
	return appdb.MemoryRepoFile{}
}

func memorySymbolByName(t *testing.T, items []appdb.MemoryRepoSymbol, fileID, name string) appdb.MemoryRepoSymbol {
	t.Helper()
	for _, item := range items {
		if item.FileID == fileID && item.Name == name {
			return item
		}
	}
	t.Fatalf("symbol %q for file %q not found", name, fileID)
	return appdb.MemoryRepoSymbol{}
}

func writeMemoryTestArtifact(t *testing.T, repoRoot, kind string, idx int) string {
	t.Helper()
	dir := filepath.Join(repoRoot, ".sapphire", "state", "memory", kind)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	path := filepath.Join(dir, fmt.Sprintf("%s-%03d.json", kind, idx))
	require.NoError(t, os.WriteFile(path, []byte(fmt.Sprintf(`{"kind":"%s","idx":%d}`, kind, idx)), 0o644))
	return filepath.ToSlash(path)
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()
	_, err := os.Stat(path)
	require.Error(t, err)
	require.True(t, os.IsNotExist(err))
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
