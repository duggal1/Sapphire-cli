package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
	persistmemory "github.com/duggal1/Sapphire-cli/internal/memory"
	"github.com/duggal1/Sapphire-cli/internal/message"
)

const (
	maxMistakeSelfHealingAttempts      = 2
	mistakeSelfHealingFailureThreshold = 2
	mistakeSelfHealingEvidenceLimit    = 4
)

type mistakeSelfHealingTrigger struct {
	Evidence []string
}

type mistakeSelfHealingMonitor struct {
	mutatedThisRun              bool
	hardFailures                int
	evidence                    []string
	evidenceSet                 map[string]struct{}
	requested                   bool
	selfHealingMode             bool
	saveMemoryCalled            bool
	persistenceReminderEvidence string
	persistenceReminderPending  bool
}

func newMistakeSelfHealingMonitor(selfHealingMode bool) *mistakeSelfHealingMonitor {
	return &mistakeSelfHealingMonitor{
		evidenceSet:     make(map[string]struct{}),
		selfHealingMode: selfHealingMode,
	}
}

func (m *mistakeSelfHealingMonitor) Observe(toolName, rawInput string, result message.ToolResult) {
	if m == nil || m.requested {
		return
	}

	if isLikelyMutatingToolCall(toolName, rawInput) {
		m.mutatedThisRun = true
	}
	if !m.mutatedThisRun {
		return
	}

	signal, ok := detectMistakeSelfHealingSignal(toolName, rawInput, result)
	if !ok {
		return
	}

	m.hardFailures++
	if len(m.evidence) < mistakeSelfHealingEvidenceLimit {
		if _, exists := m.evidenceSet[signal]; !exists {
			m.evidence = append(m.evidence, signal)
			m.evidenceSet[signal] = struct{}{}
		}
	}
	if m.hardFailures >= mistakeSelfHealingFailureThreshold {
		m.requested = true
	}
}

func (m *mistakeSelfHealingMonitor) Consume() (mistakeSelfHealingTrigger, bool) {
	if m == nil || !m.requested {
		return mistakeSelfHealingTrigger{}, false
	}
	trigger := mistakeSelfHealingTrigger{
		Evidence: append([]string(nil), m.evidence...),
	}
	m.requested = false
	return trigger, true
}

func (m *mistakeSelfHealingMonitor) ObserveSelfHealingProgress(toolName, rawInput string) {
	if m == nil || !m.selfHealingMode || m.persistenceReminderPending {
		return
	}
	if strings.TrimSpace(toolName) == persistmemory.SaveToolName {
		m.saveMemoryCalled = true
		return
	}
	if m.saveMemoryCalled || isAllowedBeforeSaveMemory(toolName, rawInput) {
		return
	}
	m.persistenceReminderEvidence = summarizeSelfHealingPhaseViolation(toolName, rawInput)
	m.persistenceReminderPending = true
}

func (m *mistakeSelfHealingMonitor) ConsumePersistenceReminder() (string, bool) {
	if m == nil || !m.persistenceReminderPending {
		return "", false
	}
	evidence := m.persistenceReminderEvidence
	m.persistenceReminderPending = false
	return evidence, true
}

func detectMistakeSelfHealingSignal(toolName, rawInput string, result message.ToolResult) (string, bool) {
	content := strings.TrimSpace(stripMistakeSelfHealingToolNoise(result.Content))
	contentLower := strings.ToLower(content)

	switch strings.TrimSpace(toolName) {
	case tools.BashToolName:
		command := strings.ToLower(strings.TrimSpace(parseBashCommand(rawInput)))
		if !isLikelyHardBashFailure(command, contentLower, result.IsError) {
			return "", false
		}
	case tools.PythonToolName:
		if !result.IsError &&
			!strings.Contains(contentLower, "traceback") &&
			!strings.Contains(contentLower, "exception") &&
			!strings.Contains(contentLower, "error") {
			return "", false
		}
	case tools.EditToolName, tools.SingleEditToolName, tools.AgenticEditToolName, tools.WriteToolName:
		if !result.IsError {
			return "", false
		}
	default:
		return "", false
	}

	summary := summarizeMistakeSelfHealingEvidence(toolName, content)
	if summary == "" {
		return "", false
	}
	return summary, true
}

