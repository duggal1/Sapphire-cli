package codeindex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type ollamaRuntime struct {
	baseURL string
	dataDir string

	client  *http.Client
	mu      sync.Mutex
	cmd     *exec.Cmd
	logFile *os.File
}

func newOllamaRuntime(baseURL, dataDir string) *ollamaRuntime {
	return &ollamaRuntime{
		baseURL: normalizeOllamaURL(baseURL),
		dataDir: dataDir,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (r *ollamaRuntime) EnsureReady(ctx context.Context) error {
	if err := r.ping(ctx); err == nil {
		return nil
	}
	if _, err := exec.LookPath("ollama"); err != nil {
		return newSetupRequiredError(ollamaSetupMessage(
			"Local Ollama is not installed.",
			"https://ollama.com/download",
			"ollama serve",
			"ollama pull "+DefaultEmbeddingModel,
		))
	}
	if err := r.start(ctx); err != nil {
		return err
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if err := r.ping(ctx); err == nil {
			return nil
		}
		time.Sleep(350 * time.Millisecond)
	}
	return newSetupRequiredError(ollamaSetupMessage(
		"Ollama is installed but the local server did not become ready.",
		"",
		"ollama serve",
		"ollama pull "+DefaultEmbeddingModel,
	))
}

func (r *ollamaRuntime) EnsureModel(ctx context.Context, model string) error {
	models, err := r.listModels(ctx)
	if err != nil {
		return err
	}
	for _, name := range models {
		if name == model {
			return nil
		}
	}
	return newSetupRequiredError(ollamaSetupMessage(
		"Required local embedding model is not installed.",
		"https://ollama.com/library/"+strings.ReplaceAll(model, ":", "%3A"),
		"ollama serve",
		"ollama pull "+model,
	))
}

func (r *ollamaRuntime) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cmd != nil && r.cmd.Process != nil {
		if err := r.cmd.Process.Kill(); err != nil && !isProcessDone(err) {
			return err
		}
	}
	return nil
}

func (r *ollamaRuntime) start(ctx context.Context) error {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cmd != nil && r.cmd.Process != nil {
		return nil
	}
	if err := ensureDir(filepath.Join(r.dataDir, "ollama")); err != nil {
		return err
	}
	logFile, err := os.OpenFile(filepath.Join(r.dataDir, "ollama", "ollama.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("code index: open ollama log file: %w", err)
	}
	cmd := exec.Command("ollama", "serve")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = append(os.Environ(),
		"OLLAMA_HOST="+strings.TrimPrefix(r.baseURL, "http://"),
		"OLLAMA_NUM_PARALLEL=1",
		"OLLAMA_MAX_LOADED_MODELS=1",
		"OLLAMA_KEEP_ALIVE=24h",
	)
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return newSetupRequiredError(ollamaSetupMessage(
			fmt.Sprintf("Failed to start Ollama automatically: %v", err),
			"https://ollama.com/download",
			"ollama serve",
			"ollama pull "+DefaultEmbeddingModel,
		))
	}
	r.cmd = cmd
	r.logFile = logFile
	go r.wait(cmd, logFile)
	return nil
}

func (r *ollamaRuntime) wait(cmd *exec.Cmd, logFile *os.File) {
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

func (r *ollamaRuntime) ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.baseURL+"/api/tags", nil)
	if err != nil {
		return err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("ollama tags returned status %d", resp.StatusCode)
}

func (r *ollamaRuntime) listModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.baseURL+"/api/tags", nil)
	if err != nil {
		return nil, err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("code index: ollama tags failed with status %d", resp.StatusCode)
	}
	var payload struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("code index: decode ollama tags: %w", err)
	}
	names := make([]string, 0, len(payload.Models))
	for _, model := range payload.Models {
		if strings.TrimSpace(model.Name) != "" {
			names = append(names, strings.TrimSpace(model.Name))
		}
	}
	return names, nil
}

func normalizeOllamaURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return DefaultOllamaURL
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return strings.TrimRight(raw, "/")
	}
	return "http://" + strings.TrimRight(raw, "/")
}

func ollamaSetupMessage(problem, installURL, serveCmd, pullCmd string) string {
	lines := []string{
		"Embedding setup required",
		"",
		problem,
		"",
		"Sapphire uses a local Ollama embedding model for codebase indexing.",
		"",
		"Do this:",
	}
	if installURL != "" {
		lines = append(lines, "1. Install Ollama: "+installURL)
		lines = append(lines, "2. Start the local server: `"+serveCmd+"`")
		lines = append(lines, "3. Pull the embedding model: `"+pullCmd+"`")
		lines = append(lines, "4. Restart Sapphire and run `Index Codebase` again")
	} else {
		lines = append(lines, "1. Start the local server: `"+serveCmd+"`")
		lines = append(lines, "2. Pull the embedding model: `"+pullCmd+"`")
		lines = append(lines, "3. Restart Sapphire and run `Index Codebase` again")
	}
	return strings.Join(lines, "\n")
}
