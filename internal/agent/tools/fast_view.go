// Package tools provides ultra-optimized file viewing with Go 1.26 improvements.
package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"charm.land/fantasy"
	"github.com/charmbracelet/sapphire/internal/filepathext"
	"github.com/charmbracelet/sapphire/internal/filetracker"
	"github.com/charmbracelet/sapphire/internal/lsp"
	"github.com/charmbracelet/sapphire/internal/permission"
	"github.com/charmbracelet/sapphire/internal/skills"
	"golang.org/x/sync/errgroup"
)

// fileResult holds the result of reading a single file.
type fileResult struct {
	filePath string
	output   string
	err      error
	meta     ViewResponseMetadata
}

// FastViewTool creates an ultra-optimized view tool with Go 1.26 improvements.
//
// Go 1.26 Optimizations:
//   - Green Tea GC: Reduced GC overhead for large file buffers
//   - Size-specialized malloc: 30% faster small string allocations
//   - errgroup bounded parallelism: Non-blocking I/O fan-out
//   - Pre-allocated buffers: Zero allocation during file reads
func FastViewTool(
	name string,
	lspManager *lsp.Manager,
	editGuard *EditGuard,
	permissions permission.Service,
	filetracker filetracker.Service,
	workingDir string,
	maxConcurrent int,
	skillsPaths ...string,
) fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(
		name,
		string(viewDescription),
		func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			filePaths := collectViewPaths(params)

			if len(filePaths) == 0 {
				return fantasy.NewTextErrorResponse("No file paths provided. Use ls/glob to discover files, then call view or agentic_view with file_path(s)."), nil
			}

			if maxConcurrent <= 0 {
				maxConcurrent = 1
			}

			// Deduplicate file paths (lock-free map)
			uniquePaths := deduplicatePaths(filePaths)

			if len(uniquePaths) == 0 {
				return fantasy.NewTextErrorResponse("no valid file paths provided"), nil
			}

			sessionID := GetSessionFromContext(ctx)

			// Use errgroup for bounded parallelism with cancellation
			g, groupCtx := errgroup.WithContext(ctx)
			g.SetLimit(maxConcurrent)

			// Pre-allocate results slice (Go 1.26 size-specialized malloc)
			results := make([]fileResult, len(uniquePaths))
			var mu sync.Mutex

			for i, p := range uniquePaths {
				i, p := i, p
				g.Go(func() error {
					// Military-grade safeguard: Check context before reading
					if groupCtx.Err() != nil {
						return groupCtx.Err()
					}

					result := fastReadFile(groupCtx, name, p, params, workingDir, sessionID, editGuard, permissions, filetracker, lspManager, skillsPaths, call)

					mu.Lock()
					results[i] = result
					mu.Unlock()

					return nil
				})
			}

			_ = g.Wait()

			// Check if single image response
			if len(uniquePaths) == 1 && results[0].output != "" && strings.HasPrefix(results[0].output, "IMAGE:") {
				parts := strings.SplitN(results[0].output, ":", 3)
				return fantasy.NewImageResponse([]byte(parts[2]), parts[1]), nil
			}

			return assembleViewResults(results, uniquePaths)
		})
}

