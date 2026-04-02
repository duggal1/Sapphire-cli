package agent

import (
	"fmt"
	"testing"

	"charm.land/fantasy"
)

// makeStep creates a StepResult with the given tool calls and results in its Content.
func makeStep(calls []fantasy.ToolCallContent, results []fantasy.ToolResultContent) fantasy.StepResult {
	var content fantasy.ResponseContent
	for _, c := range calls {
		content = append(content, c)
	}
	for _, r := range results {
		content = append(content, r)
	}
	return fantasy.StepResult{
		Response: fantasy.Response{
			Content: content,
		},
	}
}

// makeToolStep creates a step with a single tool call and matching text result.
func makeToolStep(name, input, output string) fantasy.StepResult {
	callID := fmt.Sprintf("call_%s_%s", name, input)
	return makeStep(
		[]fantasy.ToolCallContent{
			{ToolCallID: callID, ToolName: name, Input: input},
		},
		[]fantasy.ToolResultContent{
			{ToolCallID: callID, ToolName: name, Result: fantasy.ToolResultOutputContentText{Text: output}},
		},
	)
}

// makeEmptyStep creates a step with no tool calls (e.g. a text-only response).
func makeEmptyStep() fantasy.StepResult {
	return fantasy.StepResult{
		Response: fantasy.Response{
			Content: fantasy.ResponseContent{
				fantasy.TextContent{Text: "thinking..."},
			},
		},
	}
}

func makeReasoningStep(reasoning, text string) fantasy.StepResult {
	var content fantasy.ResponseContent
	if reasoning != "" {
		content = append(content, fantasy.ReasoningContent{Text: reasoning})
	}
	if text != "" {
		content = append(content, fantasy.TextContent{Text: text})
	}
	return fantasy.StepResult{
		Response: fantasy.Response{
			Content: content,
		},
	}
}

func TestHasRepeatedToolCalls(t *testing.T) {
	t.Run("no steps", func(t *testing.T) {
		result := hasRepeatedToolCalls(nil, 10, 5)
		if result {
			t.Error("expected false for empty steps")
		}
	})

	t.Run("fewer steps than window", func(t *testing.T) {
		steps := make([]fantasy.StepResult, 5)
		for i := range steps {
			steps[i] = makeToolStep("read", `{"file":"a.go"}`, "content")
		}
		result := hasRepeatedToolCalls(steps, 10, 5)
		if result {
			t.Error("expected false when fewer steps than window size")
		}
	})

	t.Run("all different signatures", func(t *testing.T) {
		steps := make([]fantasy.StepResult, 10)
		for i := range steps {
			steps[i] = makeToolStep("tool", fmt.Sprintf(`{"i":%d}`, i), fmt.Sprintf("result-%d", i))
		}
		result := hasRepeatedToolCalls(steps, 10, 5)
		if result {
			t.Error("expected false when all signatures are different")
		}
	})

	t.Run("exact repeat at threshold not detected", func(t *testing.T) {
		// maxRepeats=5 means > 5 is needed, so exactly 5 should return false
		steps := make([]fantasy.StepResult, 10)
		for i := 0; i < 5; i++ {
			steps[i] = makeToolStep("read", `{"file":"a.go"}`, "content")
		}
		for i := 5; i < 10; i++ {
			steps[i] = makeToolStep("tool", fmt.Sprintf(`{"i":%d}`, i), fmt.Sprintf("result-%d", i))
		}
		result := hasRepeatedToolCalls(steps, 10, 5)
		if result {
			t.Error("expected false when count equals maxRepeats (threshold is >)")
		}
	})

	t.Run("loop detected", func(t *testing.T) {
		// 6 identical steps in a window of 10 with maxRepeats=5 → detected
		steps := make([]fantasy.StepResult, 10)
		for i := 0; i < 6; i++ {
			steps[i] = makeToolStep("read", `{"file":"a.go"}`, "content")
		}
		for i := 6; i < 10; i++ {
			steps[i] = makeToolStep("tool", fmt.Sprintf(`{"i":%d}`, i), fmt.Sprintf("result-%d", i))
		}
		result := hasRepeatedToolCalls(steps, 10, 5)
		if !result {
			t.Error("expected true when same signature appears more than maxRepeats times")
		}
	})

	t.Run("steps without tool calls are skipped", func(t *testing.T) {
		// Mix of tool steps and empty steps — empty ones should not affect counts
		steps := make([]fantasy.StepResult, 10)
		for i := 0; i < 4; i++ {
			steps[i] = makeToolStep("read", `{"file":"a.go"}`, "content")
		}
		for i := 4; i < 8; i++ {
			steps[i] = makeEmptyStep()
		}
		for i := 8; i < 10; i++ {
			steps[i] = makeToolStep("write", `{"file":"b.go"}`, "ok")
		}
		result := hasRepeatedToolCalls(steps, 10, 5)
		if result {
			t.Error("expected false: only 4 repeated tool calls, empty steps should be skipped")
		}
	})

	t.Run("multiple different patterns alternating", func(t *testing.T) {
		// A,B repeated as a suffix pattern is a real loop and should be detected.
		steps := make([]fantasy.StepResult, 10)
		for i := range steps {
			if i%2 == 0 {
				steps[i] = makeToolStep("read", `{"file":"a.go"}`, "content-a")
			} else {
				steps[i] = makeToolStep("write", `{"file":"b.go"}`, "content-b")
			}
		}
		result := hasRepeatedToolCalls(steps, 10, 5)
		if !result {
			t.Error("expected true: alternating suffix pattern should now be treated as a loop")
		}
	})
}

