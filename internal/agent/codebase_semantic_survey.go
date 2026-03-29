package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	agentmemory "github.com/duggal1/Sapphire-cli/internal/agent/memory"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools"
	"github.com/duggal1/Sapphire-cli/internal/codebasesurvey"
	"github.com/duggal1/Sapphire-cli/internal/codeindex"
	"github.com/duggal1/Sapphire-cli/internal/config"
)

const (
	defaultSemanticSurveyAgents = 3
	maxSemanticSurveyAgents     = 4
	semanticSurveyShardTimeout  = 30 * time.Minute
	semanticSurveyReasoning     = "low"
	semanticSurveyRepairPasses  = 2
)

type indexCodebaseOptions struct {
	Force     bool
	SessionID string
	SubAgents int
}

type codebaseSemanticSurveyResult struct {
	Status       string
	AgentCount   int
	ShardCount   int
	TotalFiles   int
	ManifestPath string
	OverviewPath string
	GeneratedAt  time.Time
}

type semanticSurveyShardPlan struct {
	ID             string
	Label          string
	Files          []agentmemory.IndexedFileInfo
	TopDirectories []string
	CriticalFiles  []string
	LanguageCounts map[string]int
}

type semanticSurveyCoverage struct {
	AssignedCount int
	ReadCount     int
	ReadFiles     []string
	MissingFiles  []string
	Error         string
}

func (c *coordinator) indexCodebaseWithOptions(ctx context.Context, opts indexCodebaseOptions) (codeindex.Stats, *codebaseSemanticSurveyResult, error) {
	if c == nil || c.memoryCompiler == nil {
		return codeindex.Stats{}, nil, fmt.Errorf("durable codebase graph is not initialized")
	}
	indexCtx, release, err := c.beginCodebaseIndex(ctx)
	if err != nil {
		return codeindex.Stats{}, nil, err
	}
	defer release()

	result, err := c.memoryCompiler.WarmCodebase(indexCtx, agentmemory.WarmRequest{
		WorkingDir: c.mainWorkingDir(),
		Force:      opts.Force,
	}, func(progress agentmemory.WarmProgress) {
		codeindex.PublishProgress(codeindex.Progress{
			Workspace:       progress.Workspace,
			Phase:           progress.Phase,
			Message:         progress.Message,
			Active:          progress.Active,
			Finished:        progress.Finished,
			FilesDiscovered: progress.FilesDiscovered,
			FilesProcessed:  progress.FilesProcessed,
			FilesIndexed:    progress.FilesIndexed,
			Percent:         progress.Percent,
			StartedAt:       progress.StartedAt,
			UpdatedAt:       progress.UpdatedAt,
			Error:           progress.Error,
		})
	})
	if err != nil {
		if strings.Contains(err.Error(), "context canceled") || strings.Contains(err.Error(), "context deadline exceeded") || errorsIs(err, context.Canceled) {
			return codeindex.Stats{}, nil, fmt.Errorf("codebase indexing stopped")
		}
		return codeindex.Stats{}, nil, err
	}

	lastIndexedAt := result.Status.LastIndexedAt
	if lastIndexedAt.IsZero() {
		lastIndexedAt = time.Now().UTC()
	}
	fileCount := result.Status.FileCount
	if fileCount == 0 {
		fileCount = result.DiscoveredFiles
	}
	stats := codeindex.Stats{
		FileCount:     fileCount,
		LastIndexedAt: lastIndexedAt,
	}

	survey, err := c.runMandatorySemanticCodebaseSurvey(indexCtx, opts.SessionID, result.Status, fileCount, opts.SubAgents)
	if err != nil {
		return stats, nil, err
	}
	return stats, survey, nil
}

func errorsIs(err, target error) bool {
	return err != nil && target != nil && strings.Contains(err.Error(), target.Error())
}

