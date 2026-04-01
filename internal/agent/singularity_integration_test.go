//go:build integration

package agent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/stretchr/testify/require"
)

func TestGeminiSingularityInitializationLearningPhaseIII(t *testing.T) {
	if os.Getenv(integrationEnvFlag) != "1" {
		t.Skip("integration test disabled")
	}

	bin := buildSapphireBinary(t)
	model := singularityIntegrationModel()
	repoDir := filepath.Join(t.TempDir(), "repo")
	dataDir := filepath.Join(t.TempDir(), "data")
	require.NoError(t, os.MkdirAll(repoDir, 0o755))
	require.NoError(t, os.MkdirAll(dataDir, 0o755))
	writeSingularityIntegrationRepo(t, repoDir)

	prompt := "Initialize this codebase thoroughly. Inspect the architecture across auth, billing, config, docs, and HTTP layers before writing a comprehensive AGENTS.md for future agents. This is a repo-wide initialization task, so gather enough evidence before writing."

	out1 := runSapphirePromptWithDataDir(t, bin, repoDir, dataDir, model, prompt, 90*time.Second)
	assertNoToolErrors(t, out1)

	cfg, err := config.Init(repoDir, dataDir, false)
	require.NoError(t, err)

	policy1, err := GetSingularityPolicy(cfg, "initialize/broad/codebase")
	require.NoError(t, err)
	require.Contains(t, []string{learnedPolicyStateCandidate, learnedPolicyStatePromoted}, policy1.PromotionState)
	require.GreaterOrEqual(t, policy1.Confidence, minPolicyConfidenceForInjection)
	require.True(t, policy1.RequireHarness)
	require.True(t, policy1.PreferParallel)
	require.True(t, policy1.ForbidBashDiscovery)
	require.NotEmpty(t, policy1.SkillFilePath)

	out2 := runSapphirePromptWithDataDir(t, bin, repoDir, dataDir, model, prompt, 90*time.Second)
	assertNoToolErrors(t, out2)

	policy2, err := GetSingularityPolicy(cfg, "initialize/broad/codebase")
	require.NoError(t, err)
	require.GreaterOrEqual(t, policy2.AppliedCount, 1)
	require.GreaterOrEqual(t, policy2.Confidence, policy1.Confidence)

	audit, err := ListSingularityAudit(cfg, "initialize/broad/codebase", 10)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(audit.Records), 2)
	require.True(t, audit.Records[len(audit.Records)-1].AppliedPolicy)
}

func singularityIntegrationModel() string {
	if model := strings.TrimSpace(os.Getenv(modelEnvFlag)); model != "" {
		return model
	}
	return "gemini/gemini-3-flash-preview"
}

func runSapphirePromptWithDataDir(t *testing.T, binPath, workDir, dataDir, model, prompt string, timeout time.Duration) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binPath, "run", "--cwd", workDir, "--data-dir", dataDir, "-m", model, prompt)
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			t.Fatalf("model stall: exceeded %s\n%s", timeout, string(output))
		}
		t.Fatalf("run failed: %v\n%s", err, string(output))
	}
	return string(output)
}

func writeSingularityIntegrationRepo(t *testing.T, repoDir string) {
	t.Helper()

	files := map[string]string{
		"README.md": "Sapphire singularity integration fixture.\n",
		"go.mod":    "module singularity-fixture\n\ngo 1.26\n",
		"auth/service.go": `package auth

type Service struct{}

func (Service) Login(user string) string {
	return "token:" + user
}
`,
		"billing/invoice.go": `package billing

type Invoice struct {
	ID string
}
`,
		"config/config.go": `package config

type Config struct {
	Env string
}
`,
		"internal/http/router.go": `package http

func Routes() []string {
	return []string{"/login", "/invoice"}
}
`,
		"docs/overview.md": "# Overview\n\nAuth, billing, config, and HTTP layers live in separate packages.\n",
	}

	for rel, content := range files {
		path := filepath.Join(repoDir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
}
