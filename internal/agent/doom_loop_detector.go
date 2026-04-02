package agent

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/duggal1/Sapphire-cli/internal/agent/planmode"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/duggal1/Sapphire-cli/internal/filetracker"
	"github.com/duggal1/Sapphire-cli/internal/message"
)

//go:embed templates/harness-templates/doom-loop-reminder.md
var doomLoopReminderTemplate []byte

const (
	doomLoopFileChurnThreshold       = 4
	doomLoopReadAmnesiaThreshold     = 3
	doomLoopToolTunnelMinCalls       = 5
	doomLoopToolTunnelMaxUniqueTools = 2
	doomLoopSolutionCreepMinCreates  = 4
)

type doomLoopSignal struct {
	Code        string
	Severity    string
	Occurrences int
	Summary     string
}

type deterministicDoomLoop struct {
	TaskFamily       string
	TotalCalls       int
	UniqueToolCount  int
	ConsecutiveCalls int
	Signals          []doomLoopSignal
}

func detectDeterministicDoomLoop(state *tools.ToolUsageState, tracker filetracker.Service, sessionID, workingDir, taskFamily string) (deterministicDoomLoop, bool) {
	metrics := tools.DeterministicLoopMetrics{}
	if state != nil {
		metrics = state.SnapshotDeterministicLoopMetrics()
	}
	drift := collectWorkspaceDrift(context.Background(), tracker, sessionID, workingDir)
	report := evaluateDeterministicDoomLoop(metrics, drift, taskFamily)
	return report, len(report.Signals) > 0 && shouldBreakDeterministicDoomLoop(report)
}

func evaluateDeterministicDoomLoop(metrics tools.DeterministicLoopMetrics, drift workspaceDriftSummary, taskFamily string) deterministicDoomLoop {
	report := deterministicDoomLoop{
		TaskFamily:       strings.TrimSpace(taskFamily),
		TotalCalls:       metrics.TotalCalls,
		UniqueToolCount:  len(metrics.UniqueToolNames),
		ConsecutiveCalls: maxDeterministicLoopCount(metrics.TotalCalls),
	}

	for path, count := range metrics.WriteCounts {
		if count < doomLoopFileChurnThreshold {
			continue
		}
		report.Signals = append(report.Signals, doomLoopSignal{
			Code:        "file_churn",
			Severity:    "severe",
			Occurrences: count,
			Summary:     fmt.Sprintf("File churn: %s was rewritten %d times.", path, count),
		})
		report.ConsecutiveCalls = maxDeterministicLoopCount(report.ConsecutiveCalls, count)
	}

	for path, count := range metrics.ReadCounts {
		if count < doomLoopReadAmnesiaThreshold {
			continue
		}
		report.Signals = append(report.Signals, doomLoopSignal{
			Code:        "read_amnesia",
			Severity:    "moderate",
			Occurrences: count,
			Summary:     fmt.Sprintf("Read amnesia: %s was re-read %d times.", path, count),
		})
		report.ConsecutiveCalls = maxDeterministicLoopCount(report.ConsecutiveCalls, count)
	}

	for path, count := range metrics.BlindWriteCounts {
		if count <= 0 {
			continue
		}
		report.Signals = append(report.Signals, doomLoopSignal{
			Code:        "blind_writing",
			Severity:    "severe",
			Occurrences: count,
			Summary:     fmt.Sprintf("Blind writing: %s was written without a prior read.", path),
		})
		report.ConsecutiveCalls = maxDeterministicLoopCount(report.ConsecutiveCalls, count)
	}

	if metrics.TotalCalls >= doomLoopToolTunnelMinCalls && len(metrics.UniqueToolNames) <= doomLoopToolTunnelMaxUniqueTools {
		report.Signals = append(report.Signals, doomLoopSignal{
			Code:        "tool_tunnel_vision",
			Severity:    "moderate",
			Occurrences: metrics.TotalCalls,
			Summary:     fmt.Sprintf("Tool tunnel vision: only %d unique tools were used across %d calls.", len(metrics.UniqueToolNames), metrics.TotalCalls),
		})
		report.ConsecutiveCalls = maxDeterministicLoopCount(report.ConsecutiveCalls, metrics.TotalCalls)
	}

	createdCount := len(metrics.CreatedFiles)
	modifiedCount := len(metrics.ModifiedFiles)
	if createdCount >= doomLoopSolutionCreepMinCreates && createdCount > modifiedCount*3 {
		report.Signals = append(report.Signals, doomLoopSignal{
			Code:        "solution_creep",
			Severity:    "moderate",
			Occurrences: createdCount,
			Summary:     fmt.Sprintf("Solution creep: %d files were created versus %d modified.", createdCount, modifiedCount),
		})
		report.ConsecutiveCalls = maxDeterministicLoopCount(report.ConsecutiveCalls, createdCount)
	}

	if drift.HasDrift() {
		changed := append([]string{}, drift.Modified...)
		changed = append(changed, drift.Removed...)
		summary := strings.Join(changed, ", ")
		if summary == "" {
			summary = "previously read files changed externally"
		}
		report.Signals = append(report.Signals, doomLoopSignal{
			Code:        "context_staleness",
			Severity:    "severe",
			Occurrences: len(changed),
			Summary:     "Context staleness: " + summary + ".",
		})
		report.ConsecutiveCalls = maxDeterministicLoopCount(report.ConsecutiveCalls, len(changed))
	}

	return report
}

