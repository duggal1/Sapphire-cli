package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	promptpkg "github.com/duggal1/Sapphire-cli/internal/agent/prompt"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/duggal1/Sapphire-cli/internal/lsp"
	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genai"
)

// mockSessionAgent is a minimal mock for the SessionAgent interface.
type mockSessionAgent struct {
	model     Model
	runFunc   func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error)
	enqueued  []SessionAgentCall
	busy      bool
	cancelled []string
}

func (m *mockSessionAgent) Run(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
	return m.runFunc(ctx, call)
}

func (m *mockSessionAgent) Model() Model                        { return m.model }
func (m *mockSessionAgent) SetModels(large, small Model)        {}
func (m *mockSessionAgent) SetTools(tools []fantasy.AgentTool)  {}
func (m *mockSessionAgent) SetWorkingDir(workingDir string)     {}
func (m *mockSessionAgent) SetSystemPrompt(systemPrompt string) {}
func (m *mockSessionAgent) SessionID() string                   { return "" }
func (m *mockSessionAgent) Cancel(sessionID string) {
	m.cancelled = append(m.cancelled, sessionID)
}
func (m *mockSessionAgent) CancelAll()                                  {}
func (m *mockSessionAgent) IsSessionBusy(sessionID string) bool         { return m.busy }
func (m *mockSessionAgent) IsBusy() bool                                { return m.busy }
func (m *mockSessionAgent) QueuedPrompts(sessionID string) int          { return len(m.enqueued) }
func (m *mockSessionAgent) QueuedPromptsList(sessionID string) []string { return nil }
func (m *mockSessionAgent) ClearQueue(sessionID string)                 {}
func (m *mockSessionAgent) Enqueue(call SessionAgentCall) error {
	m.enqueued = append(m.enqueued, call)
	return nil
}
func (m *mockSessionAgent) Summarize(context.Context, string, fantasy.ProviderOptions) error {
	return nil
}

// newTestCoordinator creates a minimal coordinator for unit testing runSubAgent.
func newTestCoordinator(t *testing.T, env fakeEnv, providerID string, providerCfg config.ProviderConfig) *coordinator {
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.Providers.Set(providerID, providerCfg)
	return &coordinator{
		cfg:                       cfg,
		sessions:                  env.sessions,
		messages:                  env.messages,
		backgroundSubAgentLimiter: make(chan struct{}, maxBackgroundSubAgents),
	}
}

// newMockAgent creates a mockSessionAgent with the given provider and run function.
func newMockAgent(providerID string, maxTokens int64, runFunc func(context.Context, SessionAgentCall) (*fantasy.AgentResult, error)) *mockSessionAgent {
	return &mockSessionAgent{
		model: Model{
			CatwalkCfg: catwalk.Model{
				DefaultMaxTokens: maxTokens,
			},
			ModelCfg: config.SelectedModel{
				Provider: providerID,
			},
		},
		runFunc: runFunc,
	}
}

// agentResultWithText creates a minimal AgentResult with the given text response.
func agentResultWithText(text string) *fantasy.AgentResult {
	return &fantasy.AgentResult{
		Response: fantasy.Response{
			Content: fantasy.ResponseContent{
				fantasy.TextContent{Text: text},
			},
		},
	}
}

