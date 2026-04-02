package model

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"unsafe"

	tea "charm.land/bubbletea/v2"
	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/agent"
	agentbackground "github.com/duggal1/Sapphire-cli/internal/agent/background"
	agentformula "github.com/duggal1/Sapphire-cli/internal/agent/formula"
	"github.com/duggal1/Sapphire-cli/internal/app"
	"github.com/duggal1/Sapphire-cli/internal/codeindex"
	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/duggal1/Sapphire-cli/internal/message"
	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
	"github.com/duggal1/Sapphire-cli/internal/permission"
	"github.com/duggal1/Sapphire-cli/internal/session"
	"github.com/duggal1/Sapphire-cli/internal/ui/common"
	"github.com/stretchr/testify/require"
)

type uiCoordinatorStub struct {
	cancelled []string
}

func (s *uiCoordinatorStub) Run(context.Context, string, string, ...message.Attachment) (*fantasy.AgentResult, error) {
	return nil, nil
}
func (s *uiCoordinatorStub) Submit(context.Context, string, string, ...message.Attachment) (agent.SubmissionResult, error) {
	return agent.SubmissionResult{}, nil
}
func (s *uiCoordinatorStub) OrchestrateWorktrees(context.Context, string, agent.OrchestrateWorktreesParams) (agent.OrchestrateWorktreesResult, error) {
	return agent.OrchestrateWorktreesResult{}, nil
}
func (s *uiCoordinatorStub) ResumeWorktree(context.Context, string, string, string, string, string, string) (agent.OrchestrationAgentRef, error) {
	return agent.OrchestrationAgentRef{}, nil
}
func (s *uiCoordinatorStub) Cancel(sessionID string) { s.cancelled = append(s.cancelled, sessionID) }
func (s *uiCoordinatorStub) CancelAll()              {}
func (s *uiCoordinatorStub) IsSessionBusy(string) bool {
	return true
}
func (s *uiCoordinatorStub) IsBusy() bool { return true }
func (s *uiCoordinatorStub) QueuedPrompts(string) int {
	return 2
}
func (s *uiCoordinatorStub) QueuedPromptsList(string) []string {
	return []string{"one", "two"}
}
func (s *uiCoordinatorStub) ClearQueue(string) {}
func (s *uiCoordinatorStub) Summarize(context.Context, string) error {
	return nil
}
func (s *uiCoordinatorStub) Model() agent.Model { return agent.Model{} }
func (s *uiCoordinatorStub) UpdateModels(context.Context) error {
	return nil
}
func (s *uiCoordinatorStub) MemoryPipe() interface{} { return nil }
func (s *uiCoordinatorStub) ConsolidateMemory(context.Context, string) error {
	return nil
}
func (s *uiCoordinatorStub) DispatchBackground(context.Context, agentbackground.TaskSpec) (string, error) {
	return "", nil
}
func (s *uiCoordinatorStub) GetBackgroundStatus(string) (agentbackground.SubAgent, bool) {
	return agentbackground.SubAgent{}, false
}
func (s *uiCoordinatorStub) ListBackgroundAgents() []agentbackground.SubAgent { return nil }
func (s *uiCoordinatorStub) WaitForCompletion(context.Context, []string) ([]agentbackground.SubAgent, error) {
	return nil, nil
}
func (s *uiCoordinatorStub) IndexCodebase(context.Context, bool) (codeindex.Stats, error) {
	return codeindex.Stats{}, nil
}
func (s *uiCoordinatorStub) ListWorktrees(context.Context, string, []string, int) ([]orchestrationdb.WorktreeRun, error) {
	return nil, nil
}
func (s *uiCoordinatorStub) LandWorktree(context.Context, string, string) (orchestrationdb.WorktreeRun, error) {
	return orchestrationdb.WorktreeRun{}, nil
}
func (s *uiCoordinatorStub) RepairWorktree(context.Context, string) (orchestrationdb.WorktreeRun, error) {
	return orchestrationdb.WorktreeRun{}, nil
}
func (s *uiCoordinatorStub) RemoveManagedWorktree(context.Context, string, bool) (orchestrationdb.WorktreeRun, error) {
	return orchestrationdb.WorktreeRun{}, nil
}
func (s *uiCoordinatorStub) GetLongHorizonState(string) string          { return "" }
func (s *uiCoordinatorStub) GetLongHorizonAuditTail(string, int) string { return "" }
func (s *uiCoordinatorStub) RunPlanMode(context.Context, string, string, string) (*agentformula.ExecutionState, error) {
	return nil, nil
}
func (s *uiCoordinatorStub) ResolvePlanApproval(context.Context, string, bool) error {
	return nil
}

func TestCancelAgentInterruptsOnFirstPress(t *testing.T) {
	workingDir, err := os.MkdirTemp("", "sapphire-ui-cancel-working-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(workingDir) })
	require.NoError(t, os.WriteFile(filepath.Join(workingDir, "main.go"), []byte("package main\n"), 0o644))

	dataDir, err := os.MkdirTemp("", "sapphire-ui-cancel-data-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dataDir) })

	cfg, err := config.Load(workingDir, dataDir, false)
	require.NoError(t, err)
	cfg.Providers.Set("test", config.ProviderConfig{ID: "test"})

	coord := &uiCoordinatorStub{}
	a := &app.App{
		Permissions:      permission.NewPermissionService(workingDir, true, nil),
		AgentCoordinator: coord,
	}
	field := reflect.ValueOf(a).Elem().FieldByName("config")
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(cfg))

	ui := New(common.DefaultCommon(a))
	ui.session = &session.Session{ID: "session-1"}
	ui.state = uiChat
	ui.todoIsSpinning = true
	ui.deepPlanningPending = true
	ui.deepPlanningSessionID = "session-1"
	ui.fixedTailNotice = nil

	cmd := ui.cancelAgent()
	require.Nil(t, cmd)
	require.Equal(t, []string{"session-1"}, coord.cancelled)
	require.False(t, ui.todoIsSpinning)
	require.False(t, ui.deepPlanningPending)
	require.Empty(t, ui.deepPlanningSessionID)
	require.False(t, ui.isCanceling)
	require.Nil(t, ui.fixedTailNotice)
}

func TestEscapeKeyCancelsAgentOnFirstPress(t *testing.T) {
	workingDir, err := os.MkdirTemp("", "sapphire-ui-escape-working-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(workingDir) })
	require.NoError(t, os.WriteFile(filepath.Join(workingDir, "main.go"), []byte("package main\n"), 0o644))

	dataDir, err := os.MkdirTemp("", "sapphire-ui-escape-data-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dataDir) })

	cfg, err := config.Load(workingDir, dataDir, false)
	require.NoError(t, err)
	cfg.Providers.Set("test", config.ProviderConfig{ID: "test"})

	coord := &uiCoordinatorStub{}
	a := &app.App{
		Permissions:      permission.NewPermissionService(workingDir, true, nil),
		AgentCoordinator: coord,
	}
	field := reflect.ValueOf(a).Elem().FieldByName("config")
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(cfg))

	ui := New(common.DefaultCommon(a))
	ui.session = &session.Session{ID: "session-1"}
	ui.state = uiChat
	ui.todoIsSpinning = true
	ui.deepPlanningPending = true
	ui.deepPlanningSessionID = "session-1"

	_, cmd := ui.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	require.Equal(t, []string{"session-1"}, coord.cancelled)
	require.False(t, ui.todoIsSpinning)
	require.False(t, ui.deepPlanningPending)
	require.Empty(t, ui.deepPlanningSessionID)
	require.False(t, ui.isCanceling)
	require.Nil(t, ui.fixedTailNotice)
}
