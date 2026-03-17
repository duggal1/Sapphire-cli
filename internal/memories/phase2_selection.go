package memories

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/sapphire/internal/db"
)

type Phase2RemovedRef struct {
	SessionID          string
	SourceUpdatedAt    int64
	RolloutSlug        string
	RolloutSummaryFile string
}

type Phase2Selection struct {
	Selected         []db.MemoryStage1Output
	PreviousSelected []db.MemoryStage1Output
	Removed          []Phase2RemovedRef
	RetainedIDs      map[string]struct{}
}

func (s *Service) getPhase2Selection(ctx context.Context) (Phase2Selection, error) {
	cutoff := nowUnix() - int64(retentionWindow.Seconds())

	selected, err := s.q.ListEligibleStage1OutputsForPhase2(ctx, db.ListEligibleStage1OutputsForPhase2Params{
		Cutoff:     nullableInt64(cutoff),
		LimitCount: defaultPhase1Limit,
	})
	if err != nil {
		return Phase2Selection{}, fmt.Errorf("list eligible stage1 outputs: %w", err)
	}

	previous, err := s.q.ListPhase2BaselineOutputs(ctx)
	if err != nil {
		return Phase2Selection{}, fmt.Errorf("list phase2 baseline outputs: %w", err)
	}

	selectedOutputs := make([]db.MemoryStage1Output, 0, len(selected))
	currentBySession := make(map[string]db.MemoryStage1Output, len(selected))
	retained := make(map[string]struct{})
	for _, row := range selected {
		item := stage1OutputFromEligibleRow(row)
		selectedOutputs = append(selectedOutputs, item)
		currentBySession[item.SessionID] = item
		if item.SelectedForPhase2 != 0 && item.SelectedForPhase2SourceUpdatedAt.Valid && item.SelectedForPhase2SourceUpdatedAt.Int64 == item.SourceUpdatedAt {
			retained[item.SessionID] = struct{}{}
		}
	}

	previousOutputs := make([]db.MemoryStage1Output, 0, len(previous))
	removed := make([]Phase2RemovedRef, 0)
	for _, item := range previous {
		previousItem := stage1OutputFromBaselineRow(item)
		previousOutputs = append(previousOutputs, previousItem)
		current, ok := currentBySession[item.SessionID]
		if !ok || current.SourceUpdatedAt != item.SourceUpdatedAt {
			removed = append(removed, Phase2RemovedRef{
				SessionID:          item.SessionID,
				SourceUpdatedAt:    item.SourceUpdatedAt,
				RolloutSlug:        item.RolloutSlug,
				RolloutSummaryFile: item.RolloutSummaryFile,
			})
		}
	}

	return Phase2Selection{
		Selected:         selectedOutputs,
		PreviousSelected: previousOutputs,
		Removed:          removed,
		RetainedIDs:      retained,
	}, nil
}

func (s *Service) syncPhase2Artifacts(ctx context.Context, selection Phase2Selection) error {
	if err := s.ensureDirs(); err != nil {
		return err
	}

	artifactInputs := make([]db.MemoryStage1Output, 0, len(selection.Selected)+len(selection.PreviousSelected))
	seen := make(map[string]struct{})
	for _, item := range selection.Selected {
		key := fmt.Sprintf("%s:%d", item.SessionID, item.SourceUpdatedAt)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		artifactInputs = append(artifactInputs, item)
	}
	for _, item := range selection.PreviousSelected {
		key := fmt.Sprintf("%s:%d", item.SessionID, item.SourceUpdatedAt)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		artifactInputs = append(artifactInputs, item)
	}

	if err := s.pruneRolloutSummaries(artifactInputs); err != nil {
		return err
	}
	if err := s.rebuildRawMemories(artifactInputs); err != nil {
		return err
	}
	rawBytes, err := os.ReadFile(s.rawMemoriesPath())
	if err != nil {
		return fmt.Errorf("read raw memories for materialization: %w", err)
	}
	if _, err := s.q.UpsertMemoryMaterialization(ctx, db.UpsertMemoryMaterializationParams{
		Path:      "raw_memories.md",
		Kind:      "raw",
		Content:   strings.TrimSpace(string(rawBytes)),
		SessionID: nullableString(""),
	}); err != nil {
		return fmt.Errorf("upsert raw memories materialization: %w", err)
	}
	if len(artifactInputs) == 0 {
		for _, name := range []string{"MEMORY.md", "memory_summary.md"} {
			_ = os.Remove(filepath.Join(s.root, name))
		}
		_ = os.RemoveAll(filepath.Join(s.root, "skills"))
		return nil
	}
	for _, output := range artifactInputs {
		fullRolloutPath := filepath.Join(s.root, output.RolloutSummaryFile)
		if err := os.MkdirAll(filepath.Dir(fullRolloutPath), 0o755); err != nil {
			return fmt.Errorf("create rollout summary directory: %w", err)
		}
		if err := os.WriteFile(fullRolloutPath, []byte(strings.TrimSpace(output.RolloutSummary)+"\n"), 0o644); err != nil {
			return fmt.Errorf("write rollout summary %s: %w", fullRolloutPath, err)
		}
		if _, err := s.q.UpsertMemoryMaterialization(ctx, db.UpsertMemoryMaterializationParams{
			Path:      output.RolloutSummaryFile,
			Kind:      "rollout_summary",
			Content:   strings.TrimSpace(output.RolloutSummary),
			SessionID: nullableString(output.SessionID),
		}); err != nil {
			return fmt.Errorf("upsert rollout summary materialization: %w", err)
		}
	}
	return nil
}

