package memory

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompilerToolSearchPrefersImplementationOverGeneratedAndTestNoise(t *testing.T) {
	t.Parallel()

	repoRoot := seedMemoryTestRepo(t)
	require.NoError(t, os.MkdirAll(filepath.Join(repoRoot, "pkg", "zapier"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(repoRoot, "pkg", "generated"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(repoRoot, "pkg", "zapier", "testdata"), 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(repoRoot, "pkg", "zapier", "service.go"), []byte(`package zapier

type ZapierService struct{}

func (s *ZapierService) SyncWebhook() string {
	return "ok"
}
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repoRoot, "pkg", "zapier", "service_test.go"), []byte(`package zapier

import "testing"

func TestSyncWebhook(t *testing.T) {
	if (&ZapierService{}).SyncWebhook() == "" {
		t.Fatal("expected value")
	}
}
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repoRoot, "pkg", "generated", "zapier_service_generated.go"), []byte(`package generated

type ZapierServiceGenerated struct{}
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repoRoot, "pkg", "zapier", "testdata", "zapier_fixture.json"), []byte(`{"name":"zapier"}`), 0o644))

	conn := openMemoryTestDB(t, repoRoot)
	compiler := NewCompiler(conn, nil)

	_, err := compiler.WarmCodebase(context.Background(), WarmRequest{
		WorkingDir: repoRoot,
		Force:      true,
	}, nil)
	require.NoError(t, err)

	_, matches, err := compiler.ToolSearch(context.Background(), repoRoot, "zapier service", 5)
	require.NoError(t, err)
	require.NotEmpty(t, matches)
	require.Equal(t, "pkg/zapier/service.go", matches[0].Path)
	require.Equal(t, "ZapierService", matches[0].Name)
	require.NotContains(t, matches[0].Path, "generated")
	require.NotContains(t, matches[0].Path, "testdata")
}
