package codeindex

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Service struct {
	root       string
	store      *store
	embedder   *embedder
	initErr    error
	indexMu    sync.Mutex
	statusMu   sync.Mutex
	lastStatus Progress
	lastSentAt time.Time
}

func New(ctx context.Context, cfg Config) (*Service, error) {
	_ = ctx
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
	store, err := openStore(cfg.DataDir, root, cfg.Model, cfg.Dimensions, cfg.QdrantURL)
	if err != nil {
		return nil, err
	}
	service := &Service{
		root:  root,
		store: store,
	}
	embedder, err := newEmbedder(cfg.APIKey, cfg.Model, cfg.Dimensions)
	if err != nil {
		service.initErr = err
	} else {
		service.embedder = embedder
	}
	service.lastStatus = Progress{Workspace: root}
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
	s.indexMu.Lock()
	defer s.indexMu.Unlock()
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
		s.publish(progress, false)
		return Stats{}, err
	}
	progress.FilesDiscovered = len(files)
	progress.Message = "Preparing changed chunks"
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
				progress.Percent = computePercent(progress.FilesProcessed, len(files)) * 0.15
				s.publish(progress, false)
				continue
			}
		}
		indexed := buildIndexedFile(file)
		if _, ok := storedFiles[file.RelativePath]; ok {
			indexed.NeedsDelete = true
		}
		totalChunks += len(indexed.Chunks)
		toIndex = append(toIndex, indexed)
		progress.FilesProcessed = i + 1
		progress.ChunksTotal = totalChunks
		progress.Percent = computePercent(progress.FilesProcessed, len(files)) * 0.15
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
		progress.Stats = stats
		s.publish(progress, false)
		return stats, nil
	}

	progress.Phase = "embedding"
	progress.Message = "Embedding code chunks"
	progress.FilesIndexed = 0
	progress.FilesProcessed = 0
	progress.ChunksTotal = totalChunks
	progress.Percent = 0.15
	s.publish(progress, false)

	if err := s.embedFilesParallel(ctx, toIndex, &progress); err != nil {
		progress.Active = false
		progress.Finished = true
		progress.Error = err.Error()
		progress.Message = "Embedding code chunks failed"
		s.publish(progress, false)
		return Stats{}, err
	}
	progress.ChunksEmbedded = totalChunks
	progress.FilesIndexed = len(toIndex)

	progress.Phase = "upserting"
	progress.Message = "Writing vector index"
	progress.Percent = 0.97
	s.publish(progress, false)

	if err := s.store.ReplaceFiles(ctx, toIndex); err != nil {
		progress.Active = false
		progress.Finished = true
		progress.Error = err.Error()
		progress.Message = "Writing vector index failed"
		s.publish(progress, false)
		return Stats{}, err
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

func (s *Service) embedFilesParallel(ctx context.Context, files []indexedFile, progress *Progress) error {
	if len(files) == 0 {
		return nil
	}
	workerCount := embeddingWorkerCount(len(files))
	workCh := make(chan int)
	errCh := make(chan error, 1)
	var wg sync.WaitGroup
	var embeddedChunks atomic.Int64
	var indexedFiles atomic.Int64

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range workCh {
				if err := s.embedIndexedFile(ctx, &files[idx], progress, &embeddedChunks, &indexedFiles); err != nil {
					select {
					case errCh <- err:
					default:
					}
					return
				}
			}
		}()
	}

	for idx := range files {
		select {
		case <-ctx.Done():
			close(workCh)
			wg.Wait()
			return ctx.Err()
		case err := <-errCh:
			close(workCh)
			wg.Wait()
			return err
		case workCh <- idx:
		}
	}
	close(workCh)
	wg.Wait()

	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

func (s *Service) embedIndexedFile(ctx context.Context, file *indexedFile, progress *Progress, embeddedChunks, indexedFiles *atomic.Int64) error {
	texts := make([]string, 0, maxBatchEmbeds)
	chunkIndexes := make([]int, 0, maxBatchEmbeds)
	flush := func() error {
		if len(texts) == 0 {
			return nil
		}
		if err := s.embedChunkBatch(ctx, file, texts, chunkIndexes); err != nil {
			return err
		}
		totalEmbedded := int(embeddedChunks.Add(int64(len(texts))))
		s.updateEmbeddingProgress(progress, totalEmbedded, int(indexedFiles.Load()))
		texts = texts[:0]
		chunkIndexes = chunkIndexes[:0]
		return nil
	}

	for idx := range file.Chunks {
		texts = append(texts, file.Chunks[idx].SearchText)
		chunkIndexes = append(chunkIndexes, idx)
		if len(texts) >= maxBatchEmbeds {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := flush(); err != nil {
		return err
	}
	totalIndexed := int(indexedFiles.Add(1))
	s.updateEmbeddingProgress(progress, int(embeddedChunks.Load()), totalIndexed)
	return nil
}

func (s *Service) updateEmbeddingProgress(progress *Progress, embeddedChunks, indexedFiles int) {
	progressCopy := *progress
	progressCopy.ChunksEmbedded = embeddedChunks
	progressCopy.FilesIndexed = indexedFiles
	progressCopy.Message = fmt.Sprintf("Indexed %d/%d files", indexedFiles, max(progress.FilesDiscovered, indexedFiles))
	progressCopy.Percent = computeEmbeddingPercent(embeddedChunks, progress.ChunksTotal)
	s.publish(progressCopy, false)
}

func (s *Service) publish(progress Progress, created bool) {
	progress.UpdatedAt = time.Now()
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	s.lastStatus = progress
	if !created && !progress.Finished && progress.Error == "" && time.Since(s.lastSentAt) < 120*time.Millisecond {
		return
	}
	s.lastSentAt = progress.UpdatedAt
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

func computeEmbeddingPercent(done, total int) float64 {
	if total <= 0 {
		return 0.15
	}
	if done >= total {
		return 0.97
	}
	progress := float64(done) / float64(total)
	if progress > 0 && progress < 0.01 {
		progress = 0.01
	}
	return 0.15 + (0.82 * progress)
}

func embeddingWorkerCount(totalFiles int) int {
	workers := runtime.GOMAXPROCS(0) * 3
	if workers < 6 {
		workers = 6
	}
	if workers > 24 {
		workers = 24
	}
	if totalFiles < workers {
		return totalFiles
	}
	return workers
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