func TestDetectRepeatedToolCallsReturnsLoopDetails(t *testing.T) {
	t.Parallel()

	steps := make([]fantasy.StepResult, 10)
	for i := 0; i < 6; i++ {
		steps[i] = makeToolStep("read", `{"file":"a.go"}`, "content")
	}
	for i := 6; i < 10; i++ {
		steps[i] = makeToolStep("tool", fmt.Sprintf(`{"i":%d}`, i), fmt.Sprintf("result-%d", i))
	}

	loop, ok := detectRepeatedToolCalls(steps, 10, 5)
	if !ok {
		t.Fatal("expected loop details")
	}
	if loop.RepeatCount != 6 {
		t.Fatalf("unexpected repeat count: %d", loop.RepeatCount)
	}
	if loop.WindowSize != 10 {
		t.Fatalf("unexpected window size: %d", loop.WindowSize)
	}
	if len(loop.ToolNames) != 1 || loop.ToolNames[0] != "read" {
		t.Fatalf("unexpected tool names: %#v", loop.ToolNames)
	}
	if loop.Signature == "" {
		t.Fatal("expected loop signature to be populated")
	}
}

func TestDetectRepeatedToolCallsDetectsSuffixPattern(t *testing.T) {
	t.Parallel()

	steps := []fantasy.StepResult{
		makeToolStep("read", `{"file":"a.go"}`, "content-a"),
		makeToolStep("write", `{"file":"b.go"}`, "content-b"),
		makeToolStep("patch", `{"file":"c.go"}`, "content-c"),
		makeToolStep("read", `{"file":"a.go"}`, "content-a"),
		makeToolStep("write", `{"file":"b.go"}`, "content-b"),
		makeToolStep("patch", `{"file":"c.go"}`, "content-c"),
		makeToolStep("read", `{"file":"a.go"}`, "content-a"),
		makeToolStep("write", `{"file":"b.go"}`, "content-b"),
		makeToolStep("patch", `{"file":"c.go"}`, "content-c"),
	}

	loop, ok := detectRepeatedToolCalls(steps, 10, 5)
	if !ok {
		t.Fatal("expected suffix pattern loop")
	}
	if loop.PatternSize != 3 {
		t.Fatalf("unexpected pattern size: %d", loop.PatternSize)
	}
	if loop.RepeatCount != 3 {
		t.Fatalf("unexpected repeat count: %d", loop.RepeatCount)
	}
	if loop.WindowSize != 9 {
		t.Fatalf("unexpected loop coverage: %d", loop.WindowSize)
	}
	if len(loop.ToolNames) != 3 {
		t.Fatalf("unexpected tool names: %#v", loop.ToolNames)
	}
}

