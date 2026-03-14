# Summarization & Context Refresh

## Automatic summarization triggers
- The session agent keeps constants for large contexts so OpenAI/Gemini/Opus models never hit the hard  context limit: `largeContextWindowThreshold = 200_000`, `largeContextWindowBuffer = 20_000`, and `smallContextWindowRatio = 0.2` are defined at `internal/agent/agent.go:55-66`.
- Every `Run` call attaches a `StopWhen` condition that re-evaluates `cw` (the model context window) versus `tokens = currentSession.CompletionTokens + currentSession.PromptTokens`. When the remaining budget falls below `threshold = buffer` (or `cw * 0.2` for smaller windows), the stream exits early with `shouldSummarize = true`, and the same `agent.Stream` call is interrupted before the model runs out of context (`internal/agent/agent.go:599-657`).
- On the same stop condition we also run a pre-compaction checkpoint once when `tokens >= 65% of cw` so that `pmem` has a fresh checkpoint to include in the next injection block (`a.pmem.RunPreCompactionCheckpoint`).

```go
StopWhen: []fantasy.StopCondition{
    func(steps []fantasy.StepResult) bool {
        if genCtx.Err() != nil {
            return true
        }

        cw := int64(largeModel.CatwalkCfg.ContextWindow)
        tokens := currentSession.CompletionTokens + currentSession.PromptTokens
        remaining := cw - tokens
        var threshold int64
        if cw > largeContextWindowThreshold {
            threshold = largeContextWindowBuffer
        } else {
            threshold = int64(float64(cw) * smallContextWindowRatio)
        }

        if cw > 0 && float64(tokens) >= float64(cw)*0.65 {
            if a.pmem.ShouldRunCheckpoint() {
                a.pmem.MarkCheckpointDone()
                _ = a.pmem.RunPreCompactionCheckpoint(ctx, call.SessionID, "20")
            }
        }

        if cw > 0 && (remaining <= threshold) && !a.disableAutoSummarize {
            shouldSummarize = true
            return true
        }
        return false
    },
    func(steps []fantasy.StepResult) bool {
        return hasRepeatedToolCalls(steps, loopDetectionWindowSize, loopDetectionMaxRepeats)
    },
},
```

## Summarization execution
- When the stream exits with `shouldSummarize`, `sessionAgent.Run` calls `Summarize` before re-queueing the current user prompt (`internal/agent/agent.go:705-750`). This keeps the same `largeModel`/provider options (Opus 4.6 or similar) but starts a fresh streaming session with an updated context window.
- `Summarize` has two stages: a narrative summary that is written back as an assistant message flagged `IsSummaryMessage: true`, and a structured extraction that produces JSON matching `StructuredSummaryData` and caches it via `MemoryService.CreateStructuredSummary`. Both agents use the same large model and the templates `summary.md` and `structured_summary.md`, so the summary is faithful to the user intent and the todo list.

```go
summaryMessage, err := a.messages.Create(ctx, sessionID, message.CreateMessageParams{
    Role:             message.Assistant,
    Model:            largeModel.Model.Model(),
    Provider:         largeModel.Model.Provider(),
    IsSummaryMessage: true,
})
...
structuredAgent := fantasy.NewAgent(largeModel.Model,
    fantasy.WithSystemPrompt(string(structuredSummaryPromptTmpl)),
)
structuredResp, err := structuredAgent.Stream(...)
if err == nil {
    var data memory.StructuredSummaryData
    jsonStr := structuredResp.Response.Content.Text()
    ...
    if err := json.Unmarshal([]byte(jsonStr), &data); err == nil {
        _ = a.memory.CreateStructuredSummary(ctx, sessionID, data)
    }
}
```

- After the narrative summary finishes the session record gets `SummaryMessageID` updated, usage stats updated, and, if the persistent memory system (`pmem`) exists, `pmem.ResetCheckpointState()` ensures the next turn injects the latest summaries/constraints rather than an old checkpoint stack.

## Context refresh + re-run loop
- Once summarization finishes, the original `SessionAgentCall` is requeued. If the assistant issued tool calls before the stop, the prompt is rewritten to highlight the interruption (`"The previous session was interrupted because it got too long, the initial user request was: `%s`"`). The call sits in `messageQueue` until the current request completes.

```go
if shouldSummarize {
    a.activeRequests.Del(call.SessionID)
    if summarizeErr := a.Summarize(genCtx, call.SessionID, call.ProviderOptions); summarizeErr != nil {
        return nil, summarizeErr
    }
    existing, ok := a.messageQueue.Get(call.SessionID)
    if !ok {
        existing = []SessionAgentCall{}
    }
    if len(currentAssistant.ToolCalls()) > 0 {
        call.Prompt = fmt.Sprintf("The previous session was interrupted because it got too long, the initial user request was: `%s`", call.Prompt)
    }
    existing = append(existing, call)
    a.messageQueue.Set(call.SessionID, existing)
}
...
return a.Run(ctx, firstQueuedMessage)
```

- Because the call is retried immediately with the same model/provider options and the summary lives as an assistant message, the downstream stream sees a “fresh” context window even though long-term knowledge (summaries, constitution, structured memory) is preserved. This loop is the LLM-side equivalent of starting the same Opus 4.6 run with a new 0% context load and only the summary payload.

## Manual “Summarize session” command
- The UI exposes `Commands → Summarize Session` only when a session is active. Selecting it dispatches `dialog.ActionSummarize`, which calls `AgentCoordinator.Summarize` directly (`internal/ui/dialog/commands.go:369-406` and `internal/ui/dialog/actions.go:126-146`).
- That coordinator method is the same `sessionAgent.Summarize` pipeline, so manual summarization mirrors the automatic flow (summary message, structured JSON, memory updates) and can be invoked at will to force a fresh context before continuing.

```go
case dialog.ActionSummarize:
    if m.isAgentBusy() {
        cmds = append(cmds, util.ReportWarn("Agent is busy, please wait before summarizing session..."))
        break
    }
    cmds = append(cmds, func() tea.Msg {
        err := m.com.App.AgentCoordinator.Summarize(context.Background(), msg.SessionID)
        if err != nil {
            return util.ReportError(err)()
        }
        return nil
    })
    m.dialog.CloseDialog(dialog.CommandsID)
```

## Summaries as code / memory artifacts
- The narrative summary obeys the instructions in `internal/agent/templates/summary.md`, which demands sections for current state, files, technical context, strategy, and exact next steps so that nothing is lost when the LLM loses raw history.
- The structured summary output matches `memory.StructuredSummaryData` (`internal/agent/memory/memory.go:63-145`), so downstream tools (tiered memory, `memory_query`, dashboards) get decisions, file-change descriptions, failure modes, dependency edges, and todo states in a fixed schema.
- Together, these summaries are the “context delta” that lets the agent continue long-haul tasks with a single persistent model instance and never exceed the provider’s context window.