func (c *coordinator) runMandatorySemanticCodebaseSurvey(ctx context.Context, sessionID string, status agentmemory.IndexStatus, fileCount int, requestedAgents int) (*codebaseSemanticSurveyResult, error) {
	if c == nil || c.memoryCompiler == nil {
		return nil, fmt.Errorf("memory compiler is not initialized")
	}
	status, files, err := c.memoryCompiler.ListIndexedFiles(ctx, c.mainWorkingDir())
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("durable codebase graph contains no files to survey")
	}

	surveySessionID, err := c.ensureSemanticSurveySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	agentCount := normalizeSemanticSurveyAgentCount(requestedAgents, len(files))
	shards := buildSemanticSurveyShards(files, agentCount)
	if len(shards) == 0 {
		return nil, fmt.Errorf("no semantic survey shards were generated")
	}

	dataDir := strings.TrimSpace(c.cfg.Options.DataDirectory)
	if dataDir == "" {
		return nil, fmt.Errorf("sapphire data directory is not configured")
	}
	if err := codebasesurvey.EnsureLayout(dataDir); err != nil {
		return nil, err
	}

	manifest := codebasesurvey.Manifest{
		RepoRoot:      status.RepoRoot,
		ScopePath:     status.ScopePath,
		HeadCommit:    "",
		IndexEpoch:    status.IndexEpoch,
		GeneratedAt:   time.Now().UTC(),
		Status:        "running",
		AgentCount:    agentCount,
		TotalFiles:    len(files),
		OverviewPath:  codebasesurvey.OverviewPath(dataDir),
		CriticalFiles: collectSurveyCriticalFiles(shards),
	}
	if err := codebasesurvey.WriteManifest(dataDir, manifest); err != nil {
		return nil, err
	}

	reportProgress := func(message string, percent float64) {
		codeindex.PublishProgress(codeindex.Progress{
			Workspace:       status.ScopePath,
			Phase:           "semantic_graph",
			Message:         message,
			Active:          true,
			FilesDiscovered: len(files),
			FilesIndexed:    len(files),
			Percent:         percent,
			StartedAt:       manifest.GeneratedAt,
			UpdatedAt:       time.Now().UTC(),
		})
	}

	reportProgress(formatSemanticSurveyProgressMessage(0, len(shards), true), 0.92)

	agentIDs := make([]string, 0, len(shards))
	shardByAgent := make(map[string]semanticSurveyShardPlan, len(shards))
	for i, shard := range shards {
		input := codebasesurvey.ShardInput{
			ShardID:        shard.ID,
			Label:          shard.Label,
			RepoRoot:       status.RepoRoot,
			ScopePath:      status.ScopePath,
			IndexEpoch:     status.IndexEpoch,
			AssignedFiles:  collectShardPaths(shard.Files),
			TopDirectories: shard.TopDirectories,
			CriticalFiles:  shard.CriticalFiles,
			LanguageCounts: shard.LanguageCounts,
		}
		inputPath, err := codebasesurvey.WriteShardInput(dataDir, input)
		if err != nil {
			return nil, err
		}
		graphPath := codebasesurvey.ShardGraphPath(dataDir, shard.ID)
		if err := ensureEmptyFile(graphPath); err != nil {
			return nil, err
		}
		agentID, _, err := c.spawnSubAgent(ctx, surveySessionID, spawnAgentOptions{
			WorkItemID:       "codebase-semantic-" + shard.ID,
			Prompt:           buildSemanticSurveyShardPrompt(inputPath, graphPath, shard),
			Title:            "AI Codebase Graph " + shard.Label,
			WriteManifest:    []string{graphPath},
			DefinitionOfDone: buildSemanticSurveyDefinitionOfDone(graphPath, shard),
			AgentID:          config.AgentTask,
			ReasoningEffort:  semanticSurveyReasoning,
			TurnTimeout:      semanticSurveyShardTimeout,
		})
		if err != nil {
			return nil, err
		}
		agentIDs = append(agentIDs, agentID)
		shardByAgent[agentID] = shards[i]
	}

	waitDeadline := time.Now().Add(semanticSurveyTimeout(len(files), len(shards)))
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		remaining := time.Until(waitDeadline)
		if remaining <= 0 {
			break
		}
		waitSlice := minDuration(15*time.Second, remaining)
		snapshots, _ := c.waitSubAgents(ctx, agentIDs, waitSlice)
		done := 0
		for _, snap := range snapshots {
			if isSubAgentFinalStatus(snap.Status) {
				done++
			}
		}
		reportProgress(formatSemanticSurveyProgressMessage(done, len(shards), false), 0.92+(0.06*(float64(done)/float64(len(shards)))))
		if done == len(shards) {
			break
		}
	}

	coverageByAgent := c.repairSemanticSurveyCoverage(ctx, dataDir, agentIDs, shardByAgent)
	collected := c.collectSubAgentResults(agentIDs)
	manifest.Status = "ready"
	manifest.GeneratedAt = time.Now().UTC()
	manifest.Overview = make([]string, 0, len(collected))
	manifest.ShardArtifacts = make([]codebasesurvey.ShardArtifact, 0, len(collected))

	for _, result := range collected {
		shard := shardByAgent[result.ID]
		coverage := coverageByAgent[result.ID]
		statusText := strings.TrimSpace(string(result.Status))
		if statusText == "" {
			statusText = "unknown"
		}
		if len(coverage.MissingFiles) > 0 && statusText == string(subAgentStatusCompleted) {
			statusText = "partial"
		}
		summary := strings.TrimSpace(result.Progress)
		if summary == "" {
			summary = strings.TrimSpace(result.Result)
		}
		summary = compactSurveySummary(summary)
		graphPath := codebasesurvey.ShardGraphPath(dataDir, shard.ID)
		if strings.TrimSpace(result.Result) != "" {
			if _, statErr := os.Stat(graphPath); statErr != nil {
				_ = os.WriteFile(graphPath, []byte(strings.TrimSpace(result.Result)+"\n"), 0o644)
			}
		}
		artifact := codebasesurvey.ShardArtifact{
			ShardID:        shard.ID,
			Label:          shard.Label,
			AgentID:        result.ID,
			Status:         statusText,
			FileCount:      len(shard.Files),
			ReadCount:      coverage.ReadCount,
			CoverageStatus: describeSemanticSurveyCoverage(coverage),
			TopDirectories: shard.TopDirectories,
			CriticalFiles:  shard.CriticalFiles,
			MissingFiles:   limitSurveyStrings(coverage.MissingFiles, 12),
			Summary:        summary,
			ArtifactPath:   graphPath,
			Error:          firstNonEmptyString(strings.TrimSpace(result.Error), strings.TrimSpace(coverage.Error)),
		}
		if statusText != string(subAgentStatusCompleted) {
			manifest.Status = "partial"
		}
		manifest.ShardArtifacts = append(manifest.ShardArtifacts, artifact)
		if summary != "" {
			manifest.Overview = append(manifest.Overview, shard.Label+": "+summary)
		}
	}

	if err := codebasesurvey.WriteManifest(dataDir, manifest); err != nil {
		return nil, err
	}
	if err := c.runSemanticSurveyAggregator(ctx, surveySessionID, dataDir, manifest, len(files)); err == nil {
		manifest.OverviewPath = codebasesurvey.OverviewPath(dataDir)
	}
	if len(manifest.Overview) == 0 {
		manifest.Overview = defaultSemanticOverview(manifest)
	}
	if err := codebasesurvey.WriteManifest(dataDir, manifest); err != nil {
		return nil, err
	}
	for _, agentID := range agentIDs {
		_ = c.closeSubAgent(agentID)
	}
	if c.pmem != nil && strings.TrimSpace(sessionID) != "" {
		_ = c.pmem.RefreshMemory(ctx, sessionID, true)
	}

	reportProgress("AI codebase graph ready", 1)
	return &codebaseSemanticSurveyResult{
		Status:       manifest.Status,
		AgentCount:   manifest.AgentCount,
		ShardCount:   len(manifest.ShardArtifacts),
		TotalFiles:   manifest.TotalFiles,
		ManifestPath: codebasesurvey.ManifestPath(dataDir),
		OverviewPath: manifest.OverviewPath,
		GeneratedAt:  manifest.GeneratedAt,
	}, nil
}

