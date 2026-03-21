package codeindex

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
)

//go:embed bin/qdrant-linux-amd64
var qdrantLinuxAMD64 []byte

//go:embed bin/qdrant-darwin-arm64
var qdrantDarwinARM64 []byte

type qdrantRuntime struct {
	storageDir string
	baseURL    string

	mu      sync.Mutex
	cmd     *exec.Cmd
	logFile *os.File
}

func (r *qdrantRuntime) Start(ctx context.Context) error {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cmd != nil && r.cmd.Process != nil {
		return nil
	}

	binaryPath, err := r.ensureBinary()
	if err != nil {
		return err
	}
	logFile, err := r.openLogFile()
	if err != nil {
		return err
	}

	cmd := exec.Command(binaryPath, "--disable-telemetry")
	cmd.Env = append(os.Environ(),
		"QDRANT__STORAGE__STORAGE_PATH="+filepath.Join(r.storageDir, "storage"),
		"QDRANT__STORAGE__SNAPSHOTS_PATH="+filepath.Join(r.storageDir, "snapshots"),
		"QDRANT__SERVICE__HOST=127.0.0.1",
		"QDRANT__SERVICE__HTTP_PORT="+defaultQdrantPort,
		"QDRANT__SERVICE__GRPC_PORT="+defaultQdrantGRPC,
	)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return fmt.Errorf("code index: start bundled qdrant: %w", err)
	}

	r.cmd = cmd
	r.logFile = logFile
	go r.wait(cmd, logFile)
	return nil
}

func (r *qdrantRuntime) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var closeErr error
	if r.cmd != nil && r.cmd.Process != nil {
		if err := r.cmd.Process.Kill(); err != nil && !isProcessDone(err) {
			closeErr = err
		}
	}
	return closeErr
}

func (r *qdrantRuntime) wait(cmd *exec.Cmd, logFile *os.File) {
	_ = cmd.Wait()
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cmd == cmd {
		r.cmd = nil
	}
	if r.logFile == logFile {
		_ = r.logFile.Close()
		r.logFile = nil
	}
}

func (r *qdrantRuntime) ensureBinary() (string, error) {
	if err := ensureDir(r.storageDir); err != nil {
		return "", err
	}
	runtimeDir := filepath.Join(r.storageDir, "runtime")
	if err := ensureDir(runtimeDir); err != nil {
		return "", err
	}

	binaryName, raw, err := bundledQdrantBinary()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:8])
	targetPath := filepath.Join(runtimeDir, binaryName+"-"+digest)
	if info, err := os.Stat(targetPath); err == nil && info.Size() == int64(len(raw)) {
		return targetPath, nil
	}
	if err := os.WriteFile(targetPath, raw, 0o755); err != nil {
		return "", fmt.Errorf("code index: write bundled qdrant binary: %w", err)
	}
	if err := os.Chmod(targetPath, 0o755); err != nil {
		return "", fmt.Errorf("code index: chmod bundled qdrant binary: %w", err)
	}
	return targetPath, nil
}

func (r *qdrantRuntime) openLogFile() (*os.File, error) {
	logPath := filepath.Join(r.storageDir, "qdrant.log")
	return os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
}

func bundledQdrantBinary() (string, []byte, error) {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "darwin/arm64":
		return "qdrant-darwin-arm64", qdrantDarwinARM64, nil
	case "linux/amd64":
		return "qdrant-linux-amd64", qdrantLinuxAMD64, nil
	default:
		return "", nil, fmt.Errorf("code index: no bundled qdrant binary for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
}

func isProcessDone(err error) bool {
	return errors.Is(err, os.ErrProcessDone)
}
