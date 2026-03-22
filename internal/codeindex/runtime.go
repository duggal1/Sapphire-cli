package codeindex

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type qdrantRuntime struct {
	storageDir string
	baseURL    string

	mu      sync.Mutex
	logFile *os.File
}

func (r *qdrantRuntime) Start(ctx context.Context) error {
	_ = ctx
	return errBundledQdrantDisabled()
}

func (r *qdrantRuntime) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.logFile != nil {
		_ = r.logFile.Close()
		r.logFile = nil
	}
	return nil
}

func (r *qdrantRuntime) ensureBinary() (string, error) {
	return "", errBundledQdrantDisabled()
}

func (r *qdrantRuntime) openLogFile() (*os.File, error) {
	if err := ensureDir(r.storageDir); err != nil {
		return nil, err
	}
	logPath := filepath.Join(r.storageDir, "qdrant.log")
	return os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
}

func bundledQdrantBinary() (string, []byte, error) {
	return "", nil, errBundledQdrantDisabled()
}

func errBundledQdrantDisabled() error {
	return fmt.Errorf("code index: bundled qdrant runtime is disabled")
}
