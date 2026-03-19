package agent

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/duggal1/Sapphire-cli/internal/config"
)

//go:embed tools/orchestrate_worktrees.md
var orchestrateWorktreesDescription []byte

const OrchestrateWorktreesToolName = "orchestrate_worktrees"

type WorktreeTaskSpec struct {
	Name             string   `json:"name" description:"Short task name (used for titles and worktree defaults)"`
	Prompt           string   `json:"prompt" description:"Sub-agent task prompt"`
	Isolation        string   `json:"isolation,omitempty" description:"Isolation mode for the sub-agent. Use 'worktree'."`
	Branch           string   `json:"branch,omitempty" description:"Branch name for the worktree"`
	WorktreePath     string   `json:"worktree_path,omitempty" description:"Explicit worktree path (defaults to repo-root/.sapphire/worktrees/agent/<id>/<task>)"`
	WriteManifest    []string `json:"write_manifest" description:"Allowed write paths (relative to repo root). Empty list = read-only."`
	DefinitionOfDone string   `json:"definition_of_done,omitempty" description:"Acceptance criteria for completion"`
	Agent            string   `json:"agent,omitempty" description:"Agent profile to use (coder or task)"`
	Model            string   `json:"model,omitempty" description:"Optional model override (provider:model or model)"`
	ReasoningEffort  string   `json:"reasoning_effort,omitempty" description:"Optional reasoning effort override (low, medium, high)"`
	ForkContext      bool     `json:"fork_context,omitempty" description:"Copy recent parent context into the sub-agent session"`
}

type OrchestrateWorktreesParams struct {
	Tasks             []WorktreeTaskSpec `json:"tasks" description:"Worktree task list"`
	TestCommand       string             `json:"test_command,omitempty" description:"Test command to run in each worktree (spawns test runner agents)"`
	IntegrationPrompt string             `json:"integration_prompt,omitempty" description:"Optional prompt for integration agent (merges and validates)"`
	IntegrationBranch string             `json:"integration_branch,omitempty" description:"Branch for integration worktree (defaults to integration/<timestamp>)"`
	MaxParallel       int                `json:"max_parallel,omitempty" description:"Maximum parallel sub-agents (defaults to agent_max_threads)"`
}

type OrchestrationAgentRef struct {
	AgentID          string         `json:"agent_id"`
	SubmissionID     string         `json:"submission_id"`
	WorktreePath     string         `json:"worktree_path,omitempty"`
	Branch           string         `json:"branch,omitempty"`
	Title            string         `json:"title,omitempty"`
	Status           subAgentStatus `json:"status,omitempty"`
	ValidationPassed bool           `json:"validation_passed,omitempty"`
	ValidationErrors string         `json:"validation_errors,omitempty"`
}

type OrchestrateWorktreesResult struct {
	Tasks                    []OrchestrationAgentRef `json:"tasks"`
	TestRunners              []OrchestrationAgentRef `json:"test_runners,omitempty"`
	IntegrationAgent         *OrchestrationAgentRef  `json:"integration_agent,omitempty"`
	IntegrationSkippedReason string                  `json:"integration_skipped_reason,omitempty"`
}

func (c *coordinator) orchestrateWorktreesTool(ctx context.Context) (fantasy.AgentTool, error) {
	_ = ctx
	return fantasy.NewParallelAgentTool(
		OrchestrateWorktreesToolName,
		string(orchestrateWorktreesDescription),
		func(ctx context.Context, params OrchestrateWorktreesParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			_ = call
			sessionID := tools.GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.NewTextErrorResponse("session id missing from context"), nil
			}
			result, err := c.OrchestrateWorktrees(ctx, sessionID, params)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			payload, _ := json.Marshal(result)
			return fantasy.NewTextResponse(string(payload)), nil
		},
	), nil
}

