package memory

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompilerIncludesMistakesReadOnlyWhenRequested(t *testing.T) {
	t.Parallel()

	repoRoot := seedMemoryTestRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(repoRoot, "MISTAKES.md"), []byte("# mistakes\n\nseed\n"), 0o644))

	conn := openMemoryTestDB(t, repoRoot)
	compiler := NewCompiler(conn, nil)

	packetWithout, err := compiler.Compile(context.Background(), CompileRequest{
		SessionID:  "session-no-mistakes",
		AgentID:    "main:session-no-mistakes",
		WorkingDir: repoRoot,
		Task:       "Inspect Foo in foo.go",
	})
	require.NoError(t, err)
	for _, read := range packetWithout.RequiredReads {
		require.NotEqual(t, "MISTAKES.md", read.Path)
	}

	packetWith, err := compiler.Compile(context.Background(), CompileRequest{
		SessionID:           "session-with-mistakes",
		AgentID:             "main:session-with-mistakes",
		WorkingDir:          repoRoot,
		Task:                "Inspect Foo in foo.go",
		IncludeMistakesRead: true,
	})
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(repoRoot, ".sapphire", "mistake.md"))
	require.NotEmpty(t, packetWith.RequiredReads)
	require.Equal(t, ".sapphire/mistake.md", packetWith.RequiredReads[0].Path)
	require.Equal(t, "local mistake logging protocol", packetWith.RequiredReads[0].Reason)
	require.Equal(t, "MISTAKES.md", packetWith.RequiredReads[1].Path)
	require.Equal(t, "failure intelligence register", packetWith.RequiredReads[1].Reason)
}

func TestCompilerIncludesProtocolReadWithoutMistakeRegister(t *testing.T) {
	t.Parallel()

	repoRoot := seedMemoryTestRepo(t)
	conn := openMemoryTestDB(t, repoRoot)
	compiler := NewCompiler(conn, nil)

	packet, err := compiler.Compile(context.Background(), CompileRequest{
		SessionID:           "session-protocol-only",
		AgentID:             "main:session-protocol-only",
		WorkingDir:          repoRoot,
		Task:                "Inspect Foo in foo.go",
		IncludeMistakesRead: true,
	})
	require.NoError(t, err)
	require.NotEmpty(t, packet.RequiredReads)
	require.Equal(t, ".sapphire/mistake.md", packet.RequiredReads[0].Path)
	require.Equal(t, "local mistake logging protocol", packet.RequiredReads[0].Reason)
	for _, read := range packet.RequiredReads {
		require.NotEqual(t, "MISTAKES.md", read.Path)
	}
}
