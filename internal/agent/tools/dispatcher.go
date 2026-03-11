package tools

import (
	"context"
	"sync"
	"sync/atomic"

	"charm.land/fantasy"
	"golang.org/x/sync/errgroup"
)

// Dispatcher handles the parallel execution of tool calls.
// It uses a bounded worker pool to prevent resource exhaustion.
type Dispatcher struct {
	workers    int
	jobQueue   chan toolJob
	activeJobs sync.Map
	running    atomic.Bool
	wg         sync.WaitGroup
}

type toolJob struct {
	ctx      context.Context
	call     fantasy.ToolCall
	tool     fantasy.AgentTool
	resultCh chan toolResult
}

type toolResult struct {
	resp fantasy.ToolResponse
	err  error
}

// NewDispatcher creates a new tool dispatcher with the specified number of workers.
func NewDispatcher(workers int) *Dispatcher {
	if workers <= 0 {
		workers = 120 // Default matching MaxBackgroundJobs
	}
	d := &Dispatcher{
		workers:  workers,
		jobQueue: make(chan toolJob, 1024),
	}
	return d
}

// Start warms up the worker pool.
func (d *Dispatcher) Start() {
	if d.running.Swap(true) {
		return
	}
	d.wg.Add(d.workers)
	for i := 0; i < d.workers; i++ {
		go d.worker()
	}
}

// Stop gracefully shuts down the worker pool.
func (d *Dispatcher) Stop() {
	if !d.running.Swap(false) {
		return
	}
	close(d.jobQueue)
	d.wg.Wait()
}

func (d *Dispatcher) worker() {
	defer d.wg.Done()
	for job := range d.jobQueue {
		// Store job in activeJobs for tracking/cancellation
		d.activeJobs.Store(job.call.ID, job)

		// Execute the tool call
		// Note: We don't use zero-copy here yet as fantasy tool calls
		// are already structured. But we ensure no extra allocations.
		resp, err := job.tool.Run(job.ctx, job.call)

		job.resultCh <- toolResult{resp: resp, err: err}
		d.activeJobs.Delete(job.call.ID)
	}
}

// Dispatch executes multiple tool calls in parallel using the worker pool.
func (d *Dispatcher) Dispatch(ctx context.Context, calls []fantasy.ToolCall, tools map[string]fantasy.AgentTool) ([]fantasy.ToolResponse, error) {
	if len(calls) == 0 {
		return nil, nil
	}

	g, groupCtx := errgroup.WithContext(ctx)
	results := make([]fantasy.ToolResponse, len(calls))

	// Start dispatcher if not already running
	d.Start()

	for i, call := range calls {
		i, call := i, call
		tool, ok := tools[call.Name]
		if !ok {
			results[i] = fantasy.NewTextErrorResponse("tool not found: " + call.Name)
			continue
		}

		resultCh := make(chan toolResult, 1)

		g.Go(func() error {
			select {
			case d.jobQueue <- toolJob{
				ctx:      groupCtx,
				call:     call,
				tool:     tool,
				resultCh: resultCh,
			}:
			case <-groupCtx.Done():
				return groupCtx.Err()
			}

			select {
			case res := <-resultCh:
				results[i] = res.resp
				return res.err
			case <-groupCtx.Done():
				return groupCtx.Err()
			}
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return results, nil
}