func formatSemanticSurveyProgressMessage(done, total int, launching bool) string {
	total = max(total, 1)
	done = max(done, 0)
	if done > total {
		done = total
	}
	if launching {
		return fmt.Sprintf("Launching AI codebase survey sub-agents; shard graph runs in background (%d/%d complete)", done, total)
	}
	if done >= total {
		return fmt.Sprintf("AI codebase graph shards complete %d/%d", done, total)
	}
	return fmt.Sprintf("AI codebase graph shards complete %d/%d; sub-agents still running", done, total)
}

func (c *coordinator) ensureSemanticSurveySession(ctx context.Context, sessionID string) (string, error) {
	if strings.TrimSpace(sessionID) != "" {
		return strings.TrimSpace(sessionID), nil
	}
	if c == nil || c.sessions == nil {
		return "", fmt.Errorf("session service is not initialized")
	}
	sess, err := c.sessions.Create(ctx, "AI Codebase Survey")
	if err != nil {
		return "", fmt.Errorf("create semantic survey session: %w", err)
	}
	return sess.ID, nil
}

func buildSemanticSurveyShards(files []agentmemory.IndexedFileInfo, requestedAgents int) []semanticSurveyShardPlan {
	agentCount := normalizeSemanticSurveyAgentCount(requestedAgents, len(files))
	if agentCount == 0 || len(files) == 0 {
		return nil
	}
	groups := make(map[string][]agentmemory.IndexedFileInfo)
	for _, file := range files {
		key := topLevelPath(file.Path)
		groups[key] = append(groups[key], file)
	}
	type group struct {
		label string
		files []agentmemory.IndexedFileInfo
	}
	ordered := make([]group, 0, len(groups))
	for label, items := range groups {
		sort.Slice(items, func(i, j int) bool { return items[i].Path < items[j].Path })
		ordered = append(ordered, group{label: label, files: items})
	}
	sort.Slice(ordered, func(i, j int) bool {
		if len(ordered[i].files) == len(ordered[j].files) {
			return ordered[i].label < ordered[j].label
		}
		return len(ordered[i].files) > len(ordered[j].files)
	})

	shards := make([]semanticSurveyShardPlan, agentCount)
	loads := make([]int, agentCount)
	for i := range shards {
		shards[i] = semanticSurveyShardPlan{
			ID:             fmt.Sprintf("shard-%02d", i+1),
			Label:          fmt.Sprintf("Shard %d", i+1),
			LanguageCounts: make(map[string]int),
		}
	}
	for _, group := range ordered {
		target := 0
		for i := 1; i < len(shards); i++ {
			if loads[i] < loads[target] {
				target = i
			}
		}
		shards[target].Files = append(shards[target].Files, group.files...)
		shards[target].TopDirectories = append(shards[target].TopDirectories, group.label)
		loads[target] += len(group.files)
		for _, file := range group.files {
			shards[target].LanguageCounts[file.Language]++
		}
	}
	out := make([]semanticSurveyShardPlan, 0, len(shards))
	for i, shard := range shards {
		if len(shard.Files) == 0 {
			continue
		}
		sort.Strings(shard.TopDirectories)
		shard.Label = fmt.Sprintf("Shard %d (%s)", i+1, strings.Join(limitSurveyStrings(shard.TopDirectories, 2), ", "))
		shard.CriticalFiles = selectSemanticCriticalFiles(shard.Files, 18)
		out = append(out, shard)
	}
	return out
}

