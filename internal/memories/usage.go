package memories

import "sync"

type UsageSnapshot struct {
	ReadsByKind map[string]int
	ReadsByPath map[string]int
}

type UsageTracker struct {
	mu          sync.Mutex
	readsByKind map[string]int
	readsByPath map[string]int
}

func NewUsageTracker() *UsageTracker {
	return &UsageTracker{
		readsByKind: make(map[string]int),
		readsByPath: make(map[string]int),
	}
}

func (u *UsageTracker) Record(kind, path string) {
	if u == nil {
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	u.readsByKind[kind]++
	if path != "" {
		u.readsByPath[path]++
	}
}

func (u *UsageTracker) Snapshot() UsageSnapshot {
	if u == nil {
		return UsageSnapshot{}
	}
	u.mu.Lock()
	defer u.mu.Unlock()

	kinds := make(map[string]int, len(u.readsByKind))
	for k, v := range u.readsByKind {
		kinds[k] = v
	}
	paths := make(map[string]int, len(u.readsByPath))
	for k, v := range u.readsByPath {
		paths[k] = v
	}
	return UsageSnapshot{ReadsByKind: kinds, ReadsByPath: paths}
}