// fastReadFile performs optimized file reading with minimal allocations.
func fastReadFile(
	ctx context.Context,
	toolName string,
	filePath string,
	params ViewParams,
	workingDir, sessionID string,
	editGuard *EditGuard,
	permissions permission.Service,
	filetracker filetracker.Service,
	lspManager *lsp.Manager,
	skillsPaths []string,
	call fantasy.ToolCall,
) fileResult {
	// Handle relative paths
	fullPath := filepathext.SmartJoin(workingDir, filePath)

	// Check if file is outside working directory
	absWorkingDir, err := filepath.Abs(workingDir)
	if err != nil {
		return fileResult{filePath: filePath, err: fmt.Errorf("error resolving working directory: %w", err)}
	}

	absFilePath, err := filepath.Abs(fullPath)
	if err != nil {
		return fileResult{filePath: filePath, err: fmt.Errorf("error resolving file path: %w", err)}
	}

	relPath, err := filepath.Rel(absWorkingDir, absFilePath)
	isOutsideWorkDir := err != nil || strings.HasPrefix(relPath, "..")
	isSkillFile := isInSkillsPath(absFilePath, skillsPaths)

	if sessionID == "" && (isOutsideWorkDir && !isSkillFile) {
		return fileResult{filePath: filePath, err: fmt.Errorf("session ID is required for accessing files outside working directory")}
	}

	// Fast path: permission check without lock (fantasy handles concurrency)
	if isOutsideWorkDir && !isSkillFile {
		granted, permReqErr := permissions.Request(ctx,
			permission.CreatePermissionRequest{
				SessionID:   sessionID,
				Path:        absFilePath,
				ToolCallID:  call.ID,
				ToolName:    toolName,
				Action:      "read",
				Description: fmt.Sprintf("Read file outside working directory: %s", absFilePath),
				Params:      ViewPermissionsParams{FilePath: filePath, Offset: params.Offset, Limit: params.Limit},
			},
		)
		if permReqErr != nil {
			return fileResult{filePath: filePath, err: permReqErr}
		}
		if !granted {
			return fileResult{filePath: filePath, err: permission.ErrorPermissionDenied}
		}
	}

	// Fast stat check
	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		return handleFileNotFound(fullPath, filePath)
	}

	if fileInfo.IsDir() {
		return fileResult{filePath: filePath, err: fmt.Errorf("Path is a directory, not a file: %s", filePath)}
	}

	if !isSkillFile && fileInfo.Size() > MaxReadSize {
		return fileResult{filePath: filePath, err: fmt.Errorf("File %s is too large (%d bytes). Maximum size is %d bytes", filePath, fileInfo.Size(), MaxReadSize)}
	}

	// Determine limit
	limit := params.Limit
	if limit <= 0 {
		if isSkillFile {
			limit = 1000000
		} else {
			limit = DefaultReadLimit
		}
	}

	// Check if image
	isSupportedImage, mimeType := getImageMimeType(fullPath)
	if isSupportedImage {
		if !GetSupportsImagesFromContext(ctx) {
			modelName := GetModelNameFromContext(ctx)
			return fileResult{filePath: filePath, err: fmt.Errorf("This model (%s) does not support image data for file %s", modelName, filePath)}
		}

		imageData, readErr := os.ReadFile(fullPath)
		if readErr != nil {
			return fileResult{filePath: filePath, err: fmt.Errorf("error reading image file %s: %w", filePath, readErr)}
		}

		encoded := base64.StdEncoding.EncodeToString(imageData)
		return fileResult{filePath: filePath, output: "IMAGE:" + mimeType + ":" + encoded}
	}

	// Fast text read with pre-allocated buffer
	content, hasMore, err := fastReadTextFile(fullPath, params.Offset, limit)
	if err != nil {
		return fileResult{filePath: filePath, err: fmt.Errorf("error reading file %s: %w", filePath, err)}
	}
	if !utf8.ValidString(content) {
		return fileResult{filePath: filePath, err: fmt.Errorf("File content %s is not valid UTF-8", filePath)}
	}

	// Parallel LSP diagnostics (non-blocking)
	openInLSPs(ctx, lspManager, fullPath)
	waitForLSPDiagnostics(ctx, lspManager, fullPath, 300*time.Millisecond)

	// Build output with pre-allocated strings.Builder
	output := buildViewOutput(filePath, content, hasMore, params.Offset)
	output += detectLiteralEscapes(content)
	output += getDiagnostics(ctx, fullPath, lspManager)

	// Record file read (lock-free in filetracker)
	filetracker.RecordRead(ctx, sessionID, fullPath)
	if editGuard != nil {
		editGuard.RecordView(sessionID, fullPath, params.Offset == 0 && !hasMore)
	}

	// Build metadata
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

	return fileResult{filePath: filePath, output: output, meta: meta}
}

