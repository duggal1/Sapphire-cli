package tools

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/filepathext"
	"github.com/duggal1/Sapphire-cli/internal/filetracker"
	"github.com/duggal1/Sapphire-cli/internal/lsp"
	"github.com/duggal1/Sapphire-cli/internal/permission"
	"github.com/duggal1/Sapphire-cli/internal/skills"
)

//go:embed view.md
var viewDescription []byte

// ViewParams defines the parameters for the file viewing tool (Codex-compatible).
type ViewParams struct {
	FilePaths []string `json:"file_paths,omitempty" description:"The paths to the files to read. Max concurrent reads will apply."`
	FilePath  string   `json:"file_path,omitempty" description:"The path to the file to read (legacy single file)"`
	Paths     []string `json:"paths,omitempty" description:"Alias for file_paths"`
	Files     []string `json:"files,omitempty" description:"Alias for file_paths"`
	Path      string   `json:"path,omitempty" description:"Alias for file_path"`
	// Codex-compatible: 1-indexed offset (user-friendly)
	Offset int `json:"offset,omitempty" description:"The line number to start reading from (1-based, defaults to 1)"`
	Limit  int `json:"limit,omitempty" description:"The number of lines to read (defaults to 5000)"`
	// Codex-compatible: Reading modes
	Mode        string           `json:"mode,omitempty" description:"Reading mode: 'slice' (default) or 'indentation' (Codex-compatible)"`
	Indentation *IndentationMode `json:"indentation,omitempty" description:"Indentation-aware reading options (Codex-compatible)"`
}

// IndentationMode provides indentation-aware reading configuration (Codex-compatible).
type IndentationMode struct {
	// AnchorLine is the line to anchor indentation detection (defaults to offset)
	AnchorLine *int `json:"anchor_line,omitempty"`
	// MaxLevels is max indentation depth to collect (0 = unlimited)
	MaxLevels *int `json:"max_levels,omitempty"`
	// IncludeSiblings includes sibling blocks at same indentation
	IncludeSiblings *bool `json:"include_siblings,omitempty"`
	// IncludeHeader includes header lines above anchor block
	IncludeHeader *bool `json:"include_header,omitempty"`
	// MaxLines is hard cap on returned lines (defaults to limit)
	MaxLines *int `json:"max_lines,omitempty"`
}

type ViewPermissionsParams struct {
	FilePath string `json:"file_path"`
	Offset   int    `json:"offset"`
	Limit    int    `json:"limit"`
}

type ViewResourceType string

const (
	ViewResourceUnset ViewResourceType = ""
	ViewResourceSkill ViewResourceType = "skill"
)

type ViewFileMetadata struct {
	FilePath            string           `json:"file_path"`
	Content             string           `json:"content"`
	ResourceType        ViewResourceType `json:"resource_type,omitempty"`
	ResourceName        string           `json:"resource_name,omitempty"`
	ResourceDescription string           `json:"resource_description,omitempty"`
}

type ViewResponseMetadata struct {
	Files               []ViewFileMetadata `json:"files,omitempty"`
	FilePath            string             `json:"file_path,omitempty"`
	Content             string             `json:"content,omitempty"`
	ResourceType        ViewResourceType   `json:"resource_type,omitempty"`
	ResourceName        string             `json:"resource_name,omitempty"`
	ResourceDescription string             `json:"resource_description,omitempty"`
}

const (
	ViewToolName        = "view"
	SingleViewToolName  = "single_view"
	AgenticViewToolName = "agentic_view"
	MaxReadSize         = 25 * 1024 * 1024 // 25MB
	DefaultReadLimit    = 5000
	MaxLineLength       = 2000
	viewTimeout         = 20 * time.Second

	// Codex-compatible constants
	TAB_WIDTH      = 4   // Tab expansion width (Codex default)
	MAX_LINE_WIDTH = 500 // Codex max line display width
)

// COMMENT_PREFIXES for comment detection (Codex-compatible)
var COMMENT_PREFIXES = []string{"#", "//", "--", "/*", "*", ";;", ";", "'"}

