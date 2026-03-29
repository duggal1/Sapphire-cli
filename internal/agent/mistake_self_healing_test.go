package agent

import (
	"testing"

	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMistakeSelfHealingMonitorTriggersAfterRepeatedPostMutationFailures(t *testing.T) {
	t.Parallel()

	monitor := newMistakeSelfHealingMonitor(false)

	monitor.Observe(
		tools.BashToolName,
		`{"command":"cat <<'EOF' > experiments/mistake_lab/match.go\npackage main\nEOF"}`,
		message.ToolResult{Content: "no output"},
	)

	monitor.Observe(
		tools.BashToolName,
		`{"command":"go test ./experiments/mistake_lab"}`,
		message.ToolResult{Content: "./match_test.go:38:7: unknown escape sequence\nExit code 1"},
	)

	if _, ok := monitor.Consume(); ok {
		t.Fatal("self-healing should not trigger after only one hard failure")
	}

	monitor.Observe(
		tools.BashToolName,
		`{"command":"sed -i '' 's/foo/[/' experiments/mistake_lab/match.go"}`,
		message.ToolResult{Content: "sed: 1: unbalanced brackets\nExit code 1"},
	)

	trigger, ok := monitor.Consume()
	require.True(t, ok)
	require.Len(t, trigger.Evidence, 2)
	assert.Contains(t, trigger.Evidence[0], "unknown escape sequence")
	assert.Contains(t, trigger.Evidence[1], "unbalanced brackets")
}

func TestMistakeSelfHealingMonitorIgnoresPreexistingValidationFailure(t *testing.T) {
	t.Parallel()

	monitor := newMistakeSelfHealingMonitor(false)
	monitor.Observe(
		tools.BashToolName,
		`{"command":"go test ./... "}`,
		message.ToolResult{Content: "FAIL\tproject/pkg [build failed]\nExit code 1"},
	)

	_, ok := monitor.Consume()
	assert.False(t, ok)
}

func TestBuildMistakeSelfHealingCall(t *testing.T) {
	t.Parallel()

	call := SessionAgentCall{
		SessionID: "session-1",
		Prompt:    "original task",
	}
	followUp := buildMistakeSelfHealingCall(call, mistakeSelfHealingTrigger{
		Evidence: []string{
			"bash: ./match_test.go:38:7: unknown escape sequence | Exit code 1",
			"bash: sed: 1: unbalanced brackets | Exit code 1",
		},
	})

	assert.True(t, followUp.SkipUserMessage)
	assert.Equal(t, 1, followUp.MistakeSelfHealTry)
	assert.Contains(t, followUp.Prompt, "Read .sapphire/mistake.md fully.")
	assert.Contains(t, followUp.Prompt, "call save_memory with event_type=\\\"architectural_decision\\\"")
	assert.Contains(t, followUp.Prompt, "event_type=\\\"improvement_eval\\\"")
	assert.Contains(t, followUp.Prompt, "save_memory a strategy_pattern")
	assert.Contains(t, followUp.Prompt, "unknown escape sequence")
	assert.Contains(t, followUp.Prompt, "resume the original task already in session history")
}

func TestMistakeSelfHealingMonitorAllowsEditingMistakeFilesBeforeSaveMemory(t *testing.T) {
	t.Parallel()

	monitor := newMistakeSelfHealingMonitor(true)

	monitor.ObserveSelfHealingProgress(tools.BashToolName, `{"command":"cat .sapphire/mistake.md"}`, message.ToolResult{})
	if _, ok := monitor.ConsumePersistenceReminder(); ok {
		t.Fatal("reading the mistake protocol should be allowed before save_memory")
	}

	monitor.ObserveSelfHealingProgress(tools.WriteToolName, `{"file_path":"MISTAKES.md","content":"# MISTAKES.md"}`, message.ToolResult{})
	if _, ok := monitor.ConsumePersistenceReminder(); ok {
		t.Fatal("writing MISTAKES.md should be allowed before save_memory")
	}
}

func TestMistakeSelfHealingMonitorRequiresSaveMemoryBeforeTaskWork(t *testing.T) {
	t.Parallel()

	monitor := newMistakeSelfHealingMonitor(true)

	monitor.ObserveSelfHealingProgress(tools.BashToolName, `{"command":"cd experiments/mistake_lab && go build ./..."}`, message.ToolResult{})
	evidence, ok := monitor.ConsumePersistenceReminder()
	require.True(t, ok)
	assert.Contains(t, evidence, "go build")
}

func TestMistakeSelfHealingMonitorRequiresImprovementEvalBeforeValidation(t *testing.T) {
	t.Parallel()

	monitor := newMistakeSelfHealingMonitor(true)
	monitor.ObserveSelfHealingProgress("save_memory", `{"event_type":"architectural_decision","content":{"decision":"rule"}}`, message.ToolResult{Content: "Memory saved: architectural_decision"})
	monitor.ObserveSelfHealingProgress(tools.BashToolName, `{"command":"cd experiments/mistake_lab && go test ./..."}`, message.ToolResult{})
	if _, ok := monitor.ConsumePersistenceReminder(); ok {
		t.Fatal("architectural decision persistence should clear the pre-save reminder path")
	}

	evidence, ok := monitor.ConsumeEvalReminder()
	require.True(t, ok)
	assert.Contains(t, evidence, "go test")

	monitor.ObserveSelfHealingProgress("save_memory", `{"event_type":"improvement_eval","content":{"task_shape":"matcher repair","probe":"go test ./experiments/mistake_lab","success_criteria":"tests pass"}}`, message.ToolResult{Content: "Memory saved: improvement_eval"})
	monitor.ObserveSelfHealingProgress(tools.BashToolName, `{"command":"cd experiments/mistake_lab && go test ./..."}`, message.ToolResult{})
	_, ok = monitor.ConsumeEvalReminder()
	assert.False(t, ok)
}

func TestMistakeSelfHealingMonitorDoesNotCountFailedSaveMemoryAsProgress(t *testing.T) {
	t.Parallel()

	monitor := newMistakeSelfHealingMonitor(true)

	monitor.ObserveSelfHealingProgress("save_memory", `{"event_type":"architectural_decision","content":{"decision":"rule"}}`, message.ToolResult{
		Content: "Failed to save memory: disk offline",
	})
	monitor.ObserveSelfHealingProgress(tools.BashToolName, `{"command":"cd experiments/mistake_lab && go build ./..."}`, message.ToolResult{})

	evidence, ok := monitor.ConsumePersistenceReminder()
	require.True(t, ok)
	assert.Contains(t, evidence, "go build")
}

func TestBuildImprovementEvalCall(t *testing.T) {
	t.Parallel()

	call := SessionAgentCall{SessionID: "session-1", Prompt: "original task"}
	followUp := buildImprovementEvalCall(call, "bash: attempted validation `go test ./...` before save_memory improvement_eval")

	assert.True(t, followUp.SkipUserMessage)
	assert.Contains(t, followUp.Prompt, `event_type="improvement_eval"`)
	assert.Contains(t, followUp.Prompt, `event_type="strategy_pattern"`)
	assert.Contains(t, followUp.Prompt, "Do not rerun validation until the improvement_eval has been persisted.")
}
