package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	agenttools "github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/stretchr/testify/require"
)

func TestSingularityPolicyDemotionAfterRegression(t *testing.T) {
	t.Parallel()

	manager := newTestSingularityManager(t)
	prompt := "Initialize the codebase, map architecture across the repo, trace dependencies in parallel, and generate AGENTS.md."

	for i := 0; i < 2; i++ {
		sessionID := "promote-" + string(rune('a'+i))
		manager.StartTurn(sessionID, prompt, manager.repoRoot, nil, learnedRoutePolicy{})
		manager.RecordToolCall(sessionID, agenttools.RunHarnessToolName, "")
		manager.RecordToolResult(sessionID, agenttools.RunHarnessToolName, "", "", false)
		manager.RecordToolCall(sessionID, agenttools.ToolSearchToolName, "")
		manager.RecordToolResult(sessionID, agenttools.ToolSearchToolName, "", "", false)
		manager.RecordToolCall(sessionID, agenttools.RGFilesToolName, "")
		manager.RecordToolResult(sessionID, agenttools.RGFilesToolName, "", "", false)
		manager.RecordToolCall(sessionID, agenttools.AgenticViewToolName, "")
		manager.RecordToolResult(sessionID, agenttools.AgenticViewToolName, "", "", false)
		manager.RecordToolCall(sessionID, agenttools.DiagnosticsToolName, "")
		manager.RecordToolResult(sessionID, agenttools.DiagnosticsToolName, "", "", false)
		manager.RecordToolCall(sessionID, SpawnAgentToolName, "")
		manager.RecordToolResult(sessionID, SpawnAgentToolName, "", "", false)
		trace := manager.FinishTurn(sessionID, "completed", "Mapped the repository, validated the initialization guide against the current package structure, and completed the structured initialization route.")
		require.NotNil(t, trace)
		manager.CompileTurn(context.Background(), trace)
	}

	promoted, _, ok := manager.LookupPolicy(prompt)
	require.True(t, ok)
	require.Contains(t, []string{learnedPolicyStateCandidate, learnedPolicyStatePromoted}, promoted.PromotionState)

	for i := 0; i < 2; i++ {
		sessionID := "regress-" + string(rune('a'+i))
		manager.StartTurn(sessionID, prompt, manager.repoRoot, nil, promoted)
		manager.RecordToolCall(sessionID, agenttools.BashToolName, `{"command":"find . -name '*.go'"}`)
		manager.RecordToolResult(sessionID, agenttools.BashToolName, "learned route blocked bash", `{"tool_error":{"tool_name":"bash","code":"learned_route_policy","ui_message":"Use structured discovery tools."}}`, true)
		trace := manager.FinishTurn(sessionID, "error", "regressed into discovery bash")
		require.NotNil(t, trace)
		manager.CompileTurn(context.Background(), trace)
	}

	manager.mu.Lock()
	policy := manager.store.Policies[classifyLearnedTaskFamily(prompt).ID]
	manager.mu.Unlock()
	require.Equal(t, learnedPolicyStateDemoted, policy.PromotionState)

	_, _, ok = manager.LookupPolicy(prompt)
	require.False(t, ok)
}

func TestDiffAndResetSingularityPolicies(t *testing.T) {
	t.Parallel()

	manager := newTestSingularityManager(t)
	prompt := "Initialize the codebase, map architecture across the repo, trace dependencies in parallel, and generate AGENTS.md."

	manager.StartTurn("diff-1", prompt, manager.repoRoot, nil, learnedRoutePolicy{})
	manager.RecordToolCall("diff-1", agenttools.RunHarnessToolName, "")
	manager.RecordToolResult("diff-1", agenttools.RunHarnessToolName, "", "", false)
	manager.RecordToolCall("diff-1", agenttools.ToolSearchToolName, "")
	manager.RecordToolResult("diff-1", agenttools.ToolSearchToolName, "", "", false)
	manager.RecordToolCall("diff-1", agenttools.AgenticViewToolName, "")
	manager.RecordToolResult("diff-1", agenttools.AgenticViewToolName, "", "", false)
	manager.RecordToolCall("diff-1", SpawnAgentToolName, "")
	manager.RecordToolResult("diff-1", SpawnAgentToolName, "", "", false)
	trace := manager.FinishTurn("diff-1", "completed", "validated first policy version against the current repository")
	require.NotNil(t, trace)
	manager.CompileTurn(context.Background(), trace)

	manager.StartTurn("diff-2", prompt, manager.repoRoot, nil, learnedRoutePolicy{})
	manager.RecordToolCall("diff-2", agenttools.RunHarnessToolName, "")
	manager.RecordToolResult("diff-2", agenttools.RunHarnessToolName, "", "", false)
	manager.RecordToolCall("diff-2", agenttools.ToolSearchToolName, "")
	manager.RecordToolResult("diff-2", agenttools.ToolSearchToolName, "", "", false)
	manager.RecordToolCall("diff-2", agenttools.IndexCodebaseToolName, "")
	manager.RecordToolResult("diff-2", agenttools.IndexCodebaseToolName, "", "", false)
	manager.RecordToolCall("diff-2", agenttools.AgenticViewToolName, "")
	manager.RecordToolResult("diff-2", agenttools.AgenticViewToolName, "", "", false)
	manager.RecordToolCall("diff-2", SpawnAgentToolName, "")
	manager.RecordToolResult("diff-2", SpawnAgentToolName, "", "", false)
	trace = manager.FinishTurn("diff-2", "completed", "validated second policy version against the current repository")
	require.NotNil(t, trace)
	manager.CompileTurn(context.Background(), trace)

	cfg, err := config.Init(manager.repoRoot, manager.dataDir, false)
	require.NoError(t, err)

	listInfo, err := ListSingularityPolicies(cfg)
	require.NoError(t, err)
	require.Len(t, listInfo.Policies, 1)
	require.GreaterOrEqual(t, listInfo.SnapshotCount, 1)

	diff, err := DiffSingularityPolicies(cfg, "")
	require.NoError(t, err)
	require.NotEmpty(t, diff.PreviousPath)
	require.NotEmpty(t, diff.Changed)

	taskFamily := listInfo.Policies[0].TaskFamily
	skillPath := listInfo.Policies[0].SkillFilePath
	require.FileExists(t, skillPath)

	resetResult, err := ResetSingularityPolicies(cfg, taskFamily, false)
	require.NoError(t, err)
	require.Equal(t, []string{taskFamily}, resetResult.RemovedPolicies)
	require.NotEmpty(t, resetResult.RemovedSkills)
	_, err = os.Stat(skillPath)
	require.ErrorIs(t, err, os.ErrNotExist)

	reloaded, err := ListSingularityPolicies(cfg)
	require.NoError(t, err)
	require.Empty(t, reloaded.Policies)
}

