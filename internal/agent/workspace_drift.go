package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/duggal1/Sapphire-cli/internal/filetracker"
)

const (
	maxWorkspaceDriftCandidates = 64
	maxWorkspaceDriftReported   = 8
)

func buildWorkspaceDriftPrompt(ctx context.Context, tracker filetracker.Service, sessionID, workingDir string) string {
	if tracker == nil || strings.TrimSpace(sessionID) == "" {
		return ""
	}

	readFiles, err := tracker.ListReadFiles(ctx, sessionID)
	if err != nil || len(readFiles) == 0 {
		return ""
	}
	if len(readFiles) > maxWorkspaceDriftCandidates {
		readFiles = readFiles[:maxWorkspaceDriftCandidates]
	}

	modified := make([]string, 0, len(readFiles))
	removed := make([]string, 0, len(readFiles))
	for _, path := range readFiles {
		if strings.TrimSpace(path) == "" {
			continue
		}

		lastRead := tracker.LastReadTime(ctx, sessionID, path)
		if lastRead.IsZero() {
			continue
		}

		info, statErr := os.Stat(path)
		if os.IsNotExist(statErr) {
			removed = append(removed, displayWorkspacePath(path, workingDir))
			continue
		}
		if statErr != nil || info.IsDir() {
			continue
		}

		modTime := info.ModTime().Truncate(time.Second)
		if modTime.After(lastRead) {
			modified = append(modified, displayWorkspacePath(path, workingDir))
		}
	}

	modified = uniqueNonEmptyStrings(modified)
	removed = uniqueNonEmptyStrings(removed)
	if len(modified) == 0 && len(removed) == 0 {
		return ""
	}
	if len(modified) > maxWorkspaceDriftReported {
		modified = modified[:maxWorkspaceDriftReported]
	}
	if len(removed) > maxWorkspaceDriftReported {
		removed = removed[:maxWorkspaceDriftReported]
	}

	lines := []string{
		"<workspace_drift>",
		"Previously read workspace files changed outside the agent. Re-read them before relying on earlier observations or editing around them.",
	}
	if len(modified) > 0 {
		lines = append(lines, "- modified: "+strings.Join(modified, ", "))
	}
	if len(removed) > 0 {
		lines = append(lines, "- removed: "+strings.Join(removed, ", "))
	}
	lines = append(lines, "</workspace_drift>")
	return strings.Join(lines, "\n")
}

func displayWorkspacePath(path, workingDir string) string {
	path = filepath.Clean(path)
	if strings.TrimSpace(workingDir) == "" {
		return filepath.ToSlash(path)
	}
	if rel, err := filepath.Rel(workingDir, path); err == nil && rel != "" && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(path)
}
