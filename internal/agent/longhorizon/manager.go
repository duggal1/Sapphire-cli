package longhorizon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	runbookFilename   = "runbook.md"
	specFilename      = "frozen_spec.md"
	planFilename      = "milestones.json"
	auditFilename     = "audit.log"
	maxAuditInjectLen = 2000
)

var runbookBody = `# Long-Horizon Runbook

This runbook is the non-negotiable operating contract for long-horizon tasks.

- Work milestone-by-milestone only. Do not batch milestones.
- Keep diffs scoped to the active milestone.
- Before changing milestone: validate completion (tests, checks, criteria).
- Update docs relevant to the milestone before marking it done.
- Write every significant decision to the audit log before acting.
- If a step fails: record the failure and recovery attempt in the audit log, then retry or roll back.
- If context is compacted/refreshed: re-read the frozen spec, milestone plan, and latest audit log tail before resuming.
`

type State struct {
	SpecPath    string
	PlanPath    string
	RunbookPath string
	AuditPath   string
	Activated   bool
}

type Plan struct {
	Session      string      `json:"session"`
	GeneratedAt  string      `json:"generated_at"`
	Milestones   []Milestone `json:"milestones"`
	SourcePrompt string      `json:"source_prompt"`
}

type Milestone struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Condition string `json:"condition"`
	Status    string `json:"status"`
}

type Manager struct {
	root       string
	mu         sync.Mutex
	perSession map[string]*State
}

func NewManager(workDir string) *Manager {
	return &Manager{
		root:       filepath.Join(workDir, "long_horizon"),
		perSession: make(map[string]*State),
	}
}

func (m *Manager) state(sessionID string) *State {
	s, ok := m.perSession[sessionID]
	if !ok {
		s = &State{}
		m.perSession[sessionID] = s
	}
	return s
}

// Ensure initializes all artifacts for a long-horizon run.
func (m *Manager) Ensure(ctx context.Context, sessionID, userPrompt string) (*State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state := m.state(sessionID)
	if state.Activated {
		return state, nil
	}

	sessionDir := filepath.Join(m.root, sessionID)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		return nil, err
	}

	runbookPath := filepath.Join(sessionDir, runbookFilename)
	if err := os.WriteFile(runbookPath, []byte(runbookBody), 0o644); err != nil {
		return nil, err
	}

	specPath := filepath.Join(sessionDir, specFilename)
	if err := os.WriteFile(specPath, []byte(renderSpec(sessionID, userPrompt)), 0o644); err != nil {
		return nil, err
	}

	planPath := filepath.Join(sessionDir, planFilename)
	if err := os.WriteFile(planPath, []byte(renderPlan(sessionID, userPrompt)), 0o644); err != nil {
		return nil, err
	}

	auditPath := filepath.Join(sessionDir, auditFilename)
	if _, err := os.Stat(auditPath); err != nil {
		if err := os.WriteFile(auditPath, []byte(""), 0o644); err != nil {
			return nil, err
		}
	}

	state.SpecPath = specPath
	state.PlanPath = planPath
	state.RunbookPath = runbookPath
	state.AuditPath = auditPath
	state.Activated = true

	_ = m.appendAuditLocked(state, "Initialized long-horizon mode; spec/plan/runbook ready.")
	m.gitCommitArtifacts(sessionDir)

	return state, nil
}

// AppendAudit writes timestamped lines.
func (m *Manager) AppendAudit(ctx context.Context, sessionID string, lines ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.state(sessionID)
	if !state.Activated || state.AuditPath == "" {
		return
	}
	_ = m.appendAuditLocked(state, lines...)
}