func isLikelyMutatingToolCall(toolName, rawInput string) bool {
	switch strings.TrimSpace(toolName) {
	case tools.EditToolName, tools.SingleEditToolName, tools.AgenticEditToolName, tools.WriteToolName:
		return true
	case tools.BashToolName:
		command := strings.ToLower(strings.TrimSpace(parseBashCommand(rawInput)))
		return isLikelyMutatingBashCommand(command)
	default:
		return false
	}
}

func parseBashCommand(rawInput string) string {
	if strings.TrimSpace(rawInput) == "" {
		return ""
	}
	var params tools.BashParams
	if err := json.Unmarshal([]byte(rawInput), &params); err == nil {
		return strings.TrimSpace(params.Command)
	}
	return strings.TrimSpace(rawInput)
}

func isAllowedBeforeSaveMemory(toolName, rawInput string) bool {
	switch strings.TrimSpace(toolName) {
	case persistmemory.SaveToolName:
		return true
	case tools.BashToolName:
		command := strings.ToLower(strings.TrimSpace(parseBashCommand(rawInput)))
		if command == "" {
			return false
		}
		return strings.Contains(command, ".sapphire/mistake.md") ||
			strings.Contains(command, "mistakes.md")
	case tools.ViewToolName, tools.SingleViewToolName, tools.AgenticViewToolName:
		inputLower := strings.ToLower(strings.TrimSpace(rawInput))
		return strings.Contains(inputLower, ".sapphire/mistake.md") ||
			strings.Contains(inputLower, "mistakes.md")
	default:
		return false
	}
}

func summarizeSelfHealingPhaseViolation(toolName, rawInput string) string {
	if strings.TrimSpace(toolName) == tools.BashToolName {
		if command := strings.TrimSpace(parseBashCommand(rawInput)); command != "" {
			return fmt.Sprintf("%s: attempted `%s` before save_memory", toolName, command)
		}
	}
	if strings.TrimSpace(rawInput) != "" {
		return fmt.Sprintf("%s: attempted task work before save_memory (%s)", toolName, strings.TrimSpace(rawInput))
	}
	return fmt.Sprintf("%s: attempted task work before save_memory", toolName)
}

func isLikelyMutatingBashCommand(command string) bool {
	if command == "" {
		return false
	}

	markers := []string{
		" >",
		"> ",
		">>",
		"tee ",
		"tee\t",
		"cat >",
		"cat<<",
		"cat <<",
		"sed -i",
		"perl -pi",
		"mv ",
		"cp ",
		"rm ",
		"mkdir ",
		"touch ",
		"chmod ",
		"chown ",
		"gofmt -w",
		"prettier --write",
		"eslint --fix",
		"terraform apply",
		"kubectl apply",
	}
	for _, marker := range markers {
		if strings.Contains(command, marker) {
			return true
		}
	}
	return false
}

func isLikelyHardBashFailure(command, content string, isError bool) bool {
	if command == "" && !isError {
		return false
	}

	hasExitCode := strings.Contains(content, "exit code ")
	if isError && (isLikelyValidationCommand(command) || isLikelyMutatingBashCommand(command)) {
		return true
	}
	if hasExitCode && (isLikelyValidationCommand(command) || isLikelyMutatingBashCommand(command)) {
		return true
	}

	hardMarkers := []string{
		"unknown escape sequence",
		"syntax error",
		"unbalanced brackets",
		"newline in ",
		"undefined:",
		"cannot use ",
		"no required module provides package",
		"traceback",
		"panic:",
		"exception",
		"segmentation fault",
		"command not found",
		"permission denied",
		"no such file or directory",
		"build failed",
		"fail\t",
		"fail ",
		"test failed",
	}
	for _, marker := range hardMarkers {
		if strings.Contains(content, marker) {
			return true
		}
	}
	return false
}