func buildSemanticSurveyShardPrompt(inputPath, graphPath string, shard semanticSurveyShardPlan) string {
	return strings.TrimSpace(fmt.Sprintf(`
You are the AI author for one shard of Sapphire's durable codebase graph.

This is not optional exploration. Your job is to build the semantic graph for your shard and write it to:
%s

First, read:
- agent.md if it exists
- the shard manifest: %s

Requirements:
- Inspect the assigned shard deeply.
- Account for every assigned file in the shard manifest.
- Read every assigned file with the real repository read tools before you finalize.
- The system verifies shard coverage from your actual tool calls. If files were skipped, your shard will be sent back for repair.
- Read critical files fully.
- Build an AI-authored graph, not a generic summary.
- Do not edit repository source files.
- You may only write the graph artifact above.

The graph artifact must contain:
1. Shard metadata
2. Module graph and relationships
3. Critical files with exact responsibilities
4. File inventory coverage notes for the assigned shard
5. Key symbols, boundaries, and integration surfaces
6. Risks, stale areas, and likely change hotspots

Return a final report in this exact header format before any extra prose:
STATUS: completed
SUMMARY: one concise sentence about the shard
PROGRESS: graph written to %s
FILES: comma-separated critical file paths
COMMANDS: list the main tools/commands you used
RISKS: one concise risks line or "none"
NEXT: one concise next action for the overall repo graph
BLOCKERS: one concise blocker line or "none"

Then add any additional structured notes you want.
`, graphPath, inputPath, graphPath))
}

