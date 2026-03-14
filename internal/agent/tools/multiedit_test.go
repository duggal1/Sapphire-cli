package tools

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/sapphire/internal/filetracker"
	"github.com/charmbracelet/sapphire/internal/history"
	"github.com/charmbracelet/sapphire/internal/permission"
	"github.com/charmbracelet/sapphire/internal/pubsub"
	"github.com/stretchr/testify/require"
)

type mockPermissionService struct {
	*pubsub.Broker[permission.PermissionRequest]
}

func (m *mockPermissionService) Request(ctx context.Context, req permission.CreatePermissionRequest) (bool, error) {
	return true, nil
}

func (m *mockPermissionService) Grant(req permission.PermissionRequest) {}

func (m *mockPermissionService) Deny(req permission.PermissionRequest) {}

func (m *mockPermissionService) GrantPersistent(req permission.PermissionRequest) {}

func (m *mockPermissionService) AutoApproveSession(sessionID string) {}

func (m *mockPermissionService) SetSkipRequests(skip bool) {}

func (m *mockPermissionService) SkipRequests() bool {
	return false
}

func (m *mockPermissionService) SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[permission.PermissionNotification] {
	return make(<-chan pubsub.Event[permission.PermissionNotification])
}

type mockHistoryService struct {
	*pubsub.Broker[history.File]
}

func (m *mockHistoryService) Create(ctx context.Context, sessionID, path, content string) (history.File, error) {
	return history.File{Path: path, Content: content}, nil
}

func (m *mockHistoryService) CreateVersion(ctx context.Context, sessionID, path, content string) (history.File, error) {
	return history.File{}, nil
}

func (m *mockHistoryService) GetByPathAndSession(ctx context.Context, path, sessionID string) (history.File, error) {
	return history.File{Path: path, Content: ""}, nil
}

func (m *mockHistoryService) Get(ctx context.Context, id string) (history.File, error) {
	return history.File{}, nil
}

func (m *mockHistoryService) ListBySession(ctx context.Context, sessionID string) ([]history.File, error) {
	return nil, nil
}

func (m *mockHistoryService) ListLatestSessionFiles(ctx context.Context, sessionID string) ([]history.File, error) {
	return nil, nil
}

func (m *mockHistoryService) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *mockHistoryService) DeleteSessionFiles(ctx context.Context, sessionID string) error {
	return nil
}

type mockFileTracker struct {
	lastRead map[string]time.Time
}

func (m *mockFileTracker) RecordRead(_ context.Context, _ string, path string) {
	if m.lastRead == nil {
		m.lastRead = make(map[string]time.Time)
	}
	m.lastRead[path] = time.Now().Add(time.Minute)
}

func (m *mockFileTracker) LastReadTime(_ context.Context, _ string, path string) time.Time {
	if m.lastRead == nil {
		return time.Time{}
	}
	return m.lastRead[path]
}

func (m *mockFileTracker) ListReadFiles(_ context.Context, _ string) ([]string, error) {
	paths := make([]string, 0, len(m.lastRead))
	for path := range m.lastRead {
		paths = append(paths, path)
	}
	return paths, nil
}

var _ filetracker.Service = (*mockFileTracker)(nil)

type stubLanguageModel struct {
	generate func(ctx context.Context, call fantasy.Call) (*fantasy.Response, error)
}

var errStubUnsupported = errors.New("unsupported in stub")

func (m *stubLanguageModel) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
	return m.generate(ctx, call)
}

func (m *stubLanguageModel) Stream(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
	return nil, errStubUnsupported
}

func (m *stubLanguageModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errStubUnsupported
}

func (m *stubLanguageModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errStubUnsupported
}

func (m *stubLanguageModel) Provider() string {
	return "stub"
}

func (m *stubLanguageModel) Model() string {
	return "stub"
}

