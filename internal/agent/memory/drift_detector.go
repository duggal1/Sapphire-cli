package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DriftReport represents findings from a memory vs reality check.
type DriftReport struct {
	StaleFiles   []string
	MissingFiles []string
	PromptHints  string
}

// DetectDrift compares a given memory summary/MD against the actual workDir state.
func DetectDrift(ctx context.Context, workDir string, currentMemory string) (DriftReport, error) {
	report := DriftReport{}
	
	// 1. Basic check: verify files mentioned in memory still exist.
	// This is a simplified regex-based extractor for demo; real logic would use a proper parser.
	files := extractFilesFromMemory(currentMemory)
	
	for _, f := range files {
		abs := f
		if !filepath.IsAbs(f) {
			abs = filepath.Join(workDir, f)
		}
		
		if _, err := os.Stat(abs); os.IsNotExist(err) {
			report.MissingFiles = append(report.MissingFiles, f)
		}
	}
	
	if len(report.MissingFiles) > 0 {
		report.PromptHints = fmt.Sprintf("WARNING: Memory refers to deleted files: %s. Update MEMORY.md.", strings.Join(report.MissingFiles, ", "))
	}
	
	return report, nil
}

func extractFilesFromMemory(content string) []string {
	// Dummy implementation for prompt-based extraction logic integration.
	// In production, this would scan MEMORY.md for the "Files Tracked" or similar headers.
	return nil
}