func TestCoordinatorSubmitQueuesAcceptedPrompt(t *testing.T) {
	env := testEnv(t)
	session, err := env.sessions.Create(t.Context(), "queued")
	require.NoError(t, err)

	const providerID = "test-provider"
	coord := newTestCoordinator(t, env, providerID, config.ProviderConfig{ID: providerID})
	agent := newMockAgent(providerID, 4096, func(_ context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
		return agentResultWithText(call.Prompt), nil
	})
	agent.busy = true
	coord.currentAgent = agent

	result, err := coord.Submit(t.Context(), session.ID, "queued prompt")
	require.NoError(t, err)
	assert.Equal(t, SubmissionStatusQueued, result.Status)
	require.Len(t, agent.enqueued, 1)
	assert.True(t, agent.enqueued[0].SkipUserMessage)
	require.NotNil(t, agent.enqueued[0].PrecreatedUser)

	msgs, err := env.messages.List(t.Context(), session.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	assert.Equal(t, message.User, msgs[0].Role)
	assert.Equal(t, result.UserMessageID, msgs[0].ID)
}

func TestCoordinatorSubmitStartsDetachedExecution(t *testing.T) {
	env := testEnv(t)
	session, err := env.sessions.Create(t.Context(), "running")
	require.NoError(t, err)

	const providerID = "test-provider"
	coord := newTestCoordinator(t, env, providerID, config.ProviderConfig{ID: providerID})
	runCalls := make(chan SessionAgentCall, 1)
	agent := newMockAgent(providerID, 4096, func(_ context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
		runCalls <- call
		return agentResultWithText("ok"), nil
	})
	coord.currentAgent = agent

	result, err := coord.Submit(t.Context(), session.ID, "run prompt")
	require.NoError(t, err)
	assert.Equal(t, SubmissionStatusRunning, result.Status)

	select {
	case call := <-runCalls:
		assert.True(t, call.SkipUserMessage)
		require.NotNil(t, call.PrecreatedUser)
		assert.Equal(t, "run prompt", call.Prompt)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for detached run")
	}
}

func TestRunSubAgent(t *testing.T) {
	const providerID = "test-provider"
	providerCfg := config.ProviderConfig{ID: providerID}

	t.Run("happy path", func(t *testing.T) {
		env := testEnv(t)
		coord := newTestCoordinator(t, env, providerID, providerCfg)

		parentSession, err := env.sessions.Create(t.Context(), "Parent")
		require.NoError(t, err)

		agent := newMockAgent(providerID, 4096, func(_ context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
			assert.Equal(t, "do something", call.Prompt)
			assert.Equal(t, int64(4096), call.MaxOutputTokens)
			return agentResultWithText("done"), nil
		})

		resp, err := coord.runSubAgent(t.Context(), subAgentParams{
			Agent:          agent,
			SessionID:      parentSession.ID,
			AgentMessageID: "msg-1",
			ToolCallID:     "call-1",
			Prompt:         "do something",
			SessionTitle:   "Test Session",
		})
		require.NoError(t, err)
		assert.Equal(t, "done", resp.Content)
		assert.False(t, resp.IsError)
	})

	t.Run("ModelCfg.MaxTokens overrides default", func(t *testing.T) {
		env := testEnv(t)
		coord := newTestCoordinator(t, env, providerID, providerCfg)

		parentSession, err := env.sessions.Create(t.Context(), "Parent")
		require.NoError(t, err)

		agent := &mockSessionAgent{
			model: Model{
				CatwalkCfg: catwalk.Model{
					DefaultMaxTokens: 4096,
				},
				ModelCfg: config.SelectedModel{
					Provider:  providerID,
					MaxTokens: 8192,
				},
			},
			runFunc: func(_ context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
				assert.Equal(t, int64(8192), call.MaxOutputTokens)
				return agentResultWithText("ok"), nil
			},
		}

		resp, err := coord.runSubAgent(t.Context(), subAgentParams{
			Agent:          agent,
			SessionID:      parentSession.ID,
			AgentMessageID: "msg-1",
			ToolCallID:     "call-1",
			Prompt:         "test",
			SessionTitle:   "Test",
		})
		require.NoError(t, err)
		assert.Equal(t, "ok", resp.Content)
	})

	t.Run("session creation failure with canceled context", func(t *testing.T) {
		env := testEnv(t)
		coord := newTestCoordinator(t, env, providerID, providerCfg)

		parentSession, err := env.sessions.Create(t.Context(), "Parent")
		require.NoError(t, err)

		agent := newMockAgent(providerID, 4096, nil)

		// Use a canceled context to trigger CreateTaskSession failure.
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		_, err = coord.runSubAgent(ctx, subAgentParams{
			Agent:          agent,
			SessionID:      parentSession.ID,
			AgentMessageID: "msg-1",
			ToolCallID:     "call-1",
			Prompt:         "test",
			SessionTitle:   "Test",
		})
		require.Error(t, err)
	})

	t.Run("provider not configured", func(t *testing.T) {
		env := testEnv(t)
		coord := newTestCoordinator(t, env, providerID, providerCfg)

		parentSession, err := env.sessions.Create(t.Context(), "Parent")
		require.NoError(t, err)

		// Agent references a provider that doesn't exist in config.
		agent := newMockAgent("unknown-provider", 4096, nil)

		_, err = coord.runSubAgent(t.Context(), subAgentParams{
			Agent:          agent,
			SessionID:      parentSession.ID,
			AgentMessageID: "msg-1",
			ToolCallID:     "call-1",
			Prompt:         "test",
			SessionTitle:   "Test",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "model provider not configured")
	})

	t.Run("agent run error returns error response", func(t *testing.T) {
		env := testEnv(t)
		coord := newTestCoordinator(t, env, providerID, providerCfg)

		parentSession, err := env.sessions.Create(t.Context(), "Parent")
		require.NoError(t, err)

		agent := newMockAgent(providerID, 4096, func(_ context.Context, _ SessionAgentCall) (*fantasy.AgentResult, error) {
			return nil, errors.New("agent exploded")
		})

		resp, err := coord.runSubAgent(t.Context(), subAgentParams{
			Agent:          agent,
			SessionID:      parentSession.ID,
			AgentMessageID: "msg-1",
			ToolCallID:     "call-1",
			Prompt:         "test",
			SessionTitle:   "Test",
		})
		// runSubAgent returns (errorResponse, nil) when agent.Run fails — not a Go error.
		require.NoError(t, err)
		assert.True(t, resp.IsError)
		assert.Equal(t, "error generating response", resp.Content)
	})

	t.Run("session setup callback is invoked", func(t *testing.T) {
		env := testEnv(t)
		coord := newTestCoordinator(t, env, providerID, providerCfg)

		parentSession, err := env.sessions.Create(t.Context(), "Parent")
		require.NoError(t, err)

		var setupCalledWith string
		agent := newMockAgent(providerID, 4096, func(_ context.Context, _ SessionAgentCall) (*fantasy.AgentResult, error) {
			return agentResultWithText("ok"), nil
		})

		_, err = coord.runSubAgent(t.Context(), subAgentParams{
			Agent:          agent,
			SessionID:      parentSession.ID,
			AgentMessageID: "msg-1",
			ToolCallID:     "call-1",
			Prompt:         "test",
			SessionTitle:   "Test",
			SessionSetup: func(sessionID string) {
				setupCalledWith = sessionID
			},
		})
		require.NoError(t, err)
		assert.NotEmpty(t, setupCalledWith, "SessionSetup should have been called")
	})

	t.Run("nested sub-agent spawn is rejected", func(t *testing.T) {
		env := testEnv(t)
		coord := newTestCoordinator(t, env, providerID, providerCfg)

		parentSession, err := env.sessions.Create(t.Context(), "Parent")
		require.NoError(t, err)

		childSession, err := env.sessions.CreateTaskSession(t.Context(), "tool-1", parentSession.ID, "Child")
		require.NoError(t, err)

		agent := newMockAgent(providerID, 4096, func(_ context.Context, _ SessionAgentCall) (*fantasy.AgentResult, error) {
			t.Fatal("nested sub-agent should not run")
			return nil, nil
		})

		resp, err := coord.runSubAgent(t.Context(), subAgentParams{
			Agent:          agent,
			SessionID:      childSession.ID,
			AgentMessageID: "msg-1",
			ToolCallID:     "call-1",
			Prompt:         "test",
			SessionTitle:   "Nested",
		})
		require.NoError(t, err)
		assert.True(t, resp.IsError)
		assert.Equal(t, "sub-agents cannot spawn sub-agents", resp.Content)
	})

	t.Run("nested sub-agent spawn is allowed when requested", func(t *testing.T) {
		env := testEnv(t)
		coord := newTestCoordinator(t, env, providerID, providerCfg)

		parentSession, err := env.sessions.Create(t.Context(), "Parent")
		require.NoError(t, err)

		childSession, err := env.sessions.CreateTaskSession(t.Context(), "tool-1", parentSession.ID, "Child")
		require.NoError(t, err)

		agent := newMockAgent(providerID, 4096, func(_ context.Context, _ SessionAgentCall) (*fantasy.AgentResult, error) {
			return agentResultWithText("nested ok"), nil
		})

		resp, err := coord.runSubAgent(t.Context(), subAgentParams{
			Agent:          agent,
			SessionID:      childSession.ID,
			AgentMessageID: "msg-1",
			ToolCallID:     "call-1",
			Prompt:         "test",
			SessionTitle:   "Nested Allowed",
			AllowNesting:   true,
		})
		require.NoError(t, err)
		assert.False(t, resp.IsError)
		assert.Equal(t, "nested ok", resp.Content)
	})

	t.Run("cost propagation to parent session", func(t *testing.T) {
		env := testEnv(t)
		coord := newTestCoordinator(t, env, providerID, providerCfg)

		parentSession, err := env.sessions.Create(t.Context(), "Parent")
		require.NoError(t, err)

		agent := newMockAgent(providerID, 4096, func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
			// Simulate the agent incurring cost by updating the child session.
			childSession, err := env.sessions.Get(ctx, call.SessionID)
			if err != nil {
				return nil, err
			}
			childSession.Cost = 0.05
			_, err = env.sessions.Save(ctx, childSession)
			if err != nil {
				return nil, err
			}
			return agentResultWithText("ok"), nil
		})

		_, err = coord.runSubAgent(t.Context(), subAgentParams{
			Agent:          agent,
			SessionID:      parentSession.ID,
			AgentMessageID: "msg-1",
			ToolCallID:     "call-1",
			Prompt:         "test",
			SessionTitle:   "Test",
		})
		require.NoError(t, err)

		updated, err := env.sessions.Get(t.Context(), parentSession.ID)
		require.NoError(t, err)
		assert.InDelta(t, 0.05, updated.Cost, 1e-9)
	})
}

func TestUpdateParentSessionCost(t *testing.T) {
	t.Run("accumulates cost correctly", func(t *testing.T) {
		env := testEnv(t)
		cfg, err := config.Init(env.workingDir, "", false)
		require.NoError(t, err)
		coord := &coordinator{cfg: cfg, sessions: env.sessions, backgroundSubAgentLimiter: make(chan struct{}, maxBackgroundSubAgents)}

		parent, err := env.sessions.Create(t.Context(), "Parent")
		require.NoError(t, err)

		child, err := env.sessions.CreateTaskSession(t.Context(), "tool-1", parent.ID, "Child")
		require.NoError(t, err)

		// Set child cost.
		child.Cost = 0.10
		_, err = env.sessions.Save(t.Context(), child)
		require.NoError(t, err)

		err = coord.updateParentSessionCost(t.Context(), child.ID, parent.ID)
		require.NoError(t, err)

		updated, err := env.sessions.Get(t.Context(), parent.ID)
		require.NoError(t, err)
		assert.InDelta(t, 0.10, updated.Cost, 1e-9)
	})

	t.Run("accumulates multiple child costs", func(t *testing.T) {
		env := testEnv(t)
		cfg, err := config.Init(env.workingDir, "", false)
		require.NoError(t, err)
		coord := &coordinator{cfg: cfg, sessions: env.sessions, backgroundSubAgentLimiter: make(chan struct{}, maxBackgroundSubAgents)}

		parent, err := env.sessions.Create(t.Context(), "Parent")
		require.NoError(t, err)

		child1, err := env.sessions.CreateTaskSession(t.Context(), "tool-1", parent.ID, "Child1")
		require.NoError(t, err)
		child1.Cost = 0.05
		_, err = env.sessions.Save(t.Context(), child1)
		require.NoError(t, err)

		child2, err := env.sessions.CreateTaskSession(t.Context(), "tool-2", parent.ID, "Child2")
		require.NoError(t, err)
		child2.Cost = 0.03
		_, err = env.sessions.Save(t.Context(), child2)
		require.NoError(t, err)

		err = coord.updateParentSessionCost(t.Context(), child1.ID, parent.ID)
		require.NoError(t, err)
		err = coord.updateParentSessionCost(t.Context(), child2.ID, parent.ID)
		require.NoError(t, err)

		updated, err := env.sessions.Get(t.Context(), parent.ID)
		require.NoError(t, err)
		assert.InDelta(t, 0.08, updated.Cost, 1e-9)
	})

	t.Run("child session not found", func(t *testing.T) {
		env := testEnv(t)
		cfg, err := config.Init(env.workingDir, "", false)
		require.NoError(t, err)
		coord := &coordinator{cfg: cfg, sessions: env.sessions, backgroundSubAgentLimiter: make(chan struct{}, maxBackgroundSubAgents)}

		parent, err := env.sessions.Create(t.Context(), "Parent")
		require.NoError(t, err)

		err = coord.updateParentSessionCost(t.Context(), "non-existent", parent.ID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "get child session")
	})

	t.Run("parent session not found", func(t *testing.T) {
		env := testEnv(t)
		cfg, err := config.Init(env.workingDir, "", false)
		require.NoError(t, err)
		coord := &coordinator{cfg: cfg, sessions: env.sessions, backgroundSubAgentLimiter: make(chan struct{}, maxBackgroundSubAgents)}

		parent, err := env.sessions.Create(t.Context(), "Parent")
		require.NoError(t, err)
		child, err := env.sessions.CreateTaskSession(t.Context(), "tool-1", parent.ID, "Child")
		require.NoError(t, err)

		err = coord.updateParentSessionCost(t.Context(), child.ID, "non-existent")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "get parent session")
	})

	t.Run("zero cost handled correctly", func(t *testing.T) {
		env := testEnv(t)
		cfg, err := config.Init(env.workingDir, "", false)
		require.NoError(t, err)
		coord := &coordinator{cfg: cfg, sessions: env.sessions, backgroundSubAgentLimiter: make(chan struct{}, maxBackgroundSubAgents)}

		parent, err := env.sessions.Create(t.Context(), "Parent")
		require.NoError(t, err)
		child, err := env.sessions.CreateTaskSession(t.Context(), "tool-1", parent.ID, "Child")
		require.NoError(t, err)

		err = coord.updateParentSessionCost(t.Context(), child.ID, parent.ID)
		require.NoError(t, err)

		updated, err := env.sessions.Get(t.Context(), parent.ID)
		require.NoError(t, err)
		assert.InDelta(t, 0.0, updated.Cost, 1e-9)
	})
}

