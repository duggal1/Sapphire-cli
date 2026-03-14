package memory

import (
	"context"
	"encoding/json"
	"log/slog"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
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
	primary   MemoryExtractor
	secondary MemoryExtractor
	active    MemoryExtractor
	embedder  Embedder
	queue     chan ExtractionEvent
	done      chan struct{}
	wg        sync.WaitGroup
	stats     *pipelineStats
	rand      *rand.Rand
}

const (
	// DefaultQueueSize is the buffered channel size. Large enough to never block.
	DefaultQueueSize = 1024
	// batchWindow is how long to wait for more events before processing.
	batchWindow = 500 * time.Millisecond
	// maxBatchSize is the max events per extraction call.
	maxBatchSize = 5
	// maxRetries is how many times to retry extraction on model failure.
	maxRetries = 3
	// baseBackoff is the initial delay between retries.
	baseBackoff = 1 * time.Second
	// maxBackoff caps the exponential backoff.
	maxBackoff = 10 * time.Second
	// maxTotalBackoff caps total retry delay.
	maxTotalBackoff = 30 * time.Second
	// backpressureThreshold is when to signal the agent to slow down.
	backpressureThreshold = 0.8
	// failureWindowSize is the rolling window for failure-rate metrics.
	failureWindowSize = 20
	// failureRateThreshold triggers emergency snapshot writes.
	failureRateThreshold = 0.3
	// primaryFailureSwitchThreshold triggers fallback extractor usage.
	primaryFailureSwitchThreshold = 3
)

// NewPipeline creates a new background extraction pipeline.
func NewPipeline(store *Store, primary MemoryExtractor, secondary MemoryExtractor, embedder Embedder) *Pipeline {
	active := primary
	if active == nil {
		active = secondary
	}
	return &Pipeline{
		store:     store,
		primary:   primary,
		secondary: secondary,
		active:    active,
		embedder:  embedder,
		queue:     make(chan ExtractionEvent, DefaultQueueSize),
		done:      make(chan struct{}),
		stats:     newPipelineStats(failureWindowSize),
		rand:      rand.New(rand.NewSource(time.Now().UnixNano())),
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
		p.stats.incrementDropped()
		slog.Warn("memory: extraction queue full, routing to dead-letter",
			"session", event.SessionID, "type", event.EventType)
		if p.store != nil {
			go func() {
				if err := p.store.WriteDeadLetter(context.Background(), event, "queue_full"); err != nil {
					slog.Debug("memory: failed to write dead-letter", "error", err)
				}
			}()
		}
	}
}