func (m *Manager) appendAuditLocked(state *State, lines ...string) error {
	if state == nil || state.AuditPath == "" {
		return nil
	}
	var buf bytes.Buffer
	ts := time.Now().UTC().Format(time.RFC3339)
	for _, line := range lines {
		buf.WriteString(ts)
		buf.WriteString(" - ")
		buf.WriteString(line)
		buf.WriteString("\n")
	}
	f, err := os.OpenFile(state.AuditPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, _ = f.Write(buf.Bytes())
	return nil
}

// BuildInjection returns a context block combining runbook, spec, plan, and audit tail.
func (m *Manager) BuildInjection(sessionID string) string {
	m.mu.Lock()
	state := m.state(sessionID)
	m.mu.Unlock()

	if !state.Activated {
		return ""
	}

	var sb strings.Builder

	sb.WriteString("<long_horizon_runbook>\n")
	if body, err := os.ReadFile(state.RunbookPath); err == nil {
		sb.Write(body)
	}
	sb.WriteString("\n</long_horizon_runbook>\n\n")

	sb.WriteString("<long_horizon_frozen_spec path=\"" + state.SpecPath + "\">\n")
	if body, err := os.ReadFile(state.SpecPath); err == nil {
		sb.Write(body)
	}
	sb.WriteString("\n</long_horizon_frozen_spec>\n\n")

	sb.WriteString("<long_horizon_milestones path=\"" + state.PlanPath + "\">\n")
	if body, err := os.ReadFile(state.PlanPath); err == nil {
		sb.Write(body)
	}
	sb.WriteString("\n</long_horizon_milestones>\n\n")

	sb.WriteString("<long_horizon_audit path=\"" + state.AuditPath + "\">\n")
	if tail := tailFile(state.AuditPath, maxAuditInjectLen); tail != "" {
		sb.WriteString(tail)
	}
	sb.WriteString("\n</long_horizon_audit>\n\n")

	return sb.String()
}

func (m *Manager) ReadPlan(sessionID string) (Plan, error) {
	m.mu.Lock()
	state := m.state(sessionID)
	m.mu.Unlock()
	if !state.Activated || state.PlanPath == "" {
		return Plan{}, fmt.Errorf("long-horizon plan not initialized")
	}
	data, err := os.ReadFile(state.PlanPath)
	if err != nil {
		return Plan{}, err
	}
	var plan Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return Plan{}, err
	}
	return plan, nil
}

func (m *Manager) ReadSpec(sessionID string) (string, error) {
	m.mu.Lock()
	state := m.state(sessionID)
	m.mu.Unlock()
	if !state.Activated || state.SpecPath == "" {
		return "", fmt.Errorf("long-horizon spec not initialized")
	}
	data, err := os.ReadFile(state.SpecPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func renderSpec(sessionID, userPrompt string) string {
	return fmt.Sprintf(`# Frozen Spec (Session %s)

## Task Definition
%s

## Success Criteria
- Implement exactly the requirements stated above.
- All relevant tests/linters pass.

## Out of Scope
- Changes unrelated to the stated task.
- Optimizations or refactors not required for success criteria.
`, sessionID, userPrompt)
}

func renderPlan(sessionID, userPrompt string) string {
	return fmt.Sprintf(`{
  "session": "%s",
  "generated_at": "%s",
  "milestones": [
    {
      "id": "m1",
      "name": "Understand specification",
      "condition": "Spec read and clarified; no open questions.",
      "status": "pending"
    },
    {
      "id": "m2",
      "name": "Implement task",
      "condition": "Changes implemented per spec; local checks/tests pass.",
      "status": "pending"
    },
    {
      "id": "m3",
      "name": "Validate and document",
      "condition": "Documentation updated; final verification complete.",
      "status": "pending"
    }
  ],
  "source_prompt": %q
}
`, sessionID, time.Now().UTC().Format(time.RFC3339), userPrompt)
}

func tailFile(path string, maxBytes int) string {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return ""
	}
	if len(data) <= maxBytes {
		return string(data)
	}
	return string(data[len(data)-maxBytes:])
}

// gitCommitArtifacts creates a single commit if inside a git repo; failures are logged but not fatal.
func (m *Manager) gitCommitArtifacts(dir string) {
	if !inGitRepo(dir) {
		return
	}
	cmds := [][]string{
		{"git", "-C", dir, "add", "."},
		{"git", "-C", dir, "commit", "-m", "chore(long-horizon): freeze spec and milestones"},
	}
	for _, c := range cmds {
		if err := exec.Command(c[0], c[1:]...).Run(); err != nil {
			slog.Debug("long-horizon git step failed", "cmd", strings.Join(c, " "), "err", err)
			break
		}
	}
}

func inGitRepo(dir string) bool {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--is-inside-work-tree")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}
