package tools

import (
	"cmp"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/diff"
	"github.com/duggal1/Sapphire-cli/internal/filepathext"
	"github.com/duggal1/Sapphire-cli/internal/filetracker"
	"github.com/duggal1/Sapphire-cli/internal/fsext"
	"github.com/duggal1/Sapphire-cli/internal/history"
	"github.com/duggal1/Sapphire-cli/internal/lsp"
	"github.com/duggal1/Sapphire-cli/internal/permission"
)

type MultiEditOperation struct {
	OldString  string `json:"old_string" description:"The text to replace"`
	NewString  string `json:"new_string" description:"The text to replace it with"`
	ReplaceAll bool   `json:"replace_all,omitempty" description:"Replace all occurrences of old_string (default false)."`
}

func (o *MultiEditOperation) UnmarshalJSON(data []byte) error {
	type rawMultiEditOperation struct {
		OldString   *string `json:"old_string"`
		Old         *string `json:"old"`
		NewString   *string `json:"new_string"`
		New         *string `json:"new"`
		Replacement *string `json:"replacement"`
		Replace     *string `json:"replace"`
		ReplaceWith *string `json:"replace_with"`
		Content     *string `json:"content"`
		ReplaceAll  *bool   `json:"replace_all"`
		All         *bool   `json:"all"`
		Global      *bool   `json:"global"`
	}

	var raw rawMultiEditOperation
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	o.OldString = firstStringValue(raw.OldString, raw.Old)
	o.NewString = firstStringValue(raw.NewString, raw.New, raw.Replacement, raw.Replace, raw.ReplaceWith, raw.Content)
	o.ReplaceAll = firstBoolValue(raw.ReplaceAll, raw.All, raw.Global)

	return nil
}

type FileEdit struct {
	FilePath string               `json:"file_path" description:"The absolute path to the file to modify"`
	Edits    []MultiEditOperation `json:"edits" description:"Array of edit operations to perform sequentially on the file"`
}

func (f *FileEdit) UnmarshalJSON(data []byte) error {
	type rawFileEdit struct {
		FilePath    string          `json:"file_path"`
		Path        string          `json:"path"`
		Edits       json.RawMessage `json:"edits"`
		OldString   *string         `json:"old_string"`
		Old         *string         `json:"old"`
		NewString   *string         `json:"new_string"`
		New         *string         `json:"new"`
		Replacement *string         `json:"replacement"`
		Replace     *string         `json:"replace"`
		ReplaceWith *string         `json:"replace_with"`
		Content     *string         `json:"content"`
		ReplaceAll  *bool           `json:"replace_all"`
		All         *bool           `json:"all"`
		Global      *bool           `json:"global"`
	}

	var raw rawFileEdit
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	edits, err := decodeMultiEditOperations(raw.Edits)
	if err != nil {
		return err
	}

	f.FilePath = strings.TrimSpace(cmp.Or(raw.FilePath, raw.Path))
	f.Edits = edits

	if len(f.Edits) == 0 && hasInlineEdit(raw.OldString, raw.Old, raw.NewString, raw.New, raw.Replacement, raw.Replace, raw.ReplaceWith, raw.Content, raw.ReplaceAll, raw.All, raw.Global) {
		f.Edits = []MultiEditOperation{{
			OldString:  firstStringValue(raw.OldString, raw.Old),
			NewString:  firstStringValue(raw.NewString, raw.New, raw.Replacement, raw.Replace, raw.ReplaceWith, raw.Content),
			ReplaceAll: firstBoolValue(raw.ReplaceAll, raw.All, raw.Global),
		}}
	}

	return nil
}

type MultiEditParams struct {
	FileEdits  []FileEdit           `json:"file_edits,omitempty" description:"Array of files and their edits to apply in parallel"`
	FilePath   string               `json:"file_path,omitempty" description:"The absolute path to the file to modify (legacy)"`
	Path       string               `json:"path,omitempty" description:"Alias for file_path"`
	Edits      []MultiEditOperation `json:"edits,omitempty" description:"Array of edit operations to perform sequentially on the file (legacy)"`
	OldString  string               `json:"old_string,omitempty" description:"Single-edit compatibility alias for one-file edits"`
	NewString  string               `json:"new_string,omitempty" description:"Single-edit compatibility alias for one-file edits"`
	ReplaceAll bool                 `json:"replace_all,omitempty" description:"Single-edit compatibility alias for one-file edits"`
}

