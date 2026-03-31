package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"charm.land/fantasy"
	agentactivity "github.com/duggal1/Sapphire-cli/internal/agent/activity"
	agentmailbox "github.com/duggal1/Sapphire-cli/internal/agent/mailbox"
	agentmemory "github.com/duggal1/Sapphire-cli/internal/agent/memory"
	agentstate "github.com/duggal1/Sapphire-cli/internal/agent/state"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/duggal1/Sapphire-cli/internal/codebasesurvey"
	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/duggal1/Sapphire-cli/internal/db"
	"github.com/duggal1/Sapphire-cli/internal/message"
	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
	"github.com/stretchr/testify/require"
)

func TestBuildSemanticSurveyShardsPreservesCoverage(t *testing.T) {
	files := []agentmemory.IndexedFileInfo{
		{Path: "main.go", Language: "go", SymbolCount: 4},
		{Path: "internal/agent/coordinator.go", Language: "go", SymbolCount: 12},
		{Path: "internal/agent/subagent_manager.go", Language: "go", SymbolCount: 10},
		{Path: "internal/agent/templates/codebase_indexing.md", Language: "markdown", SymbolCount: 0},
		{Path: "internal/memory/memory_md.go", Language: "go", SymbolCount: 6},
		{Path: "internal/orchestration/db/db.go", Language: "go", SymbolCount: 9},
		{Path: "internal/ui/model/ui.go", Language: "go", SymbolCount: 7},
		{Path: "scripts/dev.sh", Language: "shell", SymbolCount: 0},
	}

	shards := buildSemanticSurveyShards(files, 3)
	require.Len(t, shards, 3)

	seen := make(map[string]struct{}, len(files))
	for _, shard := range shards {
		require.NotEmpty(t, shard.ID)
		require.NotEmpty(t, shard.Label)
		require.NotEmpty(t, shard.Files)
		require.NotEmpty(t, shard.CriticalFiles)
		for _, file := range shard.Files {
			seen[file.Path] = struct{}{}
		}
	}

	require.Len(t, seen, len(files))
	_, ok := seen["internal/agent/coordinator.go"]
	require.True(t, ok)
}

func TestNormalizeSemanticSurveyAgentCount(t *testing.T) {
	require.Equal(t, 0, normalizeSemanticSurveyAgentCount(3, 0, 6))
	require.Equal(t, 1, normalizeSemanticSurveyAgentCount(0, 1, 6))
	require.Equal(t, 1, normalizeSemanticSurveyAgentCount(0, 20, 6))
	require.Equal(t, 6, normalizeSemanticSurveyAgentCount(99, 6000, 6))
	require.Equal(t, 2, normalizeSemanticSurveyAgentCount(4, 2, 6))
	require.Equal(t, 18, normalizeSemanticSurveyAgentCount(0, 6187, 20))
}

func TestFormatSemanticSurveyProgressMessage(t *testing.T) {
	t.Parallel()

	require.Equal(t,
		"Launching AI codebase survey sub-agents in parallel (1/3 ready)",
		formatSemanticSurveyProgressMessage(1, 3, true),
	)
	require.Equal(t,
		"AI codebase graph shards complete 0/3; parallel shard sub-agents still running",
		formatSemanticSurveyProgressMessage(0, 3, false),
	)
	require.Equal(t,
		"AI codebase graph shards complete 3/3",
		formatSemanticSurveyProgressMessage(3, 3, false),
	)
}

func TestBuildSemanticSurveyShardsSplitsDenseDirectoryTrees(t *testing.T) {
	t.Parallel()

	files := make([]agentmemory.IndexedFileInfo, 0, 24)
	for i := 0; i < 12; i++ {
		files = append(files, agentmemory.IndexedFileInfo{
			Path:        filepath.ToSlash(filepath.Join("internal", "agent", fmt.Sprintf("file_%02d.go", i))),
			Language:    "go",
			SymbolCount: 5,
		})
	}
	for i := 0; i < 12; i++ {
		files = append(files, agentmemory.IndexedFileInfo{
			Path:        filepath.ToSlash(filepath.Join("internal", "memory", fmt.Sprintf("file_%02d.go", i))),
			Language:    "go",
			SymbolCount: 3,
		})
	}

	shards := buildSemanticSurveyShards(files, 4)
	require.Len(t, shards, 4)

	maxLoad := 0
	minLoad := len(files)
	for _, shard := range shards {
		maxLoad = max(maxLoad, len(shard.Files))
		minLoad = min(minLoad, len(shard.Files))
	}
	require.LessOrEqual(t, maxLoad-minLoad, 6)
}

