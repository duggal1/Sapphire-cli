package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

func recordDeterministicToolUsage(ctx context.Context, state *ToolUsageState, toolName string, input map[string]any) {
	if state == nil {
		return
	}
	canonical := canonicalToolNameForModePolicy(toolName)
	if canonical == "" {
		canonical = normalizeToolName(toolName)
	}
	if canonical == "" {
		return
	}

	state.RecordDeterministicToolCall(canonical)

	for _, path := range extractDeterministicReadPaths(canonical, input, ctx) {
		state.RecordDeterministicRead(path)
	}

	writePaths := extractWritePaths(canonical, input)
	if len(writePaths) == 0 {
		return
	}

	unread := make(map[string]struct{}, len(writePaths))
	for _, path := range unreadFilePaths(ctx, writePaths) {
		normalized, ok := normalizeDeterministicMetricPath(ctx, path)
		if !ok {
			continue
		}
		unread[normalized] = struct{}{}
	}

	createdPaths := classifyCreatedWritePaths(ctx, canonical, input)
	for _, rawPath := range writePaths {
		normalized, ok := normalizeDeterministicMetricPath(ctx, rawPath)
		if !ok {
			continue
		}
		_, blind := unread[normalized]
		_, created := createdPaths[normalized]
		state.RecordDeterministicWrite(normalized, blind, created)
	}
}

func extractDeterministicReadPaths(toolName string, input map[string]any, ctx context.Context) []string {
	switch toolName {
	case ViewToolName, SingleViewToolName, AgenticViewToolName:
		return normalizeDeterministicMetricPaths(ctx, extractViewPaths(input))
	default:
		return nil
	}
}

func classifyCreatedWritePaths(ctx context.Context, toolName string, input map[string]any) map[string]struct{} {
	created := map[string]struct{}{}
	switch toolName {
	case EditToolName, SingleEditToolName, AgenticEditToolName:
		var params MultiEditParams
		if err := decodeInto(input, &params); err == nil {
			for _, path := range extractEditCreatePaths(ctx, params) {
				created[path] = struct{}{}
			}
		}
	}

	for _, rawPath := range extractWritePaths(toolName, input) {
		normalized, ok := normalizeDeterministicMetricPath(ctx, rawPath)
		if !ok {
			continue
		}
		if _, alreadyMarked := created[normalized]; alreadyMarked {
			continue
		}
		if deterministicPathExists(normalized) {
			continue
		}
		created[normalized] = struct{}{}
	}
	return created
}

func extractEditCreatePaths(ctx context.Context, params MultiEditParams) []string {
	var candidates []string
	if len(params.FileEdits) > 0 {
		for _, fileEdit := range params.FileEdits {
			if strings.TrimSpace(fileEdit.FilePath) == "" {
				continue
			}
			if !multiEditBatchCreatesFile(fileEdit.Edits) {
				continue
			}
			candidates = append(candidates, fileEdit.FilePath)
		}
		return normalizeDeterministicMetricPaths(ctx, candidates)
	}

	filePath := strings.TrimSpace(firstNonEmptyString(params.FilePath, params.Path))
	if filePath == "" {
		return nil
	}
	if !multiEditBatchCreatesFile(params.Edits) && strings.TrimSpace(params.OldString) != "" {
		return nil
	}
	return normalizeDeterministicMetricPaths(ctx, []string{filePath})
}

func multiEditBatchCreatesFile(edits []MultiEditOperation) bool {
	if len(edits) == 0 {
		return false
	}
	first := edits[0]
	return strings.TrimSpace(first.OldString) == ""
}

func normalizeDeterministicMetricPaths(ctx context.Context, paths []string) []string {
	if len(paths) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(paths))
	for _, path := range paths {
		value, ok := normalizeDeterministicMetricPath(ctx, path)
		if !ok {
			continue
		}
		normalized = append(normalized, value)
	}
	return uniqueStrings(normalized)
}

func normalizeDeterministicMetricPath(ctx context.Context, path string) (string, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", false
	}
	if !filepath.IsAbs(path) {
		if workingDir := strings.TrimSpace(GetWorkingDirFromContext(ctx)); workingDir != "" {
			path = filepath.Join(workingDir, path)
		}
	}
	return normalizeArtifactVerificationPath(path)
}

func deterministicPathExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Stat(filepath.FromSlash(path))
	if err != nil {
		return false
	}
	return !info.IsDir()
}