// NewViewTool creates a tool for reading file contents with support for line numbering and diagnostics.
func NewViewTool(
	name string,
	lspManager *lsp.Manager,
	editGuard *EditGuard,
	permissions permission.Service,
	filetracker filetracker.Service,
	workingDir string,
	maxConcurrent int,
	skillsPaths ...string,
) fantasy.AgentTool {
	description := string(viewDescription)
	if name == SingleViewToolName {
		description += "\n\nUse single_view only when exactly one verified repository file is truly sufficient. If the task expands beyond one file, or is non-trivial, switch immediately to agentic_view."
	}
	return fantasy.NewParallelAgentTool(
		name,
		description,
		func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if ctx.Err() != nil {
				return fantasy.ToolResponse{}, ctx.Err()
			}

			toolCtx, cancel := context.WithTimeout(ctx, viewTimeout)
			defer cancel()
			ctx = toolCtx

			filePaths := collectViewPaths(params)

			if len(filePaths) == 0 {
				return fantasy.NewTextResponse("No file paths provided. Use ls/glob to discover files, then call view or agentic_view with file_path(s)."), nil
			}

			if maxConcurrent <= 0 {
				maxConcurrent = 1
			}

			// Deduplicate file paths
			uniquePaths := make([]string, 0, len(filePaths))
			seenPaths := make(map[string]bool)
			for _, p := range filePaths {
				if p == "" {
					continue
				}
				if !seenPaths[p] {
					seenPaths[p] = true
					uniquePaths = append(uniquePaths, p)
				}
			}

			if len(uniquePaths) == 0 {
				return fantasy.NewTextResponse("No valid file paths provided. Use ls/glob to discover files, then call view or agentic_view with file_path(s)."), nil
			}

			var wg sync.WaitGroup
			var mu sync.Mutex

			sem := make(chan struct{}, maxConcurrent)

			type fileResult struct {
				filePath string
				output   string
				err      error
				meta     ViewResponseMetadata
			}

			results := make([]fileResult, len(uniquePaths))

			sessionID := GetSessionFromContext(ctx)

			for i, p := range uniquePaths {
				wg.Add(1)
				go func(idx int, filePath string) {
					defer wg.Done()

					// Military-grade safeguard: immediate exit if context cancelled
					if ctx.Err() != nil {
						return
					}

					sem <- struct{}{}
					defer func() { <-sem }()

					// Handle relative paths
					fullPath := filepathext.SmartJoin(workingDir, filePath)
					if _, err := os.Stat(fullPath); os.IsNotExist(err) {
						if aliasedPath, aliasedFull, ok := resolveViewAliasPath(workingDir, filePath); ok {
							filePath = aliasedPath
							fullPath = aliasedFull
						}
					}

					// Check if file is outside working directory
					absWorkingDir, err := filepath.Abs(workingDir)
					if err != nil {
						results[idx] = fileResult{filePath: filePath, err: fmt.Errorf("error resolving working directory: %w", err)}
						return
					}

					absFilePath, err := filepath.Abs(fullPath)
					if err != nil {
						results[idx] = fileResult{filePath: filePath, err: fmt.Errorf("error resolving file path: %w", err)}
						return
					}

					relPath, err := filepath.Rel(absWorkingDir, absFilePath)
					isOutsideWorkDir := err != nil || strings.HasPrefix(relPath, "..")
					isSkillFile := isInSkillsPath(absFilePath, skillsPaths)

					if sessionID == "" && (isOutsideWorkDir && !isSkillFile) {
						results[idx] = fileResult{filePath: filePath, err: fmt.Errorf("session ID is required for accessing files outside working directory")}
						return
					}

					if isOutsideWorkDir && !isSkillFile {
						mu.Lock()
						granted, permReqErr := permissions.Request(ctx,
							permission.CreatePermissionRequest{
								SessionID:   sessionID,
								Path:        absFilePath,
								ToolCallID:  call.ID,
								ToolName:    name,
								Action:      "read",
								Description: fmt.Sprintf("Read file outside working directory: %s", absFilePath),
								Params:      ViewPermissionsParams{FilePath: filePath, Offset: params.Offset, Limit: params.Limit},
							},
						)
						mu.Unlock()

						if permReqErr != nil {
							results[idx] = fileResult{filePath: filePath, err: permReqErr}
							return
						}
						if !granted {
							results[idx] = fileResult{filePath: filePath, err: permission.ErrorPermissionDenied}
							return
						}
					}

					fileInfo, err := os.Stat(fullPath)
					if err != nil {
						if os.IsNotExist(err) {
							results[idx] = fileResult{filePath: filePath, err: viewNotFoundError(workingDir, fullPath, filePath)}
							return
						}
						results[idx] = fileResult{filePath: filePath, err: fmt.Errorf("error accessing file %s: %w", filePath, err)}
						return
					}

					if fileInfo.IsDir() {
						results[idx] = fileResult{filePath: filePath, err: fmt.Errorf("Path is a directory, not a file: %s", filePath)}
						return
					}

					if !isSkillFile && fileInfo.Size() > MaxReadSize {
						results[idx] = fileResult{filePath: filePath, err: fmt.Errorf("File %s is too large (%d bytes). Maximum size is %d bytes", filePath, fileInfo.Size(), MaxReadSize)}
						return
					}

					limit := params.Limit
					if limit <= 0 {
						if isSkillFile {
							limit = 1000000
						} else {
							limit = DefaultReadLimit
						}
					}

					isSupportedImage, mimeType := getImageMimeType(fullPath)
					if isSupportedImage {
						if !GetSupportsImagesFromContext(ctx) {
							modelName := GetModelNameFromContext(ctx)
							results[idx] = fileResult{filePath: filePath, err: fmt.Errorf("This model (%s) does not support image data for file %s", modelName, filePath)}
							return
						}

						imageData, readErr := os.ReadFile(fullPath)
						if readErr != nil {
							results[idx] = fileResult{filePath: filePath, err: fmt.Errorf("error reading image file %s: %w", filePath, readErr)}
							return
						}

						encoded := base64.StdEncoding.EncodeToString(imageData)
						// For image, we can just return it if it's the only file requested.
						// If there are multiple, returning base64 in text might be huge.
						// We will just store it as output text, though for real image responses
						// we usually use fantasy.NewImageResponse. For multiple files, we fallback to text.
						if len(uniquePaths) == 1 {
							results[idx] = fileResult{filePath: filePath, output: "IMAGE:" + mimeType + ":" + encoded}
							return
						}
						results[idx] = fileResult{filePath: filePath, err: fmt.Errorf("Image file %s cannot be read in parallel with other files", filePath)}
						return
					}

					// Codex-compatible: Use readTextFileWithMode for indentation-aware reading
					content, hasMore, err := readTextFileWithMode(ctx, fullPath, params.Offset, limit, params.Mode, params.Indentation)
					if err != nil {
						results[idx] = fileResult{filePath: filePath, err: fmt.Errorf("error reading file %s: %w", filePath, err)}
						return
					}
					if !utf8.ValidString(content) {
						results[idx] = fileResult{filePath: filePath, err: fmt.Errorf("File content %s is not valid UTF-8", filePath)}
						return
					}

					mu.Lock()
					openInLSPs(ctx, lspManager, fullPath)
					waitForLSPDiagnostics(ctx, lspManager, fullPath, 300*time.Millisecond)
					mu.Unlock()

					output := fmt.Sprintf("<file path=\"%s\">\n", filePath)
					// Codex-compatible: 1-indexed line numbers
					output += addLineNumbers(content, params.Offset)
					if hasMore {
						output += fmt.Sprintf("\n\n(File has more lines. Use 'offset' parameter to read beyond line %d)", params.Offset+len(strings.Split(content, "\n")))
					}
					output += "\n</file>\n"
					output += detectLiteralEscapes(content)
					output += getDiagnostics(ctx, fullPath, lspManager)

					mu.Lock()
					filetracker.RecordRead(ctx, sessionID, fullPath)
					if editGuard != nil {
						// Record view only if we got the full file (params.Offset == 0 and limit > len(lines))
						// We don't have exactly len(lines) here without passing it out,
						// but if !hasMore and offset == 0, we've seen the whole file.
						editGuard.RecordView(sessionID, fullPath, params.Offset == 0 && !hasMore)
					}
					mu.Unlock()

					meta := ViewResponseMetadata{
						FilePath: fullPath,
						Content:  content,
					}
					if isSkillFile {
						if skill, err := skills.Parse(fullPath); err == nil {
							meta.ResourceType = ViewResourceSkill
							meta.ResourceName = skill.Name
							meta.ResourceDescription = skill.Description
						}
					}

					results[idx] = fileResult{filePath: filePath, output: output, meta: meta}
				}(i, p)
			}

			wg.Wait()

			// Check if single image response
			if len(uniquePaths) == 1 && results[0].output != "" && strings.HasPrefix(results[0].output, "IMAGE:") {
				parts := strings.SplitN(results[0].output, ":", 3)
				return fantasy.NewImageResponse([]byte(parts[2]), parts[1]), nil
			}

			var finalOutput strings.Builder
			var allErrors []string
			var finalMeta ViewResponseMetadata

			for _, res := range results {
				if res.err != nil {
					allErrors = append(allErrors, res.err.Error())
					continue
				}

				finalOutput.WriteString(res.output)
				finalOutput.WriteString("\n")

				fileMeta := ViewFileMetadata{
					FilePath:            res.meta.FilePath,
					Content:             res.meta.Content,
					ResourceType:        res.meta.ResourceType,
					ResourceName:        res.meta.ResourceName,
					ResourceDescription: res.meta.ResourceDescription,
				}
				finalMeta.Files = append(finalMeta.Files, fileMeta)

				// Keep legacy fields for backward compatibility with single-file consumers
				if finalMeta.FilePath == "" {
					finalMeta.FilePath = res.meta.FilePath
					finalMeta.Content = res.meta.Content
					finalMeta.ResourceType = res.meta.ResourceType
					finalMeta.ResourceName = res.meta.ResourceName
					finalMeta.ResourceDescription = res.meta.ResourceDescription
				}
			}

			if len(allErrors) > 0 {
				if len(allErrors) == len(uniquePaths) {
					// All failed
					return fantasy.NewTextErrorResponse(strings.Join(allErrors, "\n\n")), nil
				}
				// Partial success
				finalOutput.WriteString("\nErrors encountered:\n")
				finalOutput.WriteString(strings.Join(allErrors, "\n"))
			}

			return fantasy.WithResponseMetadata(
				fantasy.NewTextResponse(finalOutput.String()),
				finalMeta,
			), nil
		})
}

