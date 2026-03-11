package ratelimiter

import (
	"sync"
	"time"
)

// Bucket represents a single token bucket rate limiter for a specific task type.
type Bucket struct {
	mu           sync.Mutex
	rate         float64 // Tokens added per second
	capacity     float64 // Maximum tokens the bucket can hold
	tokens       float64 // Current available tokens
	lastRefilled time.Time
}

// Manager manages multiple buckets by task type.
type Manager struct {
	mu      sync.RWMutex
	buckets map[string]*Bucket
}

// NewManager creates a new RateLimiter manager.
func NewManager() *Manager {
	return &Manager{
		buckets: make(map[string]*Bucket),
	}
}

// GetBucket retrieves or creates a bucket for a given task type.
func (m *Manager) GetBucket(taskType string, rate float64, capacity float64) *Bucket {
	m.mu.RLock()
	b, ok := m.buckets[taskType]
	m.mu.RUnlock()
	if ok {
		return b
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	// Double-check after acquiring write lock
	if b, ok = m.buckets[taskType]; ok {
		return b
	}

	b = &Bucket{
		rate:         rate,
		capacity:     capacity,
		tokens:       capacity, // Start full
		lastRefilled: time.Now(),
	}
	m.buckets[taskType] = b
	return b
}

// Allow checks if a request is allowed for the bucket.
func (b *Bucket) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastRefilled).Seconds()
	
	// Add tokens based on elapsed time
	b.tokens += elapsed * b.rate
	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}
	b.lastRefilled = now

	// Consume a token if available
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}
