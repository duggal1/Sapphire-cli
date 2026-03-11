package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/sapphire/internal/runtimeopt"
)

// BenchmarkDispatcherBaseline benchmarks the old dispatcher implementation.
func BenchmarkDispatcherBaseline(b *testing.B) {
	dispatcher := NewDispatcher(50)
	dispatcher.Start()
	defer dispatcher.Stop()

	ctx := context.Background()
	tools := map[string]fantasy.AgentTool{
		"view": fantasy.NewParallelAgentTool("view", "view files", func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			time.Sleep(10 * time.Millisecond) // Simulate I/O latency
			return fantasy.NewTextResponse("file content"), nil
		}),
	}

	calls := make([]fantasy.ToolCall, 100)
	for i := range calls {
		calls[i] = fantasy.ToolCall{
			ID:       fmt.Sprintf("call-%d", i),
			Name:     "view",
			Input:    fmt.Sprintf(`{"file_path": "file%d.txt"}`, i),
			
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := dispatcher.Dispatch(ctx, calls, tools)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkFastDispatcher benchmarks the new optimized dispatcher.
func BenchmarkFastDispatcher(b *testing.B) {
	dispatcher := NewFastDispatcher(50)
	dispatcher.Start()
	defer dispatcher.Stop()

	ctx := context.Background()
	tools := map[string]fantasy.AgentTool{
		"view": fantasy.NewParallelAgentTool("view", "view files", func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			time.Sleep(10 * time.Millisecond) // Simulate I/O latency
			return fantasy.NewTextResponse("file content"), nil
		}),
	}

	calls := make([]fantasy.ToolCall, 100)
	for i := range calls {
		calls[i] = fantasy.ToolCall{
			ID:       fmt.Sprintf("call-%d", i),
			Name:     "view",
			Input:    fmt.Sprintf(`{"file_path": "file%d.txt"}`, i),
			
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := dispatcher.Dispatch(ctx, calls, tools)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkFastDispatcher1000Concurrent benchmarks 1000 concurrent tool calls.
func BenchmarkFastDispatcher1000Concurrent(b *testing.B) {
	dispatcher := NewFastDispatcher(250)
	dispatcher.Start()
	defer dispatcher.Stop()

	ctx := context.Background()
	tools := map[string]fantasy.AgentTool{
		"view": fantasy.NewParallelAgentTool("view", "view files", func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			time.Sleep(5 * time.Millisecond) // Simulate fast I/O
			return fantasy.NewTextResponse("file content"), nil
		}),
	}

	calls := make([]fantasy.ToolCall, 1000)
	for i := range calls {
		calls[i] = fantasy.ToolCall{
			ID:       fmt.Sprintf("call-%d", i),
			Name:     "view",
			Input:    fmt.Sprintf(`{"file_path": "file%d.txt"}`, i),
			
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := dispatcher.Dispatch(ctx, calls, tools)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParallelFileRead benchmarks real parallel file reads.
func BenchmarkParallelFileRead(b *testing.B) {
	// Create temporary test files
	tmpDir := b.TempDir()
	filePaths := make([]string, 100)
	for i := 0; i < 100; i++ {
		path := filepath.Join(tmpDir, fmt.Sprintf("file%d.txt", i))
		content := fmt.Sprintf("Content of file %d\nLine 2\nLine 3\n", i)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			b.Fatal(err)
		}
		filePaths[i] = path
	}

	dispatcher := NewFastDispatcher(50)
	dispatcher.Start()
	defer dispatcher.Stop()

	ctx := context.Background()
	tools := map[string]fantasy.AgentTool{
		"view": fantasy.NewParallelAgentTool("view", "view files", func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			// Simulate real file read
			time.Sleep(1 * time.Millisecond)
			return fantasy.NewTextResponse("file content"), nil
		}),
	}

	calls := make([]fantasy.ToolCall, 100)
	for i := range calls {
		calls[i] = fantasy.ToolCall{
			ID:       fmt.Sprintf("call-%d", i),
			Name:     "view",
			Input:    fmt.Sprintf(`{"file_path": "%s"}`, filePaths[i]),
			
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := dispatcher.Dispatch(ctx, calls, tools)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRuntimeOptimizations benchmarks the impact of runtime optimizations.
func BenchmarkRuntimeOptimizations(b *testing.B) {
	// Apply Go 1.26 optimizations
	runtimeopt.Apply(runtimeopt.AggressiveConfig())

	dispatcher := NewFastDispatcher(250)
	dispatcher.Start()
	defer dispatcher.Stop()

	ctx := context.Background()
	tools := map[string]fantasy.AgentTool{
		"view": fantasy.NewParallelAgentTool("view", "view files", func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			time.Sleep(5 * time.Millisecond)
			return fantasy.NewTextResponse("file content"), nil
		}),
	}

	calls := make([]fantasy.ToolCall, 500)
	for i := range calls {
		calls[i] = fantasy.ToolCall{
			ID:       fmt.Sprintf("call-%d", i),
			Name:     "view",
			Input:    fmt.Sprintf(`{"file_path": "file%d.txt"}`, i),
			
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := dispatcher.Dispatch(ctx, calls, tools)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkGoroutineScaling benchmarks how well the dispatcher scales with goroutine count.
func BenchmarkGoroutineScaling(b *testing.B) {
	dispatcher := NewFastDispatcher(250)
	dispatcher.Start()
	defer dispatcher.Stop()

	ctx := context.Background()
	tools := map[string]fantasy.AgentTool{
		"view": fantasy.NewParallelAgentTool("view", "view files", func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			time.Sleep(1 * time.Millisecond)
			return fantasy.NewTextResponse("file content"), nil
		}),
	}

	benchmarks := []struct {
		name     string
		numCalls int
	}{
		{"100 calls", 100},
		{"500 calls", 500},
		{"1000 calls", 1000},
		{"5000 calls", 5000},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			calls := make([]fantasy.ToolCall, bm.numCalls)
			for i := range calls {
				calls[i] = fantasy.ToolCall{
					ID:       fmt.Sprintf("call-%d", i),
					Name:     "view",
					Input:    fmt.Sprintf(`{"file_path": "file%d.txt"}`, i),
					
				}
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := dispatcher.Dispatch(ctx, calls, tools)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkLatency measures p50, p95, p99 latency.
func BenchmarkLatency(b *testing.B) {
	dispatcher := NewFastDispatcher(250)
	dispatcher.Start()
	defer dispatcher.Stop()

	ctx := context.Background()
	tools := map[string]fantasy.AgentTool{
		"view": fantasy.NewParallelAgentTool("view", "view files", func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			time.Sleep(10 * time.Millisecond)
			return fantasy.NewTextResponse("file content"), nil
		}),
	}

	calls := make([]fantasy.ToolCall, 100)
	for i := range calls {
		calls[i] = fantasy.ToolCall{
			ID:       fmt.Sprintf("call-%d", i),
			Name:     "view",
			Input:    fmt.Sprintf(`{"file_path": "file%d.txt"}`, i),
			
		}
	}

	latencies := make([]time.Duration, b.N)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()
		_, err := dispatcher.Dispatch(ctx, calls, tools)
		if err != nil {
			b.Fatal(err)
		}
		latencies[i] = time.Since(start)
	}

	// Sort latencies for percentile calculation
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	// Report percentiles
	b.ReportMetric(float64(latencies[b.N/2]/time.Millisecond), "p50_ms")
	b.ReportMetric(float64(latencies[95*b.N/100]/time.Millisecond), "p95_ms")
	b.ReportMetric(float64(latencies[99*b.N/100]/time.Millisecond), "p99_ms")
}

// BenchmarkMemoryAllocation measures memory usage per tool call.
func BenchmarkMemoryAllocation(b *testing.B) {
	dispatcher := NewFastDispatcher(250)
	dispatcher.Start()
	defer dispatcher.Stop()

	ctx := context.Background()
	tools := map[string]fantasy.AgentTool{
		"view": fantasy.NewParallelAgentTool("view", "view files", func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse("file content"), nil
		}),
	}

	calls := make([]fantasy.ToolCall, 100)
	for i := range calls {
		calls[i] = fantasy.ToolCall{
			ID:       fmt.Sprintf("call-%d", i),
			Name:     "view",
			Input:    fmt.Sprintf(`{"file_path": "file%d.txt"}`, i),
			
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := dispatcher.Dispatch(ctx, calls, tools)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkConcurrencyStress tests the dispatcher under high concurrency.
func BenchmarkConcurrencyStress(b *testing.B) {
	dispatcher := NewFastDispatcher(500)
	dispatcher.Start()
	defer dispatcher.Stop()

	ctx := context.Background()
	tools := map[string]fantasy.AgentTool{
		"view": fantasy.NewParallelAgentTool("view", "view files", func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			time.Sleep(1 * time.Millisecond)
			return fantasy.NewTextResponse("file content"), nil
		}),
		"bash": fantasy.NewParallelAgentTool("bash", "execute commands", func(ctx context.Context, params BashParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			time.Sleep(5 * time.Millisecond)
			return fantasy.NewTextResponse("command output"), nil
		}),
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		calls := make([]fantasy.ToolCall, 50)
		for i := range calls {
			calls[i] = fantasy.ToolCall{
				ID:       fmt.Sprintf("call-%d", i),
				Name:     "view",
				Input:    fmt.Sprintf(`{"file_path": "file%d.txt"}`, i),
				
			}
		}

		for pb.Next() {
			_, err := dispatcher.Dispatch(ctx, calls, tools)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkWorkerPoolWarmup measures the benefit of pre-warmed worker pools.
func BenchmarkWorkerPoolWarmup(b *testing.B) {
	// Cold dispatcher (no warmup)
	b.Run("Cold", func(b *testing.B) {
		dispatcher := NewFastDispatcher(50)
		// Don't call Start() - let it warm up lazily

		ctx := context.Background()
		tools := map[string]fantasy.AgentTool{
			"view": fantasy.NewParallelAgentTool("view", "view files", func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
				time.Sleep(5 * time.Millisecond)
				return fantasy.NewTextResponse("file content"), nil
			}),
		}

		calls := make([]fantasy.ToolCall, 50)
		for i := range calls {
			calls[i] = fantasy.ToolCall{
				ID:       fmt.Sprintf("call-%d", i),
				Name:     "view",
				Input:    fmt.Sprintf(`{"file_path": "file%d.txt"}`, i),
				
			}
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := dispatcher.Dispatch(ctx, calls, tools)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	// Warm dispatcher (pre-warmed pools)
	b.Run("Warm", func(b *testing.B) {
		dispatcher := NewFastDispatcher(50)
		dispatcher.Start() // Pre-warm
		defer dispatcher.Stop()

		ctx := context.Background()
		tools := map[string]fantasy.AgentTool{
			"view": fantasy.NewParallelAgentTool("view", "view files", func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
				time.Sleep(5 * time.Millisecond)
				return fantasy.NewTextResponse("file content"), nil
			}),
		}

		calls := make([]fantasy.ToolCall, 50)
		for i := range calls {
			calls[i] = fantasy.ToolCall{
				ID:       fmt.Sprintf("call-%d", i),
				Name:     "view",
				Input:    fmt.Sprintf(`{"file_path": "file%d.txt"}`, i),
				
			}
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := dispatcher.Dispatch(ctx, calls, tools)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