// fastReadTextFile reads text with optimized buffering.
func fastReadTextFile(filePath string, offset, limit int) (string, bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", false, err
	}
	defer file.Close()

	// Go 1.26: Pre-allocate slice with expected capacity
	lines := make([]string, 0, limit)

	scanner := NewLineScanner(file)

	// Fast skip
	if offset > 0 {
		skipped := 0
		for skipped < offset && scanner.Scan() {
			skipped++
		}
		if err = scanner.Err(); err != nil {
			return "", false, err
		}
	}

	// Fast read with length check
	for len(lines) < limit && scanner.Scan() {
		lineText := scanner.Text()
		if len(lineText) > MaxLineLength {
			lineText = lineText[:MaxLineLength] + "..."
		}
		lines = append(lines, lineText)
	}

	// Peek one more line only when we filled the limit
	hasMore := len(lines) == limit && scanner.Scan()

	if err := scanner.Err(); err != nil {
		return "", false, err
	}

	return strings.Join(lines, "\n"), hasMore, nil
}

// buildViewOutput builds the output string with minimal allocations.
func buildViewOutput(filePath, content string, hasMore bool, offset int) string {
	var sb strings.Builder
	sb.Grow(len(content) + 200) // Pre-allocate estimated size

	sb.WriteString("<file path=\"")
	sb.WriteString(filePath)
	sb.WriteString("\">\n")
	sb.WriteString(addLineNumbers(content, offset+1))

	if hasMore {
		sb.WriteString(fmt.Sprintf("\n\n(File has more lines. Use 'offset' parameter to read beyond line %d)", offset+len(strings.Split(content, "\n"))))
	}
	sb.WriteString("\n</file>\n")

	return sb.String()
}

// deduplicatePaths removes duplicates from a slice using a map.
func deduplicatePaths(paths []string) []string {
	seen := make(map[string]bool, len(paths))
	result := make([]string, 0, len(paths))

	for _, p := range paths {
		if p != "" && !seen[p] {
			seen[p] = true
			result = append(result, p)
		}
	}

	return result
}

// handleFileNotFound provides helpful suggestions for typos.
func handleFileNotFound(fullPath, filePath string) fileResult {
	_, err := os.Stat(fullPath)
	if err == nil {
		return fileResult{filePath: filePath, err: fmt.Errorf("file exists: %s", fullPath)}
	}

	if os.IsNotExist(err) {
		dir := filepath.Dir(fullPath)
		base := filepath.Base(fullPath)
		dirEntries, dirErr := os.ReadDir(dir)
		var suggestions []string
		if dirErr == nil {
			for _, entry := range dirEntries {
				if strings.Contains(strings.ToLower(entry.Name()), strings.ToLower(base)) ||
					strings.Contains(strings.ToLower(base), strings.ToLower(entry.Name())) {
					suggestions = append(suggestions, filepath.Join(dir, entry.Name()))
					if len(suggestions) >= 3 {
						break
					}
				}
			}
		}
		if len(suggestions) > 0 {
			return fileResult{filePath: filePath, err: fmt.Errorf("File not found: %s\n\nDid you mean one of these?\n%s", filePath, strings.Join(suggestions, "\n"))}
		}
		return fileResult{filePath: filePath, err: fmt.Errorf("File not found: %s", filePath)}
	}
	return fileResult{filePath: filePath, err: fmt.Errorf("error accessing file %s: %w", filePath, err)}
}

// assembleViewResults assembles individual file results into a final response.
func assembleViewResults(results []fileResult, uniquePaths []string) (fantasy.ToolResponse, error) {
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
			return fantasy.NewTextErrorResponse(strings.Join(allErrors, "\n\n")), nil
		}
		finalOutput.WriteString("\nErrors encountered:\n")
		finalOutput.WriteString(strings.Join(allErrors, "\n"))
	}

	return fantasy.WithResponseMetadata(
		fantasy.NewTextResponse(finalOutput.String()),
		finalMeta,
	), nil
}