func TestDetectRepeatedToolCallsDetectsRecentSuffixPattern(t *testing.T) {
	t.Parallel()

	steps := []fantasy.StepResult{
		makeToolStep("read", `{"file":"a.go"}`, "content-a"),
		makeToolStep("write", `{"file":"b.go"}`, "content-b"),
		makeToolStep("patch", `{"file":"c.go"}`, "content-c"),
		makeToolStep("read", `{"file":"a.go"}`, "content-a"),
		makeToolStep("write", `{"file":"b.go"}`, "content-b"),
		makeToolStep("patch", `{"file":"c.go"}`, "content-c"),
		makeToolStep("shell", `{"command":"ls"}`, "list"),
		makeToolStep("grep", `{"pattern":"TODO"}`, "todo"),
		makeToolStep("shell", `{"command":"ls"}`, "list"),
		makeToolStep("grep", `{"pattern":"TODO"}`, "todo"),
		makeToolStep("shell", `{"command":"ls"}`, "list"),
		makeToolStep("grep", `{"pattern":"TODO"}`, "todo"),
	}

	loop, ok := detectRepeatedToolCalls(steps, 12, 5)
	if !ok {
		t.Fatal("expected recent suffix pattern loop")
	}
	if loop.PatternSize != 2 {
		t.Fatalf("unexpected pattern size: %d", loop.PatternSize)
	}
	if loop.RepeatCount != 3 {
		t.Fatalf("unexpected repeat count: %d", loop.RepeatCount)
	}
	if len(loop.ToolNames) != 2 {
		t.Fatalf("unexpected tool names: %#v", loop.ToolNames)
	}
}

func TestDetectRepeatedToolCallsSkipsPartialSuffixPattern(t *testing.T) {
	t.Parallel()

	steps := []fantasy.StepResult{
		makeToolStep("read", `{"file":"a.go"}`, "content-a"),
		makeToolStep("write", `{"file":"b.go"}`, "content-b"),
		makeToolStep("patch", `{"file":"c.go"}`, "content-c"),
		makeToolStep("read", `{"file":"a.go"}`, "content-a"),
		makeToolStep("write", `{"file":"b.go"}`, "content-b"),
	}

	if loop, ok := detectRepeatedToolCalls(steps, 10, 5); ok {
		t.Fatalf("did not expect partial suffix pattern loop, got %#v", loop)
	}
}

func TestDetectRepeatedToolCallsDetectsRepeatedReasoning(t *testing.T) {
	t.Parallel()

	reasoning := "We should keep the same architecture because it looks balanced across migration cost, simplicity, and validation burden, even though we have not gathered new evidence from the repo."
	text := "The current best option is still the same architecture because the tradeoffs still appear favorable without any new repository evidence."
	steps := []fantasy.StepResult{
		makeReasoningStep(reasoning, text),
		makeReasoningStep(reasoning, text),
		makeReasoningStep(reasoning, text),
		makeReasoningStep(reasoning, text),
	}

	loop, ok := detectRepeatedToolCalls(steps, 10, 5)
	if !ok {
		t.Fatal("expected repeated reasoning loop")
	}
	if loop.LoopSource != "reasoning" {
		t.Fatalf("unexpected loop source: %q", loop.LoopSource)
	}
	if loop.RepeatCount != 4 {
		t.Fatalf("unexpected repeat count: %d", loop.RepeatCount)
	}
	if loop.Summary == "" {
		t.Fatal("expected reasoning loop summary")
	}
}

func TestDetectRepeatedToolCallsDetectsReasoningSuffixPattern(t *testing.T) {
	t.Parallel()

	reasoningA := "We should preserve the current event pipeline because it minimizes surface-area change, but we still need stronger repo evidence and should stop restating this point without new proof."
	textA := "Option A still looks safer because it keeps the existing event flow and avoids widening the blast radius before validation."
	reasoningB := "We should instead replace the current path with a narrower architecture because the previous design keeps failing, but we still need real evidence and should not keep recycling this conclusion."
	textB := "Option B still looks cleaner because it simplifies the path, but repeating this conclusion without new evidence is not progress."

	steps := []fantasy.StepResult{
		makeReasoningStep(reasoningA, textA),
		makeReasoningStep(reasoningB, textB),
		makeReasoningStep(reasoningA, textA),
		makeReasoningStep(reasoningB, textB),
		makeReasoningStep(reasoningA, textA),
		makeReasoningStep(reasoningB, textB),
	}

	loop, ok := detectRepeatedToolCalls(steps, 10, 5)
	if !ok {
		t.Fatal("expected reasoning suffix pattern loop")
	}
	if loop.LoopSource != "reasoning" {
		t.Fatalf("unexpected loop source: %q", loop.LoopSource)
	}
	if loop.PatternSize != 2 {
		t.Fatalf("unexpected pattern size: %d", loop.PatternSize)
	}
	if loop.RepeatCount != 3 {
		t.Fatalf("unexpected repeat count: %d", loop.RepeatCount)
	}
}