type MultiEditPermissionsParams struct {
	FilePath   string `json:"file_path"`
	OldContent string `json:"old_content,omitempty"`
	NewContent string `json:"new_content,omitempty"`
}

type FailedEdit struct {
	Index int                `json:"index"`
	Error string             `json:"error"`
	Edit  MultiEditOperation `json:"edit"`
}

type FileEditMetadata struct {
	FilePath     string       `json:"file_path"`
	Additions    int          `json:"additions"`
	Removals     int          `json:"removals"`
	OldContent   string       `json:"old_content,omitempty"`
	NewContent   string       `json:"new_content,omitempty"`
	EditsApplied int          `json:"edits_applied"`
	EditsFailed  []FailedEdit `json:"edits_failed,omitempty"`
}

type MultiEditResponseMetadata struct {
	Files        []FileEditMetadata `json:"files,omitempty"`
	Additions    int                `json:"additions"`
	Removals     int                `json:"removals"`
	OldContent   string             `json:"old_content,omitempty"`
	NewContent   string             `json:"new_content,omitempty"`
	EditsApplied int                `json:"edits_applied"`
	EditsFailed  []FailedEdit       `json:"edits_failed,omitempty"`
}

var (
	errMultiEditMissingFileEdits = errors.New("at least one file edit operation is required")
	errMultiEditMissingEdits     = errors.New("at least one edit operation is required")
)

func (p *MultiEditParams) UnmarshalJSON(data []byte) error {
	type rawMultiEditParams struct {
		FileEdits   json.RawMessage `json:"file_edits"`
		FilePath    string          `json:"file_path"`
		Path        string          `json:"path"`
		Edits       json.RawMessage `json:"edits"`
		OldString   *string         `json:"old_string"`
		Old         *string         `json:"old"`
		NewString   *string         `json:"new_string"`
		New         *string         `json:"new"`
		Replacement *string         `json:"replacement"`
		Replace     *string         `json:"replace"`
		ReplaceWith *string         `json:"replace_with"`
		Content     *string         `json:"content"`
		ReplaceAll  *bool           `json:"replace_all"`
		All         *bool           `json:"all"`
		Global      *bool           `json:"global"`
	}

	var raw rawMultiEditParams
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	fileEdits, err := decodeFileEdits(raw.FileEdits)
	if err != nil {
		return err
	}

	promotedFileEdits, promoted, err := decodePromotedTopLevelFileEdits(raw.Edits)
	if err != nil {
		return err
	}
	if len(fileEdits) == 0 && promoted {
		fileEdits = promotedFileEdits
	}

	edits, err := decodeMultiEditOperations(raw.Edits)
	if err != nil && !promoted {
		return err
	}
	if promoted {
		edits = nil
	}

	p.FileEdits = fileEdits
	p.FilePath = strings.TrimSpace(cmp.Or(raw.FilePath, raw.Path))
	p.Path = strings.TrimSpace(raw.Path)
	p.Edits = edits
	p.OldString = firstStringValue(raw.OldString, raw.Old)
	p.NewString = firstStringValue(raw.NewString, raw.New, raw.Replacement, raw.Replace, raw.ReplaceWith, raw.Content)
	p.ReplaceAll = firstBoolValue(raw.ReplaceAll, raw.All, raw.Global)

	if len(p.Edits) == 0 && hasInlineEdit(raw.OldString, raw.Old, raw.NewString, raw.New, raw.Replacement, raw.Replace, raw.ReplaceWith, raw.Content, raw.ReplaceAll, raw.All, raw.Global) {
		p.Edits = []MultiEditOperation{{
			OldString:  p.OldString,
			NewString:  p.NewString,
			ReplaceAll: p.ReplaceAll,
		}}
	}

	return nil
}

func decodePromotedTopLevelFileEdits(raw json.RawMessage) ([]FileEdit, bool, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, false, nil
	}

	fileEdits, err := decodeFileEdits(raw)
	if err != nil {
		return nil, false, nil
	}
	if len(fileEdits) == 0 {
		return nil, false, nil
	}
	for _, fileEdit := range fileEdits {
		if strings.TrimSpace(fileEdit.FilePath) != "" {
			return fileEdits, true, nil
		}
	}
	return nil, false, nil
}

const AgenticEditToolName = "agentic_edit"

//go:embed agentic_edit.md
var agenticEditDescription []byte

