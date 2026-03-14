package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

type agentJobStatus string

const (
	agentJobStatusPending   agentJobStatus = "pending"
	agentJobStatusRunning   agentJobStatus = "running"
	agentJobStatusCompleted agentJobStatus = "completed"
	agentJobStatusFailed    agentJobStatus = "failed"
	agentJobStatusCancelled agentJobStatus = "cancelled"
)

type agentJobItemStatus string

const (
	agentJobItemStatusPending   agentJobItemStatus = "pending"
	agentJobItemStatusRunning   agentJobItemStatus = "running"
	agentJobItemStatusCompleted agentJobItemStatus = "completed"
	agentJobItemStatusFailed    agentJobItemStatus = "failed"
)

type agentJobManager struct {
	mu   sync.Mutex
	jobs map[string]*agentJob
}

func newAgentJobManager() *agentJobManager {
	return &agentJobManager{jobs: make(map[string]*agentJob)}
}

type agentJob struct {
	ID              string
	ParentSessionID string
	Instruction     string
	InputHeaders    []string
	OutputCSVPath   string
	OutputSchemaRaw json.RawMessage
	Status          agentJobStatus
	Items           []*agentJobItem
	ItemsByID       map[string]*agentJobItem
	MaxRuntime      time.Duration
	CreatedAt       time.Time
	UpdatedAt       time.Time
	LastError       string
	cancelRequested bool
	mu              sync.Mutex
}

type agentJobItem struct {
	JobID        string
	ItemID       string
	SourceID     string
	RowIndex     int
	Row          map[string]string
	Status       agentJobItemStatus
	AttemptCount int
	LastError    string
	Result       map[string]any
	AssignedID   string
	StartedAt    time.Time
	ReportedAt   time.Time
	CompletedAt  time.Time
}

func (m *agentJobManager) create(job *agentJob) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs[job.ID] = job
}

func (m *agentJobManager) get(jobID string) (*agentJob, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	job := m.jobs[jobID]
	return job, job != nil
}

func (m *agentJobManager) reportResult(jobID, itemID, reportingSession string, result map[string]any, stop bool) (bool, error) {
	job, ok := m.get(jobID)
	if !ok {
		return false, fmt.Errorf("job %s not found", jobID)
	}
	if result == nil {
		return false, errors.New("result is required")
	}
	job.mu.Lock()
	defer job.mu.Unlock()
	item := job.ItemsByID[itemID]
	if item == nil {
		return false, fmt.Errorf("item %s not found", itemID)
	}
	if item.Status == agentJobItemStatusCompleted || item.Status == agentJobItemStatusFailed {
		return false, nil
	}
	item.Result = result
	item.ReportedAt = time.Now()
	item.CompletedAt = time.Now()
	item.Status = agentJobItemStatusCompleted
	job.UpdatedAt = time.Now()
	if stop {
		job.cancelRequested = true
	}
	_ = reportingSession
	return true, nil
}

func (job *agentJob) snapshotProgress() agentJobProgress {
	job.mu.Lock()
	defer job.mu.Unlock()
	progress := agentJobProgress{TotalItems: len(job.Items)}
	for _, item := range job.Items {
		switch item.Status {
		case agentJobItemStatusPending:
			progress.PendingItems++
		case agentJobItemStatusRunning:
			progress.RunningItems++
		case agentJobItemStatusCompleted:
			progress.CompletedItems++
		case agentJobItemStatusFailed:
			progress.FailedItems++
		}
	}
	return progress
}

func (job *agentJob) markFailed(message string) {
	job.mu.Lock()
	defer job.mu.Unlock()
	job.Status = agentJobStatusFailed
	job.LastError = message
	job.UpdatedAt = time.Now()
}

func (job *agentJob) markCancelled(message string) {
	job.mu.Lock()
	defer job.mu.Unlock()
	job.Status = agentJobStatusCancelled
	job.LastError = message
	job.UpdatedAt = time.Now()
}

func (job *agentJob) markCompleted() {
	job.mu.Lock()
	defer job.mu.Unlock()
	job.Status = agentJobStatusCompleted
	job.UpdatedAt = time.Now()
}

func (job *agentJob) nextPendingItem() *agentJobItem {
	job.mu.Lock()
	defer job.mu.Unlock()
	for _, item := range job.Items {
		if item.Status == agentJobItemStatusPending {
			return item
		}
	}
	return nil
}

func (job *agentJob) hasCancelRequest() bool {
	job.mu.Lock()
	defer job.mu.Unlock()
	return job.cancelRequested
}

func (job *agentJob) setItemRunning(item *agentJobItem, agentID string) {
	job.mu.Lock()
	defer job.mu.Unlock()
	item.Status = agentJobItemStatusRunning
	item.AssignedID = agentID
	item.AttemptCount++
	item.StartedAt = time.Now()
	job.UpdatedAt = time.Now()
}

func (job *agentJob) failItem(item *agentJobItem, message string) {
	job.mu.Lock()
	defer job.mu.Unlock()
	if item.Status == agentJobItemStatusCompleted || item.Status == agentJobItemStatusFailed {
		return
	}
	item.Status = agentJobItemStatusFailed
	item.LastError = message
	item.CompletedAt = time.Now()
	job.UpdatedAt = time.Now()
}

type agentJobProgress struct {
	TotalItems     int
	PendingItems   int
	RunningItems   int
	CompletedItems int
	FailedItems    int
}