func isLikelyValidationCommand(command string) bool {
	command = strings.TrimSpace(command)
	if command == "" {
		return false
	}
	markers := []string{
		"go test",
		"go build",
		"go vet",
		"go run",
		"cargo test",
		"cargo build",
		"cargo check",
		"pytest",
		"python -m pytest",
		"npm test",
		"npm run test",
		"npm run build",
		"pnpm test",
		"pnpm build",
		"yarn test",
		"yarn build",
		"bun test",
		"bun run build",
		"tsc",
		"eslint",
		"ruff",
		"mypy",
		"javac",
		"gradle test",
		"mvn test",
		"make test",
		"make build",
		"task test",
		"task build",
	}
	for _, marker := range markers {
		if strings.Contains(command, marker) {
			return true
		}
	}
	return false
}

func stripMistakeSelfHealingToolNoise(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ""
	}
	if idx := strings.Index(trimmed, "\n<cwd>"); idx >= 0 {
		trimmed = trimmed[:idx]
	}
	return strings.TrimSpace(trimmed)
}

func summarizeMistakeSelfHealingEvidence(toolName, content string) string {
	if strings.TrimSpace(content) == "" {
		return fmt.Sprintf("%s: tool reported an error", toolName)
	}

	lines := strings.Split(content, "\n")
	selected := make([]string, 0, 2)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		selected = append(selected, line)
		if len(selected) == 2 {
			break
		}
	}
	if len(selected) == 0 {
		return fmt.Sprintf("%s: tool reported an error", toolName)
	}
	return fmt.Sprintf("%s: %s", toolName, strings.Join(selected, " | "))
}

func buildMistakeSelfHealingCall(call SessionAgentCall, trigger mistakeSelfHealingTrigger) SessionAgentCall {
	followUp := call
	followUp.SkipUserMessage = true
	followUp.MistakeSelfHealTry++

	evidenceLines := []string{"- repeated non-trivial failures occurred after your own mutation attempts"}
	for _, item := range trigger.Evidence {
		if strings.TrimSpace(item) == "" {
			continue
		}
		evidenceLines = append(evidenceLines, "- "+item)
	}

	followUp.Prompt = strings.TrimSpace(fmt.Sprintf(`Pause normal implementation and enter the autonomous self-healing loop now.

You have crossed a repeated non-trivial failure threshold after your own changes. Do not keep coding past it.

Required actions:
1. Read .sapphire/mistake.md fully.
2. Read MISTAKES.md if it exists.
3. Decide whether this is a new lesson or a stronger instance of an existing lesson.
4. Write or strengthen the canonical MISTAKES.md entry yourself.
5. If the root cause class is not HALLUCINATION, call save_memory with event_type=\"architectural_decision\" before any more task-file edits or validation.
6. Only after save_memory succeeds may you run 1-3 narrow validation probes that directly challenge the prevention rule.
7. If the rule is weak, revise the same entry instead of appending a near-duplicate.
8. After the lesson survives validation, resume the original task already in session history and finish it.

Current failure evidence:
%s

Do not explain the protocol. Execute it.`, strings.Join(evidenceLines, "\n")))
	return followUp
}

func buildMistakePersistenceCall(call SessionAgentCall, evidence string) SessionAgentCall {
	followUp := call
	followUp.SkipUserMessage = true
	followUp.Prompt = strings.TrimSpace(fmt.Sprintf(`Stop. You resumed normal task work before completing durable mistake persistence.

Before any more code edits, debugging, or validation:
1. Read the prevention rule you just wrote in MISTAKES.md.
2. Call save_memory with event_type="architectural_decision" containing that prevention rule.
3. After save_memory succeeds, continue the self-healing loop and then resume the original task.

Phase violation:
- %s

Do not do more task work until save_memory has completed.`, strings.TrimSpace(evidence)))
	return followUp
}
