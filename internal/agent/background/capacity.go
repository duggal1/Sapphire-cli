package background

import "context"

type CapacityController struct {
	semaphore chan struct{}
}

func NewCapacityController(maxConcurrent int) *CapacityController {
	if maxConcurrent <= 0 {
		maxConcurrent = 5
	}
	return &CapacityController{semaphore: make(chan struct{}, maxConcurrent)}
}

func (c *CapacityController) Acquire(ctx context.Context) error {
	if c == nil || c.semaphore == nil {
		return nil
	}
	select {
	case c.semaphore <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *CapacityController) Release() {
	if c == nil || c.semaphore == nil {
		return
	}
	select {
	case <-c.semaphore:
	default:
	}
}

func (c *CapacityController) ActiveCount() int {
	if c == nil || c.semaphore == nil {
		return 0
	}
	return len(c.semaphore)
}

func (c *CapacityController) MaxConcurrent() int {
	if c == nil || c.semaphore == nil {
		return 0
	}
	return cap(c.semaphore)
}