func TestExtractSemanticSurveyToolPaths(t *testing.T) {
	t.Parallel()

	input, err := json.Marshal(map[string]any{
		"file_paths": []string{
			"/tmp/repo/main.go",
			"internal/agent/coordinator.go",
			"../outside.go",
		},
	})
	require.NoError(t, err)

	paths := extractSemanticSurveyToolPaths("agentic_view", string(input), "/tmp/repo")
	require.Equal(t, []string{"internal/agent/coordinator.go", "main.go"}, paths)
}

func TestRunMandatorySemanticCodebaseSurveyUsesLowReasoningAndWritesArtifacts(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "internal", "agent"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "internal", "memory"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "main.go"), []byte("package main\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "internal", "agent", "coordinator.go"), []byte("package agent\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "internal", "agent", "subagent_manager.go"), []byte("package agent\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "internal", "memory", "memory_md.go"), []byte("package memory\n"), 0o644))

	env := testEnv(t)
	cfg, err := config.Init(projectRoot, "", false)
	require.NoError(t, err)
	cfg.Options.DataDirectory = filepath.Join(projectRoot, ".sapphire")
	cfg.Providers.Set("test-provider", config.ProviderConfig{ID: "test-provider"})

	conn, err := db.Connect(context.Background(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
	})

	store, err := orchestrationdb.Open(context.Background(), cfg.Options.DataDirectory)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	coord := &coordinator{
		cfg:                       cfg,
		sessions:                  env.sessions,
		messages:                  env.messages,
		orchestrationStore:        store,
		memoryCompiler:            agentmemory.NewCompiler(conn, store),
		stateService:              agentstate.NewService(store),
		activityService:           agentactivity.NewService(store),
		mailbox:                   agentmailbox.NewService(store, nil),
		backgroundSubAgentLimiter: make(chan struct{}, maxBackgroundSubAgents),
		subAgents:                 make(map[string]*subAgentRunner),
		subAgentRegistry:          newSubAgentRegistry(),
	}

	var (
		spawnMu      sync.Mutex
		capturedOpts []spawnAgentOptions
		runCounts    = make(map[string]int)
		runPrompts   []string
	)
	coord.subAgentFactory = func(ctx context.Context, workDir string, normalizedManifest []string, opts spawnAgentOptions) (SessionAgent, error) {
		spawnMu.Lock()
		capturedOpts = append(capturedOpts, opts)
		spawnMu.Unlock()

		writeTarget := ""
		if len(opts.WriteManifest) > 0 {
			writeTarget = opts.WriteManifest[0]
		}
		title := opts.Title
		return newMockAgent("test-provider", 4096, func(_ context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
			spawnMu.Lock()
			runCounts[title]++
			runNumber := runCounts[title]
			runPrompts = append(runPrompts, call.Prompt)
			spawnMu.Unlock()

			if strings.TrimSpace(writeTarget) != "" {
				content := "# AI Codebase Graph\n\n- generated_by: " + title + "\n"
				if strings.Contains(strings.ToLower(title), "overview") {
					content += "- repo_summary: combined shard view\n"
				}
				if err := os.WriteFile(writeTarget, []byte(content), 0o644); err != nil {
					return nil, err
				}
			}
			if !strings.Contains(strings.ToLower(title), "overview") {
				assigned := assignedFilesFromShardPrompt(t, call.Prompt)
				readFiles := assigned
				if runNumber == 1 && len(assigned) > 1 {
					readFiles = assigned[:1]
				}
				inputJSON, err := json.Marshal(map[string]any{"file_paths": readFiles})
				require.NoError(t, err)
				_, err = env.messages.Create(context.Background(), call.SessionID, message.CreateMessageParams{
					Role: message.Assistant,
					Parts: []message.ContentPart{
						message.ToolCall{
							ID:       "tool-" + strings.ReplaceAll(title, " ", "-"),
							Name:     tools.AgenticViewToolName,
							Input:    string(inputJSON),
							Finished: true,
						},
					},
				})
				require.NoError(t, err)
			}
			return agentResultWithText(strings.Join([]string{
				"STATUS: completed",
				"SUMMARY: semantic graph written",
				"PROGRESS: graph artifact updated",
				"FILES: main.go, internal/agent/coordinator.go",
				"COMMANDS: view, agent_mail_inbox",
				"RISKS: none",
				"NEXT: synthesize overall architecture",
				"BLOCKERS: none",
			}, "\n")), nil
		}), nil
	}

	_, err = coord.memoryCompiler.WarmCodebase(context.Background(), agentmemory.WarmRequest{
		WorkingDir: projectRoot,
		Force:      true,
	}, nil)
	require.NoError(t, err)

	status, files, err := coord.memoryCompiler.ListIndexedFiles(context.Background(), projectRoot)
	require.NoError(t, err)
	require.NotEmpty(t, files)

	parentSession, err := env.sessions.Create(context.Background(), "Semantic Survey Parent")
	require.NoError(t, err)

	started := time.Now()
	result, err := coord.runMandatorySemanticCodebaseSurvey(context.Background(), parentSession.ID, status, len(files), 3, time.Now().Add(-12*time.Second))
	elapsed := time.Since(started)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotEmpty(t, result.ManifestPath)
	require.NotEmpty(t, result.OverviewPath)
	require.FileExists(t, result.ManifestPath)
	require.FileExists(t, result.OverviewPath)

	overview, err := os.ReadFile(result.OverviewPath)
	require.NoError(t, err)
	require.Contains(t, string(overview), "AI Codebase Graph")

	manifest, ok, err := codebasesurvey.ReadManifest(cfg.Options.DataDirectory)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotEmpty(t, manifest.ShardArtifacts)
	require.Equal(t, "ready", result.Status)
	for _, artifact := range manifest.ShardArtifacts {
		require.Equal(t, "verified", artifact.CoverageStatus)
		require.Empty(t, artifact.MissingFiles)
		require.GreaterOrEqual(t, artifact.ReadCount, artifact.FileCount)
	}

	spawnMu.Lock()
	defer spawnMu.Unlock()
	require.GreaterOrEqual(t, len(capturedOpts), 2)
	for _, opts := range capturedOpts {
		require.Equal(t, semanticSurveyReasoning, opts.ReasoningEffort)
	}
	require.Condition(t, func() bool {
		for _, prompt := range runPrompts {
			if strings.Contains(prompt, "Coverage verifier found missing assigned files") {
				return true
			}
		}
		return false
	})
	t.Logf("semantic survey smoke elapsed=%s spawns=%d shards=%d", elapsed.Truncate(time.Millisecond), len(capturedOpts), result.ShardCount)
}