// ExtractSync runs a synchronous extraction pass on the given raw source.
// Used for pre-compaction checkpoints where we need the result immediately.
func (p *Pipeline) ExtractSync(ctx context.Context, sessionID string, turnIndex int, rawSource string) error {
	if p.active == nil {
		return nil
	}

	result, err := p.active.Extract(ctx, rawSource)
	if err != nil {
		slog.Warn("memory: sync extraction failed, writing raw", "error", err)
		// Fallback: write raw source as unparsed record
		rec := MemoryRecord{
			SessionID:   sessionID,
			EventType:   "raw_unparsed",
			Timestamp:   time.Now().Unix(),
			TurnIndex:   turnIndex,
			Salience:    0.4,
			ContentJSON: `{"raw": true}`,
			RawSource:   rawSource,
		}
		recordID, writeErr := p.store.WriteRecord(ctx, rec)
		if writeErr == nil {
			p.embedRecords(ctx, []embeddingJob{{recordID: recordID, text: recordEmbeddingText(rec)}})
		}
		return writeErr
	}

	records := ResultToRecords(sessionID, turnIndex, result, rawSource)
	var embedJobs []embeddingJob
	for _, rec := range records {
		recordID, writeErr := p.store.WriteRecord(ctx, rec)
		if writeErr != nil {
			slog.Debug("memory: failed to write sync record", "error", writeErr)
			continue
		}
		embedJobs = append(embedJobs, embeddingJob{
			recordID: recordID,
			text:     recordEmbeddingText(rec),
		})
	}
	p.embedRecords(ctx, embedJobs)
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
	if p.active == nil || len(events) == 0 {
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

	var (
		result    *ExtractionResult
		err       error
		totalWait time.Duration
	)

	extractor := p.active

	// Retry loop with exponential backoff + jitter.
	for attempt := 0; attempt <= maxRetries; attempt++ {
		result, err = extractor.Extract(ctx, combined)
		if err == nil {
			break
		}
		slog.Warn("memory: extraction failed",
			"attempt", attempt+1,
			"extractor", extractor.Name(),
			"error", err)

		if attempt < maxRetries {
			wait := p.nextBackoff(attempt)
			if totalWait+wait > maxTotalBackoff {
				wait = maxTotalBackoff - totalWait
			}
			if wait > 0 {
				select {
				case <-ctx.Done():
					return
				case <-time.After(wait):
				}
				totalWait += wait
			}
		}
	}

	if err != nil {
		failureRate := p.stats.record(false, extractor == p.primary)
		slog.Info("memory: extraction failure rate",
			"failure_rate", failureRate,
			"total", p.stats.total.Load(),
			"failed", p.stats.failed.Load(),
		)

		if failureRate >= failureRateThreshold && p.stats.windowFilled() {
			p.writeEmergencySnapshot(ctx, events, combined, failureRate, extractor.Name())
		}

		if extractor == p.primary {
			p.maybeSwitchExtractor()
		}

		// All retries failed — write raw source directly
		slog.Warn("memory: extraction failed after retries, writing raw source")
		var embedJobs []embeddingJob
		for _, e := range events {
			rec := MemoryRecord{
				SessionID:   e.SessionID,
				EventType:   "raw_" + e.EventType,
				Timestamp:   time.Now().Unix(),
				TurnIndex:   e.TurnIndex,
				Salience:    salienceForEventType(e.EventType),
				ContentJSON: `{"raw": true, "event_type": "` + e.EventType + `"}`,
				RawSource:   e.RawSource,
			}
			recordID, writeErr := p.store.WriteRecord(ctx, rec)
			if writeErr != nil {
				slog.Debug("memory: failed to write raw fallback", "error", writeErr)
				continue
			}
			embedJobs = append(embedJobs, embeddingJob{
				recordID: recordID,
				text:     recordEmbeddingText(rec),
			})
		}
		p.embedRecords(ctx, embedJobs)
		return
	}

	p.stats.record(true, extractor == p.primary)

	// Write extracted records
	turnIndex := events[0].TurnIndex
	sessionID := events[0].SessionID
	records := ResultToRecords(sessionID, turnIndex, result, combined)

	var embedJobs []embeddingJob
	for _, rec := range records {
		recordID, writeErr := p.store.WriteRecord(ctx, rec)
		if writeErr != nil {
			slog.Debug("memory: failed to write extracted record", "error", writeErr)
			continue
		}
		embedJobs = append(embedJobs, embeddingJob{
			recordID: recordID,
			text:     recordEmbeddingText(rec),
		})
	}
	p.embedRecords(ctx, embedJobs)

	// Check if architectural decisions were made — potentially update constitution
	if len(result.ArchitecturalDecisions) > 0 {
		p.maybeUpdateConstitution(ctx, result)
	}
}

func (p *Pipeline) maybeUpdateConstitution(ctx context.Context, result *ExtractionResult) {
	existing, _ := p.store.GetConstitution(ctx)
	if existing != "" {
		// Core constitution is immutable once established.
		return
	}

	var decisions []string
	for _, ad := range result.ArchitecturalDecisions {
		decisions = append(decisions, "- "+ad.Decision+": "+ad.Rationale)
	}

	content := "# Project Architecture Decisions (Core)\n\n"
	for _, d := range decisions {
		content += d + "\n"
	}
	if len(content) > 1024 {
		content = content[:1024]
	}
	if err := p.store.UpsertConstitution(ctx, content); err != nil {
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

// PressureState describes queue utilization.
type PressureState struct {
	Len   int     `json:"len"`
	Cap   int     `json:"cap"`
	Ratio float64 `json:"ratio"`
	High  bool    `json:"high"`
}

// PipelineStatsSnapshot is a read-only view of pipeline health.
type PipelineStatsSnapshot struct {
	TotalBatches               uint64        `json:"total_batches"`
	SuccessfulBatches          uint64        `json:"successful_batches"`
	FailedBatches              uint64        `json:"failed_batches"`
	FailureRate                float64       `json:"failure_rate"`
	ConsecutiveFailures        uint64        `json:"consecutive_failures"`
	PrimaryConsecutiveFailures uint64        `json:"primary_consecutive_failures"`
	DroppedEvents              uint64        `json:"dropped_events"`
	LastFailureUnix            int64         `json:"last_failure_unix"`
	ActiveExtractor            string        `json:"active_extractor"`
	PrimaryExtractor           string        `json:"primary_extractor"`
	SecondaryExtractor         string        `json:"secondary_extractor"`
	Queue                      PressureState `json:"queue"`
	WindowSize                 int           `json:"window_size"`
}

func (p *Pipeline) Pressure() PressureState {
	if p == nil || p.queue == nil {
		return PressureState{}
	}
	length := len(p.queue)
	capacity := cap(p.queue)
	ratio := 0.0
	if capacity > 0 {
		ratio = float64(length) / float64(capacity)
	}
	return PressureState{
		Len:   length,
		Cap:   capacity,
		Ratio: ratio,
		High:  ratio >= backpressureThreshold,
	}
}

func (p *Pipeline) StatsSnapshot() PipelineStatsSnapshot {
	if p == nil || p.stats == nil {
		return PipelineStatsSnapshot{}
	}
	activeName := ""
	if p.active != nil {
		activeName = p.active.Name()
	}
	primaryName := ""
	if p.primary != nil {
		primaryName = p.primary.Name()
	}
	secondaryName := ""
	if p.secondary != nil {
		secondaryName = p.secondary.Name()
	}
	return p.stats.snapshot(activeName, primaryName, secondaryName, p.Pressure())
}

type pipelineStats struct {
	total           atomic.Uint64
	success         atomic.Uint64
	failed          atomic.Uint64
	consecutive     atomic.Uint64
	primaryConsec   atomic.Uint64
	dropped         atomic.Uint64
	lastFailureUnix atomic.Int64
	mu              sync.Mutex
	window          []bool
	windowIndex     int
	windowCount     int
}

func newPipelineStats(windowSize int) *pipelineStats {
	if windowSize <= 0 {
		windowSize = 10
	}
	return &pipelineStats{
		window: make([]bool, windowSize),
	}
}

func (ps *pipelineStats) record(success bool, primary bool) float64 {
	ps.total.Add(1)
	if success {
		ps.success.Add(1)
		ps.consecutive.Store(0)
		if primary {
			ps.primaryConsec.Store(0)
		}
	} else {
		ps.failed.Add(1)
		ps.consecutive.Add(1)
		if primary {
			ps.primaryConsec.Add(1)
		}
		ps.lastFailureUnix.Store(time.Now().Unix())
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()

	if len(ps.window) > 0 {
		ps.window[ps.windowIndex] = success
		ps.windowIndex = (ps.windowIndex + 1) % len(ps.window)
		if ps.windowCount < len(ps.window) {
			ps.windowCount++
		}
	}
	return ps.failureRateLocked()
}

func (ps *pipelineStats) incrementDropped() {
	ps.dropped.Add(1)
}

func (ps *pipelineStats) windowFilled() bool {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return ps.windowCount >= len(ps.window)
}

func (ps *pipelineStats) failureRateLocked() float64 {
	if ps.windowCount == 0 {
		return 0
	}
	failures := 0
	for i := 0; i < ps.windowCount; i++ {
		if !ps.window[i] {
			failures++
		}
	}
	return float64(failures) / float64(ps.windowCount)
}

func (ps *pipelineStats) snapshot(active, primary, secondary string, pressure PressureState) PipelineStatsSnapshot {
	ps.mu.Lock()
	rate := ps.failureRateLocked()
	windowSize := len(ps.window)
	ps.mu.Unlock()

	return PipelineStatsSnapshot{
		TotalBatches:               ps.total.Load(),
		SuccessfulBatches:          ps.success.Load(),
		FailedBatches:              ps.failed.Load(),
		FailureRate:                rate,
		ConsecutiveFailures:        ps.consecutive.Load(),
		PrimaryConsecutiveFailures: ps.primaryConsec.Load(),
		DroppedEvents:              ps.dropped.Load(),
		LastFailureUnix:            ps.lastFailureUnix.Load(),
		ActiveExtractor:            active,
		PrimaryExtractor:           primary,
		SecondaryExtractor:         secondary,
		Queue:                      pressure,
		WindowSize:                 windowSize,
	}
}

type embeddingJob struct {
	recordID int64
	text     string
}

func (p *Pipeline) embedRecords(ctx context.Context, jobs []embeddingJob) {
	if p.embedder == nil || len(jobs) == 0 {
		return
	}
	texts := make([]string, 0, len(jobs))
	validJobs := make([]embeddingJob, 0, len(jobs))
	for _, job := range jobs {
		if job.recordID == 0 || job.text == "" {
			continue
		}
		texts = append(texts, job.text)
		validJobs = append(validJobs, job)
	}
	if len(texts) == 0 {
		return
	}
	vectors, err := p.embedder.EmbedDocuments(ctx, texts)
	if err != nil {
		slog.Debug("memory: embedding failed", "error", err, "embedder", p.embedder.Name())
		return
	}
	for i, vec := range vectors {
		if i >= len(validJobs) {
			break
		}
		if err := p.store.UpsertEmbedding(ctx, validJobs[i].recordID, vec, p.embedder.Dimensions()); err != nil {
			slog.Debug("memory: failed to write embedding", "error", err)
		}
	}
}

func recordEmbeddingText(rec MemoryRecord) string {
	var sb strings.Builder
	sb.WriteString("event_type: ")
	sb.WriteString(rec.EventType)
	sb.WriteString("\ncontent: ")
	sb.WriteString(rec.ContentJSON)
	if rec.RawSource != "" {
		sb.WriteString("\nraw: ")
		sb.WriteString(truncate(rec.RawSource, 800))
	}
	return sb.String()
}

func (p *Pipeline) nextBackoff(attempt int) time.Duration {
	backoff := baseBackoff << attempt
	if backoff > maxBackoff {
		backoff = maxBackoff
	}
	jitter := 0.5 + p.rand.Float64()
	return time.Duration(float64(backoff) * jitter)
}

func (p *Pipeline) maybeSwitchExtractor() {
	if p.secondary == nil || p.active == p.secondary {
		return
	}
	if p.stats.primaryConsec.Load() >= primaryFailureSwitchThreshold {
		p.active = p.secondary
		slog.Warn("memory: switching to fallback extractor", "extractor", p.secondary.Name())
	}
}

func (p *Pipeline) writeEmergencySnapshot(ctx context.Context, events []ExtractionEvent, combined string, failureRate float64, extractor string) {
	if p.store == nil || len(events) == 0 {
		return
	}
	payload, err := json.Marshal(map[string]any{
		"reason":          "extraction_failure_rate",
		"failure_rate":    failureRate,
		"extractor":       extractor,
		"events_in_batch": len(events),
	})
	if err != nil {
		return
	}
	rec := MemoryRecord{
		SessionID:   events[0].SessionID,
		EventType:   "emergency_snapshot",
		Timestamp:   time.Now().Unix(),
		TurnIndex:   events[0].TurnIndex,
		Salience:    1.0,
		ContentJSON: string(payload),
		RawSource:   truncate(combined, 2000),
	}
	recordID, writeErr := p.store.WriteRecord(ctx, rec)
	if writeErr != nil {
		slog.Debug("memory: failed to write emergency snapshot", "error", writeErr)
		return
	}
	p.embedRecords(ctx, []embeddingJob{{recordID: recordID, text: recordEmbeddingText(rec)}})
}
