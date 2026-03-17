package memories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/sapphire/internal/db"
	"github.com/charmbracelet/sapphire/internal/message"
	"github.com/charmbracelet/sapphire/internal/session"
)

const (
	defaultPhase1Lease      = time.Hour
	defaultPhase2Lease      = time.Hour
	defaultPhase2RetryDelay = time.Hour
	defaultPhase1Limit      = 8
	retentionWindow         = 30 * 24 * time.Hour
)

type Service struct {
	q        *db.Queries
	rawDB    *sql.DB
	sessions session.Service
	messages message.Service
	root     string
	usage    *UsageTracker

	started sync.Map

	runnersMu sync.Mutex
	runners   PhaseRunners
}

type StartOptions struct {
	SessionID  string
	WorkingDir string
	IsSubAgent bool
}

func NewService(q *db.Queries, rawDB *sql.DB, sessions session.Service, messages message.Service, dataDir string) *Service {
	root := filepath.Join(dataDir, "memories")
	return &Service{
		q:        q,
		rawDB:    rawDB,
		sessions: sessions,
		messages: messages,
		root:     root,
		usage:    NewUsageTracker(),
	}
}

func (s *Service) Root() string {
	if s == nil {
		return ""
	}
	return s.root
}

func (s *Service) summaryPath() string {
	return filepath.Join(s.root, "memory_summary.md")
}

func (s *Service) registryPath() string {
	return filepath.Join(s.root, "MEMORY.md")
}

func (s *Service) rawMemoriesPath() string {
	return filepath.Join(s.root, "raw_memories.md")
}

func (s *Service) rolloutSummariesDir() string {
	return filepath.Join(s.root, "rollout_summaries")
}

func (s *Service) ensureDirs() error {
	if s == nil {
		return nil
	}
	for _, dir := range []string{s.root, s.rolloutSummariesDir()} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create memory directory %s: %w", dir, err)
		}
	}
	return nil
}

func (s *Service) Start(ctx context.Context, opts StartOptions) {
	if s == nil || opts.IsSubAgent || strings.TrimSpace(opts.SessionID) == "" {
		return
	}
	if _, loaded := s.started.LoadOrStore(opts.SessionID, struct{}{}); loaded {
		return
	}
	go func() {
		defer s.started.Delete(opts.SessionID)
		if err := s.runStartup(context.Background(), opts); err != nil {
			// Best-effort background memory path. Fail closed without blocking chat.
		}
	}()
}

func (s *Service) RuntimePrompt(ctx context.Context) string {
	if s == nil {
		return ""
	}
	summaryBytes, err := os.ReadFile(s.summaryPath())
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return ""
		}
		return ""
	}
	s.usage.Record("memory_summary", s.summaryPath())
	return BuildReadPathPrompt(s.root, strings.TrimSpace(string(summaryBytes)))
}

func (s *Service) SyncMaterializations(ctx context.Context) error {
	if s == nil {
		return nil
	}
	if err := s.ensureDirs(); err != nil {
		return err
	}
	materializations, err := s.q.ListMemoryMaterializations(ctx)
	if err != nil {
		return fmt.Errorf("list memory materializations for sync: %w", err)
	}
	return s.restoreMaterializations(materializations)
}

