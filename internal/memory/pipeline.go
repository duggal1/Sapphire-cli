package memory

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"
)

// ExtractionEvent is an event pushed to the pipeline for background extraction.
type ExtractionEvent struct {
	SessionID string
	TurnIndex int
	EventType string // "tool_result", "user_message", etc.
	RawSource string // The raw text to extract from (last 2-4 turns)
}

// Pipeline is the background memory extraction pipeline.
// It receives events from tool hooks, batches them, and writes structured records
// to the Store. Never blocks the main agent.
type Pipeline struct {
	store     *Store
	extractor *Extractor
	queue     chan ExtractionEvent
	done      chan struct{}
	wg        sync.WaitGroup
}

const (
	// DefaultQueueSize is the buffered channel size. Large enough to never block.
	DefaultQueueSize = 256
	// batchWindow is how long to wait for more events before processing.
	batchWindow = 500 * time.Millisecond
	// maxBatchSize is the max events per extraction call.
	maxBatchSize = 5
	// maxRetries is how many times to retry extraction on model failure.
	maxRetries = 1
	// retryBackoff is the delay between retries.
	retryBackoff = 2 * time.Second
)

// NewPipeline creates a new background extraction pipeline.
func NewPipeline(store *Store, extractor *Extractor) *Pipeline {
	return &Pipeline{
		store:     store,
		extractor: extractor,
		queue:     make(chan ExtractionEvent, DefaultQueueSize),
		done:      make(chan struct{}),
	}
}

// Start begins the background worker goroutine.
func (p *Pipeline) Start(ctx context.Context) {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.worker(ctx)
	}()
}

// Stop signals the worker to drain and exit, then waits.
func (p *Pipeline) Stop() {
	close(p.done)
	p.wg.Wait()
}

// Push adds an event to the extraction queue. Never blocks — drops with warning if full.
func (p *Pipeline) Push(event ExtractionEvent) {
	select {
	case p.queue <- event:
	default:
		slog.Warn("memory: extraction queue full, dropping event",
			"session", event.SessionID, "type", event.EventType)
	}
}

// ExtractSync runs a synchronous extraction pass on the given raw source.
// Used for pre-compaction checkpoints where we need the result immediately.
func (p *Pipeline) ExtractSync(ctx context.Context, sessionID string, turnIndex int, rawSource string) error {
	if p.extractor == nil {
		return nil
	}

	result, err := p.extractor.Extract(ctx, rawSource)
	if err != nil {
		slog.Warn("memory: sync extraction failed, writing raw", "error", err)
		// Fallback: write raw source as unparsed record
		return p.store.WriteRecord(ctx, MemoryRecord{
			SessionID:   sessionID,
			EventType:   "raw_unparsed",
			Timestamp:   time.Now().Unix(),
			TurnIndex:   turnIndex,
			Salience:    0.4,
			ContentJSON: `{"raw": true}`,
			RawSource:   rawSource,
		})
	}

	records := ResultToRecords(sessionID, turnIndex, result, rawSource)
	for _, rec := range records {
		if writeErr := p.store.WriteRecord(ctx, rec); writeErr != nil {
			slog.Debug("memory: failed to write sync record", "error", writeErr)
		}
	}
	return nil
}

func (p *Pipeline) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			p.drain(context.Background())
			return
		case <-p.done:
			p.drain(context.Background())
			return
		case event := <-p.queue:
			batch := []ExtractionEvent{event}
			batch = p.collectBatch(batch)
			p.processBatch(ctx, batch)
		}
	}
}

func (p *Pipeline) collectBatch(batch []ExtractionEvent) []ExtractionEvent {
	timer := time.NewTimer(batchWindow)
	defer timer.Stop()

	for len(batch) < maxBatchSize {
		select {
		case event := <-p.queue:
			batch = append(batch, event)
		case <-timer.C:
			return batch
		}
	}
	return batch
}

func (p *Pipeline) drain(ctx context.Context) {
	for {
		select {
		case event := <-p.queue:
			p.processBatch(ctx, []ExtractionEvent{event})
		default:
			return
		}
	}
}

