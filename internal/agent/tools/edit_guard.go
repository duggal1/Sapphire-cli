package tools

import (
	"errors"
	"sync"
)

var errEditRequiresView = errors.New("edit blocked: file must be viewed before editing")

// EditGuard enforces read-before-edit. Diagnostics are reported after edits,
// but they do not hard-block coordinated follow-up edits across files.
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

	if isEdit {
		if sessionViews, ok := g.viewed[sessionID]; !ok || !sessionViews[filePath] {
			return errEditRequiresView
		}
	}
	return nil
}

func (g *EditGuard) SetLockedIfErrors(sessionID, filePath string, hasErrors bool) {
	_ = g
	_ = sessionID
	_ = filePath
	_ = hasErrors
}
