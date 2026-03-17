package memories

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/sapphire/internal/db"
	"github.com/google/uuid"
)

func (s *Service) runPhase2(ctx context.Context, parentSessionID string) error {
	if err := s.q.EnsureGlobalPhase2Job(ctx); err != nil {
		return fmt.Errorf("ensure phase2 job: %w", err)
	}
	token := uuid.NewString()
	job, err := s.q.ClaimGlobalPhase2Job(ctx, db.ClaimGlobalPhase2JobParams{
		ClaimToken:     nullableString(token),
		LeaseExpiresAt: nullableInt64(nowUnix() + int64(defaultPhase2Lease.Seconds())),
		Now:            nowUnix(),
	})
	if err != nil {
		return nil
	}

	selection, err := s.getPhase2Selection(ctx)
	if err != nil {
		_, _ = s.q.FailGlobalPhase2Job(ctx, db.FailGlobalPhase2JobParams{
			RetryAfter: nowUnix() + int64(defaultPhase2RetryDelay.Seconds()),
			LastError:  err.Error(),
			ClaimToken: nullableString(token),
		})
		return fmt.Errorf("get phase2 input selection: %w", err)
	}

	if err := s.syncPhase2Artifacts(ctx, selection); err != nil {
		_, _ = s.q.FailGlobalPhase2Job(ctx, db.FailGlobalPhase2JobParams{
			RetryAfter: nowUnix() + int64(defaultPhase2RetryDelay.Seconds()),
			LastError:  err.Error(),
			ClaimToken: nullableString(token),
		})
		return fmt.Errorf("sync phase2 artifacts: %w", err)
	}
	completionWatermark := job.InputWatermark
	for _, output := range selection.Selected {
		if output.SourceUpdatedAt > completionWatermark {
			completionWatermark = output.SourceUpdatedAt
		}
	}
	if len(selection.Selected) > 0 {
		runner := s.getPhase2Runner()
		if runner == nil {
			_, _ = s.q.FailGlobalPhase2Job(ctx, db.FailGlobalPhase2JobParams{
				RetryAfter: nowUnix() + int64(defaultPhase2RetryDelay.Seconds()),
				LastError:  "phase2 runner not configured",
				ClaimToken: nullableString(token),
			})
			return fmt.Errorf("phase2 runner not configured")
		}

		promptSelection := make([]Phase2SelectionItem, 0, len(selection.Selected))
		for _, output := range selection.Selected {
			promptSelection = append(promptSelection, Phase2SelectionItem{
				SessionID:          output.SessionID,
				RolloutSummaryFile: output.RolloutSummaryFile,
				Status:             phase2SelectionStatus(output.SessionID, selection),
			})
		}
		promptRemoved := make([]Phase2RemovedItem, 0, len(selection.Removed))
		for _, removed := range selection.Removed {
			promptRemoved = append(promptRemoved, Phase2RemovedItem{
				SessionID:          removed.SessionID,
				RolloutSummaryFile: removed.RolloutSummaryFile,
			})
		}
		if err := s.runPhase2Consolidation(ctx, runner, Phase2Invocation{
			Prompt:          BuildConsolidationPrompt(s.root, RenderPhase2InputSelection(promptSelection, promptRemoved, len(selection.RetainedIDs))),
			Root:            s.root,
			ParentSessionID: parentSessionID,
		}, token); err != nil {
			_, _ = s.q.FailGlobalPhase2Job(ctx, db.FailGlobalPhase2JobParams{
				RetryAfter: nowUnix() + int64(defaultPhase2RetryDelay.Seconds()),
				LastError:  err.Error(),
				ClaimToken: nullableString(token),
			})
			return fmt.Errorf("run phase2 consolidation: %w", err)
		}
		if err := s.ingestConsolidatedArtifacts(ctx); err != nil {
			_, _ = s.q.FailGlobalPhase2Job(ctx, db.FailGlobalPhase2JobParams{
				RetryAfter: nowUnix() + int64(defaultPhase2RetryDelay.Seconds()),
				LastError:  err.Error(),
				ClaimToken: nullableString(token),
			})
			return fmt.Errorf("ingest phase2 artifacts: %w", err)
		}
	}

	if err := s.q.ClearPhase2BaselineSelection(ctx); err != nil {
		return fmt.Errorf("clear phase2 baseline selection: %w", err)
	}
	for _, output := range selection.Selected {
		if _, err := s.q.MarkStage1OutputSelectedForPhase2(ctx, db.MarkStage1OutputSelectedForPhase2Params{
			SessionID:       output.SessionID,
			SourceUpdatedAt: output.SourceUpdatedAt,
		}); err != nil {
			return fmt.Errorf("mark stage1 output selected for phase2: %w", err)
		}
	}
	if _, err := s.q.SucceedGlobalPhase2Job(ctx, db.SucceedGlobalPhase2JobParams{
		InputWatermark:      completionWatermark,
		LastOutputWatermark: completionWatermark,
		ClaimToken:          nullableString(token),
	}); err != nil {
		return fmt.Errorf("mark phase2 succeeded: %w", err)
	}
	return nil
}