func (p *Pipeline) processBatch(ctx context.Context, events []ExtractionEvent) {
	if p.extractor == nil || len(events) == 0 {
		return
	}

	// Combine raw sources for batch extraction
	var combined string
	for i, e := range events {
		if i > 0 {
			combined += "\n---\n"
		}
		combined += e.RawSource
	}

	var result *ExtractionResult
	var err error

	// Retry loop
	for attempt := range maxRetries + 1 {
		result, err = p.extractor.Extract(ctx, combined)
		if err == nil {
			break
		}
		slog.Warn("memory: extraction failed",
			"attempt", attempt+1, "error", err)
		if attempt < maxRetries {
			select {
			case <-ctx.Done():
				return
			case <-time.After(retryBackoff):
			}
		}
	}

	if err != nil {
		// All retries failed — write raw source directly
		slog.Warn("memory: extraction failed after retries, writing raw source")
		for _, e := range events {
			writeErr := p.store.WriteRecord(ctx, MemoryRecord{
				SessionID:   e.SessionID,
				EventType:   "raw_" + e.EventType,
				Timestamp:   time.Now().Unix(),
				TurnIndex:   e.TurnIndex,
				Salience:    salienceForEventType(e.EventType),
				ContentJSON: `{"raw": true, "event_type": "` + e.EventType + `"}`,
				RawSource:   e.RawSource,
			})
			if writeErr != nil {
				slog.Debug("memory: failed to write raw fallback", "error", writeErr)
			}
		}
		return
	}

	// Write extracted records
	turnIndex := events[0].TurnIndex
	sessionID := events[0].SessionID
	records := ResultToRecords(sessionID, turnIndex, result, combined)

	for _, rec := range records {
		if writeErr := p.store.WriteRecord(ctx, rec); writeErr != nil {
			slog.Debug("memory: failed to write extracted record", "error", writeErr)
		}
	}

	// Check if architectural decisions were made — potentially update constitution
	if len(result.ArchitecturalDecisions) > 0 {
		p.maybeUpdateConstitution(ctx, result)
	}
}

func (p *Pipeline) maybeUpdateConstitution(ctx context.Context, result *ExtractionResult) {
	existing, _ := p.store.GetConstitution(ctx)

	var decisions []string
	for _, ad := range result.ArchitecturalDecisions {
		decisions = append(decisions, "- "+ad.Decision+": "+ad.Rationale)
	}

	if existing == "" {
		content := "# Project Architecture Decisions\n\n"
		for _, d := range decisions {
			content += d + "\n"
		}
		if err := p.store.UpsertConstitution(ctx, content); err != nil {
			slog.Debug("memory: failed to create constitution", "error", err)
		}
		return
	}

	// Append new decisions
	updated := existing + "\n"
	for _, d := range decisions {
		updated += d + "\n"
	}
	// Cap constitution at ~2K characters
	if len(updated) > 2048 {
		updated = updated[:2048]
	}
	if err := p.store.UpsertConstitution(ctx, updated); err != nil {
		slog.Debug("memory: failed to update constitution", "error", err)
	}
}

// BuildCheckpointJSON creates a checkpoint JSON from recent memory records.
func BuildCheckpointJSON(ctx context.Context, store *Store, sessionID string) (string, error) {
	records, err := store.QueryRecords(ctx, "all", 50)
	if err != nil {
		return "", err
	}

	type checkpoint struct {
		SessionID string         `json:"session_id"`
		Timestamp int64          `json:"timestamp"`
		Records   []MemoryRecord `json:"records"`
	}

	cp := checkpoint{
		SessionID: sessionID,
		Timestamp: time.Now().Unix(),
		Records:   records,
	}

	data, err := json.Marshal(cp)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func salienceForEventType(eventType string) float64 {
	switch eventType {
	case "error", "failure":
		return 0.9
	case "file_write", "file_edit":
		return 0.7
	case "user_message":
		return 0.6
	case "terminal_execution", "bash":
		return 0.55
	default:
		return 0.5
	}
}
