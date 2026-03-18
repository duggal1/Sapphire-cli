package chat

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/sapphire/internal/fsext"
)

// formatRelativePath returns a repo-scoped relative path when possible.
// Falls back to home-shortened paths when relative resolution isn't safe.
func formatRelativePath(path string) string {
	if path == "" {
		return ""
	}
	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) {
		return filepath.ToSlash(clean)
	}
	if cwd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(cwd, clean); err == nil {
			rel = filepath.Clean(rel)
			if rel != "." && !strings.HasPrefix(rel, "..") {
				return filepath.ToSlash(rel)
			}
		}
	}
	return fsext.PrettyPath(clean)
}
