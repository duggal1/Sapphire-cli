package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/duggal1/Sapphire-cli/internal/csync"
)

func TestRenderDurableMemoryReadPromptUsesReadPathTemplate(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	memoryRoot := filepath.Join(workDir, memoryFolderName)
	if err := os.MkdirAll(memoryRoot, 0o755); err != nil {
		t.Fatalf("mkdir memory root: %v", err)
	}
	summary := "# Memory Summary\n\n- prefer structured tools first"
	if err := os.WriteFile(filepath.Join(memoryRoot, memorySummaryFile), []byte(summary), 0o644); err != nil {
		t.Fatalf("write memory summary: %v", err)
	}

	rendered := renderDurableMemoryReadPrompt(workDir)
	if !strings.Contains(rendered, "Quick memory pass procedure") {
		t.Fatalf("expected read_path.md content in rendered prompt")
	}
	if !strings.Contains(rendered, ".sapphire-memory/memory_summary.md") {
		t.Fatalf("expected repo-local memory path in rendered prompt")
	}
	if !strings.Contains(rendered, summary) {
		t.Fatalf("expected durable memory summary contents in rendered prompt")
	}
}

func TestInjectTieredMemoryPrependsDurableMemoryReadPrompt(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	memoryRoot := filepath.Join(workDir, memoryFolderName)
	if err := os.MkdirAll(memoryRoot, 0o755); err != nil {
		t.Fatalf("mkdir memory root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(memoryRoot, memorySummaryFile), []byte("# Memory Summary\n\n- prior fix"), 0o644); err != nil {
		t.Fatalf("write memory summary: %v", err)
	}

	agent := &sessionAgent{
		workingDir: csync.NewValue(workDir),
	}
	ctx := context.WithValue(context.Background(), tools.TurnPolicyContextKey, tools.TurnPolicy{
		AllowMemoryRead:          true,
		AllowMemoryWrite:         true,
		AllowAutoMemoryInjection: true,
	})

	history := []fantasy.Message{fantasy.NewSystemMessage("existing")}
	got := agent.injectTieredMemory(ctx, history, SessionAgentCall{SessionID: "session-1", Prompt: "resume prior work"}, 100000)
	if len(got) != 2 {
		t.Fatalf("expected durable memory read prompt to be prepended, got %d messages", len(got))
	}
	part, ok := fantasy.AsMessagePart[fantasy.TextPart](got[0].Content[0])
	if !ok {
		t.Fatalf("expected first injected message part to be text")
	}
	if !strings.Contains(part.Text, "Memory layout (general") {
		t.Fatalf("expected injected system message to come from durable memory read prompt")
	}
	if !strings.Contains(part.Text, "prior fix") {
		t.Fatalf("expected injected system message to include memory summary contents")
	}
}
