package memory

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	appdb "github.com/duggal1/Sapphire-cli/internal/db"
	"github.com/stretchr/testify/require"
)

func TestCompilerBuildsBootPacketWithGraphSlice(t *testing.T) {
	t.Parallel()

	repoRoot := seedMemoryTestRepo(t)
	conn := openMemoryTestDB(t, repoRoot)
	compiler := NewCompiler(conn, nil)

	packet, err := compiler.Compile(context.Background(), CompileRequest{
		SessionID:  "session-1",
		AgentID:    "main:session-1",
		WorkingDir: repoRoot,
		Task:       "Debug Foo in foo.go and verify TestFoo coverage",
	})
	require.NoError(t, err)
	require.Equal(t, bootPacketVersion, packet.Version)
	require.NotEmpty(t, packet.GraphSlice.Files)
	require.NotEmpty(t, packet.GraphSlice.Symbols)
	require.NotEmpty(t, packet.RequiredReads)
	require.Contains(t, packet.ArtifactPath, ".sapphire/state/memory/boot_packets/")
	require.FileExists(t, packet.ArtifactPath)

	filePaths := make([]string, 0, len(packet.GraphSlice.Files))
	for _, item := range packet.GraphSlice.Files {
		filePaths = append(filePaths, item.Path)
	}
	require.Contains(t, filePaths, "foo.go")
	require.Contains(t, filePaths, "foo_test.go")

	symbolNames := make([]string, 0, len(packet.GraphSlice.Symbols))
	for _, item := range packet.GraphSlice.Symbols {
		symbolNames = append(symbolNames, item.Name)
	}
	require.Contains(t, symbolNames, "Foo")
	require.Contains(t, symbolNames, "TestFoo")
}

func TestCompilerPersistsStructuredHandoff(t *testing.T) {
	t.Parallel()

	repoRoot := seedMemoryTestRepo(t)
	conn := openMemoryTestDB(t, repoRoot)
	compiler := NewCompiler(conn, nil)

	require.NoError(t, compiler.PersistHandoff(context.Background(), CompileRequest{
		SessionID:  "session-2",
		AgentID:    "main:session-2",
		WorkingDir: repoRoot,
		Task:       "Refactor Foo in foo.go",
	}))

	row := conn.QueryRowContext(context.Background(), `SELECT objective, artifact_path FROM memory_handoffs WHERE session_id = ? ORDER BY created_at DESC LIMIT 1`, "session-2")
	var objective string
	var artifactPath string
	require.NoError(t, row.Scan(&objective, &artifactPath))
	require.Equal(t, "Refactor Foo in foo.go", objective)
	require.Contains(t, artifactPath, ".sapphire/state/memory/handoffs/")
	require.FileExists(t, artifactPath)
}

func TestCompilerIncrementsIndexEpochWhenTrackedFileChanges(t *testing.T) {
	t.Parallel()

	repoRoot := seedMemoryTestRepo(t)
	conn := openMemoryTestDB(t, repoRoot)
	compiler := NewCompiler(conn, nil)

	packet1, err := compiler.Compile(context.Background(), CompileRequest{
		SessionID:  "session-3",
		AgentID:    "main:session-3",
		WorkingDir: repoRoot,
		Task:       "Inspect Foo in foo.go",
	})
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(repoRoot, "foo.go"), []byte(`package sample

func Foo() string {
	return "updated"
}
`), 0o644))

	packet2, err := compiler.Compile(context.Background(), CompileRequest{
		SessionID:  "session-3",
		AgentID:    "main:session-3",
		WorkingDir: repoRoot,
		Task:       "Inspect Foo in foo.go",
	})
	require.NoError(t, err)
	require.Greater(t, packet2.RepoSnapshot.IndexEpoch, packet1.RepoSnapshot.IndexEpoch)
}

func TestCompilerReusesFreshCompiledPacketForImmediateRepeat(t *testing.T) {
	t.Parallel()

	repoRoot := seedMemoryTestRepo(t)
	initMemoryTestGitRepo(t, repoRoot)
	conn := openMemoryTestDB(t, repoRoot)
	compiler := NewCompiler(conn, nil)

	req := CompileRequest{
		SessionID:  "session-cache",
		AgentID:    "main:session-cache",
		WorkingDir: repoRoot,
		Task:       "Inspect Foo in foo.go",
	}

	packet1, err := compiler.Compile(context.Background(), req)
	require.NoError(t, err)

	packet2, err := compiler.Compile(context.Background(), req)
	require.NoError(t, err)

	require.Equal(t, packet1.ArtifactPath, packet2.ArtifactPath)

	bootPackets, err := appdb.New(conn).ListMemoryBootPacketsBySession(context.Background(), req.SessionID)
	require.NoError(t, err)
	require.Len(t, bootPackets, 1)
}

