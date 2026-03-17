package memories

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/sapphire/internal/db"
)

var (
	taskGroupHeadingPattern   = regexp.MustCompile(`(?m)^# Task Group:\s*(.+?)\s*$`)
	rolloutSummaryPathPattern = regexp.MustCompile(`(?m)^-\s+(rollout_summaries/[^\s]+\.md)`)
	threadIDReferencePattern  = regexp.MustCompile(`thread_id=([a-zA-Z0-9-]+)`)
)

func (s *Service) ingestConsolidatedArtifacts(ctx context.Context) error {
	registryBytes, err := os.ReadFile(s.registryPath())
	if err != nil {
		return fmt.Errorf("read MEMORY.md: %w", err)
	}
	summaryBytes, err := os.ReadFile(s.summaryPath())
	if err != nil {
		return fmt.Errorf("read memory_summary.md: %w", err)
	}

	registryDoc := strings.TrimSpace(string(registryBytes))
	summaryDoc := strings.TrimSpace(string(summaryBytes))

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

	entries, err := parseRegistryEntries(registryDoc)
	if err != nil {
		return err
	}
	for i, entry := range entries {
		id := entry.CanonicalKey
		if id == "" {
			id = fmt.Sprintf("memory-block-%d", i+1)
		}
		record, err := s.q.UpsertMemoryRegistryEntry(ctx, db.UpsertMemoryRegistryEntryParams{
			ID:                 id,
			CanonicalKey:       entry.CanonicalKey,
			Kind:               "task_group",
			Title:              entry.Title,
			Body:               entry.Body,
			SourceSessionID:    nullableString(""),
			RolloutSummaryFile: entry.RolloutSummaryFile,
			Stale:              0,
		})
		if err != nil {
			return fmt.Errorf("upsert parsed registry entry %s: %w", entry.Title, err)
		}
		if err := s.q.ReplaceMemoryRegistryCitations(ctx, record.ID); err != nil {
			return fmt.Errorf("replace parsed citations %s: %w", entry.Title, err)
		}
		for _, threadID := range entry.ThreadIDs {
			if err := s.q.InsertMemoryRegistryCitation(ctx, db.InsertMemoryRegistryCitationParams{
				RegistryEntryID: record.ID,
				SessionID:       threadID,
				CitationType:    "thread",
			}); err != nil {
				if strings.Contains(err.Error(), "FOREIGN KEY constraint failed") {
					continue
				}
				return fmt.Errorf("insert parsed citation %s: %w", entry.Title, err)
			}
		}
	}

	skillsRoot := filepath.Join(s.root, "skills")
	if err := filepath.Walk(skillsRoot, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(s.root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		_, err = s.q.UpsertMemoryMaterialization(ctx, db.UpsertMemoryMaterializationParams{
			Path:      rel,
			Kind:      "skill",
			Content:   strings.TrimSpace(string(data)),
			SessionID: nullableString(""),
		})
		return err
	}); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("walk skills materializations: %w", err)
	}

	return nil
}

type parsedRegistryEntry struct {
	CanonicalKey       string
	Title              string
	Body               string
	RolloutSummaryFile string
	ThreadIDs          []string
}

func parseRegistryEntries(content string) ([]parsedRegistryEntry, error) {
	matches := taskGroupHeadingPattern.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return nil, nil
	}
	entries := make([]parsedRegistryEntry, 0, len(matches))
	for i, match := range matches {
		start := match[0]
		end := len(content)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		title := strings.TrimSpace(content[match[2]:match[3]])
		body := strings.TrimSpace(content[start:end])
		rolloutSummaryFile := ""
		if m := rolloutSummaryPathPattern.FindStringSubmatch(body); len(m) > 1 {
			rolloutSummaryFile = strings.TrimSpace(m[1])
		}
		threadIDs := uniqueThreadIDs(body)
		entries = append(entries, parsedRegistryEntry{
			CanonicalKey:       sanitizeSlug(title),
			Title:              title,
			Body:               body,
			RolloutSummaryFile: rolloutSummaryFile,
			ThreadIDs:          threadIDs,
		})
	}
	return entries, nil
}

func uniqueThreadIDs(body string) []string {
	matches := threadIDReferencePattern.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	ids := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		id := strings.TrimSpace(match[1])
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}
