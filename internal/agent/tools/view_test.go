package tools

import (
	"os"
	"strings"
	"testing"
)

func TestHandleFileNotFoundUsesRelativeSuggestions(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	missing := workingDir + "/internal/ui/messag.go"
	if err := os.MkdirAll(workingDir+"/internal/ui", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(workingDir+"/internal/ui/messages.go", []byte("package ui"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := viewNotFoundError(workingDir, missing, "internal/ui/messag.go")
	if err == nil {
		t.Fatal("expected missing file error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "Use ls or glob") {
		t.Fatalf("expected recovery hint, got %q", msg)
	}
	if !strings.Contains(msg, "internal/ui/messages.go") {
		t.Fatalf("expected relative suggestion, got %q", msg)
	}
}

func TestPrettyViewPathUsesRepoRelativePath(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	got := prettyViewPath(workingDir, workingDir+"/internal/ui/messages.go")
	if got != "internal/ui/messages.go" {
		t.Fatalf("expected repo-relative path, got %q", got)
	}
}
