package memory

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

type IndexedFileInfo struct {
	Path        string
	Language    string
	Role        string
	Status      string
	SymbolCount int
}

func (c *Compiler) ListIndexedFiles(ctx context.Context, workingDir string) (IndexStatus, []IndexedFileInfo, error) {
	if c == nil || c.q == nil {
		return IndexStatus{}, nil, fmt.Errorf("memory compiler is not initialized")
	}
	status, err := c.IndexStatus(ctx, workingDir)
	if err != nil {
		return IndexStatus{}, nil, err
	}
	if !status.Available {
		return status, nil, fmt.Errorf("durable codebase graph is not available")
	}

	snapshot, err := captureRepoSnapshot(ctx, workingDir)
	if err != nil {
		return IndexStatus{}, nil, err
	}
	scope, err := c.loadExistingScope(ctx, snapshot)
	if err != nil {
		return IndexStatus{}, nil, err
	}

	rows, err := c.q.ListMemoryRepoFilesByScope(ctx, scope.ID)
	if err != nil {
		return IndexStatus{}, nil, err
	}
	files := make([]IndexedFileInfo, 0, len(rows))
	for _, row := range rows {
		files = append(files, IndexedFileInfo{
			Path:        strings.TrimSpace(row.Path),
			Language:    strings.TrimSpace(row.Language),
			Role:        strings.TrimSpace(row.Role),
			Status:      strings.TrimSpace(row.Status),
			SymbolCount: int(row.SymbolCount),
		})
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})
	if status.LastIndexedAt.IsZero() && scope.LastIndexedAt > 0 {
		status.LastIndexedAt = time.Unix(scope.LastIndexedAt, 0).UTC()
	}
	return status, files, nil
}
