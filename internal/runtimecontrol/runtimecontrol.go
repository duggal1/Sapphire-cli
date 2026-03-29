package runtimecontrol

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	ActionStopBackground = "stop_background"
	heartbeatTTL         = 5 * time.Second
)

type RuntimeStatus struct {
	PID        int       `json:"pid"`
	StartedAt  time.Time `json:"started_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	WorkingDir string    `json:"working_dir,omitempty"`
}

type Request struct {
	ID          string    `json:"id"`
	Action      string    `json:"action"`
	RequestedAt time.Time `json:"requested_at"`
}

type Response struct {
	ID        string         `json:"id"`
	Action    string         `json:"action"`
	Status    string         `json:"status"`
	Message   string         `json:"message,omitempty"`
	Summary   map[string]int `json:"summary,omitempty"`
	HandledAt time.Time      `json:"handled_at"`
}

func RuntimePath(dataDir string) string {
	return filepath.Join(dataDir, "state", "runtime", "runtime.json")
}

func RequestPath(dataDir string) string {
	return filepath.Join(dataDir, "state", "runtime", "request.json")
}

func ResponsePath(dataDir string) string {
	return filepath.Join(dataDir, "state", "runtime", "response.json")
}

func WriteRuntimeStatus(dataDir string, status RuntimeStatus) error {
	if status.PID <= 0 {
		return fmt.Errorf("runtime pid is required")
	}
	if status.StartedAt.IsZero() {
		status.StartedAt = time.Now().UTC()
	}
	if status.UpdatedAt.IsZero() {
		status.UpdatedAt = time.Now().UTC()
	}
	return writeJSONAtomic(RuntimePath(dataDir), status)
}

func RemoveRuntimeStatus(dataDir string) error {
	err := os.Remove(RuntimePath(dataDir))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func ReadRuntimeStatus(dataDir string) (RuntimeStatus, error) {
	var status RuntimeStatus
	err := readJSON(RuntimePath(dataDir), &status)
	return status, err
}

func WriteRequest(dataDir string, req Request) error {
	if req.ID == "" {
		return fmt.Errorf("request id is required")
	}
	if req.Action == "" {
		return fmt.Errorf("request action is required")
	}
	if req.RequestedAt.IsZero() {
		req.RequestedAt = time.Now().UTC()
	}
	return writeJSONAtomic(RequestPath(dataDir), req)
}

func RemoveRequest(dataDir string) error {
	err := os.Remove(RequestPath(dataDir))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func ReadRequest(dataDir string) (Request, error) {
	var req Request
	err := readJSON(RequestPath(dataDir), &req)
	return req, err
}

func WriteResponse(dataDir string, resp Response) error {
	if resp.ID == "" {
		return fmt.Errorf("response id is required")
	}
	if resp.Action == "" {
		return fmt.Errorf("response action is required")
	}
	if resp.Status == "" {
		resp.Status = "ok"
	}
	if resp.HandledAt.IsZero() {
		resp.HandledAt = time.Now().UTC()
	}
	return writeJSONAtomic(ResponsePath(dataDir), resp)
}

func ReadResponse(dataDir string) (Response, error) {
	var resp Response
	err := readJSON(ResponsePath(dataDir), &resp)
	return resp, err
}

func IsLive(status RuntimeStatus, now time.Time) bool {
	if status.PID <= 0 || status.UpdatedAt.IsZero() {
		return false
	}
	return now.Sub(status.UpdatedAt) <= heartbeatTTL
}

func writeJSONAtomic(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func readJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}
