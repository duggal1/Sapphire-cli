package agent

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	durableMemorySummaryPromptCharLimit  = 12_000
	durableMemoryFallbackPromptCharLimit = 8_000
)

func renderDurableMemoryReadPrompt(workingDir string) string {
	workingDir = strings.TrimSpace(workingDir)
	if workingDir == "" {
		return ""
	}

	memoryRoot := filepath.Join(workingDir, memoryFolderName)
	summary := readDurableMemoryPromptFile(filepath.Join(memoryRoot, memorySummaryFile), durableMemorySummaryPromptCharLimit)
	if summary == "" {
		handbook := readDurableMemoryPromptFile(filepath.Join(memoryRoot, memoryMDFile), durableMemoryFallbackPromptCharLimit)
		if handbook == "" {
			return ""
		}
		summary = "memory_summary.md is missing or empty. Start from MEMORY.md until the summary is regenerated.\n\n" + handbook
	}

	rendered := string(memoryReadPrompt)
	rendered = strings.ReplaceAll(rendered, "{{ base_path }}", filepath.ToSlash(memoryFolderName))
	rendered = strings.ReplaceAll(rendered, "{{ memory_summary }}", summary)
	return rendered
}

func readDurableMemoryPromptFile(path string, limit int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return ""
	}
	if limit > 0 && len(text) > limit {
		text = strings.TrimSpace(text[:limit]) + "\n\n[truncated for prompt injection]"
	}
	return text
}
