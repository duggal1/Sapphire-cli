package codeindex

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Service struct {
	root       string
	store      *store
	embedder   *embedder
	initErr    error
	mu         sync.Mutex
	lastStatus Progress
}

func New(ctx context.Context, cfg Config) (*Service, error) {
	if cfg.WorkspaceRoot == "" {
		return nil, fmt.Errorf("code index: workspace root is required")
	}
	if cfg.Model == "" {
		cfg.Model = DefaultEmbeddingModel
	}
	if cfg.Dimensions <= 0 {
		cfg.Dimensions = DefaultEmbeddingDimensions
	}
	root, err := filepath.Abs(cfg.WorkspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("code index: resolve workspace root: %w", err)
	}
	_ = removeLegacySQLiteIndex(cfg.DataDir)
	store, err := openStore(cfg.DataDir, root, cfg.Dimensions, cfg.QdrantURL)
	if err != nil {
		return nil, err
	}
	service := &Service{
		root:     root,
		store:    store,
	}
	embedder, err := newEmbedder(cfg.APIKey, cfg.Model, cfg.Dimensions)
	if err != nil {
		service.initErr = err
	} else {
		service.embedder = embedder
	}
	stats, _ := store.Stats(ctx)
	service.lastStatus = Progress{
		Workspace: root,
		Finished:  stats.ChunkCount > 0,
		Stats:     stats,
	}
	return service, nil
}

func (s *Service) Close() error {
	if s == nil {
		return nil
	}
	return s.store.Close()
}

func (s *Service) EnsureReady(ctx context.Context) (Stats, error) {
	return s.index(ctx, false)
}

func (s *Service) Refresh(ctx context.Context) (Stats, error) {
	return s.index(ctx, true)
}

func (s *Service) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if s.initErr != nil {
		return nil, s.initErr
	}
	stats, err := s.store.Stats(ctx)
	if err != nil {
		return nil, err
	}
	if stats.ChunkCount == 0 {
		if _, err := s.EnsureReady(ctx); err != nil {
			return nil, err
		}
	}
	queryVector, err := s.embedder.EmbedQuery(ctx, query)
	if err != nil {
		return nil, err
	}
	return s.store.Search(ctx, query, queryVector, limit)
}

func (s *Service) Stats(ctx context.Context) (Stats, error) {
	return s.store.Stats(ctx)
}

