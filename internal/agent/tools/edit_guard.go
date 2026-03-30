package tools

import (
	"fmt"
	"slices"
	"strings"
	"sync"
)

// EditGuard tracks full-file reads and blocks unrelated edits while a session
// still has files with unresolved diagnostics.
type EditGuard struct {
	mu     sync.Mutex
	viewed map[string]map[string]bool
	dirty  map[string]map[string]struct{}
}

func NewEditGuard() *EditGuard {
	return &EditGuard{
		viewed: make(map[string]map[string]bool),
		dirty:  make(map[string]map[string]struct{}),
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

	if dirty := g.dirty[sessionID]; len(dirty) > 0 {
		if _, ok := dirty[filePath]; !ok {
			return fmt.Errorf("edit blocked: fix all current-file errors and warnings in %s before editing %s", formatDirtyFiles(dirty), filePath)
		}
	}

	return nil
}

func (g *EditGuard) SetLockedIfErrors(sessionID, filePath string, hasErrors bool) {
	if g == nil || sessionID == "" || filePath == "" {
		return
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if g.dirty[sessionID] == nil {
		g.dirty[sessionID] = make(map[string]struct{})
	}

	if hasErrors {
		g.dirty[sessionID][filePath] = struct{}{}
		return
	}

	delete(g.dirty[sessionID], filePath)
	if len(g.dirty[sessionID]) == 0 {
		delete(g.dirty, sessionID)
	}
}

func formatDirtyFiles(dirty map[string]struct{}) string {
	files := make([]string, 0, len(dirty))
	for file := range dirty {
		files = append(files, file)
	}
	slices.Sort(files)
	return strings.Join(files, ", ")
}