func buildSemanticSurveyDefinitionOfDone(graphPath string, shard semanticSurveyShardPlan) string {
	return strings.TrimSpace(fmt.Sprintf(
		"Write the shard's AI-authored semantic graph to %s. The file must explain the shard architecture, file responsibilities, key boundaries, and risks, and it must cover the assigned shard inventory rather than stopping at a few files.",
		graphPath,
	))
}

func (c *coordinator) repairSemanticSurveyCoverage(ctx context.Context, dataDir string, agentIDs []string, shardByAgent map[string]semanticSurveyShardPlan) map[string]semanticSurveyCoverage {
	coverageByAgent := make(map[string]semanticSurveyCoverage, len(agentIDs))
	pending := append([]string{}, agentIDs...)
	for pass := 0; pass <= semanticSurveyRepairPasses && len(pending) > 0; pass++ {
		next := make([]string, 0, len(pending))
		for _, agentID := range pending {
			shard, ok := shardByAgent[agentID]
			if !ok {
				continue
			}
			coverage := c.inspectSemanticSurveyCoverage(ctx, agentID, shard)
			coverageByAgent[agentID] = coverage
			if len(coverage.MissingFiles) == 0 {
				continue
			}
			if pass >= semanticSurveyRepairPasses {
				continue
			}
			graphPath := codebasesurvey.ShardGraphPath(dataDir, shard.ID)
			if _, err := c.sendSubAgentInput(ctx, agentID, buildSemanticSurveyCoverageRepairPrompt(graphPath, shard, coverage.MissingFiles), nil, false); err != nil {
				coverage.Error = firstNonEmptyString(coverage.Error, err.Error())
				coverageByAgent[agentID] = coverage
				continue
			}
			next = append(next, agentID)
		}
		if len(next) == 0 {
			break
		}
		waitDeadline := time.Now().Add(minDuration(10*time.Minute, semanticSurveyTimeout(0, len(next))))
		for {
			if err := ctx.Err(); err != nil {
				return coverageByAgent
			}
			remaining := time.Until(waitDeadline)
			if remaining <= 0 {
				break
			}
			snaps, _ := c.waitSubAgents(ctx, next, minDuration(15*time.Second, remaining))
			done := 0
			for _, snap := range snaps {
				if isSubAgentFinalStatus(snap.Status) {
					done++
				}
			}
			if done == len(next) {
				break
			}
		}
		pending = next
	}
	for _, agentID := range agentIDs {
		if _, ok := coverageByAgent[agentID]; ok {
			continue
		}
		if shard, ok := shardByAgent[agentID]; ok {
			coverageByAgent[agentID] = c.inspectSemanticSurveyCoverage(ctx, agentID, shard)
		}
	}
	return coverageByAgent
}