func (s *Service) index(ctx context.Context, force bool) (Stats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.initErr != nil {
		return Stats{}, s.initErr
	}

	startedAt := time.Now()
	progress := Progress{
		Workspace: s.root,
		Phase:     "discovering",
		Message:   "Discovering indexable files",
		Active:    true,
		StartedAt: startedAt,
		UpdatedAt: startedAt,
	}
	s.publish(progress, true)

	if force {
		if err := s.store.Clear(ctx); err != nil {
			progress.Active = false
			progress.Finished = true
			progress.Error = err.Error()
			progress.Message = "Failed to clear previous index"
			progress.UpdatedAt = time.Now()
			s.publish(progress, false)
			return Stats{}, err
		}
	}

	files, err := discoverFiles(s.root)
	if err != nil {
		progress.Active = false
		progress.Finished = true
		progress.Error = err.Error()
		progress.Message = "File discovery failed"
		progress.UpdatedAt = time.Now()
		s.publish(progress, false)
		return Stats{}, err
	}
	progress.FilesDiscovered = len(files)
	progress.Message = "Preparing changed chunks"
	progress.UpdatedAt = time.Now()
	s.publish(progress, false)

	storedFiles, err := s.store.ListFiles(ctx)
	if err != nil {
		return Stats{}, err
	}

	currentPaths := make(map[string]struct{}, len(files))
	toIndex := make([]indexedFile, 0, len(files))
	totalChunks := 0
	for i, file := range files {
		currentPaths[file.RelativePath] = struct{}{}
		if !force {
			if existing, ok := storedFiles[file.RelativePath]; ok && existing.ContentHash == file.ContentHash {
				progress.FilesProcessed = i + 1
				progress.Percent = computePercent(progress.FilesProcessed, len(files))
				progress.UpdatedAt = time.Now()
				s.publish(progress, false)
				continue
			}
		}
		indexed := buildIndexedFile(file)
		totalChunks += len(indexed.Chunks)
		toIndex = append(toIndex, indexed)
		progress.FilesProcessed = i + 1
		progress.ChunksTotal = totalChunks
		progress.Percent = computePercent(progress.FilesProcessed, len(files))
		progress.UpdatedAt = time.Now()
		s.publish(progress, false)
	}

	for path := range storedFiles {
		if _, ok := currentPaths[path]; !ok {
			if err := s.store.DeleteFile(ctx, path); err != nil {
				return Stats{}, err
			}
		}
	}

	if len(toIndex) == 0 {
		stats, err := s.store.Stats(ctx)
		if err != nil {
			return Stats{}, err
		}
		progress.Active = false
		progress.Finished = true
		progress.Phase = "ready"
		progress.Message = "Codebase index is up to date"
		progress.Percent = 1
		progress.UpdatedAt = time.Now()
		progress.Stats = stats
		s.publish(progress, false)
		return stats, nil
	}

	progress.Phase = "embedding"
	progress.Message = "Embedding code chunks"
	progress.FilesIndexed = 0
	progress.FilesProcessed = 0
	progress.ChunksTotal = totalChunks
	progress.Percent = 0
	progress.UpdatedAt = time.Now()
	s.publish(progress, false)

	embeddedChunks := 0
	for fileIdx := range toIndex {
		file := &toIndex[fileIdx]
		texts := make([]string, 0, len(file.Chunks))
		chunkIndexes := make([]int, 0, len(file.Chunks))
		for idx := range file.Chunks {
			texts = append(texts, file.Chunks[idx].SearchText)
			chunkIndexes = append(chunkIndexes, idx)
			if len(texts) == maxBatchEmbeds {
				if err := s.embedChunkBatch(ctx, file, texts, chunkIndexes); err != nil {
					return Stats{}, err
				}
				embeddedChunks += len(texts)
				progress.ChunksEmbedded = embeddedChunks
				progress.Percent = computePercent(embeddedChunks, totalChunks)
				progress.UpdatedAt = time.Now()
				s.publish(progress, false)
				texts = texts[:0]
				chunkIndexes = chunkIndexes[:0]
			}
		}
		if len(texts) > 0 {
			if err := s.embedChunkBatch(ctx, file, texts, chunkIndexes); err != nil {
				return Stats{}, err
			}
			embeddedChunks += len(texts)
			progress.ChunksEmbedded = embeddedChunks
			progress.Percent = computePercent(embeddedChunks, totalChunks)
			progress.UpdatedAt = time.Now()
			s.publish(progress, false)
		}
		if err := s.store.ReplaceFile(ctx, *file); err != nil {
			return Stats{}, err
		}
		progress.FilesIndexed++
		progress.Message = fmt.Sprintf("Indexed %d/%d files", progress.FilesIndexed, len(toIndex))
		progress.UpdatedAt = time.Now()
		s.publish(progress, false)
	}

	stats, err := s.store.Stats(ctx)
	if err != nil {
		return Stats{}, err
	}
	stats.LastIndexedAt = time.Now()
	if err := s.store.UpdateStats(ctx, stats); err != nil {
		return Stats{}, err
	}
	progress.Active = false
	progress.Finished = true
	progress.Phase = "ready"
	progress.Message = "Codebase indexing complete"
	progress.Percent = 1
	progress.UpdatedAt = time.Now()
	progress.Stats = stats
	s.publish(progress, false)
	return stats, nil
}

func (s *Service) embedChunkBatch(ctx context.Context, file *indexedFile, texts []string, indexes []int) error {
	vectors, err := s.embedder.EmbedDocuments(ctx, texts)
	if err != nil {
		return err
	}
	for i, chunkIndex := range indexes {
		file.Chunks[chunkIndex].Embedding = vectors[i]
	}
	return nil
}

func (s *Service) publish(progress Progress, created bool) {
	progress.UpdatedAt = time.Now()
	s.lastStatus = progress
	if created {
		publishProgress("created", progress)
		return
	}
	publishProgress("updated", progress)
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

func workspaceKey(root string) string {
	sum := sha1.Sum([]byte(root))
	return "codebase-" + hex.EncodeToString(sum[:8])
}

func computePercent(done, total int) float64 {
	if total <= 0 {
		return 0
	}
	if done >= total {
		return 1
	}
	return float64(done) / float64(total)
}

func parseInt64(value string) (int64, error) {
	var out int64
	_, err := fmt.Sscanf(value, "%d", &out)
	return out, err
}

func removeLegacySQLiteIndex(dataDir string) error {
	if strings.TrimSpace(dataDir) == "" {
		return nil
	}
	matches, err := filepath.Glob(filepath.Join(dataDir, "index", "codebase-*.sqlite*"))
	if err != nil {
		return err
	}
	for _, match := range matches {
		_ = os.Remove(match)
	}
	return nil
}