// NewMultiEditTool creates a tool for performing multiple sequential find-and-replace operations.
func NewMultiEditTool(
	lspManager *lsp.Manager,
	editGuard *EditGuard,
	permissions permission.Service,
	files history.Service,
	filetracker filetracker.Service,
	workingDir string,
) fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(
		AgenticEditToolName,
		string(agenticEditDescription),
		func(ctx context.Context, params MultiEditParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			normalizedParams, err := normalizeMultiEditParams(params)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			params = normalizedParams

			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.NewTextErrorResponse("session_id is required"), nil
			}

			resolvedEdits := make([]FileEdit, len(params.FileEdits))
			for i, fileEdit := range params.FileEdits {
				if fileEdit.FilePath == "" {
					return fantasy.NewTextErrorResponse("file_path is required"), nil
				}
				fileEdit.FilePath = filepathext.SmartJoin(workingDir, fileEdit.FilePath)
				if err := editGuard.EnsureAllowed(sessionID, fileEdit.FilePath, true); err != nil {
					return fantasy.NewTextErrorResponse(err.Error()), nil
				}
				resolvedEdits[i] = fileEdit
			}
			params.FileEdits = resolvedEdits

			type fileResult struct {
				filePath string
				output   string
				meta     MultiEditResponseMetadata
			}

			results := make([]fileResult, 0, len(params.FileEdits))

			var finalOutput strings.Builder
			var allErrors []string
			var finalMeta MultiEditResponseMetadata

			for _, fe := range params.FileEdits {
				if err := editGuard.EnsureAllowed(sessionID, fe.FilePath, true); err != nil {
					allErrors = append(allErrors, fmt.Sprintf("- %s: %v", fe.FilePath, err))
					continue
				}

				if err := validateEdits(fe.Edits); err != nil {
					allErrors = append(allErrors, fmt.Sprintf("%s: %s", fe.FilePath, err.Error()))
					continue
				}

				editCtx := editContext{ctx: ctx, permissions: permissions, files: files, filetracker: filetracker, workingDir: workingDir, toolName: AgenticEditToolName}

				var response fantasy.ToolResponse
				var err error
				if fe.Edits[0].OldString == "" {
					response, err = processMultiEditWithCreation(editCtx, fe, call)
				} else {
					response, err = processMultiEditExistingFile(editCtx, fe, call)
				}

				if err != nil {
					allErrors = append(allErrors, fmt.Sprintf("%s: %s", fe.FilePath, err.Error()))
					continue
				}

				if response.IsError {
					allErrors = append(allErrors, fmt.Sprintf("%s: %s", fe.FilePath, response.Content))
					continue
				}

				notifyLSPs(ctx, lspManager, fe.FilePath)
				text := fmt.Sprintf("<result>\n%s\n</result>\n", response.Content)
				diagnostics, summary := getDiagnosticsWithSummary(ctx, fe.FilePath, lspManager)
				text += diagnostics

				var meta MultiEditResponseMetadata
				if response.Metadata != "" {
					_ = json.Unmarshal([]byte(response.Metadata), &meta)
				}

				results = append(results, fileResult{filePath: fe.FilePath, output: text, meta: meta})

				editGuard.SetLockedIfErrors(sessionID, fe.FilePath, summary.FileErrors+summary.CompilerErrors+summary.FileWarnings+summary.CompilerWarnings > 0)
			}

			// Use Go 1.26 iterators to process results
			for res := range slices.Values(results) {
				if res.output != "" {
					finalOutput.WriteString(res.output)
					finalOutput.WriteString("\n")
				}

				fileMeta := FileEditMetadata{
					FilePath:     res.filePath,
					Additions:    res.meta.Additions,
					Removals:     res.meta.Removals,
					OldContent:   res.meta.OldContent,
					NewContent:   res.meta.NewContent,
					EditsApplied: res.meta.EditsApplied,
					EditsFailed:  res.meta.EditsFailed,
				}
				finalMeta.Files = append(finalMeta.Files, fileMeta)

				finalMeta.Additions += res.meta.Additions
				finalMeta.Removals += res.meta.Removals
				finalMeta.EditsApplied += res.meta.EditsApplied
				finalMeta.EditsFailed = append(finalMeta.EditsFailed, res.meta.EditsFailed...)

				if finalMeta.OldContent == "" && finalMeta.NewContent == "" {
					finalMeta.OldContent = res.meta.OldContent
					finalMeta.NewContent = res.meta.NewContent
				}
			}

			if len(allErrors) > 0 {
				if len(allErrors) == len(params.FileEdits) {
					return fantasy.NewTextErrorResponse(strings.Join(allErrors, "\n\n")), nil
				}
				finalOutput.WriteString("\nErrors encountered:\n")
				finalOutput.WriteString(strings.Join(allErrors, "\n"))
			}

			return fantasy.WithResponseMetadata(
				fantasy.NewTextResponse(finalOutput.String()),
				finalMeta,
			), nil
		})
}