func shouldBreakDeterministicDoomLoop(report deterministicDoomLoop) bool {
	severe := 0
	moderate := 0
	for _, signal := range report.Signals {
		switch signal.Severity {
		case "severe":
			severe++
		case "moderate":
			moderate++
		}
	}
	if severe > 0 {
		return true
	}
	return moderate >= 2
}

func renderDeterministicDoomLoopReminder(loop deterministicDoomLoop) string {
	consecutive := loop.ConsecutiveCalls
	if consecutive < 1 {
		consecutive = 1
	}
	body := strings.ReplaceAll(string(doomLoopReminderTemplate), "{{consecutive_calls}}", strconv.Itoa(consecutive))
	if len(loop.Signals) == 0 {
		return strings.TrimSpace(body)
	}

	var lines []string
	lines = append(lines, strings.TrimSpace(body), "", "## Detected Deterministic Signals")
	for _, signal := range loop.Signals {
		lines = append(lines, "- "+signal.Summary)
	}
	lines = append(lines, "", "## Immediate Execution Constraints")
	lines = append(lines, "- Read exact target files before any further write or delegation.")
	lines = append(lines, "- If the same design keeps failing, abandon it and choose a materially different architecture.")
	lines = append(lines, "- Do not create new files when modifying the real execution path is sufficient.")
	lines = append(lines, "- Use run_harness, update_plan, tool_search, rg_files, or agentic_view if the current tool path is too narrow.")
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func renderRepeatedToolLoopReminder(loop repeatedToolLoop) string {
	consecutive := loop.WindowSize
	if consecutive < loop.RepeatCount {
		consecutive = loop.RepeatCount
	}
	if consecutive < 1 {
		consecutive = 1
	}
	body := strings.ReplaceAll(string(doomLoopReminderTemplate), "{{consecutive_calls}}", strconv.Itoa(consecutive))

	lines := []string{strings.TrimSpace(body), "", "## Detected Repeated Interaction Loop"}
	if loop.PatternSize > 1 {
		lines = append(lines, fmt.Sprintf("- A %d-step tool/result suffix pattern repeated %d times.", loop.PatternSize, loop.RepeatCount))
	} else {
		lines = append(lines, fmt.Sprintf("- The same tool/result interaction repeated %d times.", loop.RepeatCount))
	}
	if len(loop.ToolNames) > 0 {
		lines = append(lines, "- Repeating tools: "+strings.Join(loop.ToolNames, ", ")+".")
	}
	lines = append(lines, "", "## Immediate Execution Constraints")
	lines = append(lines, "- Stop replaying the same suffix pattern or tool/result interaction.")
	lines = append(lines, "- Change the architecture, execution order, or evidence path before the next step.")
	lines = append(lines, "- If the pattern came from narrow serial file reads, widen to structured discovery before acting again.")
	lines = append(lines, "- If the pattern came from edit-verify churn, replace the failing design instead of retrying it.")
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func buildDoomLoopRecoveryCall(mode planmode.SessionMode, call SessionAgentCall, err error, partialAssistant *message.Message) (SessionAgentCall, bool) {
	if mode != planmode.DefaultSessionMode || call.DoomLoopRecoveryTry >= maxDoomLoopRecoveryAttempts {
		return SessionAgentCall{}, false
	}
	var loopErr *deterministicDoomLoopError
	var repeatedErr *repeatedToolLoopError

	reminder := ""
	switch {
	case errors.As(err, &loopErr) && loopErr != nil:
		reminder = renderDeterministicDoomLoopReminder(loopErr.loop)
	case errors.As(err, &repeatedErr) && repeatedErr != nil:
		reminder = renderRepeatedToolLoopReminder(repeatedErr.loop)
	default:
		return SessionAgentCall{}, false
	}

	followUp := call
	followUp.SkipUserMessage = true
	followUp.DoomLoopRecoveryTry++

	partialTail := ""
	if partialAssistant != nil {
		partialTail = strings.TrimSpace(partialAssistant.Content().Text)
		if len(partialTail) > 1200 {
			partialTail = partialTail[len(partialTail)-1200:]
		}
	}

	base := fmt.Sprintf(`Continue the same turn from the existing repository context. Do not restart from scratch, do not ask the user for permission, and do not repeat the previous answer verbatim.

Original user request:
%s`, strings.TrimSpace(call.Prompt))
	if partialTail != "" {
		base += fmt.Sprintf("\n\nPrevious draft tail:\n%s", partialTail)
	}

	followUp.Prompt = strings.TrimSpace(base + "\n\n" + reminder)
	return followUp, true
}

func maxDeterministicLoopCount(values ...int) int {
	best := 0
	for _, value := range values {
		if value > best {
			best = value
		}
	}
	return best
}
