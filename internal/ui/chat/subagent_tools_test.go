package chat

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/duggal1/Sapphire-cli/internal/agent"
	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
)

func TestShortenSubAgentID(t *testing.T) {
	t.Parallel()

	if got := shortenSubAgentID("agent-d5655f40-f83b-46af-8b5d-2ff2becf4b78"); got != "d5655f40" {
		t.Fatalf("expected shortened agent id, got %q", got)
	}
	if got := shortenSubAgentID("e29e755d-11da-4d84-99d0-7031eae4f338"); got != "e29e755d" {
		t.Fatalf("expected shortened submission id, got %q", got)
	}
}

func TestFormatSubAgentElapsedAt(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 3, 20, 1, 0, 0, 0, time.UTC)
	now := start.Add(79 * time.Second)
	if got := formatSubAgentElapsedAt(start, now); got != "1m 19s" {
		t.Fatalf("expected 1m 19s, got %q", got)
	}
}

func TestRenderSubAgentWaitBodyUsesFriendlySummaryLine(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	rendered := ansi.Strip(renderSubAgentWaitBody(&sty, subAgentWaitResult{
		Agents: []subAgentStatusEntry{
			{
				ID:            "agent-d5655f40-f83b-46af-8b5d-2ff2becf4b78",
				Status:        "running",
				StartedAt:     time.Now().UTC().Add(-12 * time.Second),
				ToolCallCount: 3,
				CurrentTool:   "ReadFile",
			},
		},
	}, 120))

	if !strings.Contains(rendered, "Agent 1") {
		t.Fatalf("expected friendly label, got %q", rendered)
	}
	if strings.Contains(rendered, "d5655f40") {
		t.Fatalf("expected internal id to be hidden, got %q", rendered)
	}
	if !strings.Contains(rendered, "Agent 1 — running") {
		t.Fatalf("expected compact status line, got %q", rendered)
	}
	if !strings.Contains(rendered, "3 tools") {
		t.Fatalf("expected tool count, got %q", rendered)
	}
	if !strings.Contains(rendered, "current: ReadFile") {
		t.Fatalf("expected current tool summary, got %q", rendered)
	}
}

func TestSpawnAgentToolKeepsAnimatingWhileSubAgentIsRunning(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	payload, err := json.Marshal(subAgentSpawnResult{
		AgentID:   "agent-1",
		Status:    "running",
		StartedAt: time.Now().UTC().Add(-2 * time.Minute),
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	item := NewSpawnAgentToolMessageItem(&sty, message.ToolCall{
		ID:       "tool-1",
		Name:     agent.SpawnAgentToolName,
		Finished: true,
	}, &message.ToolResult{
		ToolCallID: "tool-1",
		Name:       agent.SpawnAgentToolName,
		Content:    string(payload),
	}, false)

	animating, ok := item.(AnimationActive)
	if !ok {
		t.Fatal("expected animation-capable spawn agent tool item")
	}
	if !animating.HasActiveAnimation() {
		t.Fatal("expected spawn agent row to keep animating while sub-agent is running")
	}
}

func TestRenderSubAgentSpawnBodyShowsPromptPreview(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	rendered := ansi.Strip(renderSubAgentSpawnBody(&sty, &agent.SpawnAgentParams{
		Title:   "Agent 1: Core Agent Logic Analysis",
		Message: "Read the core agent orchestration paths, identify the real control flow, and report exact risks with absolute file paths.",
		Agent:   "coder",
	}, &subAgentSpawnResult{
		AgentID: "agent-1",
		Status:  "running",
	}, 120, false))

	if !strings.Contains(rendered, "Task: Agent 1: Core Agent Logic Analysis") {
		t.Fatalf("expected task title, got %q", rendered)
	}
	if !strings.Contains(rendered, "Brief: Read the core agent orchestration paths") {
		t.Fatalf("expected prompt preview, got %q", rendered)
	}
}

func TestMergeSubAgentSpawnResultAppliesLiveTelemetry(t *testing.T) {
	t.Parallel()

	raw, err := json.Marshal(subAgentSpawnResult{
		AgentID:   "agent-1",
		Status:    "queued",
		StartedAt: time.Now().UTC().Add(-30 * time.Second),
	})
	if err != nil {
		t.Fatalf("marshal spawn result: %v", err)
	}

	merged, ok := MergeSubAgentSpawnResult(string(raw), agent.SubAgentLifecycleEvent{
		AgentID:          "agent-1",
		Status:           "running",
		Title:            "Core Agent Logic",
		HeartbeatContext: "running tool agentic_view",
		CurrentTool:      "agentic_view",
		ToolCallCount:    3,
	})
	if !ok {
		t.Fatal("expected spawn result merge to succeed")
	}

	sty := styles.DefaultStyles(false)
	rendered := ansi.Strip(renderSubAgentSpawnBody(&sty, &agent.SpawnAgentParams{
		Title:   "Core Agent Logic",
		Message: "Inspect the runtime path.",
	}, func() *subAgentSpawnResult {
		var payload subAgentSpawnResult
		if err := json.Unmarshal([]byte(merged), &payload); err != nil {
			t.Fatalf("unmarshal merged payload: %v", err)
		}
		return &payload
	}(), 120, false))

	if !strings.Contains(rendered, "Agent 1 — running") {
		t.Fatalf("expected compact status line, got %q", rendered)
	}
	if !strings.Contains(rendered, "3 tools") {
		t.Fatalf("expected tool call count, got %q", rendered)
	}
	if !strings.Contains(rendered, "current: agentic_view") {
		t.Fatalf("expected current tool telemetry, got %q", rendered)
	}
}

func TestRenderAgentDirectoryBodyHidesRawJSONAndIDs(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	raw, err := json.Marshal(agentDirectorySnapshot{
		SessionID:       "session-1",
		ParentSessionID: "session-1",
		CurrentAgentID:  "main:session-1",
		Agents: []agentDirectoryAgent{
			{
				AgentID:       "agent-12345678",
				Title:         "UI Analysis",
				Status:        "running",
				ToolCallCount: 2,
				CurrentTool:   "agentic_view",
			},
		},
		WorkItems: []agentDirectoryWorkItem{
			{ID: "work-1", Title: "UI audit", Status: "running"},
		},
	})
	if err != nil {
		t.Fatalf("marshal agent directory: %v", err)
	}

	rendered := ansi.Strip(renderAgentDirectoryBody(&sty, string(raw), 120))
	if !strings.Contains(rendered, "Agent Directory") {
		t.Fatalf("expected agent directory label, got %q", rendered)
	}
	if !strings.Contains(rendered, "Agent 1") && !strings.Contains(rendered, "Current Agent") {
		t.Fatalf("expected friendly agent label, got %q", rendered)
	}
	if strings.Contains(rendered, "session_id") || strings.Contains(rendered, "\"agents\"") {
		t.Fatalf("expected raw JSON to be hidden, got %q", rendered)
	}
}

func TestBackgroundSubAgentsPayloadActiveWhileRunning(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(agent.BackgroundSubAgentsToolPayload{
		Status: "running",
		Count:  1,
		Active: 1,
		Agents: []agent.BackgroundSubAgentView{{
			ID:        "agent-1",
			Status:    "running",
			StartedAt: time.Now().UTC(),
		}},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	if !backgroundSubAgentsPayloadActive(&message.ToolResult{Content: string(payload)}) {
		t.Fatal("expected background payload to remain active while sub-agent is running")
	}
}