func validateEdits(edits []MultiEditOperation) error {
	for i, edit := range edits {
		// Only the first edit can have empty old_string (for file creation)
		if i > 0 && edit.OldString == "" {
			return fmt.Errorf("edit %d: only the first edit can have empty old_string (for file creation)", i+1)
		}
	}
	return nil
}

func normalizeMultiEditParams(params MultiEditParams) (MultiEditParams, error) {
	if params.FilePath == "" && params.Path != "" {
		params.FilePath = params.Path
	}

	if len(params.Edits) == 0 && (params.OldString != "" || params.NewString != "" || params.ReplaceAll) {
		params.Edits = []MultiEditOperation{{
			OldString:  params.OldString,
			NewString:  params.NewString,
			ReplaceAll: params.ReplaceAll,
		}}
	}

	if len(params.FileEdits) == 0 && params.FilePath != "" {
		params.FileEdits = []FileEdit{{
			FilePath: params.FilePath,
			Edits:    params.Edits,
		}}
	}

	if len(params.FileEdits) == 0 {
		return MultiEditParams{}, errMultiEditMissingFileEdits
	}

	if len(params.FileEdits) > 25 {
		return MultiEditParams{}, fmt.Errorf("maximum of 25 file edits allowed per call")
	}

	normalizedFileEdits := make([]FileEdit, 0, len(params.FileEdits))
	for _, fileEdit := range params.FileEdits {
		if strings.TrimSpace(fileEdit.FilePath) == "" && len(fileEdit.Edits) == 0 {
			continue
		}
		if fileEdit.FilePath == "" {
			return MultiEditParams{}, fmt.Errorf("file_path is required")
		}
		if len(fileEdit.Edits) == 0 {
			if len(params.FileEdits) == 1 {
				return MultiEditParams{}, errMultiEditMissingEdits
			}
			continue
		}
		if err := validateEdits(fileEdit.Edits); err != nil {
			return MultiEditParams{}, fmt.Errorf("%s: %w", fileEdit.FilePath, err)
		}
		normalizedFileEdits = append(normalizedFileEdits, FileEdit{
			FilePath: fileEdit.FilePath,
			Edits:    fileEdit.Edits,
		})
	}
	if len(normalizedFileEdits) == 0 {
		return MultiEditParams{}, errMultiEditMissingFileEdits
	}

	params.FileEdits = normalizedFileEdits
	return params, nil
}

func decodeFileEdits(raw json.RawMessage) ([]FileEdit, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}

	var edits []FileEdit
	if err := json.Unmarshal(raw, &edits); err == nil {
		return edits, nil
	}

	var edit FileEdit
	if err := json.Unmarshal(raw, &edit); err == nil {
		return []FileEdit{edit}, nil
	}

	return nil, fmt.Errorf("invalid file_edits payload")
}

func decodeMultiEditOperations(raw json.RawMessage) ([]MultiEditOperation, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}

	var edits []MultiEditOperation
	if err := json.Unmarshal(raw, &edits); err == nil {
		return edits, nil
	}

	var edit MultiEditOperation
	if err := json.Unmarshal(raw, &edit); err == nil {
		return []MultiEditOperation{edit}, nil
	}

	return nil, fmt.Errorf("invalid edits payload")
}

func firstStringValue(values ...*string) string {
	for _, value := range values {
		if value != nil {
			return *value
		}
	}
	return ""
}

func firstBoolValue(values ...*bool) bool {
	for _, value := range values {
		if value != nil {
			return *value
		}
	}
	return false
}

func hasInlineEdit(values ...any) bool {
	for _, value := range values {
		switch typed := value.(type) {
		case *string:
			if typed != nil {
				return true
			}
		case *bool:
			if typed != nil {
				return true
			}
		}
	}
	return false
}

