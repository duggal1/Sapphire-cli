package tools

import (
	"fmt"
	"sync"
)

// EditGuard enforces error-first editing by blocking edits to other files
// until the current file is error-free.
type EditGuard struct {
	mu     sync.Mutex
	locked map[string]string
}

func NewEditGuard() *EditGuard {
	return &EditGuard{
		locked: make(map[string]string),
	}
}

func (g *EditGuard) EnsureAllowed(sessionID, filePath string) error {
	if g == nil || sessionID == "" || filePath == "" {
		return nil
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if locked, ok := g.locked[sessionID]; ok && locked != "" && locked != filePath {
		return fmt.Errorf("edits are blocked until errors in %s are resolved", locked)
	}
	return nil
}

func (g *EditGuard) SetLockedIfErrors(sessionID, filePath string, hasErrors bool) {
	if g == nil || sessionID == "" || filePath == "" {
		return
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if hasErrors {
		g.locked[sessionID] = filePath
		return
	}

	if locked, ok := g.locked[sessionID]; ok && locked == filePath {
		delete(g.locked, sessionID)
	}
}