func TestListSingularityPoliciesReturnsEmptyStoreWhenNoPoliciesExist(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	dataDir := filepath.Join(repoRoot, ".sapphire")
	cfg, err := config.Init(repoRoot, dataDir, false)
	require.NoError(t, err)

	info, err := ListSingularityPolicies(cfg)
	require.NoError(t, err)
	require.Empty(t, info.Policies)
	require.Equal(t, 0, info.SnapshotCount)
}

func TestListSingularityAuditReturnsRecentRecords(t *testing.T) {
	repoRoot, err := os.MkdirTemp("", "singularity-audit-*")
	require.NoError(t, err)
	t.Cleanup(func() {
		for i := 0; i < 5; i++ {
			if err := os.RemoveAll(repoRoot); err == nil || os.IsNotExist(err) {
				return
			}
			time.Sleep(25 * time.Millisecond)
		}
	})
	dataDir := filepath.Join(repoRoot, ".sapphire")
	manager := newTestSingularityManagerAt(t, repoRoot, dataDir)
	prompt := "Initialize this codebase thoroughly, inspect architecture across auth and billing, and write AGENTS.md."

	manager.StartTurn("audit-1", prompt, manager.repoRoot, nil, learnedRoutePolicy{})
	manager.RecordToolCall("audit-1", agenttools.RunHarnessToolName, "")
	manager.RecordToolResult("audit-1", agenttools.RunHarnessToolName, "", "", false)
	manager.RecordToolCall("audit-1", agenttools.BashToolName, `{"command":"find . -name '*.go'"}`)
	manager.RecordToolResult("audit-1", agenttools.BashToolName, "learned route blocked bash", `{"tool_error":{"tool_name":"bash","code":"learned_route_policy","ui_message":"Use structured discovery tools."}}`, true)
	manager.RecordToolCall("audit-1", agenttools.ToolSearchToolName, "")
	manager.RecordToolResult("audit-1", agenttools.ToolSearchToolName, "", "", false)
	manager.RecordToolCall("audit-1", agenttools.AgenticViewToolName, "")
	manager.RecordToolResult("audit-1", agenttools.AgenticViewToolName, "", "", false)
	manager.RecordToolCall("audit-1", agenttools.UpdatePlanToolName, "")
	manager.RecordToolResult("audit-1", agenttools.UpdatePlanToolName, "", "", false)
	manager.RecordToolCall("audit-1", SpawnAgentToolName, "")
	manager.RecordToolResult("audit-1", SpawnAgentToolName, "", "", false)
	trace := manager.FinishTurn("audit-1", "completed", "first recovered after blocking bash, then completed the route")
	require.NotNil(t, trace)
	manager.CompileTurn(context.Background(), trace)

	cfg, err := config.Init(manager.repoRoot, manager.dataDir, false)
	require.NoError(t, err)

	audit, err := ListSingularityAudit(cfg, "initialize/broad/codebase", 5)
	require.NoError(t, err)
	require.Len(t, audit.Records, 1)
	require.Equal(t, "initialize/broad/codebase", audit.Records[0].TaskFamily)
	require.Equal(t, 1, audit.Records[0].BlockedBashDiscovery)
	require.Equal(t, 1, audit.Records[0].ToolErrorCodes["learned_route_policy"])
	require.Equal(t, "adequate", audit.Records[0].ContextDiscipline)
	require.Equal(t, "strong", audit.Records[0].PlanningDiscipline)
	require.Equal(t, "weak", audit.Records[0].ValidationDiscipline)
	require.Equal(t, "strong", audit.Records[0].RecoveryDiscipline)
	require.Equal(t, "medium", audit.Records[0].ExecutionRisk)
	require.FileExists(t, audit.AuditPath)
}