func TestShouldPrimeAutonomousSubAgents(t *testing.T) {
	t.Run("small prompt stays inline", func(t *testing.T) {
		assert.False(t, shouldPrimeAutonomousSubAgents("rename this variable"))
	})

	t.Run("complex prompt primes sub-agents", func(t *testing.T) {
		assert.True(t, shouldPrimeAutonomousSubAgents("Investigate the root cause across the codebase, trace dependencies, and review risks before refactoring the architecture."))
	})

	t.Run("explicit worktree orchestration does not double-prime", func(t *testing.T) {
		assert.False(t, shouldPrimeAutonomousSubAgents("Large repo: spawn 5-12 subagents, one per distinct domain, and use worktree orchestration for each task."))
	})
}

func TestBuildAutonomousSubAgentTasks(t *testing.T) {
	tasks := buildAutonomousSubAgentTasks("Fix the Gemini API integration using the latest docs and review implementation risks.")
	require.Len(t, tasks, 1)
	assert.Equal(t, "risk-review", tasks[0].Name)
}

func TestBuildToolsTaskAgentMatchesCoderCapabilities(t *testing.T) {
	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)

	cfg.Providers.Set("gemini-test", config.ProviderConfig{
		ID:     "gemini-test",
		Type:   "gemini",
		APIKey: "test-key",
		Models: []catwalk.Model{
			{ID: "gemini-3-flash-preview"},
		},
	})
	cfg.Models[config.SelectedModelTypeLarge] = config.SelectedModel{
		Provider: "gemini-test",
		Model:    "gemini-3-flash-preview",
	}
	cfg.Models[config.SelectedModelTypeSmall] = config.SelectedModel{
		Provider: "gemini-test",
		Model:    "gemini-3-flash-preview",
	}
	cfg.Options.GoogleGrounding = true

	coord := &coordinator{
		cfg:                       cfg,
		sessions:                  env.sessions,
		messages:                  env.messages,
		permissions:               env.permissions,
		history:                   env.history,
		filetracker:               *env.filetracker,
		editGuard:                 tools.NewEditGuard(),
		lspManager:                lsp.NewManager(cfg),
		memory:                    env.memory,
		backgroundSubAgentLimiter: make(chan struct{}, maxBackgroundSubAgents),
		googleSearchClient:        &genai.Client{},
	}

	coderTools, err := coord.buildTools(t.Context(), cfg.Agents[config.AgentCoder])
	require.NoError(t, err)
	taskTools, err := coord.buildTools(t.Context(), cfg.Agents[config.AgentTask])
	require.NoError(t, err)

	coderNames := toolNames(coderTools)
	taskNames := toolNames(taskTools)

	require.Contains(t, taskNames, tools.BashToolName)
	require.Contains(t, taskNames, tools.JobOutputToolName)
	require.Contains(t, taskNames, tools.JobKillToolName)
	require.Contains(t, taskNames, tools.UpdatePlanToolName)
	require.Contains(t, taskNames, tools.ViewToolName)
	require.Contains(t, taskNames, tools.SingleViewToolName)
	require.Contains(t, taskNames, tools.AgenticViewToolName)
	require.Contains(t, taskNames, tools.PythonToolName)
	require.Contains(t, taskNames, tools.GoogleSearchToolName)
	require.NotContains(t, taskNames, AgentToolName)
	require.NotContains(t, taskNames, SpawnAgentToolName)
	require.NotContains(t, taskNames, ResumeAgentToolName)
	require.NotContains(t, taskNames, SendInputToolName)
	require.NotContains(t, taskNames, WaitAgentsToolName)
	require.NotContains(t, taskNames, CollectResultToolName)
	require.NotContains(t, taskNames, CloseAgentToolName)
	require.NotContains(t, taskNames, tools.EditToolName)
	require.NotContains(t, taskNames, tools.SingleEditToolName)
	require.NotContains(t, taskNames, tools.AgenticEditToolName)
	require.NotContains(t, taskNames, tools.WriteToolName)

	coderNamesWithoutAgent := make([]string, 0, len(coderNames))
	for _, name := range coderNames {
		if name != AgentToolName &&
			name != SpawnAgentToolName &&
			name != ResumeAgentToolName &&
			name != SendInputToolName &&
			name != WaitAgentsToolName &&
			name != CollectResultToolName &&
			name != CloseAgentToolName &&
			name != tools.EditToolName &&
			name != tools.SingleEditToolName &&
			name != tools.AgenticEditToolName &&
			name != tools.WriteToolName {
			coderNamesWithoutAgent = append(coderNamesWithoutAgent, name)
		}
	}
	assert.ElementsMatch(t, coderNamesWithoutAgent, taskNames)
}

