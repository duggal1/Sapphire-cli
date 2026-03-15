package tools

import "sync"

// EditGuard tracks full-file reads for prompt/runtime coordination, but it no
// longer hard-blocks edit execution. Read-before-edit remains a tool contract
// preference, not a fatal runtime gate.
type EditGuard struct {
	mu     sync.Mutex
	viewed map[string]map[string]bool
}

func NewEditGuard() *EditGuard {
	return &EditGuard{
		viewed: make(map[string]map[string]bool),
	}
}

func (g *EditGuard) RecordView(sessionID, filePath string, fullFile bool) {
	if g == nil || sessionID == "" || filePath == "" {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.viewed[sessionID] == nil {
		g.viewed[sessionID] = make(map[string]bool)
	}
	if fullFile {
		g.viewed[sessionID][filePath] = true
	}
}

func (g *EditGuard) EnsureAllowed(sessionID, filePath string, isEdit bool) error {
	if g == nil || sessionID == "" || filePath == "" {
		return nil
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if g.viewed[sessionID] == nil {
		g.viewed[sessionID] = make(map[string]bool)
	}
	if isEdit {
		// Self-heal unread edit attempts instead of failing the turn. This keeps
		// the active path reliable even when the model chooses edit before view.
		g.viewed[sessionID][filePath] = true
	}

	return nil
}

func (g *EditGuard) SetLockedIfErrors(sessionID, filePath string, hasErrors bool) {
	_ = g
	_ = sessionID
	_ = filePath
	_ = hasErrors
}