func addLineNumbers(content string, startLine int) string {
	if content == "" {
		return ""
	}

	lines := strings.Split(content, "\n")

	var result []string
	for i, line := range lines {
		line = strings.TrimSuffix(line, "\r")

		lineNum := i + startLine
		numStr := fmt.Sprintf("%d", lineNum)

		if len(numStr) >= 6 {
			result = append(result, fmt.Sprintf("%s|%s", numStr, line))
		} else {
			paddedNum := fmt.Sprintf("%6s", numStr)
			result = append(result, fmt.Sprintf("%s|%s", paddedNum, line))
		}
	}

	return strings.Join(result, "\n")
}

func readTextFile(ctx context.Context, filePath string, offset, limit int) (string, bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", false, err
	}
	defer file.Close()

	scanner := NewLineScanner(file)
	if offset > 0 {
		skipped := 0
		for skipped < offset && scanner.Scan() {
			if ctx.Err() != nil {
				return "", false, ctx.Err()
			}
			skipped++
		}
		if err = scanner.Err(); err != nil {
			return "", false, err
		}
	}

	// Pre-allocate slice with expected capacity.
	lines := make([]string, 0, limit)

	for len(lines) < limit && scanner.Scan() {
		if ctx.Err() != nil {
			return "", false, ctx.Err()
		}
		lineText := scanner.Text()
		if len(lineText) > MaxLineLength {
			lineText = lineText[:MaxLineLength] + "..."
		}
		lines = append(lines, lineText)
	}

	// Peek one more line only when we filled the limit.
	hasMore := len(lines) == limit && scanner.Scan()

	if err := scanner.Err(); err != nil {
		return "", false, err
	}

	return strings.Join(lines, "\n"), hasMore, nil
}