func TestApplyEditToContentPartialSuccess(t *testing.T) {
	t.Parallel()

	content := "line 1\nline 2\nline 3\n"

	// Test successful edit.
	newContent, err := applyEditToContent(content, MultiEditOperation{
		OldString: "line 1",
		NewString: "LINE 1",
	})
	require.NoError(t, err)
	require.Contains(t, newContent, "LINE 1")
	require.Contains(t, newContent, "line 2")

	// Test failed edit (string not found).
	_, err = applyEditToContent(content, MultiEditOperation{
		OldString: "line 99",
		NewString: "LINE 99",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestMultiEditSequentialApplication(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create test file.
	content := "line 1\nline 2\nline 3\nline 4\n"
	err := os.WriteFile(testFile, []byte(content), 0o644)
	require.NoError(t, err)

	// Manually test the sequential application logic.
	currentContent := content

	// Apply edits sequentially, tracking failures.
	edits := []MultiEditOperation{
		{OldString: "line 1", NewString: "LINE 1"},   // Should succeed
		{OldString: "line 99", NewString: "LINE 99"}, // Should fail - doesn't exist
		{OldString: "line 3", NewString: "LINE 3"},   // Should succeed
		{OldString: "line 2", NewString: "LINE 2"},   // Should succeed - still exists
	}

	var failedEdits []FailedEdit
	successCount := 0

	for i, edit := range edits {
		newContent, err := applyEditToContent(currentContent, edit)
		if err != nil {
			failedEdits = append(failedEdits, FailedEdit{
				Index: i + 1,
				Error: err.Error(),
				Edit:  edit,
			})
			continue
		}
		currentContent = newContent
		successCount++
	}

	// Verify results.
	require.Equal(t, 3, successCount, "Expected 3 successful edits")
	require.Len(t, failedEdits, 1, "Expected 1 failed edit")

	// Check failed edit details.
	require.Equal(t, 2, failedEdits[0].Index)
	require.Contains(t, failedEdits[0].Error, "not found")

	// Verify content changes.
	require.Contains(t, currentContent, "LINE 1")
	require.Contains(t, currentContent, "LINE 2")
	require.Contains(t, currentContent, "LINE 3")
	require.Contains(t, currentContent, "line 4") // Original unchanged
	require.NotContains(t, currentContent, "LINE 99")
}

func TestMultiEditAllEditsSucceed(t *testing.T) {
	t.Parallel()

	content := "line 1\nline 2\nline 3\n"

	edits := []MultiEditOperation{
		{OldString: "line 1", NewString: "LINE 1"},
		{OldString: "line 2", NewString: "LINE 2"},
		{OldString: "line 3", NewString: "LINE 3"},
	}

	currentContent := content
	successCount := 0

	for _, edit := range edits {
		newContent, err := applyEditToContent(currentContent, edit)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		currentContent = newContent
		successCount++
	}

	require.Equal(t, 3, successCount)
	require.Contains(t, currentContent, "LINE 1")
	require.Contains(t, currentContent, "LINE 2")
	require.Contains(t, currentContent, "LINE 3")
}

func TestMultiEditAllEditsFail(t *testing.T) {
	t.Parallel()

	content := "line 1\nline 2\n"

	edits := []MultiEditOperation{
		{OldString: "line 99", NewString: "LINE 99"},
		{OldString: "line 100", NewString: "LINE 100"},
	}

	currentContent := content
	var failedEdits []FailedEdit

	for i, edit := range edits {
		newContent, err := applyEditToContent(currentContent, edit)
		if err != nil {
			failedEdits = append(failedEdits, FailedEdit{
				Index: i + 1,
				Error: err.Error(),
				Edit:  edit,
			})
			continue
		}
		currentContent = newContent
	}

	require.Len(t, failedEdits, 2)
	require.Equal(t, content, currentContent, "Content should be unchanged")
}

func TestMultiEditParamsNormalizeNestedShorthand(t *testing.T) {
	t.Parallel()

	var params MultiEditParams
	err := json.Unmarshal([]byte(`{
		"file_edits": [{
			"file_path": "/tmp/test.txt",
			"old_string": "line 1",
			"new_string": "LINE 1"
		}]
	}`), &params)
	require.NoError(t, err)

	normalized, err := normalizeMultiEditParams(params)
	require.NoError(t, err)
	require.Len(t, normalized.FileEdits, 1)
	require.Equal(t, "/tmp/test.txt", normalized.FileEdits[0].FilePath)
	require.Len(t, normalized.FileEdits[0].Edits, 1)
	require.Equal(t, "line 1", normalized.FileEdits[0].Edits[0].OldString)
	require.Equal(t, "LINE 1", normalized.FileEdits[0].Edits[0].NewString)
}

func TestMultiEditParamsNormalizeSingleFileShorthand(t *testing.T) {
	t.Parallel()

	var params MultiEditParams
	err := json.Unmarshal([]byte(`{
		"file_path": "/tmp/test.txt",
		"old": "line 1",
		"replacement": "LINE 1"
	}`), &params)
	require.NoError(t, err)

	normalized, err := normalizeMultiEditParams(params)
	require.NoError(t, err)
	require.Len(t, normalized.FileEdits, 1)
	require.Equal(t, "/tmp/test.txt", normalized.FileEdits[0].FilePath)
	require.Len(t, normalized.FileEdits[0].Edits, 1)
	require.Equal(t, "line 1", normalized.FileEdits[0].Edits[0].OldString)
	require.Equal(t, "LINE 1", normalized.FileEdits[0].Edits[0].NewString)
}

func TestMultiEditParamsDecodeFileEditsObject(t *testing.T) {
	t.Parallel()

	var params MultiEditParams
	err := json.Unmarshal([]byte(`{
		"file_edits": {
			"path": "/tmp/test.txt",
			"edits": {
				"old": "line 1",
				"replace_with": "LINE 1"
			}
		}
	}`), &params)
	require.NoError(t, err)

	normalized, err := normalizeMultiEditParams(params)
	require.NoError(t, err)
	require.Len(t, normalized.FileEdits, 1)
	require.Equal(t, "/tmp/test.txt", normalized.FileEdits[0].FilePath)
	require.Len(t, normalized.FileEdits[0].Edits, 1)
	require.Equal(t, "line 1", normalized.FileEdits[0].Edits[0].OldString)
	require.Equal(t, "LINE 1", normalized.FileEdits[0].Edits[0].NewString)
}

func TestMultiEditParamsRejectNestedEmptyEditsDeterministically(t *testing.T) {
	t.Parallel()

	var params MultiEditParams
	err := json.Unmarshal([]byte(`{
		"file_edits": [{
			"file_path": "/tmp/test.txt"
		}]
	}`), &params)
	require.NoError(t, err)

	_, err = normalizeMultiEditParams(params)
	require.EqualError(t, err, "at least one edit operation is required")
}

func TestMultiEditRuntimeNormalizesNestedShorthandWithoutRepair(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("line 1\nline 2\n"), 0o644)
	require.NoError(t, err)

	fileTracker := &mockFileTracker{lastRead: map[string]time.Time{
		testFile: time.Now().Add(time.Minute),
	}}
	editGuard := NewEditGuard()
	editGuard.RecordView("session-1", testFile, true)

	tool := NewMultiEditTool(
		nil,
		editGuard,
		&mockPermissionService{},
		&mockHistoryService{},
		fileTracker,
		tmpDir,
	)

	model := &stubLanguageModel{
		generate: func(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
			return &fantasy.Response{
				Content: []fantasy.Content{
					fantasy.ToolCallContent{
						ToolCallID: "call-1",
						ToolName:   AgenticEditToolName,
						Input:      `{"file_edits":[{"file_path":"` + testFile + `","old_string":"line 1","new_string":"LINE 1"}]}`,
					},
				},
				Usage:        fantasy.Usage{TotalTokens: 10},
				FinishReason: fantasy.FinishReasonStop,
			}, nil
		},
	}

	agent := fantasy.NewAgent(model, fantasy.WithTools(tool))
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "session-1")

	result, err := agent.Generate(ctx, fantasy.AgentCall{Prompt: "update the file"})
	require.NoError(t, err)
	require.NotNil(t, result)

	content, err := os.ReadFile(testFile)
	require.NoError(t, err)
	require.Contains(t, string(content), "LINE 1")

	var toolResults []fantasy.ToolResultContent
	for _, part := range result.Response.Content {
		if toolResult, ok := fantasy.AsContentType[fantasy.ToolResultContent](part); ok {
			toolResults = append(toolResults, toolResult)
		}
	}
	require.Len(t, toolResults, 1)
	require.Equal(t, AgenticEditToolName, toolResults[0].ToolName)
}