func TestDetectRepeatedToolCallsSkipsShortReasoning(t *testing.T) {
	t.Parallel()

	steps := []fantasy.StepResult{
		makeReasoningStep("", "same thought"),
		makeReasoningStep("", "same thought"),
		makeReasoningStep("", "same thought"),
		makeReasoningStep("", "same thought"),
	}

	if loop, ok := detectRepeatedToolCalls(steps, 10, 5); ok {
		t.Fatalf("did not expect short reasoning loop, got %#v", loop)
	}
}

func TestGetToolInteractionSignature(t *testing.T) {
	t.Run("empty content returns empty string", func(t *testing.T) {
		sig := getToolInteractionSignature(fantasy.ResponseContent{})
		if sig != "" {
			t.Errorf("expected empty string, got %q", sig)
		}
	})

	t.Run("text only content returns empty string", func(t *testing.T) {
		content := fantasy.ResponseContent{
			fantasy.TextContent{Text: "hello"},
		}
		sig := getToolInteractionSignature(content)
		if sig != "" {
			t.Errorf("expected empty string, got %q", sig)
		}
	})

	t.Run("tool call with result produces signature", func(t *testing.T) {
		content := fantasy.ResponseContent{
			fantasy.ToolCallContent{ToolCallID: "1", ToolName: "read", Input: `{"file":"a.go"}`},
			fantasy.ToolResultContent{ToolCallID: "1", ToolName: "read", Result: fantasy.ToolResultOutputContentText{Text: "content"}},
		}
		sig := getToolInteractionSignature(content)
		if sig == "" {
			t.Error("expected non-empty signature")
		}
	})

	t.Run("same interactions produce same signature", func(t *testing.T) {
		content1 := fantasy.ResponseContent{
			fantasy.ToolCallContent{ToolCallID: "1", ToolName: "read", Input: `{"file":"a.go"}`},
			fantasy.ToolResultContent{ToolCallID: "1", ToolName: "read", Result: fantasy.ToolResultOutputContentText{Text: "content"}},
		}
		content2 := fantasy.ResponseContent{
			fantasy.ToolCallContent{ToolCallID: "2", ToolName: "read", Input: `{"file":"a.go"}`},
			fantasy.ToolResultContent{ToolCallID: "2", ToolName: "read", Result: fantasy.ToolResultOutputContentText{Text: "content"}},
		}
		sig1 := getToolInteractionSignature(content1)
		sig2 := getToolInteractionSignature(content2)
		if sig1 != sig2 {
			t.Errorf("expected same signature for same interactions, got %q and %q", sig1, sig2)
		}
	})

	t.Run("different inputs produce different signatures", func(t *testing.T) {
		content1 := fantasy.ResponseContent{
			fantasy.ToolCallContent{ToolCallID: "1", ToolName: "read", Input: `{"file":"a.go"}`},
			fantasy.ToolResultContent{ToolCallID: "1", ToolName: "read", Result: fantasy.ToolResultOutputContentText{Text: "content"}},
		}
		content2 := fantasy.ResponseContent{
			fantasy.ToolCallContent{ToolCallID: "1", ToolName: "read", Input: `{"file":"b.go"}`},
			fantasy.ToolResultContent{ToolCallID: "1", ToolName: "read", Result: fantasy.ToolResultOutputContentText{Text: "content"}},
		}
		sig1 := getToolInteractionSignature(content1)
		sig2 := getToolInteractionSignature(content2)
		if sig1 == sig2 {
			t.Error("expected different signatures for different inputs")
		}
	})
}
