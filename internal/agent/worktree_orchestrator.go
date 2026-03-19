package agent

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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
	Tasks                          []WorktreeTaskSpec `json:"tasks" description:"Worktree task list"`
	TestCommand                    string             `json:"test_command,omitempty" description:"Test command to run in each worktree (spawns test runner agents)"`
	IntegrationPrompt              string             `json:"integration_prompt,omitempty" description:"Optional prompt for integration agent (merges and validates)"`
	IntegrationBranch              string             `json:"integration_branch,omitempty" description:"Branch for integration worktree (defaults to integration/<timestamp>)"`
	MaxParallel                    int                `json:"max_parallel,omitempty" description:"Maximum parallel sub-agents (defaults to agent_max_threads)"`
	AllowCoordinatorImplementation bool               `json:"allow_coordinator_implementation,omitempty" description:"Allow the orchestrator integration path to create implementation changes. Defaults to false."`
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
	SignalPath       string         `json:"signal_path,omitempty"`
}

type OrchestrateWorktreesResult struct {
	Tasks                    []OrchestrationAgentRef `json:"tasks"`
	TestRunners              []OrchestrationAgentRef `json:"test_runners,omitempty"`
	IntegrationAgent         *OrchestrationAgentRef  `json:"integration_agent,omitempty"`
	IntegrationSkippedReason string                  `json:"integration_skipped_reason,omitempty"`
	BaseBranch               string                  `json:"base_branch,omitempty"`
	CoordinatorMode          string                  `json:"coordinator_mode,omitempty"`
	SignalDirectory          string                  `json:"signal_directory,omitempty"`
	ConflictReportPath       string                  `json:"conflict_report_path,omitempty"`
	PRDraftPath              string                  `json:"pr_draft_path,omitempty"`
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
		Tasks:           make([]OrchestrationAgentRef, 0, len(params.Tasks)),
		TestRunners:     nil,
		CoordinatorMode: "coordinate_only",
	}
	root := c.cfg.WorkingDir()
	baseRef := resolveWorktreeBaseRef(ctx, root)
	result.BaseBranch = baseRef
	result.SignalDirectory = filepath.Join(root, ".sapphire", "signals")

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
			if ref.WorktreePath != "" {
				if signalPath, err := writeOrchestrationSignal(root, ref); err == nil {
					ref.SignalPath = signalPath
				}
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
	if err := verifyOrchestrationSignals(result.Tasks); err != nil {
		return OrchestrateWorktreesResult{}, err
	}

	conflictReportPath, err := writeConflictReport(ctx, root, baseRef, result.Tasks)
	if err != nil {
		return OrchestrateWorktreesResult{}, err
	}
	result.ConflictReportPath = conflictReportPath

	prDraftPath, err := writePRDraftMetadata(root, baseRef, result.Tasks)
	if err != nil {
		return OrchestrateWorktreesResult{}, err
	}
	result.PRDraftPath = prDraftPath

	integrationPrompt := strings.TrimSpace(params.IntegrationPrompt)
	if !params.AllowCoordinatorImplementation {
		result.IntegrationSkippedReason = "coordinator-only mode enforced; generated signals, conflict report, and draft PR metadata without integration"
		return result, nil
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
			WriteManifest: []string{},
			AgentID:       config.AgentTask,
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

func writeOrchestrationSignal(root string, ref OrchestrationAgentRef) (string, error) {
	if root == "" || ref.WorktreePath == "" {
		return "", nil
	}
	head, err := gitOutputAt(ref.WorktreePath, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	signalDir := filepath.Join(root, ".sapphire", "signals")
	if err := os.MkdirAll(signalDir, 0o755); err != nil {
		return "", err
	}
	name := sanitizeSignalName(firstNonEmptyOrchestration(ref.Title, ref.AgentID, ref.Branch))
	if name == "" {
		name = "agent"
	}
	signalPath := filepath.Join(signalDir, name+".signal")
	status := "DONE"
	if ref.Status != subAgentStatusCompleted {
		status = "STATUS"
	}
	payload := fmt.Sprintf("%s:%s\n", status, strings.TrimSpace(head))
	if err := os.WriteFile(signalPath, []byte(payload), 0o644); err != nil {
		return "", err
	}
	return signalPath, nil
}

func verifyOrchestrationSignals(tasks []OrchestrationAgentRef) error {
	for _, task := range tasks {
		if strings.TrimSpace(task.SignalPath) == "" {
			return fmt.Errorf("missing completion signal for %s", firstNonEmptyOrchestration(task.Title, task.AgentID, task.Branch))
		}
	}
	return nil
}

func writeConflictReport(ctx context.Context, root, baseRef string, tasks []OrchestrationAgentRef) (string, error) {
	reportDir := filepath.Join(root, ".sapphire", "reports", "conflicts")
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(reportDir, fmt.Sprintf("conflict-report-%d.md", time.Now().Unix()))

	var b strings.Builder
	b.WriteString("# Conflict Report\n\n")
	b.WriteString("Mode: non-destructive\n")
	b.WriteString("Base branch: " + baseRef + "\n\n")
	if len(tasks) < 2 {
		b.WriteString("No parallel branch pairs to compare.\n")
		return path, os.WriteFile(path, []byte(b.String()), 0o644)
	}

	anyOverlap := false
	for i := 0; i < len(tasks); i++ {
		for j := i + 1; j < len(tasks); j++ {
			a := tasks[i]
			candidate, err := overlappingChangedFiles(ctx, root, baseRef, a.Branch, tasks[j].Branch)
			if err != nil {
				return "", err
			}
			b.WriteString(fmt.Sprintf("## %s vs %s\n", a.Branch, tasks[j].Branch))
			if len(candidate) == 0 {
				b.WriteString("No overlapping changed files.\n\n")
				continue
			}
			anyOverlap = true
			b.WriteString("Potential conflicting files:\n")
			for _, file := range candidate {
				b.WriteString("- " + file + "\n")
			}
			b.WriteString("\n")
		}
	}
	if !anyOverlap {
		b.WriteString("No overlapping changed files detected across task branches.\n")
	}
	return path, os.WriteFile(path, []byte(b.String()), 0o644)
}

func writePRDraftMetadata(root, baseRef string, tasks []OrchestrationAgentRef) (string, error) {
	reportDir := filepath.Join(root, ".sapphire", "reports", "pr-drafts")
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(reportDir, fmt.Sprintf("pr-draft-%d.md", time.Now().Unix()))

	branches := make([]string, 0, len(tasks))
	body := make([]string, 0, len(tasks)+4)
	body = append(body, "# Draft PR Metadata", "")
	body = append(body, "Base branch: "+baseRef, "")
	body = append(body, "## Branches")
	for _, task := range tasks {
		if strings.TrimSpace(task.Branch) == "" {
			continue
		}
		branches = append(branches, task.Branch)
		body = append(body, "- "+task.Branch+" — "+firstNonEmptyOrchestration(task.Title, task.AgentID))
	}
	body = append(body, "", "## Signals")
	for _, task := range tasks {
		if task.SignalPath != "" {
			body = append(body, "- "+task.SignalPath)
		}
	}
	body = append(body, "", "## Review Notes", "- Human review required", "- Human merge required", "- Human push required")
	if len(branches) > 0 {
		body = append(body, "", "Suggested title: feat: integrate "+strings.Join(branches, ", "))
	}
	return path, os.WriteFile(path, []byte(strings.Join(body, "\n")+"\n"), 0o644)
}

func overlappingChangedFiles(ctx context.Context, root, baseRef, branchA, branchB string) ([]string, error) {
	filesA, err := changedFilesAgainst(ctx, root, baseRef, branchA)
	if err != nil {
		return nil, err
	}
	filesB, err := changedFilesAgainst(ctx, root, baseRef, branchB)
	if err != nil {
		return nil, err
	}
	setB := make(map[string]struct{}, len(filesB))
	for _, file := range filesB {
		setB[file] = struct{}{}
	}
	overlap := make([]string, 0)
	for _, file := range filesA {
		if _, ok := setB[file]; ok {
			overlap = append(overlap, file)
		}
	}
	slices.Sort(overlap)
	return overlap, nil
}

func changedFilesAgainst(ctx context.Context, root, baseRef, branch string) ([]string, error) {
	if strings.TrimSpace(branch) == "" {
		return nil, nil
	}
	out, err := gitOutputAt(root, "diff", "--name-only", fmt.Sprintf("%s..%s", baseRef, branch))
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) == "" {
		return nil, nil
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	slices.Sort(lines)
	return lines, nil
}

func gitOutputAt(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func sanitizeSignalName(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, "/", "-")
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.ReplaceAll(value, "_", "-")
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-':
			b.WriteRune(r)
		}
	}
	return strings.Trim(b.String(), "-")
}

func firstNonEmptyOrchestration(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
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
