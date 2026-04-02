package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strings"

	"charm.land/fantasy"
)

const (
	loopDetectionWindowSize = 10
	loopDetectionMaxRepeats = 5
	loopPatternMinRepeats   = 3
	reasoningLoopMaxRepeats = 3
	reasoningLoopMinChars   = 120
)

type repeatedToolLoop struct {
	Signature   string
	RepeatCount int
	WindowSize  int
	ToolNames   []string
	PatternSize int
	LoopSource  string
	Summary     string
}

// hasRepeatedToolCalls checks whether the agent is stuck in a loop by looking
// at recent steps. It examines the last windowSize steps and returns true if
// any tool-call signature appears more than maxRepeats times.
func hasRepeatedToolCalls(steps []fantasy.StepResult, windowSize, maxRepeats int) bool {
	_, detected := detectRepeatedToolCalls(steps, windowSize, maxRepeats)
	return detected
}

func detectRepeatedToolCalls(steps []fantasy.StepResult, windowSize, maxRepeats int) (repeatedToolLoop, bool) {
	if len(steps) == 0 {
		return repeatedToolLoop{}, false
	}

	window := steps
	if windowSize > 0 && len(window) > windowSize {
		window = steps[len(steps)-windowSize:]
	}
	toolInteractions := make([]repeatedToolLoop, 0, len(window))
	reasoningInteractions := make([]repeatedToolLoop, 0, len(window))
	for _, step := range window {
		if loop, ok := summarizeToolInteraction(step.Content); ok && loop.Signature != "" {
			toolInteractions = append(toolInteractions, loop)
			continue
		}
		if loop, ok := summarizeReasoningInteraction(step.Content); ok && loop.Signature != "" {
			reasoningInteractions = append(reasoningInteractions, loop)
		}
	}

	if loop, ok := detectRepeatedInteractionWindow(toolInteractions, len(window), maxRepeats, loopPatternMinRepeats); ok {
		return loop, true
	}
	if loop, ok := detectRepeatedInteractionWindow(reasoningInteractions, len(window), reasoningLoopMaxRepeats, loopPatternMinRepeats); ok {
		return loop, true
	}

	return repeatedToolLoop{}, false
}

func detectRepeatedInteractionWindow(sequence []repeatedToolLoop, windowSize, maxRepeats, minPatternRepeats int) (repeatedToolLoop, bool) {
	if len(sequence) == 0 {
		return repeatedToolLoop{}, false
	}

	counts := make(map[string]int, len(sequence))
	for _, loop := range sequence {
		if loop.Signature == "" {
			continue
		}
		counts[loop.Signature]++
		if counts[loop.Signature] > maxRepeats {
			loop.RepeatCount = counts[loop.Signature]
			loop.WindowSize = windowSize
			return loop, true
		}
	}

	return detectRepeatedInteractionSuffixPattern(sequence, minPatternRepeats)
}

func detectRepeatedInteractionSuffixPattern(sequence []repeatedToolLoop, minRepeats int) (repeatedToolLoop, bool) {
	if minRepeats < 2 || len(sequence) < minRepeats*2 {
		return repeatedToolLoop{}, false
	}

	best := repeatedToolLoop{}
	bestCoverage := 0
	maxPatternSize := len(sequence) / minRepeats
	for patternSize := 2; patternSize <= maxPatternSize; patternSize++ {
		pattern := sequence[len(sequence)-patternSize:]
		if !patternHasDiversity(pattern) {
			continue
		}

		repeatCount := 1
		for pos := len(sequence) - patternSize; pos >= patternSize; pos -= patternSize {
			chunk := sequence[pos-patternSize : pos]
			if !sameToolInteractionPattern(chunk, pattern) {
				break
			}
			repeatCount++
		}
		if repeatCount < minRepeats {
			continue
		}

		coverage := repeatCount * patternSize
		if coverage < bestCoverage || (coverage == bestCoverage && patternSize <= best.PatternSize) {
			continue
		}
		bestCoverage = coverage
		best = repeatedToolLoop{
			Signature:   getToolPatternSignature(pattern),
			RepeatCount: repeatCount,
			WindowSize:  coverage,
			ToolNames:   collectPatternToolNames(pattern),
			PatternSize: patternSize,
			LoopSource:  dominantLoopSource(pattern),
			Summary:     collectPatternSummary(pattern),
		}
	}

	return best, bestCoverage > 0
}

func patternHasDiversity(pattern []repeatedToolLoop) bool {
	if len(pattern) < 2 {
		return false
	}
	seen := make(map[string]struct{}, len(pattern))
	for _, interaction := range pattern {
		if interaction.Signature == "" {
			continue
		}
		seen[interaction.Signature] = struct{}{}
		if len(seen) > 1 {
			return true
		}
	}
	return false
}

func sameToolInteractionPattern(a, b []repeatedToolLoop) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Signature != b[i].Signature {
			return false
		}
	}
	return true
}

func collectPatternToolNames(pattern []repeatedToolLoop) []string {
	names := make([]string, 0, len(pattern))
	for _, interaction := range pattern {
		names = append(names, interaction.ToolNames...)
	}
	return uniqueNonEmptyStrings(names)
}

func dominantLoopSource(pattern []repeatedToolLoop) string {
	if len(pattern) == 0 {
		return ""
	}
	best := strings.TrimSpace(pattern[0].LoopSource)
	for _, interaction := range pattern {
		source := strings.TrimSpace(interaction.LoopSource)
		if source != "" {
			best = source
			break
		}
	}
	return best
}

func collectPatternSummary(pattern []repeatedToolLoop) string {
	for _, interaction := range pattern {
		if summary := strings.TrimSpace(interaction.Summary); summary != "" {
			return summary
		}
	}
	return ""
}

func getToolPatternSignature(pattern []repeatedToolLoop) string {
	if len(pattern) == 0 {
		return ""
	}
	h := sha256.New()
	for _, interaction := range pattern {
		io.WriteString(h, interaction.Signature)
		io.WriteString(h, "\x00")
		io.WriteString(h, strings.Join(interaction.ToolNames, ","))
		io.WriteString(h, "\x00")
	}
	return hex.EncodeToString(h.Sum(nil))
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
		Signature:  getToolInteractionSignature(content),
		ToolNames:  uniqueNonEmptyStrings(toolNames),
		LoopSource: "tool",
	}, true
}

func summarizeReasoningInteraction(content fantasy.ResponseContent) (repeatedToolLoop, bool) {
	if len(content.ToolCalls()) > 0 {
		return repeatedToolLoop{}, false
	}

	reasoning := normalizeRepeatedReasoningText(content.ReasoningText())
	text := normalizeRepeatedReasoningText(content.Text())
	combined := reasoning
	if combined == "" {
		combined = text
	} else if text != "" {
		combined += "\n" + text
	}
	if len(combined) < reasoningLoopMinChars {
		return repeatedToolLoop{}, false
	}

	h := sha256.New()
	io.WriteString(h, combined)

	return repeatedToolLoop{
		Signature:  hex.EncodeToString(h.Sum(nil)),
		LoopSource: "reasoning",
		Summary:    summarizeRepeatedReasoningExcerpt(text, reasoning),
	}, true
}

func normalizeRepeatedReasoningText(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return ""
	}
	return strings.Join(strings.Fields(text), " ")
}

func summarizeRepeatedReasoningExcerpt(text, reasoning string) string {
	summary := strings.TrimSpace(text)
	if summary == "" {
		summary = strings.TrimSpace(reasoning)
	}
	if len(summary) > 160 {
		summary = summary[:160]
	}
	return summary
}