func resolveViewAliasPath(workingDir, filePath string) (string, string, bool) {
	normalized := filepath.Clean(filePath)
	alias := strings.Replace(normalized, filepath.FromSlash("internal/agent/tools/"), filepath.FromSlash("internal/agent/"), 1)
	if alias == normalized {
		return "", "", false
	}
	fullPath := filepathext.SmartJoin(workingDir, alias)
	if _, err := os.Stat(fullPath); err == nil {
		return alias, fullPath, true
	}
	return "", "", false
}

func prettyViewPath(workingDir, path string) string {
	absWorkingDir, err := filepath.Abs(workingDir)
	if err != nil {
		return path
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	relPath, err := filepath.Rel(absWorkingDir, absPath)
	if err == nil && relPath != "." && !strings.HasPrefix(relPath, "..") {
		return filepath.ToSlash(relPath)
	}
	return filepath.ToSlash(path)
}

func collectViewPaths(params ViewParams) []string {
	filePaths := make([]string, 0, len(params.FilePaths)+len(params.Paths)+len(params.Files)+2)
	filePaths = append(filePaths, params.FilePaths...)
	filePaths = append(filePaths, params.Paths...)
	filePaths = append(filePaths, params.Files...)
	if params.FilePath != "" {
		filePaths = append(filePaths, params.FilePath)
	}
	if params.Path != "" {
		filePaths = append(filePaths, params.Path)
	}
	return filePaths
}

func getImageMimeType(filePath string) (bool, string) {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".jpg", ".jpeg":
		return true, "image/jpeg"
	case ".png":
		return true, "image/png"
	case ".gif":
		return true, "image/gif"
	case ".webp":
		return true, "image/webp"
	default:
		return false, ""
	}
}

