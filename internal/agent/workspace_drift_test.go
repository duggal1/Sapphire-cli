package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type workspaceDriftTracker struct {
	reads map[string]time.Time
	files []string
}

func (m *workspaceDriftTracker) RecordRead(context.Context, string, string) {}

func (m *workspaceDriftTracker) LastReadTime(_ context.Context, _ string, path string) time.Time {
	if m == nil {
		return time.Time{}
	}
	return m.reads[path]
}

func (m *workspaceDriftTracker) ListReadFiles(_ context.Context, _ string) ([]string, error) {
	if m == nil {
		return nil, nil
	}
	return append([]string{}, m.files...), nil
}

func TestBuildWorkspaceDriftPromptReportsModifiedAndRemovedFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	modified := filepath.Join(dir, "modified.go")
	removed := filepath.Join(dir, "removed.go")
	if err := os.WriteFile(modified, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write modified file: %v", err)
	}
	if err := os.WriteFile(removed, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write removed file: %v", err)
	}

	lastRead := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	later := lastRead.Add(10 * time.Minute)
	if err := os.Chtimes(modified, later, later); err != nil {
		t.Fatalf("chtimes modified file: %v", err)
	}
	if err := os.Remove(removed); err != nil {
		t.Fatalf("remove tracked file: %v", err)
	}

	tracker := &workspaceDriftTracker{
		reads: map[string]time.Time{
			modified: lastRead,
			removed:  lastRead,
		},
		files: []string{modified, removed},
	}

	prompt := buildWorkspaceDriftPrompt(context.Background(), tracker, "sess", dir)
	if !strings.Contains(prompt, "<workspace_drift>") {
		t.Fatalf("expected workspace drift wrapper, got %q", prompt)
	}
	if !strings.Contains(prompt, "modified.go") {
		t.Fatalf("expected modified file to be reported, got %q", prompt)
	}
	if !strings.Contains(prompt, "removed.go") {
		t.Fatalf("expected removed file to be reported, got %q", prompt)
	}
}

func TestBuildWorkspaceDriftPromptSkipsFreshFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "fresh.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write fresh file: %v", err)
	}

	now := time.Now().Add(time.Minute).Truncate(time.Second)
	if err := os.Chtimes(path, now, now); err != nil {
		t.Fatalf("chtimes fresh file: %v", err)
	}

	tracker := &workspaceDriftTracker{
		reads: map[string]time.Time{
			path: now,
		},
		files: []string{path},
	}

	if prompt := buildWorkspaceDriftPrompt(context.Background(), tracker, "sess", dir); prompt != "" {
		t.Fatalf("expected no prompt for fresh files, got %q", prompt)
	}
}
