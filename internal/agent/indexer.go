package agent

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/duggal1/Sapphire-cli/internal/agent/memory"
	"github.com/duggal1/Sapphire-cli/internal/db"
	"github.com/duggal1/Sapphire-cli/internal/lsp"
)

type Indexer struct {
	workingDir string
	lspManager *lsp.Manager
	mem        memory.MemoryService
	mu         sync.Mutex
	indexed    map[string]time.Time
	isBusy     func() bool
}

func NewIndexer(workingDir string, lspManager *lsp.Manager, mem memory.MemoryService, isBusy func() bool) *Indexer {
	return &Indexer{
		workingDir: workingDir,
		lspManager: lspManager,
		mem:        mem,
		indexed:    make(map[string]time.Time),
		isBusy:     isBusy,
	}
}

// Start runs the periodic indexing loop.
func (idx *Indexer) Start(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	// Run initial index
	go idx.IndexAll(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			go idx.IndexAll(ctx)
		}
	}
}

func (idx *Indexer) IndexAll(ctx context.Context) {
	if idx.isBusy != nil && idx.isBusy() {
		slog.Debug("Indexing skipped; agent busy")
		return
	}

	var errIndexingPaused = errors.New("indexing paused")
	slog.Info("Starting proactive codebase indexing...")
	err := filepath.Walk(idx.workingDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if idx.isBusy != nil && idx.isBusy() {
			return errIndexingPaused
		}
		// Skip non-code files and ignored dirs
		if strings.Contains(path, "/.git/") || strings.Contains(path, "/vendor/") || strings.Contains(path, "/node_modules/") {
			return nil
		}

		ext := filepath.Ext(path)
		if ext != ".go" && ext != ".ts" && ext != ".tsx" && ext != ".js" && ext != ".py" {
			return nil
		}

		idx.IndexFile(ctx, path)
		return nil
	})
	if err != nil && !errors.Is(err, errIndexingPaused) {
		slog.Error("Indexing failed", "error", err)
	}
	slog.Info("Proactive indexing complete.")
}

var symbolRegex = regexp.MustCompile(`(?m)^(?:func|type|class|interface|const|var)\s+([A-Z][a-zA-Z0-9_]*)`)

func (idx *Indexer) IndexFile(ctx context.Context, path string) {
	if idx.isBusy != nil && idx.isBusy() {
		return
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return
	}

	matches := symbolRegex.FindAllStringSubmatch(string(content), -1)
	if len(matches) == 0 {
		return
	}

	// Try to get LSP client for high-signal data
	client, _ := idx.lspManager.GetClientFor(path)

	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		symbol := m[1]
		if idx.isBusy != nil && idx.isBusy() {
			return
		}

		// If we have LSP, try to get more info via hover or doc
		doc := ""
		signature := ""
		if client != nil {
			// Find offset of symbol in content (simple approach)
			offset := strings.Index(string(content), symbol)
			if offset != -1 {
				// Convert offset to Line/Col if needed, but for now we just use the name
				// Probing with Hover at the symbol position
				lines := strings.Split(string(content[:offset]), "\n")
				line := len(lines) - 1
				character := len(lines[len(lines)-1])

				hover, err := client.RequestHover(ctx, path, line, character)
				if err == nil && hover != "" {
					doc = hover
				}
			}
		}

		err := idx.mem.UpsertCodebaseKnowledge(ctx, db.UpsertCodebaseKnowledgeParams{
			FilePath:      path,
			SymbolName:    symbol,
			SymbolType:    "identifier", // generic, refined by regex later
			Signature:     sql.NullString{String: signature, Valid: signature != ""},
			Documentation: sql.NullString{String: doc, Valid: doc != ""},
		})
		if err != nil {
			slog.Debug("Failed to save symbol knowledge", "symbol", symbol, "error", err)
		}
	}
}
