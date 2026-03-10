// Package runtimeopt provides Go 1.26 runtime optimizations for high-performance workloads.
//
// Go 1.26 Features:
//   - Green Tea GC (default): 10-40% lower GC overhead
//   - Size-specialized malloc: Up to 30% faster small allocations (1-512 bytes)
//   - Faster cgo calls: ~30% reduction in cgo runtime overhead
//   - SIMD support (experimental): Vectorized operations via GOEXPERIMENT=simd
//
// References:
//   - https://go.dev/doc/go1.26
//   - https://go.dev/blog/go1.26
package runtimeopt

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"time"
)

// Config holds runtime optimization settings.
type Config struct {
	// GOMAXPROCS sets the maximum number of CPUs for Go routines.
	// If 0, uses runtime.NumCPU().
	GOMAXPROCS int

	// GOGC sets the garbage collection target percentage.
	// Default is 100 (Go 1.26 Green Tea GC).
	// Lower values = more frequent GC, less memory.
	// Higher values = less frequent GC, more memory.
	GOGC int

	// GOMEMLIMIT sets a soft memory limit for the GC.
	// Useful for memory-constrained environments.
	// In bytes. 0 = no limit.
	GOMEMLIMIT int64

	// MutexProfileFraction enables mutex profiling.
	// 0 = disabled, 1 = always profile.
	MutexProfileFraction int

	// BlockProfileRate enables block profiling.
	// 0 = disabled, >0 = sample rate.
	BlockProfileRate int
}

// DefaultConfig returns optimized defaults for Go 1.26.
func DefaultConfig() Config {
	return Config{
		GOMAXPROCS:           runtime.NumCPU(),
		GOGC:                 100, // Green Tea GC default (Go 1.26+)
		GOMEMLIMIT:           0,   // No limit by default
		MutexProfileFraction: 0,
		BlockProfileRate:     0,
	}
}

// AggressiveConfig returns aggressive optimizations for maximum throughput.
// Use for high-concurrency workloads (100+ parallel tool calls).
func AggressiveConfig() Config {
	return Config{
		GOMAXPROCS:           runtime.NumCPU(),
		GOGC:                 80, // More frequent GC for lower latency
		GOMEMLIMIT:           0,
		MutexProfileFraction: 0,
		BlockProfileRate:     0,
	}
}

// Apply applies the runtime optimizations.
//
// Go 1.26 Specific:
//   - GOGC=100 enables Green Tea GC (default in Go 1.26+)
//   - Size-specialized malloc is automatic for small allocations
//   - Faster cgo calls are automatic
//
// Universal Go Features:
//   - GOMAXPROCS tuning (available since Go 1.5)
//   - GOMEMLIMIT (available since Go 1.19)
func Apply(cfg Config) {
	// Set GOMAXPROCS
	if cfg.GOMAXPROCS > 0 {
		runtime.GOMAXPROCS(cfg.GOMAXPROCS)
	}

	// Set GOGC (Go 1.26 Green Tea GC)
	if cfg.GOGC > 0 {
		os.Setenv("GOGC", strconv.Itoa(cfg.GOGC))
		debug.SetGCPercent(cfg.GOGC)
	}

	// Set GOMEMLIMIT
	if cfg.GOMEMLIMIT > 0 {
		debug.SetMemoryLimit(cfg.GOMEMLIMIT)
	}

	// Enable profiling if requested
	if cfg.MutexProfileFraction > 0 {
		runtime.SetMutexProfileFraction(cfg.MutexProfileFraction)
	}
	if cfg.BlockProfileRate > 0 {
		runtime.SetBlockProfileRate(cfg.BlockProfileRate)
	}
}

// AutoTune automatically applies optimal settings based on workload type.
func AutoTune(workload string) {
	var cfg Config

	switch workload {
	case "io-bound":
		// I/O bound workloads (file reads, network calls)
		// Can handle more goroutines, lower GC pressure
		cfg = Config{
			GOMAXPROCS: runtime.NumCPU() * 2,
			GOGC:       100,
		}
	case "cpu-bound":
		// CPU bound workloads (computation, parsing)
		// Limit goroutines, optimize for throughput
		cfg = Config{
			GOMAXPROCS: runtime.NumCPU(),
			GOGC:       100,
		}
	case "latency-sensitive":
		// Low-latency workloads (real-time responses)
		// Aggressive GC to minimize pause times
		cfg = Config{
			GOMAXPROCS: runtime.NumCPU(),
			GOGC:       80,
		}
	case "high-concurrency":
		// High concurrency (100+ parallel operations)
		// Balance between throughput and latency
		cfg = AggressiveConfig()
	default:
		cfg = DefaultConfig()
	}

	Apply(cfg)
}

// Stats returns current runtime statistics.
type Stats struct {
	GOMAXPROCS   int
	GOGC         int
	GOMEMLIMIT   int64
	NumGoroutine int
	NumCPU       int
	MemStats     runtime.MemStats
	GCStats      debug.GCStats
}

// GetStats returns current runtime statistics.
func GetStats() Stats {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	var gcs debug.GCStats
	debug.ReadGCStats(&gcs)

	gogc := 100
	if val := os.Getenv("GOGC"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			gogc = parsed
		}
	}

	return Stats{
		GOMAXPROCS:   runtime.GOMAXPROCS(0),
		GOGC:         gogc,
		GOMEMLIMIT:   debug.SetMemoryLimit(-1),
		NumGoroutine: runtime.NumGoroutine(),
		NumCPU:       runtime.NumCPU(),
		MemStats:     ms,
		GCStats:      gcs,
	}
}

// ForceGC triggers an immediate garbage collection.
// Use sparingly - Go 1.26 Green Tea GC is very efficient.
func ForceGC() {
	runtime.GC()
}

// SetMaxThreads sets the maximum number of OS threads.
// Default is 10000. Only reduce if you have specific reasons.
func SetMaxThreads(max int) {
	debug.SetMaxThreads(max)
}

// EnableMemoryLogging enables periodic memory usage logging.
// Useful for debugging memory leaks.
func EnableMemoryLogging(intervalSeconds int) {
	go func() {
		ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			var ms runtime.MemStats
			runtime.ReadMemStats(&ms)
			fmt.Printf("[MEM] Alloc=%d MB, Sys=%d MB, NumGC=%d, Goroutines=%d\n",
				ms.Alloc/1024/1024,
				ms.Sys/1024/1024,
				ms.NumGC,
				runtime.NumGoroutine(),
			)
		}
	}()
}