func (c *coordinator) inspectSemanticSurveyCoverage(ctx context.Context, agentID string, shard semanticSurveyShardPlan) semanticSurveyCoverage {
	runner, err := c.getSubAgent(agentID)
	if err != nil {
		return semanticSurveyCoverage{
			AssignedCount: len(shard.Files),
			MissingFiles:  collectShardPaths(shard.Files),
			Error:         err.Error(),
		}
	}
	runner.mu.Lock()
	sessionID := runner.sessionID
	workDir := runner.workDir
	runner.mu.Unlock()

	assigned := collectShardPaths(shard.Files)
	readFiles, err := c.collectSemanticSurveyReadFiles(ctx, sessionID, workDir)
	if err != nil {
		return semanticSurveyCoverage{
			AssignedCount: len(assigned),
			MissingFiles:  assigned,
			Error:         err.Error(),
		}
	}
	readSet := make(map[string]struct{}, len(readFiles))
	for _, path := range readFiles {
		readSet[path] = struct{}{}
	}
	missing := make([]string, 0, len(assigned))
	for _, path := range assigned {
		if _, ok := readSet[path]; ok {
			continue
		}
		missing = append(missing, path)
	}
	return semanticSurveyCoverage{
		AssignedCount: len(assigned),
		ReadCount:     len(readSet),
		ReadFiles:     readFiles,
		MissingFiles:  missing,
	}
}

func (c *coordinator) collectSemanticSurveyReadFiles(ctx context.Context, sessionID, workDir string) ([]string, error) {
	if c == nil || c.messages == nil || strings.TrimSpace(sessionID) == "" {
		return nil, fmt.Errorf("semantic survey session is unavailable")
	}
	msgs, err := c.messages.List(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{})
	files := make([]string, 0, 64)
	for _, msg := range msgs {
		for _, call := range msg.ToolCalls() {
			for _, path := range extractSemanticSurveyToolPaths(call.Name, call.Input, workDir) {
				if _, ok := seen[path]; ok {
					continue
				}
				seen[path] = struct{}{}
				files = append(files, path)
			}
		}
	}
	sort.Strings(files)
	return files, nil
}

func extractSemanticSurveyToolPaths(toolName, input, workDir string) []string {
	switch strings.TrimSpace(toolName) {
	case tools.ViewToolName, tools.SingleViewToolName, tools.AgenticViewToolName:
	default:
		return nil
	}
	var params tools.ViewParams
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return nil
	}
	paths := make([]string, 0, 8)
	if trimmed := strings.TrimSpace(params.FilePath); trimmed != "" {
		paths = append(paths, trimmed)
	}
	paths = append(paths, params.FilePaths...)
	paths = append(paths, params.Paths...)
	paths = append(paths, params.Files...)
	if trimmed := strings.TrimSpace(params.Path); trimmed != "" {
		paths = append(paths, trimmed)
	}
	seen := make(map[string]struct{}, len(paths))
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		normalized := normalizeSemanticSurveyPath(workDir, path)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	sort.Strings(out)
	return out
}

func normalizeSemanticSurveyPath(workDir, path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if workDir != "" && filepath.IsAbs(path) {
		if rel, err := filepath.Rel(workDir, path); err == nil {
			path = rel
		}
	}
	path = filepath.ToSlash(filepath.Clean(path))
	if path == "." || path == "" || path == ".." || strings.HasPrefix(path, "../") {
		return ""
	}
	return path
}

func describeSemanticSurveyCoverage(coverage semanticSurveyCoverage) string {
	if strings.TrimSpace(coverage.Error) != "" {
		return "unverified"
	}
	if coverage.AssignedCount == 0 {
		return "empty"
	}
	if len(coverage.MissingFiles) == 0 {
		return "verified"
	}
	return fmt.Sprintf("partial %d/%d", coverage.AssignedCount-len(coverage.MissingFiles), coverage.AssignedCount)
}

