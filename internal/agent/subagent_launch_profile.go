package agent

import (
	"context"
	"runtime"
	"sync"
	"syscall"
	"time"
)

type subAgentFactoryFunc func(ctx context.Context, workDir string, normalizedManifest []string, opts spawnAgentOptions) (SessionAgent, error)

type subAgentLaunchProbe interface {
	ObserveSubAgentStep(name string, duration time.Duration)
	AddSubAgentCounter(name string, delta int64)
}

type subAgentLaunchMetrics struct {
	mu       sync.Mutex
	Steps    map[string]time.Duration `json:"steps"`
	Counters map[string]int64         `json:"counters"`
}

func newSubAgentLaunchMetrics() *subAgentLaunchMetrics {
	return &subAgentLaunchMetrics{
		Steps:    make(map[string]time.Duration),
		Counters: make(map[string]int64),
	}
}

func (m *subAgentLaunchMetrics) ObserveSubAgentStep(name string, duration time.Duration) {
	if m == nil || name == "" {
		return
	}
	m.mu.Lock()
	m.Steps[name] += duration
	m.mu.Unlock()
}

func (m *subAgentLaunchMetrics) AddSubAgentCounter(name string, delta int64) {
	if m == nil || name == "" || delta == 0 {
		return
	}
	m.mu.Lock()
	m.Counters[name] += delta
	m.mu.Unlock()
}

func (m *subAgentLaunchMetrics) snapshot() (map[string]time.Duration, map[string]int64) {
	if m == nil {
		return nil, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	steps := make(map[string]time.Duration, len(m.Steps))
	for key, value := range m.Steps {
		steps[key] = value
	}
	counters := make(map[string]int64, len(m.Counters))
	for key, value := range m.Counters {
		counters[key] = value
	}
	return steps, counters
}

type subAgentLaunchRuntimeSnapshot struct {
	Timestamp   time.Time     `json:"timestamp"`
	UserCPU     time.Duration `json:"user_cpu"`
	SystemCPU   time.Duration `json:"system_cpu"`
	HeapAlloc   uint64        `json:"heap_alloc"`
	HeapObjects uint64        `json:"heap_objects"`
	TotalAlloc  uint64        `json:"total_alloc"`
	NumGC       uint32        `json:"num_gc"`
	Goroutines  int           `json:"goroutines"`
	LiveMallocs uint64        `json:"live_mallocs"`
	LiveFrees   uint64        `json:"live_frees"`
}

type SubAgentLaunchProfile struct {
	Before   subAgentLaunchRuntimeSnapshot `json:"before"`
	After    subAgentLaunchRuntimeSnapshot `json:"after"`
	Steps    map[string]time.Duration      `json:"steps"`
	Counters map[string]int64              `json:"counters"`
}

func captureSubAgentLaunchRuntimeSnapshot() subAgentLaunchRuntimeSnapshot {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	snapshot := subAgentLaunchRuntimeSnapshot{
		Timestamp:   time.Now().UTC(),
		HeapAlloc:   mem.HeapAlloc,
		HeapObjects: mem.HeapObjects,
		TotalAlloc:  mem.TotalAlloc,
		NumGC:       mem.NumGC,
		Goroutines:  runtime.NumGoroutine(),
		LiveMallocs: mem.Mallocs,
		LiveFrees:   mem.Frees,
	}
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err == nil {
		snapshot.UserCPU = time.Duration(usage.Utime.Sec)*time.Second + time.Duration(usage.Utime.Usec)*time.Microsecond
		snapshot.SystemCPU = time.Duration(usage.Stime.Sec)*time.Second + time.Duration(usage.Stime.Usec)*time.Microsecond
	}
	return snapshot
}

func (c *coordinator) observeSubAgentLaunchStep(name string, startedAt time.Time) {
	if c == nil || c.subAgentLaunchProbe == nil || name == "" || startedAt.IsZero() {
		return
	}
	c.subAgentLaunchProbe.ObserveSubAgentStep(name, time.Since(startedAt))
}

func (c *coordinator) countSubAgentLaunchMetric(name string, delta int64) {
	if c == nil || c.subAgentLaunchProbe == nil || name == "" || delta == 0 {
		return
	}
	c.subAgentLaunchProbe.AddSubAgentCounter(name, delta)
}
