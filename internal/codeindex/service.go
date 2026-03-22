package codeindex

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
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
		Message:   "Scanning workspace",
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
	s.publish(progress, false)

	storedFiles, err := s.store.ListFiles(ctx)
	if err != nil {
		return Stats{}, err
	}

	currentPaths := make(map[string]struct{}, len(files))
	changedFiles := make([]discoveredFile, 0, len(files))
	needsDelete := make(map[string]struct{}, len(files))
	for i, file := range files {
		currentPaths[file.RelativePath] = struct{}{}
		if !force {
			if existing, ok := storedFiles[file.RelativePath]; ok && existing.ContentHash == file.ContentHash {
				progress.FilesProcessed = i + 1
				progress.Percent = computePercent(progress.FilesProcessed, len(files))
				s.publish(progress, false)
				continue
			}
		}
		if _, ok := storedFiles[file.RelativePath]; ok {
			needsDelete[file.RelativePath] = struct{}{}
		}
		changedFiles = append(changedFiles, file)
		progress.FilesProcessed = i + 1
		progress.Percent = computePercent(progress.FilesProcessed, len(files))
		s.publish(progress, false)
	}

	for path := range storedFiles {
		if _, ok := currentPaths[path]; !ok {
			if err := s.store.DeleteFile(ctx, path); err != nil {
				return Stats{}, err
			}
		}
	}

	if len(changedFiles) == 0 {
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

	progress.Phase = "preparing"
	progress.Message = "Preparing changed chunks"
	progress.FilesProcessed = 0
	progress.FilesIndexed = 0
	progress.ChunksTotal = 0
	progress.ChunksEmbedded = 0
	progress.Percent = 0
	s.publish(progress, false)

	toIndex, totalChunks, err := s.buildIndexedFilesParallel(ctx, changedFiles, needsDelete, &progress)
	if err != nil {
		progress.Active = false
		progress.Finished = true
		progress.Error = err.Error()
		progress.Message = "Preparing changed chunks failed"
		s.publish(progress, false)
		return Stats{}, err
	}

	progress.Phase = "embedding"
	progress.Message = "Embedding code chunks"
	progress.FilesProcessed = 0
	progress.FilesIndexed = 0
	progress.ChunksTotal = totalChunks
	progress.ChunksEmbedded = 0
	progress.Percent = 0
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
	progress.Percent = 0
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

func (s *Service) buildIndexedFilesParallel(ctx context.Context, files []discoveredFile, needsDelete map[string]struct{}, progress *Progress) ([]indexedFile, int, error) {
	if len(files) == 0 {
		return nil, 0, nil
	}
	type result struct {
		index int
		file  indexedFile
		err   error
	}
	workerCount := prepWorkerCount(len(files))
	workCh := make(chan int)
	resultCh := make(chan result, len(files))
	var wg sync.WaitGroup

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range workCh {
				select {
				case <-ctx.Done():
					resultCh <- result{index: idx, err: ctx.Err()}
					return
				default:
				}
				indexed := buildIndexedFile(files[idx])
				if _, ok := needsDelete[indexed.Path]; ok {
					indexed.NeedsDelete = true
				}
				resultCh <- result{index: idx, file: indexed}
			}
		}()
	}

	for idx := range files {
		workCh <- idx
	}
	close(workCh)
	wg.Wait()
	close(resultCh)

	out := make([]indexedFile, len(files))
	totalChunks := 0
	processed := 0
	for res := range resultCh {
		if res.err != nil {
			return nil, 0, res.err
		}
		out[res.index] = res.file
		totalChunks += len(res.file.Chunks)
		processed++
		progressCopy := *progress
		progressCopy.FilesProcessed = processed
		progressCopy.Message = fmt.Sprintf("Prepared %d/%d changed files", processed, len(files))
		progressCopy.ChunksTotal = totalChunks
		progressCopy.Percent = computePercent(processed, len(files))
		s.publish(progressCopy, false)
	}
	return out, totalChunks, nil
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

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for batch := range workCh {
				if err := s.embedChunkRefBatch(ctx, files, batch, progress, &embeddedChunks); err != nil {
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

func (s *Service) embedChunkRefBatch(ctx context.Context, files []indexedFile, batch []chunkRef, progress *Progress, embeddedChunks *atomic.Int64) error {
	texts := make([]string, 0, len(batch))
	for _, ref := range batch {
		texts = append(texts, ref.text)
	}
	vectors, err := s.embedder.EmbedDocuments(ctx, texts)
	if err != nil {
		if isJinaEncodeTextError(err) && len(batch) > 1 {
			return s.embedChunkRefSplit(ctx, files, batch, progress, embeddedChunks)
		}
		if isJinaEncodeTextError(err) && len(batch) == 1 {
			s.skipRejectedChunk(files, batch[0], err)
			return nil
		}
		return err
	}
	for i, ref := range batch {
		files[ref.fileIdx].Chunks[ref.chunkIdx].Embedding = vectors[i]
	}
	totalEmbedded := int(embeddedChunks.Add(int64(len(batch))))
	s.updateEmbeddingProgress(progress, totalEmbedded)
	return nil
}

func (s *Service) embedChunkRefSplit(ctx context.Context, files []indexedFile, batch []chunkRef, progress *Progress, embeddedChunks *atomic.Int64) error {
	if len(batch) == 0 {
		return nil
	}
	if len(batch) == 1 {
		return s.embedSingleChunk(ctx, files, batch[0], progress, embeddedChunks)
	}
	mid := len(batch) / 2
	if err := s.embedChunkRefBatch(ctx, files, batch[:mid], progress, embeddedChunks); err != nil {
		return err
	}
	return s.embedChunkRefBatch(ctx, files, batch[mid:], progress, embeddedChunks)
}

func (s *Service) embedSingleChunk(ctx context.Context, files []indexedFile, ref chunkRef, progress *Progress, embeddedChunks *atomic.Int64) error {
	vectors, err := s.embedder.EmbedDocuments(ctx, []string{ref.text})
	if err != nil {
		if isJinaEncodeTextError(err) {
			s.skipRejectedChunk(files, ref, err)
			return nil
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return err
	}
	if len(vectors) == 0 {
		return nil
	}
	files[ref.fileIdx].Chunks[ref.chunkIdx].Embedding = vectors[0]
	totalEmbedded := int(embeddedChunks.Add(1))
	s.updateEmbeddingProgress(progress, totalEmbedded)
	return nil
}

func (s *Service) skipRejectedChunk(files []indexedFile, ref chunkRef, err error) {
	files[ref.fileIdx].Chunks[ref.chunkIdx].Embedding = nil
	slog.Warn("Skipping chunk rejected by Jina tokenizer", "path", files[ref.fileIdx].Path, "chunk_index", ref.chunkIdx, "error", err)
}

func (s *Service) updateEmbeddingProgress(progress *Progress, embeddedChunks int) {
	progressCopy := *progress
	progressCopy.ChunksEmbedded = embeddedChunks
	progressCopy.Percent = computePercent(embeddedChunks, progress.ChunksTotal)
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

func prepWorkerCount(totalFiles int) int {
	workers := runtime.GOMAXPROCS(0)
	if workers < 2 {
		workers = 2
	}
	if workers > 16 {
		workers = 16
	}
	if totalFiles < workers {
		return totalFiles
	}
	return workers
}

func embeddingWorkerCount(totalBatches int) int {
	if totalBatches <= 0 {
		return 1
	}
	workers := runtime.GOMAXPROCS(0) * 4
	if workers < 12 {
		workers = 12
	}
	if workers > 32 {
		workers = 32
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
		if len(current) >= maxBatchEmbeds || (len(current) > 0 && nextBytes > maxEmbeddingBatchBytes) {
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
