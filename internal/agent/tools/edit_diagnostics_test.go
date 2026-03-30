package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/duggal1/Sapphire-cli/internal/db"
	"github.com/duggal1/Sapphire-cli/internal/filetracker"
	"github.com/duggal1/Sapphire-cli/internal/lsp"
	"github.com/duggal1/Sapphire-cli/internal/permission"
	"github.com/duggal1/Sapphire-cli/internal/session"
	"github.com/stretchr/testify/require"
)

func TestEditToolIncludesImmediateDiagnostics(t *testing.T) {
	t.Parallel()

	ctx, workingDir, tracker, sessions, permissions, manager := newEditTestHarness(t)
	filePath := filepath.Join(workingDir, "main.diag")
	require.NoError(t, os.WriteFile(filePath, []byte("hello world\n"), 0o644))

	sess, err := sessions.Create(ctx, "Edit Diagnostics")
	require.NoError(t, err)

	ctx = context.WithValue(ctx, SessionIDContextKey, sess.ID)
	tracker.RecordRead(ctx, sess.ID, filePath)
	guard := NewEditGuard()
	guard.RecordView(sess.ID, filePath, true)
	client, ok := manager.Clients().Get("fake")
	require.True(t, ok)
	publishDiagnosticsSoon(filePath, client)

	tool := NewEditTool(manager, guard, permissions, &mockHistoryService{}, tracker, workingDir)
	input, err := json.Marshal(EditParams{
		FilePath:  filePath,
		OldString: "hello world",
		NewString: "hello sapphire",
	})
	require.NoError(t, err)

	resp, err := tool.Run(ctx, fantasy.ToolCall{
		ID:    "edit-1",
		Name:  EditToolName,
		Input: string(input),
	})
	require.NoError(t, err)
	require.False(t, resp.IsError)
	require.Contains(t, resp.Content, "<diagnostic_summary>")
	require.Contains(t, resp.Content, "Current file: 1 errors, 1 warnings")
	require.Contains(t, resp.Content, "<diagnostic_gate>")
	require.Contains(t, resp.Content, "Error:")
	require.Contains(t, resp.Content, "Warn:")
}

func TestMultiEditStopsOtherFilesAfterDiagnosticsErrors(t *testing.T) {
	t.Parallel()

	ctx, workingDir, tracker, sessions, permissions, manager := newEditTestHarness(t)
	fileOne := filepath.Join(workingDir, "first.diag")
	fileTwo := filepath.Join(workingDir, "second.diag")
	require.NoError(t, os.WriteFile(fileOne, []byte("alpha\n"), 0o644))
	require.NoError(t, os.WriteFile(fileTwo, []byte("beta\n"), 0o644))

	sess, err := sessions.Create(ctx, "Multi Edit Diagnostics")
	require.NoError(t, err)

	ctx = context.WithValue(ctx, SessionIDContextKey, sess.ID)
	tracker.RecordRead(ctx, sess.ID, fileOne)
	tracker.RecordRead(ctx, sess.ID, fileTwo)
	guard := NewEditGuard()
	guard.RecordView(sess.ID, fileOne, true)
	guard.RecordView(sess.ID, fileTwo, true)
	client, ok := manager.Clients().Get("fake")
	require.True(t, ok)
	publishDiagnosticsSoon(fileOne, client)

	tool := NewMultiEditTool(manager, guard, permissions, &mockHistoryService{}, tracker, workingDir)
	input, err := json.Marshal(MultiEditParams{
		FileEdits: []FileEdit{
			{
				FilePath: fileOne,
				Edits: []MultiEditOperation{
					{OldString: "alpha", NewString: "ALPHA"},
				},
			},
			{
				FilePath: fileTwo,
				Edits: []MultiEditOperation{
					{OldString: "beta", NewString: "BETA"},
				},
			},
		},
	})
	require.NoError(t, err)

	resp, err := tool.Run(ctx, fantasy.ToolCall{
		ID:    "agentic-edit-1",
		Name:  AgenticEditToolName,
		Input: string(input),
	})
	require.NoError(t, err)
	require.False(t, resp.IsError)
	require.Contains(t, resp.Content, "<diagnostic_summary>")
	require.Contains(t, resp.Content, "Current file: 1 errors, 1 warnings")
	require.Contains(t, resp.Content, "edit blocked: fix all current-file errors and warnings")

	content, err := os.ReadFile(fileTwo)
	require.NoError(t, err)
	require.Equal(t, "beta\n", string(content))
}

func TestEditGuardBlocksOtherFilesAfterDiagnostics(t *testing.T) {
	t.Parallel()

	guard := NewEditGuard()
	guard.RecordView("session-1", "/tmp/first.go", true)
	guard.RecordView("session-1", "/tmp/second.go", true)

	guard.SetLockedIfErrors("session-1", "/tmp/first.go", true)

	err := guard.EnsureAllowed("session-1", "/tmp/second.go", true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "/tmp/first.go")
	require.Contains(t, err.Error(), "/tmp/second.go")
}

