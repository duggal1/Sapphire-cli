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

func TestPromptBuildExposesRuntimeClockFields(t *testing.T) {
	fixedTime, err := time.Parse(time.RFC3339, "2026-04-01T12:34:56Z")
	if err != nil {
		t.Fatalf("parse time: %v", err)
	}

	workingDir := t.TempDir()
	t.Cleanup(func() {
		_ = os.RemoveAll(filepath.Join(workingDir, ".sapphire"))
	})

	cfg, err := config.Load(workingDir, "", false)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	p, err := NewPrompt("test", "{{.RuntimeClock}}|{{.RuntimeClockNewYork}}|{{.RuntimeClockSanFrancisco}}|{{.RuntimeClockKolkata}}|{{.RuntimeYear}}|{{.RuntimeDate}}|{{.RuntimeTime}}|{{.Date}}", WithTimeFunc(func() time.Time {
		return fixedTime
	}))
	if err != nil {
		t.Fatalf("new prompt: %v", err)
	}

	out, err := p.Build(context.Background(), "", "", *cfg)
	if err != nil {
		t.Fatalf("build prompt: %v", err)
	}

	const want = "2026-04-01T12:34:56Z|2026-04-01T08:34:56-04:00|2026-04-01T05:34:56-07:00|2026-04-01T18:04:56+05:30|2026|Wednesday, April 1, 2026|12:34:56 UTC|4/1/2026"
	if out != want {
		t.Fatalf("unexpected prompt output:\nwant %q\ngot  %q", want, out)
	}
}