func (s *Service) runPhase2Consolidation(ctx context.Context, runner func(context.Context, Phase2Invocation) error, invocation Phase2Invocation, token string) error {
	done := make(chan error, 1)
	go func() {
		done <- runner(ctx, invocation)
	}()

	ticker := time.NewTicker(90 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case err := <-done:
			return err
		case <-ticker.C:
			if _, err := s.q.HeartbeatGlobalPhase2Job(ctx, db.HeartbeatGlobalPhase2JobParams{
				LeaseExpiresAt: nullableInt64(nowUnix() + int64(defaultPhase2Lease.Seconds())),
				ClaimToken:     nullableString(token),
			}); err != nil {
				return fmt.Errorf("heartbeat phase2 job: %w", err)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func phase2SelectionStatus(sessionID string, selection Phase2Selection) string {
	if _, ok := selection.RetainedIDs[sessionID]; ok {
		return "retained"
	}
	return "added"
}

func (s *Service) materializeOutputs(ctx context.Context, outputs []db.MemoryStage1Output) error {
	if err := s.ensureDirs(); err != nil {
		return err
	}
	existingEntries, err := s.q.ListMemoryRegistryEntries(ctx)
	if err != nil {
		return fmt.Errorf("list existing registry entries: %w", err)
	}
	existingByKey := make(map[string]db.MemoryRegistryEntry, len(existingEntries))
	for _, entry := range existingEntries {
		existingByKey[entry.CanonicalKey] = entry
	}

	var rawSB strings.Builder

	for _, output := range outputs {
		title := strings.TrimSpace(existingByKey[output.SessionID].Title)
		if title == "" {
			title = output.RolloutSlug
			if title == "" {
				title = output.SessionID
			}
		}
		rawSB.WriteString("## Thread ")
		rawSB.WriteString(output.SessionID)
		rawSB.WriteString("\n\n")
		rawSB.WriteString(strings.TrimSpace(output.RawMemory))
		rawSB.WriteString("\n\n")

		registryEntry, err := s.q.UpsertMemoryRegistryEntry(ctx, db.UpsertMemoryRegistryEntryParams{
			ID:                 output.SessionID,
			CanonicalKey:       output.SessionID,
			Kind:               "task_group",
			Title:              title,
			Body:               strings.TrimSpace(output.RolloutSummary),
			SourceSessionID:    nullableString(output.SessionID),
			RolloutSummaryFile: output.RolloutSummaryFile,
			Stale:              0,
		})
		if err != nil {
			return fmt.Errorf("upsert registry entry for %s: %w", output.SessionID, err)
		}
		if err := s.q.ReplaceMemoryRegistryCitations(ctx, registryEntry.ID); err != nil {
			return fmt.Errorf("replace citations for %s: %w", output.SessionID, err)
		}
		if err := s.q.InsertMemoryRegistryCitation(ctx, db.InsertMemoryRegistryCitationParams{
			RegistryEntryID: registryEntry.ID,
			SessionID:       output.SessionID,
			CitationType:    "thread",
		}); err != nil {
			return fmt.Errorf("insert citation for %s: %w", output.SessionID, err)
		}

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

	entries, err := s.q.ListMemoryRegistryEntries(ctx)
	if err != nil {
		return fmt.Errorf("list registry entries for materialization: %w", err)
	}

	registryDoc, summaryDoc := buildRegistryMaterializations(entries)

	if err := os.WriteFile(s.rawMemoriesPath(), []byte(strings.TrimSpace(rawSB.String())+"\n"), 0o644); err != nil {
		return fmt.Errorf("write raw memories: %w", err)
	}
	if err := os.WriteFile(s.registryPath(), []byte(registryDoc+"\n"), 0o644); err != nil {
		return fmt.Errorf("write MEMORY.md: %w", err)
	}
	if err := os.WriteFile(s.summaryPath(), []byte(summaryDoc+"\n"), 0o644); err != nil {
		return fmt.Errorf("write memory summary: %w", err)
	}

	if _, err := s.q.UpsertMemoryMaterialization(ctx, db.UpsertMemoryMaterializationParams{
		Path:      "raw_memories.md",
		Kind:      "raw",
		Content:   strings.TrimSpace(rawSB.String()),
		SessionID: nullableString(""),
	}); err != nil {
		return fmt.Errorf("upsert raw memory materialization: %w", err)
	}
	if _, err := s.q.UpsertMemoryMaterialization(ctx, db.UpsertMemoryMaterializationParams{
		Path:      "MEMORY.md",
		Kind:      "registry",
		Content:   registryDoc,
		SessionID: nullableString(""),
	}); err != nil {
		return fmt.Errorf("upsert registry materialization: %w", err)
	}
	if _, err := s.q.UpsertMemoryMaterialization(ctx, db.UpsertMemoryMaterializationParams{
		Path:      "memory_summary.md",
		Kind:      "summary",
		Content:   summaryDoc,
		SessionID: nullableString(""),
	}); err != nil {
		return fmt.Errorf("upsert summary materialization: %w", err)
	}
	return nil
}

func buildRegistryMaterializations(entries []db.MemoryRegistryEntry) (string, string) {
	var registrySB strings.Builder
	var summarySB strings.Builder

	summarySB.WriteString("# Memory Summary\n\n")
	summarySB.WriteString("Generated from canonical SQL-backed memory artifacts.\n\n")

	if len(entries) == 0 {
		summarySB.WriteString("- No durable memory has been consolidated yet.\n")
		registrySB.WriteString("# Task Group: Empty memory registry\n\nscope: No durable memories consolidated yet.\napplies_to: cwd=global; reuse_rule=wait for future consolidated runs\n")
		return strings.TrimSpace(registrySB.String()), strings.TrimSpace(summarySB.String())
	}

	for _, entry := range entries {
		title := strings.TrimSpace(entry.Title)
		if title == "" {
			title = entry.CanonicalKey
		}

		registrySB.WriteString("# Task Group: ")
		registrySB.WriteString(title)
		registrySB.WriteString("\n\n")
		registrySB.WriteString("scope: Consolidated durable memory for this session.\n")
		registrySB.WriteString("applies_to: cwd=session-scoped; reuse_rule=reuse when the same repo/workflow context clearly matches\n\n")
		registrySB.WriteString("## Task 1: Consolidated rollout memory\n\n")
		if strings.TrimSpace(entry.RolloutSummaryFile) != "" {
			registrySB.WriteString("### rollout_summary_files\n\n")
			registrySB.WriteString("- ")
			registrySB.WriteString(entry.RolloutSummaryFile)
			if entry.SourceSessionID.Valid {
				registrySB.WriteString(" (thread_id=")
				registrySB.WriteString(entry.SourceSessionID.String)
				registrySB.WriteString(")")
			}
			registrySB.WriteString("\n\n")
		}
		registrySB.WriteString("### keywords\n\n")
		registrySB.WriteString("- ")
		registrySB.WriteString(stableKeywords(entry.CanonicalKey, title))
		registrySB.WriteString("\n\n")
		registrySB.WriteString("## Reusable knowledge\n\n")
		registrySB.WriteString("- ")
		registrySB.WriteString(strings.ReplaceAll(strings.TrimSpace(entry.Body), "\n", "\n- "))
		registrySB.WriteString("\n\n")

		summarySB.WriteString("- ")
		summarySB.WriteString(title)
		if strings.TrimSpace(entry.RolloutSummaryFile) != "" {
			summarySB.WriteString(": see ")
			summarySB.WriteString(entry.RolloutSummaryFile)
		}
		summarySB.WriteString("\n")
	}

	return strings.TrimSpace(registrySB.String()), strings.TrimSpace(summarySB.String())
}
