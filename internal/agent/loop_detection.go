package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"io"

	"charm.land/fantasy"
)

const (
	loopDetectionWindowSize = 10
	loopDetectionMaxRepeats = 5
)

type repeatedToolLoop struct {
	Signature   string
	RepeatCount int
	WindowSize  int
	ToolNames   []string
}

// hasRepeatedToolCalls checks whether the agent is stuck in a loop by looking
// at recent steps. It examines the last windowSize steps and returns true if
// any tool-call signature appears more than maxRepeats times.
func hasRepeatedToolCalls(steps []fantasy.StepResult, windowSize, maxRepeats int) bool {
	_, detected := detectRepeatedToolCalls(steps, windowSize, maxRepeats)
	return detected
}

func detectRepeatedToolCalls(steps []fantasy.StepResult, windowSize, maxRepeats int) (repeatedToolLoop, bool) {
	if len(steps) < windowSize {
		return repeatedToolLoop{}, false
	}

	window := steps[len(steps)-windowSize:]
	counts := make(map[string]int)

	for _, step := range window {
		loop, ok := summarizeToolInteraction(step.Content)
		if !ok || loop.Signature == "" {
			continue
		}
		counts[loop.Signature]++
		if counts[loop.Signature] > maxRepeats {
			loop.RepeatCount = counts[loop.Signature]
			loop.WindowSize = len(window)
			return loop, true
		}
	}

	return repeatedToolLoop{}, false
}

// getToolInteractionSignature computes a hash signature for the tool
// interactions in a single step's content. It pairs tool calls with their
// results (matched by ToolCallID) and returns a hex-encoded SHA-256 hash.
// If the step contains no tool calls, it returns "".
func getToolInteractionSignature(content fantasy.ResponseContent) string {
	toolCalls := content.ToolCalls()
	if len(toolCalls) == 0 {
		return ""
	}

	// Index tool results by their ToolCallID for fast lookup.
	resultsByID := make(map[string]fantasy.ToolResultContent)
	for _, tr := range content.ToolResults() {
		resultsByID[tr.ToolCallID] = tr
	}

	h := sha256.New()
	for _, tc := range toolCalls {
		output := ""
		if tr, ok := resultsByID[tc.ToolCallID]; ok {
			output = toolResultOutputString(tr.Result)
		}
		io.WriteString(h, tc.ToolName)
		io.WriteString(h, "\x00")
		io.WriteString(h, tc.Input)
		io.WriteString(h, "\x00")
		io.WriteString(h, output)
		io.WriteString(h, "\x00")
	}
	return hex.EncodeToString(h.Sum(nil))
}

// toolResultOutputString converts a ToolResultOutputContent to a stable string
// representation for signature comparison.
func toolResultOutputString(result fantasy.ToolResultOutputContent) string {
	if result == nil {
		return ""
	}
	if text, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentText](result); ok {
		return text.Text
	}
	if errResult, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentError](result); ok {
		if errResult.Error != nil {
			return errResult.Error.Error()
		}
		return ""
	}
	if media, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentMedia](result); ok {
		return media.Data
	}
	return ""
}

func summarizeToolInteraction(content fantasy.ResponseContent) (repeatedToolLoop, bool) {
	toolCalls := content.ToolCalls()
	if len(toolCalls) == 0 {
		return repeatedToolLoop{}, false
	}

	toolNames := make([]string, 0, len(toolCalls))
	for _, tc := range toolCalls {
		toolNames = append(toolNames, tc.ToolName)
	}
	return repeatedToolLoop{
		Signature: getToolInteractionSignature(content),
		ToolNames: uniqueNonEmptyStrings(toolNames),
	}, true
}
