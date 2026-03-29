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
	require.Equal(t, "MISTAKES.md", packetWith.RequiredReads[0].Path)
	require.Equal(t, "failure intelligence register", packetWith.RequiredReads[0].Reason)
}