func (s *Service) ApplyStaleCorrection(ctx context.Context, canonicalKey, title, body, sessionID, rolloutSummaryFile string, citations []string) error {
	if s == nil {
		return nil
	}
	entry, err := s.q.UpsertMemoryRegistryEntry(ctx, db.UpsertMemoryRegistryEntryParams{
		ID:                 canonicalKey,
		CanonicalKey:       canonicalKey,
		Kind:               "memory",
		Title:              title,
		Body:               body,
		SourceSessionID:    nullableString(sessionID),
		RolloutSummaryFile: rolloutSummaryFile,
		Stale:              0,
	})
	if err != nil {
		return fmt.Errorf("upsert memory registry entry: %w", err)
	}
	if err := s.q.ReplaceMemoryRegistryCitations(ctx, entry.ID); err != nil {
		return fmt.Errorf("replace memory citations: %w", err)
	}
	for _, citation := range citations {
		if strings.TrimSpace(citation) == "" {
			continue
		}
		if err := s.q.InsertMemoryRegistryCitation(ctx, db.InsertMemoryRegistryCitationParams{
			RegistryEntryID: entry.ID,
			SessionID:       citation,
			CitationType:    "thread",
		}); err != nil {
			return fmt.Errorf("insert memory citation: %w", err)
		}
	}
	if strings.TrimSpace(sessionID) != "" {
		stage1, err := s.q.GetStage1OutputBySessionID(ctx, sessionID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("get stage1 output for stale correction: %w", err)
		}
		if err == nil {
			filename := strings.TrimSpace(rolloutSummaryFile)
			if filename == "" {
				filename = stage1.RolloutSummaryFile
			}
			if err := s.q.UpsertStage1Output(ctx, db.UpsertStage1OutputParams{
				SessionID:          sessionID,
				SourceUpdatedAt:    stage1.SourceUpdatedAt,
				RawMemory:          stage1.RawMemory,
				RolloutSummary:     body,
				RolloutSlug:        sanitizeSlug(title),
				RolloutSummaryFile: filename,
			}); err != nil {
				return fmt.Errorf("update stage1 output during stale correction: %w", err)
			}
		}
	}
	if err := s.q.MarkPhase2Dirty(ctx); err != nil {
		return fmt.Errorf("mark phase2 dirty: %w", err)
	}
	return s.runPhase2(ctx, sessionID)
}

func (s *Service) restoreMaterializations(materializations []db.MemoryMaterialization) error {
	if err := s.ensureDirs(); err != nil {
		return err
	}
	for _, path := range []string{s.registryPath(), s.summaryPath(), s.rawMemoriesPath()} {
		_ = os.Remove(path)
	}
	_ = os.RemoveAll(filepath.Join(s.root, "skills"))
	_ = os.RemoveAll(s.rolloutSummariesDir())
	if err := os.MkdirAll(s.rolloutSummariesDir(), 0o755); err != nil {
		return fmt.Errorf("recreate rollout summaries directory: %w", err)
	}

	for _, item := range materializations {
		target := filepath.Join(s.root, filepath.FromSlash(item.Path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("create materialization directory %s: %w", item.Path, err)
		}
		if err := os.WriteFile(target, []byte(strings.TrimSpace(item.Content)+"\n"), 0o644); err != nil {
			return fmt.Errorf("write materialization %s: %w", item.Path, err)
		}
	}
	return nil
}

func (s *Service) UsageSnapshot() UsageSnapshot {
	if s == nil {
		return UsageSnapshot{}
	}
	return s.usage.Snapshot()
}

func nullableString(v string) sql.NullString {
	return sql.NullString{String: v, Valid: strings.TrimSpace(v) != ""}
}

func nullableInt64(v int64) sql.NullInt64 {
	return sql.NullInt64{Int64: v, Valid: v != 0}
}

func nowUnix() int64 {
	return time.Now().Unix()
}

func sanitizeSlug(input string) string {
	input = strings.ToLower(strings.TrimSpace(input))
	if input == "" {
		return ""
	}
	re := regexp.MustCompile(`[^a-z0-9]+`)
	input = re.ReplaceAllString(input, "-")
	input = strings.Trim(input, "-")
	if len(input) > 48 {
		input = input[:48]
	}
	return input
}

func canonicalRolloutSummaryFilename(updatedAt int64, sessionID, slug string) string {
	ts := time.Unix(updatedAt, 0).UTC().Format("2006-01-02T15-04-05")
	hash := sessionID
	if len(hash) > 4 {
		hash = hash[:4]
	}
	stem := ts + "-" + hash
	if slug = sanitizeSlug(slug); slug != "" {
		stem += "-" + slug
	}
	return stem + ".md"
}

func topNonEmpty(lines []string, limit int) []string {
	out := make([]string, 0, min(limit, len(lines)))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func stableKeywords(values ...string) string {
	seen := map[string]struct{}{}
	parts := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(strings.ToLower(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		parts = append(parts, value)
	}
	slices.Sort(parts)
	return strings.Join(parts, ", ")
}