func TestCompilerRenderCachedPromptInjectionUsesFreshPacket(t *testing.T) {
	t.Parallel()

	repoRoot := seedMemoryTestRepo(t)
	initMemoryTestGitRepo(t, repoRoot)
	conn := openMemoryTestDB(t, repoRoot)
	compiler := NewCompiler(conn, nil)

	req := CompileRequest{
		SessionID:  "session-render-cache",
		AgentID:    "main:session-render-cache",
		WorkingDir: repoRoot,
		Task:       "Inspect Foo in foo.go",
	}

	require.Empty(t, compiler.RenderCachedPromptInjection(context.Background(), req))

	packet, err := compiler.Compile(context.Background(), req)
	require.NoError(t, err)

	rendered := compiler.RenderCachedPromptInjection(context.Background(), req)
	require.Contains(t, rendered, "## COMPILED BOOT PACKET")
	require.Contains(t, rendered, packet.ArtifactPath)
}

func TestCompilerIndexStatusAndWarmCodebase(t *testing.T) {
	t.Parallel()

	repoRoot := seedMemoryTestRepo(t)
	conn := openMemoryTestDB(t, repoRoot)
	compiler := NewCompiler(conn, nil)

	status, err := compiler.IndexStatus(context.Background(), repoRoot)
	require.NoError(t, err)
	require.False(t, status.Available)

	var progress []WarmProgress
	result, err := compiler.WarmCodebase(context.Background(), WarmRequest{
		WorkingDir: repoRoot,
		Force:      true,
	}, func(item WarmProgress) {
		progress = append(progress, item)
	})
	require.NoError(t, err)
	require.True(t, result.Status.Available)
	require.GreaterOrEqual(t, result.Status.FileCount, 3)
	require.NotEmpty(t, progress)
	require.Equal(t, "ready", progress[len(progress)-1].Phase)
	require.True(t, progress[len(progress)-1].Finished)
}

func TestCompilerWarmCodebaseReportsCanceledState(t *testing.T) {
	t.Parallel()

	repoRoot := seedMemoryTestRepo(t)
	conn := openMemoryTestDB(t, repoRoot)
	compiler := NewCompiler(conn, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var progress []WarmProgress
	_, err := compiler.WarmCodebase(ctx, WarmRequest{
		WorkingDir: repoRoot,
		Force:      true,
	}, func(item WarmProgress) {
		progress = append(progress, item)
	})
	require.ErrorIs(t, err, context.Canceled)
	require.NotEmpty(t, progress)
	require.Equal(t, "canceled", progress[len(progress)-1].Phase)
	require.Equal(t, "Codebase graph indexing stopped", progress[len(progress)-1].Message)
	require.Empty(t, progress[len(progress)-1].Error)
	require.True(t, progress[len(progress)-1].Finished)
}

func seedMemoryTestRepo(t *testing.T) string {
	t.Helper()

	repoRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(repoRoot, "go.mod"), []byte("module example.com/memorytest\n\ngo 1.25\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repoRoot, "foo.go"), []byte(`package sample

func Foo() string {
	return helper()
}

func helper() string {
	return "ok"
}
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repoRoot, "foo_test.go"), []byte(`package sample

import "testing"

func TestFoo(t *testing.T) {
	if Foo() == "" {
		t.Fatal("expected value")
	}
}
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repoRoot, "config.yaml"), []byte("mode: test\n"), 0o644))
	return repoRoot
}

func openMemoryTestDB(t *testing.T, repoRoot string) *sql.DB {
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

func initMemoryTestGitRepo(t *testing.T, repoRoot string) {
	t.Helper()

	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}

	run("init")
	run("config", "user.name", "Sapphire Tests")
	run("config", "user.email", "sapphire-tests@example.com")
	run("add", ".")
	run("commit", "-m", "initial")
}