func buildSemanticSurveyCoverageRepairPrompt(graphPath string, shard semanticSurveyShardPlan, missing []string) string {
	missing = append([]string{}, missing...)
	sort.Strings(missing)
	return strings.TrimSpace(fmt.Sprintf(`
Coverage verifier found missing assigned files for %s.

You must repair the shard graph now.

Requirements:
- Read every missing file below with the real repository read tools.
- Use "single_view" for exactly one file and "agentic_view" for multiple files.
- Do not claim completion until every missing file has been read.
- Update the existing shard graph at %s so it covers the full assigned inventory.

Missing files:
- %s

Return the same required final header block:
STATUS: completed
SUMMARY: one concise sentence about the shard
PROGRESS: graph written to %s
FILES: comma-separated critical file paths
COMMANDS: list the main tools/commands you used
RISKS: one concise risks line or "none"
NEXT: one concise next action for the overall repo graph
BLOCKERS: one concise blocker line or "none"
`, shard.Label, graphPath, strings.Join(missing, "\n- "), graphPath))
}

func (c *coordinator) runSemanticSurveyAggregator(ctx context.Context, sessionID, dataDir string, manifest codebasesurvey.Manifest, totalFiles int) error {
	overviewPath := codebasesurvey.OverviewPath(dataDir)
	if err := ensureEmptyFile(overviewPath); err != nil {
		return err
	}
	manifestPath := codebasesurvey.ManifestPath(dataDir)
	agentID, _, err := c.spawnSubAgent(ctx, sessionID, spawnAgentOptions{
		WorkItemID:       "codebase-semantic-overview",
		Prompt:           buildSemanticSurveyOverviewPrompt(manifestPath, overviewPath, manifest),
		Title:            "AI Codebase Graph Overview",
		WriteManifest:    []string{overviewPath},
		DefinitionOfDone: "Write the overall AI-authored codebase graph overview with architecture, critical systems, and cross-shard relationships.",
		AgentID:          config.AgentTask,
		ReasoningEffort:  semanticSurveyReasoning,
		TurnTimeout:      semanticSurveyShardTimeout,
	})
	if err != nil {
		return err
	}
	defer func() { _ = c.closeSubAgent(agentID) }()

	waitDeadline := time.Now().Add(minDuration(semanticSurveyTimeout(totalFiles, len(manifest.ShardArtifacts)), 20*time.Minute))
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		remaining := time.Until(waitDeadline)
		if remaining <= 0 {
			break
		}
		snaps, _ := c.waitSubAgents(ctx, []string{agentID}, minDuration(15*time.Second, remaining))
		if len(snaps) == 1 && isSubAgentFinalStatus(snaps[0].Status) {
			return nil
		}
	}
	results := c.collectSubAgentResults([]string{agentID})
	if len(results) == 0 {
		return fmt.Errorf("semantic overview agent produced no result")
	}
	if results[0].Status != subAgentStatusCompleted {
		return fmt.Errorf("semantic overview agent status: %s", results[0].Status)
	}
	return nil
}

func buildSemanticSurveyOverviewPrompt(manifestPath, overviewPath string, manifest codebasesurvey.Manifest) string {
	return strings.TrimSpace(fmt.Sprintf(`
You are the AI author of Sapphire's overall durable codebase graph.

Read the manifest first:
- %s

Then read every shard artifact referenced there and write the overall graph to:
%s

Requirements:
- Synthesize the shard graphs into one repo-wide architecture map.
- Explain the main subsystems, their boundaries, and the important cross-links.
- Identify the most critical files and why they matter.
- Call out long-horizon risks, coordination hotspots, and likely drift points.
- Do not edit repository source files.
- You may only write the overview artifact above.
`, manifestPath, overviewPath))
}

