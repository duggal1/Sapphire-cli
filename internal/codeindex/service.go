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

type chunkRef struct {
	fileIdx  int
	chunkIdx int
	text     string
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
	embedder, err := newEmbedder(cfg.APIKey, cfg.Model, cfg.Dimensions, cfg.DataDir, cfg.OllamaURL)
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
	if s.embedder != nil {
		_ = s.embedder.Close()
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
		progress.SetupRequired = IsSetupRequired(err)
		progress.Error = err.Error()
		if progress.SetupRequired {
			progress.Phase = "setup_required"
			progress.Message = err.Error()
			progress.Error = ""
		} else {
			progress.Message = "Embedding code chunks failed"
		}
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
		progress.SetupRequired = IsSetupRequired(err)
		progress.Error = err.Error()
		if progress.SetupRequired {
			progress.Phase = "setup_required"
			progress.Message = err.Error()
			progress.Error = ""
		} else {
			progress.Message = "Writing vector index failed"
		}
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
	refs := flattenChunkRefs(files)
	if len(refs) == 0 {
		return nil
	}
	batches := buildEmbeddingBatches(refs)
	workerCount := embeddingWorkerCount(len(batches))
	workCh := make(chan []chunkRef)
	errCh := make(chan error, 1)
	var wg sync.WaitGroup
	var embeddedChunks atomic.Int64
	var indexedFiles atomic.Int64
	fileChunkCounts := fileChunkCountMap(files)
	fileEmbeddedCounts := make([]atomic.Int64, len(files))

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for batch := range workCh {
				if err := s.embedChunkRefBatch(ctx, files, batch, progress, &embeddedChunks, &indexedFiles, fileChunkCounts, fileEmbeddedCounts); err != nil {
					select {
					case errCh <- err:
					default:
					}
					return
				}
			}
		}()
	}

	for _, batch := range batches {
		select {
		case <-ctx.Done():
			close(workCh)
			wg.Wait()
			return ctx.Err()
		case err := <-errCh:
			close(workCh)
			wg.Wait()
			return err
		case workCh <- batch:
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

func (s *Service) embedChunkRefBatch(
	ctx context.Context,
	files []indexedFile,
	batch []chunkRef,
	progress *Progress,
	embeddedChunks, indexedFiles *atomic.Int64,
	fileChunkCounts map[int]int,
	fileEmbeddedCounts []atomic.Int64,
) error {
	texts := make([]string, 0, len(batch))
	for _, ref := range batch {
		texts = append(texts, ref.text)
	}
	vectors, err := s.embedder.EmbedDocuments(ctx, texts)
	if err != nil {
		return err
	}
	for i, ref := range batch {
		files[ref.fileIdx].Chunks[ref.chunkIdx].Embedding = vectors[i]
	}
	totalEmbedded := int(embeddedChunks.Add(int64(len(batch))))
	for _, ref := range batch {
		current := fileEmbeddedCounts[ref.fileIdx].Add(1)
		if current == int64(fileChunkCounts[ref.fileIdx]) {
			indexedFiles.Add(1)
		}
	}
	s.updateEmbeddingProgress(progress, totalEmbedded, int(indexedFiles.Load()))
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

func embeddingWorkerCount(totalBatches int) int {
	workers := max(2, runtime.GOMAXPROCS(0)-1)
	if workers > 10 {
		workers = 10
	}
	if totalBatches < workers {
		return totalBatches
	}
	return workers
}

func flattenChunkRefs(files []indexedFile) []chunkRef {
	total := 0
	for _, file := range files {
		total += len(file.Chunks)
	}
	refs := make([]chunkRef, 0, total)
	for fileIdx := range files {
		for chunkIdx := range files[fileIdx].Chunks {
			refs = append(refs, chunkRef{
				fileIdx:  fileIdx,
				chunkIdx: chunkIdx,
				text:     files[fileIdx].Chunks[chunkIdx].SearchText,
			})
		}
	}
	return refs
}

func buildEmbeddingBatches(refs []chunkRef) [][]chunkRef {
	batches := make([][]chunkRef, 0, max(1, len(refs)/maxBatchEmbeds))
	current := make([]chunkRef, 0, maxBatchEmbeds)
	currentBytes := 0
	for _, ref := range refs {
		nextBytes := currentBytes + len(ref.text)
		if len(current) >= maxBatchEmbeds || (len(current) > 0 && nextBytes > 512*1024) {
			batch := make([]chunkRef, len(current))
			copy(batch, current)
			batches = append(batches, batch)
			current = current[:0]
			currentBytes = 0
		}
		current = append(current, ref)
		currentBytes += len(ref.text)
	}
	if len(current) > 0 {
		batch := make([]chunkRef, len(current))
		copy(batch, current)
		batches = append(batches, batch)
	}
	return batches
}

func fileChunkCountMap(files []indexedFile) map[int]int {
	counts := make(map[int]int, len(files))
	for idx, file := range files {
		counts[idx] = len(file.Chunks)
	}
	return counts
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