func processMultiEditWithCreation(edit editContext, params FileEdit, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	// First edit creates the file
	firstEdit := params.Edits[0]
	if firstEdit.OldString != "" {
		return fantasy.NewTextErrorResponse("first edit must have empty old_string for file creation"), nil
	}

	// Check if file already exists
	if _, err := os.Stat(params.FilePath); err == nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("file already exists: %s", params.FilePath)), nil
	} else if !os.IsNotExist(err) {
		return fantasy.ToolResponse{}, fmt.Errorf("failed to access file: %w", err)
	}

	// Create parent directories
	dir := filepath.Dir(params.FilePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fantasy.ToolResponse{}, fmt.Errorf("failed to create parent directories: %w", err)
	}

	// Start with the content from the first edit
	currentContent := firstEdit.NewString

	// Apply remaining edits to the content, tracking failures
	var failedEdits []FailedEdit
	for i := 1; i < len(params.Edits); i++ {
		edit := params.Edits[i]
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

	// Get session and message IDs
	sessionID := GetSessionFromContext(edit.ctx)
	if sessionID == "" {
		return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for creating a new file")
	}

	// Check permissions
	_, additions, removals := diff.GenerateDiff("", currentContent, strings.TrimPrefix(params.FilePath, edit.workingDir))

	editsApplied := len(params.Edits) - len(failedEdits)
	var description string
	if len(failedEdits) > 0 {
		description = fmt.Sprintf("Create file %s with %d of %d edits (%d failed)", params.FilePath, editsApplied, len(params.Edits), len(failedEdits))
	} else {
		description = fmt.Sprintf("Create file %s with %d edits", params.FilePath, editsApplied)
	}
	p, err := edit.permissions.Request(edit.ctx, permission.CreatePermissionRequest{
		SessionID:   sessionID,
		Path:        fsext.PathOrPrefix(params.FilePath, edit.workingDir),
		ToolCallID:  call.ID,
		ToolName:    AgenticEditToolName,
		Action:      "write",
		Description: description,
		Params: MultiEditPermissionsParams{
			FilePath:   params.FilePath,
			OldContent: "",
			NewContent: currentContent,
		},
	})
	if err != nil {
		return fantasy.ToolResponse{}, err
	}
	if !p {
		return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
	}

	// Write the file
	err = os.WriteFile(params.FilePath, []byte(currentContent), 0o644)
	if err != nil {
		return fantasy.ToolResponse{}, fmt.Errorf("failed to write file: %w", err)
	}
	QueueGitSnapshot(edit.ctx, params.FilePath)

	// Update file history
	_, err = edit.files.Create(edit.ctx, sessionID, params.FilePath, "")
	if err != nil {
		return fantasy.ToolResponse{}, fmt.Errorf("error creating file history: %w", err)
	}

	_, err = edit.files.CreateVersion(edit.ctx, sessionID, params.FilePath, currentContent)
	if err != nil {
		slog.Error("Error creating file history version", "error", err)
	}

	edit.filetracker.RecordRead(edit.ctx, sessionID, params.FilePath)

	var message string
	if len(failedEdits) > 0 {
		message = fmt.Sprintf("File created with %d of %d edits: %s (%d edit(s) failed)", editsApplied, len(params.Edits), params.FilePath, len(failedEdits))
	} else {
		message = fmt.Sprintf("File created with %d edits: %s", len(params.Edits), params.FilePath)
	}

	return fantasy.WithResponseMetadata(
		fantasy.NewTextResponse(message),
		MultiEditResponseMetadata{
			OldContent:   "",
			NewContent:   currentContent,
			Additions:    additions,
			Removals:     removals,
			EditsApplied: editsApplied,
			EditsFailed:  failedEdits,
		},
	), nil
}