func TestEditGuardAllowsLockedFileUntilDiagnosticsAreClear(t *testing.T) {
	t.Parallel()

	guard := NewEditGuard()
	guard.RecordView("session-1", "/tmp/first.go", true)
	guard.RecordView("session-1", "/tmp/second.go", true)

	guard.SetLockedIfErrors("session-1", "/tmp/first.go", true)
	require.NoError(t, guard.EnsureAllowed("session-1", "/tmp/first.go", true))

	guard.SetLockedIfErrors("session-1", "/tmp/first.go", false)
	require.NoError(t, guard.EnsureAllowed("session-1", "/tmp/second.go", true))
}

func TestEditGuardDoesNotBlockUnreadFile(t *testing.T) {
	t.Parallel()

	guard := NewEditGuard()

	require.NoError(t, guard.EnsureAllowed("session-1", "/tmp/unread.go", true))
	require.NoError(t, guard.EnsureAllowed("session-1", "/tmp/unread.go", true))
}

func TestMultiEditAcceptsSingleEditShape(t *testing.T) {
	t.Parallel()

	ctx, workingDir, tracker, sessions, permissions, _ := newEditTestHarness(t)
	filePath := filepath.Join(workingDir, "single.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("hello world\n"), 0o644))

	sess, err := sessions.Create(ctx, "Multi Edit Single Shape")
	require.NoError(t, err)

	ctx = context.WithValue(ctx, SessionIDContextKey, sess.ID)
	tracker.RecordRead(ctx, sess.ID, filePath)
	guard := NewEditGuard()
	guard.RecordView(sess.ID, filePath, true)

	tool := NewMultiEditTool(nil, guard, permissions, &mockHistoryService{}, tracker, workingDir)
	resp, err := tool.Run(ctx, fantasy.ToolCall{
		ID:    "agentic-edit-single",
		Name:  AgenticEditToolName,
		Input: fmt.Sprintf(`{"file_path":%q,"old_string":"hello world","new_string":"hello sapphire"}`, filePath),
	})
	require.NoError(t, err)
	require.False(t, resp.IsError)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	require.Equal(t, "hello sapphire\n", string(content))
}

func TestWriteDiagnosticsDoesNotTruncateEntries(t *testing.T) {
	t.Parallel()

	var output strings.Builder
	diagnostics := make([]string, 0, 12)
	for i := 0; i < 12; i++ {
		diagnostics = append(diagnostics, fmt.Sprintf("Error: /tmp/test.ts:%d:1 [tsserver] synthetic %d", i+1, i+1))
	}

	writeDiagnostics(&output, "file_diagnostics", diagnostics)

	rendered := output.String()
	require.Contains(t, rendered, "synthetic 1")
	require.Contains(t, rendered, "synthetic 12")
	require.NotContains(t, rendered, "... and")
}

func newEditTestHarness(t *testing.T) (context.Context, string, filetracker.Service, session.Service, permission.Service, *lsp.Manager) {
	t.Helper()

	ctx := t.Context()
	workingDir := t.TempDir()

	conn, err := db.Connect(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
	})

	q := db.New(conn)
	tracker := filetracker.NewService(q)
	sessions := session.NewService(q, conn)
	permissions := permission.NewPermissionService(workingDir, true, []string{})

	cfg, err := config.Init(workingDir, "", false)
	require.NoError(t, err)

	manager := lsp.NewManager(cfg)
	client, err := lsp.New(ctx, "fake", config.LSPConfig{
		Command:   "cat",
		FileTypes: []string{".diag"},
	}, cfg.Resolver(), workingDir, false)
	require.NoError(t, err)
	manager.Clients().Set("fake", client)

	return ctx, workingDir, tracker, sessions, permissions, manager
}

func publishDiagnosticsSoon(filePath string, client *lsp.Client) {
	go func() {
		time.Sleep(100 * time.Millisecond)

		payload, _ := json.Marshal(protocol.PublishDiagnosticsParams{
			URI: protocol.URIFromPath(filePath),
			Diagnostics: []protocol.Diagnostic{
				{
					Severity: protocol.SeverityError,
					Range: protocol.Range{
						Start: protocol.Position{Line: 0, Character: 0},
						End:   protocol.Position{Line: 0, Character: 5},
					},
					Message: "synthetic error",
				},
				{
					Severity: protocol.SeverityWarning,
					Range: protocol.Range{
						Start: protocol.Position{Line: 0, Character: 6},
						End:   protocol.Position{Line: 0, Character: 11},
					},
					Message: "synthetic warning",
				},
			},
		})

		lsp.HandleDiagnostics(client, payload)
	}()
}
