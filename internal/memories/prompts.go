package memories

import (
	_ "embed"
	"fmt"
	"strings"
)

//go:embed templates/compact/prompt.md
var compactPrompt string

//go:embed templates/compact/summary_prefix.md
var compactSummaryPrefix string

//go:embed templates/consolidation.md
var consolidationPrompt string

//go:embed templates/read_path.md
var readPathPrompt string

//go:embed templates/stage_one_input.md
var stageOneInputPrompt string

//go:embed templates/stage_one_system.md
var stageOneSystemPrompt string

func CompactPrompt() string {
	return compactPrompt
}

func CompactSummaryPrefix() string {
	return compactSummaryPrefix
}

func StageOneInputPrompt() string {
	return stageOneInputPrompt
}

func StageOneSystemPrompt() string {
	return stageOneSystemPrompt
}

func ConsolidationPrompt() string {
	return consolidationPrompt
}

func BuildReadPathPrompt(basePath, memorySummary string) string {
	replacer := strings.NewReplacer(
		"{{ base_path }}", basePath,
		"{{ memory_summary }}", memorySummary,
	)
	return replacer.Replace(readPathPrompt)
}

func BuildStageOneInputPrompt(rolloutPath, rolloutCwd, rolloutContents string) string {
	replacer := strings.NewReplacer(
		"{{ rollout_path }}", rolloutPath,
		"{{ rollout_cwd }}", rolloutCwd,
		"{{ rollout_contents }}", rolloutContents,
	)
	return replacer.Replace(stageOneInputPrompt)
}

func BuildConsolidationPrompt(memoryRoot string, selectionSummary string) string {
	replacer := strings.NewReplacer(
		"{{ memory_root }}", memoryRoot,
		"{{ phase2_input_selection }}", selectionSummary,
	)
	return replacer.Replace(consolidationPrompt)
}

type Phase2SelectionItem struct {
	SessionID          string
	RolloutSummaryFile string
	Status             string
}

type Phase2RemovedItem struct {
	SessionID          string
	RolloutSummaryFile string
}

func RenderPhase2InputSelection(selected []Phase2SelectionItem, removed []Phase2RemovedItem, retainedCount int) string {
	removedText := "- none"
	if len(removed) > 0 {
		lines := make([]string, 0, len(removed))
		for _, item := range removed {
			lines = append(lines, fmt.Sprintf("- thread_id=%s, rollout_summary_file=%s", item.SessionID, item.RolloutSummaryFile))
		}
		removedText = strings.Join(lines, "\n")
	}
	if len(selected) == 0 {
		return fmt.Sprintf("- selected inputs this run: 0\n- newly added since the last successful Phase 2 run: 0\n- retained from the last successful Phase 2 run: %d\n- removed from the last successful Phase 2 run: %d\n\nCurrent selected Phase 1 inputs:\n- none\n\nRemoved from the last successful Phase 2 selection:\n%s\n", retainedCount, len(removed), removedText)
	}

	lines := make([]string, 0, len(selected))
	for _, item := range selected {
		status := item.Status
		if status == "" {
			status = "added"
		}
		lines = append(lines, fmt.Sprintf("- [%s] thread_id=%s, rollout_summary_file=%s", status, item.SessionID, item.RolloutSummaryFile))
	}

	return fmt.Sprintf(
		"- selected inputs this run: %d\n- newly added since the last successful Phase 2 run: %d\n- retained from the last successful Phase 2 run: %d\n- removed from the last successful Phase 2 run: %d\n\nCurrent selected Phase 1 inputs:\n%s\n\nRemoved from the last successful Phase 2 selection:\n%s\n",
		len(selected),
		max(len(selected)-retainedCount, 0),
		retainedCount,
		len(removed),
		strings.Join(lines, "\n"),
		removedText,
	)
}