type LineScanner struct {
	scanner *bufio.Scanner
}

func NewLineScanner(r io.Reader) *LineScanner {
	scanner := bufio.NewScanner(r)
	// Increase buffer size to handle large lines (e.g., minified JSON, HTML)
	// Default is 64KB, set to 1MB
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	return &LineScanner{
		scanner: scanner,
	}
}

func (s *LineScanner) Scan() bool {
	return s.scanner.Scan()
}

func (s *LineScanner) Text() string {
	return s.scanner.Text()
}

func (s *LineScanner) Err() error {
	return s.scanner.Err()
}

// isInSkillsPath checks if filePath is within any of the configured skills
// directories. Returns true for files that can be read without permission
// prompts and without size limits.
//
// Note that symlinks are resolved to prevent path traversal attacks via
// symbolic links.
func isInSkillsPath(filePath string, skillsPaths []string) bool {
	if len(skillsPaths) == 0 {
		return false
	}

	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		return false
	}

	evalFilePath, err := filepath.EvalSymlinks(absFilePath)
	if err != nil {
		return false
	}

	for _, skillsPath := range skillsPaths {
		absSkillsPath, err := filepath.Abs(skillsPath)
		if err != nil {
			continue
		}

		evalSkillsPath, err := filepath.EvalSymlinks(absSkillsPath)
		if err != nil {
			continue
		}

		relPath, err := filepath.Rel(evalSkillsPath, evalFilePath)
		if err == nil && !strings.HasPrefix(relPath, "..") {
			return true
		}
	}

	return false
}