func processMultiEditExistingFile(edit editContext, params FileEdit, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	// Validate file exists and is readable
	fileInfo, err := os.Stat(params.FilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fantasy.NewTextErrorResponse(fmt.Sprintf("file not found: %s", params.FilePath)), nil
		}
		return fantasy.ToolResponse{}, fmt.Errorf("failed to access file: %w", err)
	}

	if fileInfo.IsDir() {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("path is a directory, not a file: %s", params.FilePath)), nil
	}

	sessionID := GetSessionFromContext(edit.ctx)
	if sessionID == "" {
		return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for editing file")
	}

	// Check if file was read before editing
	lastRead := edit.filetracker.LastReadTime(edit.ctx, sessionID, params.FilePath)
	if lastRead.IsZero() {
		return fantasy.NewTextErrorResponse("you must read the file before editing it. Use the View tool first"), nil
	}

	// Check if file was modified since last read.
	modTime := fileInfo.ModTime().Truncate(time.Second)
	if modTime.After(lastRead) {
		return fantasy.NewTextErrorResponse(
			fmt.Sprintf("file %s has been modified since it was last read (mod time: %s, last read: %s)",
				params.FilePath, modTime.Format(time.RFC3339), lastRead.Format(time.RFC3339),
			)), nil
	}

	// Read current file content
	content, err := os.ReadFile(params.FilePath)
	if err != nil {
		return fantasy.ToolResponse{}, fmt.Errorf("failed to read file: %w", err)
	}

	oldContent, isCrlf := fsext.ToUnixLineEndings(string(content))
	currentContent := oldContent

	// Apply all edits sequentially, tracking failures
	var failedEdits []FailedEdit
	for i, edit := range params.Edits {
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

	// Check if content actually changed
	if oldContent == currentContent {
		// If we have failed edits, report them with specific details and hints
		if len(failedEdits) > 0 {
			var errMsgs []string
			for _, fe := range failedEdits {
				errMsgs = append(errMsgs, fmt.Sprintf("Edit #%d failed: %v", fe.Index, fe.Error))
			}
			fullError := fmt.Sprintf("OPERATIONAL FAILURE: no changes made - all %d edit(s) failed.\n\n%s\n\nPrecision violation. You MUST re-read the file(s) to establish character-perfect ground truth before retrying.",
				len(failedEdits), strings.Join(errMsgs, "\n"))

			return fantasy.WithResponseMetadata(
				fantasy.NewTextErrorResponse(fullError),
				MultiEditResponseMetadata{
					EditsApplied: 0,
					EditsFailed:  failedEdits,
				},
			), nil
		}
		return fantasy.NewTextErrorResponse("OPERATIONAL FAILURE: no changes made - resulting content is identical to source."), nil
	}

	// Generate diff and check permissions
	_, additions, removals := diff.GenerateDiff(oldContent, currentContent, strings.TrimPrefix(params.FilePath, edit.workingDir))

	editsApplied := len(params.Edits) - len(failedEdits)
	var description string
	if len(failedEdits) > 0 {
		description = fmt.Sprintf("Apply %d of %d edits to file %s (%d failed)", editsApplied, len(params.Edits), params.FilePath, len(failedEdits))
	} else {
		description = fmt.Sprintf("Apply %d edits to file %s", editsApplied, params.FilePath)
	}
	p, err := edit.permissions.Request(edit.ctx, permission.CreatePermissionRequest{
		SessionID:   sessionID,
		Path:        fsext.PathOrPrefix(params.FilePath, edit.workingDir),
		ToolCallID:  call.ID,
		ToolName:    AgenticEditToolName,
		Action:      "write",
		Description: description,
		Params: MultiEditPermissionsParams{
			FilePath:   params.FilePath,
			OldContent: oldContent,
			NewContent: currentContent,
		},
	})
	if err != nil {
		return fantasy.ToolResponse{}, err
	}
	if !p {
		return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
	}

	if isCrlf {
		currentContent, _ = fsext.ToWindowsLineEndings(currentContent)
	}

	// Write the updated content
	err = os.WriteFile(params.FilePath, []byte(currentContent), 0o644)
	if err != nil {
		return fantasy.ToolResponse{}, fmt.Errorf("failed to write file: %w", err)
	}
	QueueGitSnapshot(edit.ctx, params.FilePath)

	// Update file history
	file, err := edit.files.GetByPathAndSession(edit.ctx, params.FilePath, sessionID)
	if err != nil {
		_, err = edit.files.Create(edit.ctx, sessionID, params.FilePath, oldContent)
		if err != nil {
			return fantasy.ToolResponse{}, fmt.Errorf("error creating file history: %w", err)
		}
	}
	if file.Content != oldContent {
		// User manually changed the content, store an intermediate version
		_, err = edit.files.CreateVersion(edit.ctx, sessionID, params.FilePath, oldContent)
		if err != nil {
			slog.Error("Error creating file history version", "error", err)
		}
	}

	// Store the new version
	_, err = edit.files.CreateVersion(edit.ctx, sessionID, params.FilePath, currentContent)
	if err != nil {
		slog.Error("Error creating file history version", "error", err)
	}

	edit.filetracker.RecordRead(edit.ctx, sessionID, params.FilePath)

	var message string
	if len(failedEdits) > 0 {
		message = fmt.Sprintf("Applied %d of %d edits to file: %s (%d edit(s) failed)", editsApplied, len(params.Edits), params.FilePath, len(failedEdits))
	} else {
		message = fmt.Sprintf("Applied %d edits to file: %s", len(params.Edits), params.FilePath)
	}

	return fantasy.WithResponseMetadata(
		fantasy.NewTextResponse(message),
		MultiEditResponseMetadata{
			OldContent:   oldContent,
			NewContent:   currentContent,
			Additions:    additions,
			Removals:     removals,
			EditsApplied: editsApplied,
			EditsFailed:  failedEdits,
		},
	), nil
}