func TestRunMandatorySemanticCodebaseSurveyLaunchesShardAgentsInParallel(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	for _, path := range []string{
		"main.go",
		"internal/agent/coordinator.go",
		"internal/agent/subagent_manager.go",
		"internal/memory/memory_md.go",
		"internal/memory/runtime.go",
		"internal/memory/compiler.go",
	} {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(projectRoot, path)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(projectRoot, path), []byte("package main\n"), 0o644))
	}

	env := testEnv(t)
	cfg, err := config.Init(projectRoot, "", false)
	require.NoError(t, err)
	cfg.Options.DataDirectory = filepath.Join(projectRoot, ".sapphire")
	cfg.Providers.Set("test-provider", config.ProviderConfig{ID: "test-provider"})

	conn, err := db.Connect(context.Background(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
	})

	store, err := orchestrationdb.Open(context.Background(), cfg.Options.DataDirectory)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	coord := &coordinator{
		cfg:                       cfg,
		sessions:                  env.sessions,
		messages:                  env.messages,
		orchestrationStore:        store,
		memoryCompiler:            agentmemory.NewCompiler(conn, store),
		stateService:              agentstate.NewService(store),
		activityService:           agentactivity.NewService(store),
		mailbox:                   agentmailbox.NewService(store, nil),
		backgroundSubAgentLimiter: make(chan struct{}, maxBackgroundSubAgents),
		subAgents:                 make(map[string]*subAgentRunner),
		subAgentRegistry:          newSubAgentRegistry(),
	}

	expectedAgents := 3
	release := make(chan struct{})
	var (
		mu      sync.Mutex
		started int
	)
	coord.subAgentFactory = func(ctx context.Context, workDir string, normalizedManifest []string, opts spawnAgentOptions) (SessionAgent, error) {
		isOverview := strings.Contains(strings.ToLower(opts.Title), "overview")
		if !isOverview {
			mu.Lock()
			started++
			if started == expectedAgents {
				close(release)
			}
			mu.Unlock()

			select {
			case <-release:
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(2 * time.Second):
				t.Fatal("semantic shard launch did not become parallel before timeout")
			}
		}

		writeTarget := ""
		if len(opts.WriteManifest) > 0 {
			writeTarget = opts.WriteManifest[0]
		}
		title := opts.Title
		return newMockAgent("test-provider", 4096, func(_ context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
			if strings.TrimSpace(writeTarget) != "" {
				require.NoError(t, os.WriteFile(writeTarget, []byte("# graph\n"), 0o644))
			}
			if !isOverview {
				assigned := assignedFilesFromShardPrompt(t, call.Prompt)
				inputJSON, err := json.Marshal(map[string]any{"file_paths": assigned})
				require.NoError(t, err)
				_, err = env.messages.Create(context.Background(), call.SessionID, message.CreateMessageParams{
					Role: message.Assistant,
					Parts: []message.ContentPart{
						message.ToolCall{
							ID:       "tool-" + strings.ReplaceAll(title, " ", "-"),
							Name:     tools.AgenticViewToolName,
							Input:    string(inputJSON),
							Finished: true,
						},
					},
				})
				require.NoError(t, err)
			}
			return agentResultWithText(strings.Join([]string{
				"STATUS: completed",
				"SUMMARY: semantic graph written",
				"PROGRESS: graph artifact updated",
				"FILES: main.go",
				"COMMANDS: agentic_view",
				"RISKS: none",
				"NEXT: synthesize overall architecture",
				"BLOCKERS: none",
			}, "\n")), nil
		}), nil
	}

	_, err = coord.memoryCompiler.WarmCodebase(context.Background(), agentmemory.WarmRequest{
		WorkingDir: projectRoot,
		Force:      true,
	}, nil)
	require.NoError(t, err)

	status, files, err := coord.memoryCompiler.ListIndexedFiles(context.Background(), projectRoot)
	require.NoError(t, err)
	require.NotEmpty(t, files)

	parentSession, err := env.sessions.Create(context.Background(), "Semantic Survey Parent")
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := coord.runMandatorySemanticCodebaseSurvey(ctx, parentSession.ID, status, len(files), expectedAgents, time.Now().Add(-12*time.Second))
	require.NoError(t, err)
	require.NotNil(t, result)

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, expectedAgents, started)
}

func assignedFilesFromShardPrompt(t *testing.T, prompt string) []string {
	t.Helper()

	prefixes := []string{"- the shard manifest: ", "- shard manifest: "}
	for _, line := range strings.Split(prompt, "\n") {
		line = strings.TrimSpace(line)
		for _, prefix := range prefixes {
			if !strings.HasPrefix(line, prefix) {
				continue
			}
			path := strings.TrimSpace(strings.TrimPrefix(line, prefix))
			payload, err := os.ReadFile(path)
			require.NoError(t, err)
			var input codebasesurvey.ShardInput
			require.NoError(t, json.Unmarshal(payload, &input))
			return input.AssignedFiles
		}
	}
	t.Fatalf("shard manifest path not found in prompt")
	return nil
}