func normalizeSemanticSurveyAgentCount(requested, totalFiles int) int {
	if totalFiles <= 0 {
		return 0
	}
	count := requested
	if count <= 0 {
		count = defaultSemanticSurveyAgents
	}
	if count > maxSemanticSurveyAgents {
		count = maxSemanticSurveyAgents
	}
	if count > totalFiles {
		count = totalFiles
	}
	if count < 1 {
		count = 1
	}
	return count
}

func topLevelPath(path string) string {
	path = filepath.ToSlash(strings.TrimSpace(path))
	if path == "" {
		return "."
	}
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		return "."
	}
	return parts[0]
}

func selectSemanticCriticalFiles(files []agentmemory.IndexedFileInfo, limit int) []string {
	type scored struct {
		path  string
		score int
	}
	items := make([]scored, 0, len(files))
	for _, file := range files {
		score := file.SymbolCount
		path := strings.ToLower(file.Path)
		if path == "main.go" {
			score += 200
		}
		for _, needle := range []string{"coordinator.go", "agent.go", "manager.go", "service.go", "router.go", "mailbox.go", "runtime.go", "memory", "index", "db.go", "root.go"} {
			if strings.Contains(path, needle) {
				score += 75
			}
		}
		if strings.Contains(path, "/cmd/") || strings.HasPrefix(path, "internal/agent/") || strings.HasPrefix(path, "internal/orchestration/") {
			score += 35
		}
		if strings.HasSuffix(path, "_test.go") {
			score -= 40
		}
		items = append(items, scored{path: file.Path, score: score})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].score == items[j].score {
			return items[i].path < items[j].path
		}
		return items[i].score > items[j].score
	})
	seen := make(map[string]struct{}, limit)
	out := make([]string, 0, limit)
	for _, item := range items {
		if _, ok := seen[item.path]; ok {
			continue
		}
		seen[item.path] = struct{}{}
		out = append(out, item.path)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func collectShardPaths(files []agentmemory.IndexedFileInfo) []string {
	out := make([]string, 0, len(files))
	for _, file := range files {
		if trimmed := strings.TrimSpace(file.Path); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	sort.Strings(out)
	return out
}

func collectSurveyCriticalFiles(shards []semanticSurveyShardPlan) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, 24)
	for _, shard := range shards {
		for _, path := range shard.CriticalFiles {
			if _, ok := seen[path]; ok {
				continue
			}
			seen[path] = struct{}{}
			out = append(out, path)
			if len(out) >= 24 {
				return out
			}
		}
	}
	return out
}

func semanticSurveyTimeout(totalFiles, shardCount int) time.Duration {
	base := 4 * time.Minute
	extra := time.Duration(totalFiles/25) * time.Second
	if shardCount > 0 {
		extra += time.Duration(shardCount) * time.Minute
	}
	timeout := base + extra
	if timeout > 45*time.Minute {
		return 45 * time.Minute
	}
	if timeout < 8*time.Minute {
		return 8 * time.Minute
	}
	return timeout
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func ensureEmptyFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, nil, 0o644)
}

func compactSurveySummary(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	text = strings.ReplaceAll(text, "\n", " ")
	if len(text) > 220 {
		return text[:220] + "..."
	}
	return text
}

func defaultSemanticOverview(manifest codebasesurvey.Manifest) []string {
	lines := make([]string, 0, len(manifest.ShardArtifacts))
	for _, shard := range manifest.ShardArtifacts {
		line := fmt.Sprintf("%s covers %d files", shard.Label, shard.FileCount)
		if shard.Summary != "" {
			line += " and reports: " + shard.Summary
		}
		lines = append(lines, line)
	}
	return lines
}

func limitSurveyStrings(values []string, limit int) []string {
	if len(values) == 0 || limit <= 0 {
		return nil
	}
	if len(values) <= limit {
		return append([]string{}, values...)
	}
	return append([]string{}, values[:limit]...)
}