// detectEscapeConfusion checks if the user is trying to match control characters but the file contains literal escapes.
func detectEscapeConfusion(content, oldString string) string {
	var issues []string

	// Check for literal newlines
	if strings.Contains(oldString, "\n") {
		literalNewline := strings.ReplaceAll(oldString, "\n", "\\n")
		if strings.Contains(content, literalNewline) {
			issues = append(issues, "The target file contains literal '\\n' strings. Your `old_string` uses actual newline bytes (0x0A). Match '\\n' exactly.")
		}
	}

	// Check for literal tabs
	if strings.Contains(oldString, "\t") {
		literalTab := strings.ReplaceAll(oldString, "\t", "\\t")
		if strings.Contains(content, literalTab) {
			issues = append(issues, "The target file contains literal '\\t' strings. Your `old_string` uses actual tab bytes (0x09). Match '\\t' exactly.")
		}
	}

	if len(issues) > 0 {
		return "\n\nCRITICAL HINT: " + strings.Join(issues, " ") + " Establish ground truth via `agentic_view` and match the EXACT characters shown."
	}
	return ""
}

func applyEditToContent(content string, edit MultiEditOperation) (string, error) {
	if edit.OldString == "" && edit.NewString == "" {
		return content, nil
	}

	hint := detectEscapeConfusion(content, edit.OldString)

	newContent, err := tryApplyEdit(content, edit)
	if err == nil {
		return newContent, nil
	}

	// Staff Engineer Logic: Auto-remediation for literal escape confusion.
	// If the match failed but it's clearly an escape confusion issue,
	// try to match with literalized escapes.
	fuzzyEdit := edit
	if strings.Contains(edit.OldString, "\n") {
		fuzzyEdit.OldString = strings.ReplaceAll(fuzzyEdit.OldString, "\n", "\\n")
		fuzzyEdit.NewString = strings.ReplaceAll(fuzzyEdit.NewString, "\n", "\\n")
	}
	if strings.Contains(edit.OldString, "\t") {
		fuzzyEdit.OldString = strings.ReplaceAll(fuzzyEdit.OldString, "\t", "\\t")
		fuzzyEdit.NewString = strings.ReplaceAll(fuzzyEdit.NewString, "\t", "\\t")
	}

	if fuzzyEdit.OldString != edit.OldString {
		fuzzyContent, fuzzyErr := tryApplyEdit(content, fuzzyEdit)
		if fuzzyErr == nil {
			slog.Warn("Applied fuzzy escape edit due to newline/tab confusion", "file", edit.OldString)
			return fuzzyContent, nil
		}
	}

	return "", fmt.Errorf("%w%s", err, hint)
}

func tryApplyEdit(content string, edit MultiEditOperation) (string, error) {
	if edit.OldString == "" {
		return "", fmt.Errorf("old_string cannot be empty")
	}

	var newContent string

	if edit.ReplaceAll {
		newContent = strings.ReplaceAll(content, edit.OldString, edit.NewString)
		replacementCount := strings.Count(content, edit.OldString)
		if replacementCount == 0 {
			return "", fmt.Errorf("CRITICAL FAILURE: old_string not found. Character-perfect match required")
		}
	} else {
		index := strings.Index(content, edit.OldString)
		if index == -1 {
			return "", fmt.Errorf("CRITICAL FAILURE: old_string not found. Character-perfect match required")
		}

		lastIndex := strings.LastIndex(content, edit.OldString)
		if index != lastIndex {
			return "", fmt.Errorf("CRITICAL FAILURE: ambiguous match. Multiple instances detected. Increase context for unique character-perfect identification")
		}

		newContent = content[:index] + edit.NewString + content[index+len(edit.OldString):]
	}

	return newContent, nil
}