func (c *coordinator) OrchestrateWorktrees(ctx context.Context, sessionID string, params OrchestrateWorktreesParams) (OrchestrateWorktreesResult, error) {
	if sessionID == "" {
		return OrchestrateWorktreesResult{}, fmt.Errorf("session id is required")
	}
	if len(params.Tasks) == 0 {
		return OrchestrateWorktreesResult{}, fmt.Errorf("tasks are required")
	}
	if err := validateWorktreeSpecs(params.Tasks); err != nil {
		return OrchestrateWorktreesResult{}, err
	}

	result := OrchestrateWorktreesResult{
		Tasks:       make([]OrchestrationAgentRef, 0, len(params.Tasks)),
		TestRunners: nil,
	}

	type taskOutcome struct {
		name string
		ref  OrchestrationAgentRef
		err  error
	}

	maxParallel := params.MaxParallel
	if maxParallel <= 0 {
		maxParallel = c.subAgentThreadLimit()
	}
	if maxParallel <= 0 {
		maxParallel = len(params.Tasks)
	}
	if maxParallel > len(params.Tasks) {
		maxParallel = len(params.Tasks)
	}

	queue := make(chan WorktreeTaskSpec)
	outcomes := make(chan taskOutcome, len(params.Tasks))
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		control := c.subAgentControl()
		for task := range queue {
			prompt := strings.TrimSpace(task.Prompt)
			if prompt == "" {
				outcomes <- taskOutcome{name: task.Name, err: fmt.Errorf("task prompt is required")}
				continue
			}
			if task.DefinitionOfDone != "" {
				prompt = fmt.Sprintf("%s\n\nDefinition of done:\n%s", prompt, strings.TrimSpace(task.DefinitionOfDone))
			}

			agentID, submissionID, err := c.spawnSubAgent(ctx, sessionID, spawnAgentOptions{
				Prompt:           prompt,
				Title:            task.Name,
				Worktree:         true,
				WorktreePath:     task.WorktreePath,
				Branch:           task.Branch,
				WriteManifest:    task.WriteManifest,
				DefinitionOfDone: task.DefinitionOfDone,
				TestCommand:      params.TestCommand,
				AgentID:          task.Agent,
				Model:            task.Model,
				ReasoningEffort:  task.ReasoningEffort,
				ForkContext:      task.ForkContext,
			})
			if err != nil {
				outcomes <- taskOutcome{name: task.Name, err: err}
				continue
			}

			ref := OrchestrationAgentRef{
				AgentID:      agentID,
				SubmissionID: submissionID,
				Title:        task.Name,
			}
			if runner, err := c.getSubAgent(agentID); err == nil {
				ref.WorktreePath = runner.workDir
				ref.Branch = runner.assignment.Branch
			}
			_, _ = control.wait(ctx, []string{agentID}, 0)
			results := control.collectResult([]string{agentID})
			_ = control.close(agentID)
			if len(results) > 0 {
				ref.Status = results[0].Status
				ref.ValidationPassed = results[0].ValidationPassed
				ref.ValidationErrors = results[0].ValidationErrors
			}
			outcomes <- taskOutcome{name: task.Name, ref: ref}
		}
	}

	for i := 0; i < maxParallel; i++ {
		wg.Add(1)
		go worker()
	}

	go func() {
		for _, task := range params.Tasks {
			queue <- task
		}
		close(queue)
	}()

	go func() {
		wg.Wait()
		close(outcomes)
	}()

	order := make([]string, 0, len(params.Tasks))
	for _, task := range params.Tasks {
		order = append(order, task.Name)
	}
	outcomesMap := make(map[string]OrchestrationAgentRef, len(params.Tasks))
	var firstErr error
	for outcome := range outcomes {
		if outcome.err != nil && firstErr == nil {
			firstErr = outcome.err
		}
		if outcome.name != "" {
			outcomesMap[outcome.name] = outcome.ref
		}
	}
	if firstErr != nil {
		return OrchestrateWorktreesResult{}, firstErr
	}
	for _, name := range order {
		if ref, ok := outcomesMap[name]; ok {
			result.Tasks = append(result.Tasks, ref)
		}
	}

	integrationPrompt := strings.TrimSpace(params.IntegrationPrompt)
	if integrationPrompt == "" {
		branches := make([]string, 0, len(result.Tasks))
		for _, task := range result.Tasks {
			if task.Branch != "" {
				branches = append(branches, task.Branch)
			}
		}
		if len(branches) > 0 {
			integrationPrompt = fmt.Sprintf("Integrate branches: %s. Review diffs, resolve conflicts, and run project tests. Report results and do not push.", strings.Join(branches, ", "))
		}
	}
	if integrationPrompt != "" {
		allPassed := true
		for _, task := range result.Tasks {
			if task.Status != subAgentStatusCompleted || !task.ValidationPassed {
				allPassed = false
				break
			}
		}
		if !allPassed {
			result.IntegrationSkippedReason = "validation failed or task incomplete"
			return result, nil
		}
		branch := strings.TrimSpace(params.IntegrationBranch)
		if branch == "" {
			branch = fmt.Sprintf("integration/%d", time.Now().Unix())
		}
		agentID, submissionID, err := c.spawnSubAgent(ctx, sessionID, spawnAgentOptions{
			Prompt:        integrationPrompt,
			Title:         "Integration Agent",
			Worktree:      true,
			Branch:        branch,
			WriteManifest: []string{"**"},
			AgentID:       config.AgentCoder,
		})
		if err != nil {
			return OrchestrateWorktreesResult{}, err
		}
		ref := &OrchestrationAgentRef{
			AgentID:      agentID,
			SubmissionID: submissionID,
			Title:        "Integration Agent",
		}
		control := c.subAgentControl()
		if runner, err := c.getSubAgent(agentID); err == nil {
			ref.WorktreePath = runner.workDir
			ref.Branch = runner.assignment.Branch
		}
		_, _ = control.wait(ctx, []string{agentID}, 0)
		results := control.collectResult([]string{agentID})
		_ = control.close(agentID)
		if len(results) > 0 {
			ref.Status = results[0].Status
			ref.ValidationPassed = results[0].ValidationPassed
			ref.ValidationErrors = results[0].ValidationErrors
		}
		result.IntegrationAgent = ref
	}

	return result, nil
}