func TestBuildSubAgentKeepsExplicitCollabLifecycleOnly(t *testing.T) {
	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)

	cfg.Providers.Set("gemini-test", config.ProviderConfig{
		ID:     "gemini-test",
		Type:   "gemini",
		APIKey: "test-key",
		Models: []catwalk.Model{
			{ID: "gemini-3-flash-preview"},
		},
	})
	cfg.Models[config.SelectedModelTypeLarge] = config.SelectedModel{
		Provider: "gemini-test",
		Model:    "gemini-3-flash-preview",
	}
	cfg.Models[config.SelectedModelTypeSmall] = config.SelectedModel{
		Provider: "gemini-test",
		Model:    "gemini-3-flash-preview",
	}

	coord := &coordinator{
		cfg:                       cfg,
		sessions:                  env.sessions,
		messages:                  env.messages,
		permissions:               env.permissions,
		history:                   env.history,
		filetracker:               *env.filetracker,
		editGuard:                 tools.NewEditGuard(),
		lspManager:                lsp.NewManager(cfg),
		memory:                    env.memory,
		backgroundSubAgentLimiter: make(chan struct{}, maxBackgroundSubAgents),
		googleSearchClient:        &genai.Client{},
	}

	prompt, err := coderPrompt(promptpkg.WithWorkingDir(env.workingDir))
	require.NoError(t, err)

	built, err := coord.buildAgentWithWorkingDir(t.Context(), prompt, cfg.Agents[config.AgentCoder], true, env.workingDir)
	require.NoError(t, err)

	subAgent, ok := built.(*sessionAgent)
	require.True(t, ok)

	names := toolNames(subAgent.tools.Copy())
	require.NotContains(t, names, AgentToolName)
	require.Contains(t, names, SpawnAgentToolName)
	require.Contains(t, names, ResumeAgentToolName)
	require.Contains(t, names, SendInputToolName)
	require.Contains(t, names, WaitAgentsToolName)
	require.Contains(t, names, CollectResultToolName)
	require.Contains(t, names, CloseAgentToolName)
	require.NotContains(t, names, SpawnAgentsOnCSVToolName)
	require.NotContains(t, names, ReportAgentJobResultToolName)
	require.NotContains(t, names, OrchestrateWorktreesToolName)
}

func toolNames(agentTools []fantasy.AgentTool) []string {
	names := make([]string, 0, len(agentTools))
	for _, tool := range agentTools {
		names = append(names, tool.Info().Name)
	}
	return names
}