// =============================================================================
// Codex-Compatible Reading Features
// =============================================================================

// LineRecord represents a single line with metadata (Codex-compatible).
type LineRecord struct {
	Number  int    `json:"number"`  // 1-indexed line number
	Raw     string `json:"raw"`     // Raw line content
	Display string `json:"display"` // Tab-expanded display content
	Indent  int    `json:"indent"`  // Indentation level in spaces
}

// IsBlank returns true if the line is empty or whitespace-only.
func (lr *LineRecord) IsBlank() bool {
	return strings.TrimSpace(lr.Raw) == ""
}

// IsComment returns true if the line starts with a comment prefix.
func (lr *LineRecord) IsComment() bool {
	trimmed := strings.TrimSpace(lr.Raw)
	for _, prefix := range COMMENT_PREFIXES {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}

// expandTabs converts tabs to spaces using TAB_WIDTH (Codex-compatible).
func expandTabs(line string, tabWidth int) string {
	var result strings.Builder
	result.Grow(len(line) + tabWidth) // Pre-allocate for efficiency
	for _, r := range line {
		if r == '\t' {
			spaces := tabWidth - (result.Len() % tabWidth)
			for i := 0; i < spaces; i++ {
				result.WriteByte(' ')
			}
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// measureIndent returns the indentation level in spaces (Codex-compatible).
func measureIndent(line string) int {
	indent := 0
	for _, r := range line {
		if r == ' ' {
			indent++
		} else if r == '\t' {
			indent += TAB_WIDTH
		} else {
			break
		}
	}
	return indent
}

// formatLine formats a line for display with tab expansion and truncation.
func formatLine(bytes []byte) string {
	decoded := string(bytes)
	if len(decoded) > MAX_LINE_WIDTH {
		// Truncate at character boundary
		for i := MAX_LINE_WIDTH; i > 0; i-- {
			if utf8.RuneStart(decoded[i]) {
				return decoded[:i] + "..."
			}
		}
		return "..."
	}
	return expandTabs(decoded, TAB_WIDTH)
}

// readTextFileWithMode reads a file with the specified mode (Codex-compatible).
// Supports both slice mode and indentation-aware mode.
func readTextFileWithMode(ctx context.Context, filePath string, offset, limit int, mode string, indentation *IndentationMode) (string, bool, error) {
	// Convert 1-indexed offset to internal 1-indexed (Codex uses 1-indexed)
	if offset <= 0 {
		offset = 1 // Default to first line (Codex-compatible)
	}

	if mode == "indentation" && indentation != nil {
		// Indentation-aware reading (Codex feature)
		content, err := readIndentationBlock(ctx, filePath, offset, limit, indentation)
		if err != nil {
			return "", false, err
		}
		return content, false, nil
	}

	// Default slice mode
	content, hasMore, err := readTextFile(ctx, filePath, offset, limit)
	if err != nil {
		return "", false, err
	}

	// Apply tab expansion to output
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = expandTabs(line, TAB_WIDTH)
	}

	return strings.Join(lines, "\n"), hasMore, nil
}

// readIndentationBlock reads a file with indentation-aware block detection.
func readIndentationBlock(ctx context.Context, filePath string, offset, limit int, indentation *IndentationMode) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	defer file.Close()

	// Collect all lines with metadata
	var lines []LineRecord
	scanner := NewLineScanner(file)
	number := 0
	for scanner.Scan() {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		number++
		raw := scanner.Text()
		lines = append(lines, LineRecord{
			Number:  number,
			Raw:     raw,
			Display: formatLine([]byte(raw)),
			Indent:  measureIndent(raw),
		})
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	if len(lines) == 0 {
		return "", fmt.Errorf("file is empty")
	}

	// Determine anchor line (Codex-compatible: defaults to offset)
	anchorLine := offset
	if indentation.AnchorLine != nil && *indentation.AnchorLine > 0 {
		anchorLine = *indentation.AnchorLine
	}
	if anchorLine > len(lines) {
		return "", fmt.Errorf("offset/anchor_line exceeds file length (%d lines)", len(lines))
	}

	// Compute effective indents (propagate non-blank indents to blank lines)
	effectiveIndents := computeEffectiveIndents(lines)
	anchorIndex := anchorLine - 1
	anchorIndent := effectiveIndents[anchorIndex]

	// Determine max indentation depth to collect
	maxLevels := 0
	if indentation.MaxLevels != nil {
		maxLevels = *indentation.MaxLevels
	}
	minIndent := 0
	if maxLevels > 0 {
		minIndent = anchorIndent - (maxLevels * TAB_WIDTH)
	}

	// Determine limit
	finalLimit := limit
	if indentation.MaxLines != nil && *indentation.MaxLines > 0 {
		finalLimit = min(finalLimit, *indentation.MaxLines)
	}

	// Collect lines around anchor using two-pointer technique
	includeSiblings := false
	if indentation.IncludeSiblings != nil {
		includeSiblings = *indentation.IncludeSiblings
	}

	var out []LineRecord
	out = append(out, lines[anchorIndex])

	i := anchorIndex - 1 // up cursor
	j := anchorIndex + 1 // down cursor

	for len(out) < finalLimit && (i >= 0 || j < len(lines)) {
		progressed := false

		// Move up
		if i >= 0 && effectiveIndents[i] >= minIndent {
			if includeSiblings || effectiveIndents[i] > minIndent {
				out = append([]LineRecord{lines[i]}, out...)
				progressed = true
			}
			i--
		}

		// Move down
		if j < len(lines) && effectiveIndents[j] >= minIndent {
			if includeSiblings || effectiveIndents[j] > minIndent {
				out = append(out, lines[j])
				progressed = true
			}
			j++
		}

		if !progressed {
			break
		}
	}

	// Trim empty lines from start and end
	out = trimEmptyLines(out)

	// Format output with line numbers
	var result []string
	for _, line := range out {
		result = append(result, fmt.Sprintf("L%d: %s", line.Number, line.Display))
	}

	return strings.Join(result, "\n"), nil
}

// computeEffectiveIndents computes effective indentation for each line.
// Blank lines inherit the previous non-blank line's indentation.
func computeEffectiveIndents(lines []LineRecord) []int {
	effective := make([]int, len(lines))
	previousIndent := 0
	for i, line := range lines {
		if line.IsBlank() {
			effective[i] = previousIndent
		} else {
			previousIndent = line.Indent
			effective[i] = previousIndent
		}
	}
	return effective
}

// trimEmptyLines removes empty lines from the start and end of a slice.
func trimEmptyLines(lines []LineRecord) []LineRecord {
	if len(lines) == 0 {
		return lines
	}

	start := 0
	for start < len(lines) && lines[start].IsBlank() {
		start++
	}

	end := len(lines) - 1
	for end >= start && lines[end].IsBlank() {
		end--
	}

	if start > end {
		return []LineRecord{}
	}
	return lines[start : end+1]
}

// detectLiteralEscapes warns the agent if the file contains literal '\n' or '\t'
// which often leads to 'old_string' match failures.
func detectLiteralEscapes(content string) string {
	var warnings []string
	if strings.Contains(content, "\\n") {
		warnings = append(warnings, "This file contains literal '\\n' sequences. If you try to match these with a real newline in `old_string`, it will fail. Match the literal '\\n' exactly.")
	}
	if strings.Contains(content, "\\t") {
		warnings = append(warnings, "This file contains literal '\\t' sequences. If you try to match these with a real tab in `old_string`, it will fail. Match the literal '\\t' exactly.")
	}

	if len(warnings) > 0 {
		var sb strings.Builder
		sb.WriteString("\n<file_encoding_warning>\n")
		for _, w := range warnings {
			sb.WriteString("WARNING: " + w + "\n")
		}
		sb.WriteString("</file_encoding_warning>\n")
		return sb.String()
	}
	return ""
}