func validateWorktreeSpecs(tasks []WorktreeTaskSpec) error {
	seen := map[string]struct{}{}
	for _, task := range tasks {
		name := strings.TrimSpace(task.Name)
		if name == "" {
			return fmt.Errorf("task name is required")
		}
		if _, ok := seen[name]; ok {
			return fmt.Errorf("duplicate task name: %s", name)
		}
		seen[name] = struct{}{}
		if isolation := strings.TrimSpace(task.Isolation); isolation != "" && !strings.EqualFold(isolation, "worktree") {
			return fmt.Errorf("task %s uses unsupported isolation %q; orchestrate_worktrees requires isolation=worktree", name, isolation)
		}
		if task.WriteManifest == nil {
			return fmt.Errorf("write_manifest is required for task %s", name)
		}
	}
	if err := validateManifestOverlaps(tasks); err != nil {
		return err
	}
	return nil
}

func (c *coordinator) ResumeWorktree(ctx context.Context, sessionID, worktreePath, prompt, agentKey, model, reasoningEffort string) (OrchestrationAgentRef, error) {
	if sessionID == "" {
		return OrchestrationAgentRef{}, fmt.Errorf("session id is required")
	}
	worktreePath = strings.TrimSpace(worktreePath)
	if worktreePath == "" {
		return OrchestrationAgentRef{}, fmt.Errorf("worktree path is required")
	}
	taskPrompt := strings.TrimSpace(prompt)
	if taskPrompt == "" {
		taskPrompt = "Resume worktree task"
	}
	branch, err := c.resumeWorktree(ctx, worktreePath)
	if err != nil {
		return OrchestrationAgentRef{}, err
	}
	agentID, submissionID, err := c.spawnSubAgent(ctx, sessionID, spawnAgentOptions{
		Prompt:          taskPrompt,
		Title:           "Resume Worktree",
		Worktree:        true,
		ReuseWorktree:   true,
		AllowReuse:      true,
		WorktreePath:    worktreePath,
		Branch:          branch,
		WriteManifest:   []string{},
		AgentID:         agentKey,
		Model:           model,
		ReasoningEffort: reasoningEffort,
	})
	if err != nil {
		return OrchestrationAgentRef{}, err
	}
	ref := OrchestrationAgentRef{
		AgentID:      agentID,
		SubmissionID: submissionID,
		Title:        "Resume Worktree",
		WorktreePath: worktreePath,
		Branch:       branch,
	}
	control := c.subAgentControl()
	_, _ = control.wait(ctx, []string{agentID}, 0)
	results := control.collectResult([]string{agentID})
	_ = control.close(agentID)
	if len(results) > 0 {
		ref.Status = results[0].Status
		ref.ValidationPassed = results[0].ValidationPassed
		ref.ValidationErrors = results[0].ValidationErrors
	}
	return ref, nil
}

func validateManifestOverlaps(tasks []WorktreeTaskSpec) error {
	type entry struct {
		task string
		path string
		raw  string
	}
	var entries []entry
	for _, task := range tasks {
		taskName := strings.TrimSpace(task.Name)
		for _, raw := range task.WriteManifest {
			rawEntry := strings.TrimSpace(raw)
			if rawEntry == "" {
				continue
			}
			if strings.ContainsAny(rawEntry, "*?[") && !strings.Contains(rawEntry, "**") {
				return fmt.Errorf("manifest entry %q in task %s is too ambiguous; use explicit paths or prefix/**", rawEntry, taskName)
			}
			key := normalizeManifestKey(rawEntry)
			entries = append(entries, entry{task: taskName, path: key, raw: rawEntry})
		}
	}
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			a := entries[i]
			b := entries[j]
			if a.task == b.task {
				continue
			}
			if pathsOverlap(a.path, b.path) {
				return fmt.Errorf("manifest overlap: %s and %s both include %s / %s", a.task, b.task, a.raw, b.raw)
			}
		}
	}
	return nil
}

func normalizeManifestKey(entry string) string {
	entry = strings.TrimSpace(entry)
	entry = strings.TrimSuffix(entry, "/")
	entry = strings.TrimSuffix(entry, "/**")
	entry = strings.TrimSuffix(entry, string('\\'))
	entry = strings.TrimSuffix(entry, "\\**")
	entry = strings.TrimPrefix(entry, "./")
	if entry == "" {
		return "/"
	}
	return entry
}

func pathsOverlap(a, b string) bool {
	if a == b {
		return true
	}
	if a == "/" || b == "/" {
		return true
	}
	a = strings.TrimSuffix(a, "/")
	b = strings.TrimSuffix(b, "/")
	if strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/") {
		return true
	}
	return false
}