func (s *Service) pruneRolloutSummaries(outputs []db.MemoryStage1Output) error {
	keep := make(map[string]struct{}, len(outputs))
	for _, output := range outputs {
		keep[filepath.Base(output.RolloutSummaryFile)] = struct{}{}
	}
	dir := s.rolloutSummariesDir()
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read rollout summaries directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if _, ok := keep[entry.Name()]; ok {
			continue
		}
		if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove outdated rollout summary %s: %w", entry.Name(), err)
		}
	}
	return nil
}

func (s *Service) rebuildRawMemories(outputs []db.MemoryStage1Output) error {
	var rawSB strings.Builder
	rawSB.WriteString("# Raw Memories\n\n")
	if len(outputs) == 0 {
		rawSB.WriteString("No raw memories yet.\n")
		return os.WriteFile(s.rawMemoriesPath(), []byte(rawSB.String()), 0o644)
	}
	rawSB.WriteString("Merged stage-1 raw memories (latest first):\n\n")
	for _, output := range outputs {
		rawSB.WriteString("## Thread `")
		rawSB.WriteString(output.SessionID)
		rawSB.WriteString("`\n")
		rawSB.WriteString("updated_at: ")
		rawSB.WriteString(fmt.Sprintf("%d", output.SourceUpdatedAt))
		rawSB.WriteString("\n")
		rawSB.WriteString("rollout_summary_file: ")
		rawSB.WriteString(output.RolloutSummaryFile)
		rawSB.WriteString("\n\n")
		rawSB.WriteString(strings.TrimSpace(output.RawMemory))
		rawSB.WriteString("\n\n")
	}
	return os.WriteFile(s.rawMemoriesPath(), []byte(rawSB.String()), 0o644)
}

func stage1OutputFromEligibleRow(row db.ListEligibleStage1OutputsForPhase2Row) db.MemoryStage1Output {
	return db.MemoryStage1Output{
		SessionID:                        row.SessionID,
		RawMemory:                        row.RawMemory,
		RolloutSummary:                   row.RolloutSummary,
		RolloutSlug:                      row.RolloutSlug,
		RolloutSummaryFile:               row.RolloutSummaryFile,
		UsedAt:                           row.UsedAt,
		UpdatedAt:                        row.UpdatedAt,
		CreatedAt:                        row.CreatedAt,
		SourceUpdatedAt:                  row.SourceUpdatedAt,
		GeneratedAt:                      row.GeneratedAt,
		UsageCount:                       row.UsageCount,
		LastUsage:                        row.LastUsage,
		SelectedForPhase2:                row.SelectedForPhase2,
		SelectedForPhase2SourceUpdatedAt: row.SelectedForPhase2SourceUpdatedAt,
	}
}

func stage1OutputFromBaselineRow(row db.ListPhase2BaselineOutputsRow) db.MemoryStage1Output {
	return db.MemoryStage1Output{
		SessionID:                        row.SessionID,
		RawMemory:                        row.RawMemory,
		RolloutSummary:                   row.RolloutSummary,
		RolloutSlug:                      row.RolloutSlug,
		RolloutSummaryFile:               row.RolloutSummaryFile,
		UsedAt:                           row.UsedAt,
		UpdatedAt:                        row.UpdatedAt,
		CreatedAt:                        row.CreatedAt,
		SourceUpdatedAt:                  row.SourceUpdatedAt,
		GeneratedAt:                      row.GeneratedAt,
		UsageCount:                       row.UsageCount,
		LastUsage:                        row.LastUsage,
		SelectedForPhase2:                row.SelectedForPhase2,
		SelectedForPhase2SourceUpdatedAt: row.SelectedForPhase2SourceUpdatedAt,
	}
}

func stage1OutputFromListRow(row db.ListStage1OutputsForPhase2Row) db.MemoryStage1Output {
	return db.MemoryStage1Output{
		SessionID:                        row.SessionID,
		RawMemory:                        row.RawMemory,
		RolloutSummary:                   row.RolloutSummary,
		RolloutSlug:                      row.RolloutSlug,
		RolloutSummaryFile:               row.RolloutSummaryFile,
		UsedAt:                           row.UsedAt,
		UpdatedAt:                        row.UpdatedAt,
		CreatedAt:                        row.CreatedAt,
		SourceUpdatedAt:                  row.SourceUpdatedAt,
		GeneratedAt:                      row.GeneratedAt,
		UsageCount:                       row.UsageCount,
		LastUsage:                        row.LastUsage,
		SelectedForPhase2:                row.SelectedForPhase2,
		SelectedForPhase2SourceUpdatedAt: row.SelectedForPhase2SourceUpdatedAt,
	}
}
