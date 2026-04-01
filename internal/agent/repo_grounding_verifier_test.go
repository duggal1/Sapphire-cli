package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	agenttools "github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/stretchr/testify/require"
)

func TestVerifyRepoGroundingClaimsRejectsMissingFileGroundedSymbol(t *testing.T) {
	t.Parallel()

	repoRoot := newRepoGroundingTestRepo(t)
	ctx := context.WithValue(context.Background(), agenttools.WorkingDirContextKey, repoRoot)
	policy := agenttools.LearnedToolPolicy{TaskFamily: "design/broad/backend"}

	err := verifyRepoGroundingClaims(ctx, policy, "The cleanest path is to call `platform.NewRuntimeConfig()` from internal/platform/runtime.go before wiring cmd/api.")
	require.Error(t, err)
	require.Contains(t, err.Error(), "internal/platform/runtime.go -> NewRuntimeConfig")
}

func TestVerifyRepoGroundingClaimsRejectsMissingPackageQualifiedSymbol(t *testing.T) {
	t.Parallel()

	repoRoot := newRepoGroundingTestRepo(t)
	ctx := context.WithValue(context.Background(), agenttools.WorkingDirContextKey, repoRoot)
	policy := agenttools.LearnedToolPolicy{TaskFamily: "design/broad/backend"}

	err := verifyRepoGroundingClaims(ctx, policy, "The current repo already exposes platform.NewRuntimeConfig() and auth.Status(), so wire both directly.")
	require.Error(t, err)
	require.Contains(t, err.Error(), "platform.NewRuntimeConfig")
	require.NotContains(t, err.Error(), "auth.Status")
}

func TestVerifyRepoGroundingClaimsAllowsRealRepoGroundedSymbols(t *testing.T) {
	t.Parallel()

	repoRoot := newRepoGroundingTestRepo(t)
	ctx := context.WithValue(context.Background(), agenttools.WorkingDirContextKey, repoRoot)
	policy := agenttools.LearnedToolPolicy{TaskFamily: "research/broad/backend"}

	err := verifyRepoGroundingClaims(ctx, policy, "internal/platform/runtime.go already defines `RuntimeConfig`, and the repository exposes auth.Status() and billing.Status() as existing helpers.")
	require.NoError(t, err)
}

func TestVerifyRepoGroundingClaimsSkipsInitializationFamilies(t *testing.T) {
	t.Parallel()

	repoRoot := newRepoGroundingTestRepo(t)
	ctx := context.WithValue(context.Background(), agenttools.WorkingDirContextKey, repoRoot)
	policy := agenttools.LearnedToolPolicy{TaskFamily: "initialize/broad/codebase"}

	err := verifyRepoGroundingClaims(ctx, policy, "AGENTS.md should mention platform.NewRuntimeConfig() if we add it later.")
	require.NoError(t, err)
}

func newRepoGroundingTestRepo(t *testing.T) string {
	t.Helper()

	repoRoot := t.TempDir()
	writeRepoGroundingFile(t, repoRoot, "internal/platform/runtime.go", "package platform\n\ntype RuntimeConfig struct{}\n")
	writeRepoGroundingFile(t, repoRoot, "internal/auth/status.go", "package auth\n\nfunc Status() string { return \"ok\" }\n")
	writeRepoGroundingFile(t, repoRoot, "internal/billing/status.go", "package billing\n\nfunc Status() string { return \"ok\" }\n")
	writeRepoGroundingFile(t, repoRoot, "cmd/api/main.go", "package main\n\nfunc main() {}\n")
	return repoRoot
}

func writeRepoGroundingFile(t *testing.T, repoRoot, relPath, content string) {
	t.Helper()

	path := filepath.Join(repoRoot, filepath.FromSlash(relPath))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
