package prompt

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/duggal1/Sapphire-cli/internal/config"
)

func TestGetGitStatusReturnsQuicklyOutsideGitRepo(t *testing.T) {
	dir := t.TempDir()

	start := time.Now()
	status, err := getGitStatus(context.Background(), dir)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("getGitStatus returned error: %v", err)
	}
	if status != "" {
		t.Fatalf("expected empty status outside git repo, got %q", status)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("getGitStatus took too long: %v", elapsed)
	}
}

func TestPromptBuildIgnoresGitStatusFailure(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	cfg, err := config.Load(dir, "", false)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	p, err := NewPrompt("test", "{{.WorkingDir}}|{{.GitStatus}}", WithWorkingDir(dir))
	if err != nil {
		t.Fatalf("new prompt: %v", err)
	}

	out, err := p.Build(context.Background(), "google", "gemini-3-flash-preview", *cfg)
	if err != nil {
		t.Fatalf("build prompt: %v", err)
	}
	if out == "" {
		t.Fatal("expected prompt output")
	}
}
