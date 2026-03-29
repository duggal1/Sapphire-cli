package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/duggal1/Sapphire-cli/internal/codebasesurvey"
)

const memoryFileName = "memory.md"

type memoryFileManager struct {
	filePath    string
	projectRoot string

	mu               sync.Mutex
	lastRefreshTurn  map[string]uint64
	lastRefreshState map[string]string
	codebaseSnapshot memoryCodebaseSnapshot
}

type memoryCodebaseSnapshot struct {
	TotalFiles      int
	Architecture    []string
	CriticalFiles   []string
	SupportingFiles []string
}

type memoryFileSummary struct {
	path     string
	summary  string
	priority int
}

func newMemoryFileManager(dataDir, projectRoot string) (*memoryFileManager, error) {
	dir := filepath.Join(dataDir, "memory")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("memory file: create dir: %w", err)
	}
	return &memoryFileManager{
		filePath:         filepath.Join(dir, memoryFileName),
		projectRoot:      projectRoot,
		lastRefreshTurn:  make(map[string]uint64),
		lastRefreshState: make(map[string]string),
	}, nil
}

func (m *memoryFileManager) Read() (string, error) {
	if m == nil {
		return "", nil
	}
	data, err := os.ReadFile(m.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func (m *memoryFileManager) MaybeRefresh(ctx context.Context, sessionID string, history *sessionHistoryManager, store *Store, force bool) error {
	if m == nil || history == nil {
		return nil
	}

	currentTurn, err := history.CurrentTurn(ctx, sessionID)
	if err != nil {
		return err
	}
	state, err := history.BuildSessionState(ctx, sessionID)
	if err != nil {
		return err
	}
	stateKey := strings.Join([]string{
		state.CurrentTask,
		state.LastDecision,
		state.LastModifiedFile,
		strings.Join(state.RecentFiles, ","),
		strings.Join(state.AchievementSignals, ","),
	}, "|")

	m.mu.Lock()
	shouldRefresh := force
	if !shouldRefresh {
		lastTurn := m.lastRefreshTurn[sessionID]
		lastState := m.lastRefreshState[sessionID]
		if lastTurn == 0 {
			shouldRefresh = true
		} else if currentTurn >= lastTurn+100 {
			shouldRefresh = true
		} else if state.MajorAchievementLikely && stateKey != lastState && currentTurn > 0 {
			shouldRefresh = true
		} else if state.MajorChangeLikely && stateKey != lastState && currentTurn >= lastTurn+20 {
			shouldRefresh = true
		}
	}
	m.mu.Unlock()

	if !shouldRefresh {
		return nil
	}

	content, err := m.buildContent(ctx, sessionID, state, store, force || state.MajorAchievementLikely || state.MajorChangeLikely)
	if err != nil {
		return err
	}
	if strings.TrimSpace(content) == "" {
		return nil
	}
	if err := os.WriteFile(m.filePath, []byte(content), 0o644); err != nil {
		return err
	}

	m.mu.Lock()
	m.lastRefreshTurn[sessionID] = currentTurn
	m.lastRefreshState[sessionID] = stateKey
	m.mu.Unlock()
	return nil
}

func (m *memoryFileManager) buildContent(ctx context.Context, sessionID string, state sessionStateSnapshot, store *Store, rebuildCodebase bool) (string, error) {
	codebase, err := m.loadCodebaseSnapshot(rebuildCodebase)
	if err != nil {
		return "", err
	}

	var constitution string
	var architectural []MemoryRecord
	var failures []MemoryRecord
	var constraints []MemoryRecord
	var progress []MemoryRecord
	if store != nil {
		constitution, _ = store.GetConstitution(ctx)
		architectural, _ = store.QueryRecordsBySession(ctx, sessionID, "architectural", 0)
		failures, _ = store.QueryRecordsBySession(ctx, sessionID, "failures", 0)
		constraints, _ = store.QueryRecordsBySession(ctx, sessionID, "negative_constraints", 0)
		progress, _ = store.QueryRecordsBySession(ctx, sessionID, "progress", 0)
	}

	var lines []string
	lines = append(lines,
		"# Sapphire Memory Handbook",
		"",
		"Durable repo memory for long-horizon work in this codebase. Treat it as an operating handbook, not a transcript. Re-read exact files before editing drift-prone code.",
		"",
		"## Session Snapshot",
	)
	lines = append(lines, fmt.Sprintf("- session_id: %s", strings.TrimSpace(sessionID)))
	lines = append(lines, fmt.Sprintf("- project_root: %s", m.projectRoot))
	if state.CurrentTask != "" {
		lines = append(lines, fmt.Sprintf("- current_task: %s", state.CurrentTask))
	}
	if len(state.AchievementSignals) > 0 {
		lines = append(lines, fmt.Sprintf("- achievement_signals: %s", strings.Join(state.AchievementSignals, ", ")))
	}
	if state.LastModifiedFile != "" {
		lines = append(lines, fmt.Sprintf("- last_modified_file: %s", state.LastModifiedFile))
	}
	if len(state.RecentFiles) > 0 {
		lines = append(lines, fmt.Sprintf("- recently_touched_files: %s", strings.Join(state.RecentFiles, ", ")))
	}
	if len(state.RecentTools) > 0 {
		lines = append(lines, fmt.Sprintf("- recent_tools: %s", strings.Join(state.RecentTools, ", ")))
	}

	lines = append(lines, "", "## Active Workstreams")
	workstreamLines := renderWorkstreamLines(state, progress)
	lines = append(lines, workstreamLines...)

	if strings.TrimSpace(constitution) != "" {
		lines = append(lines, "", "## Repo Constitution", strings.TrimSpace(constitution))
	}

	lines = append(lines, "", "## Stable Decisions")
	if decisionLines := renderDecisionLines(architectural, state); len(decisionLines) > 0 {
		lines = append(lines, decisionLines...)
	} else {
		lines = append(lines, "- no durable architectural decisions captured yet")
	}

	lines = append(lines, "", "## Failures and Guardrails")
	if guardrailLines := renderGuardrailLines(failures, constraints, state); len(guardrailLines) > 0 {
		lines = append(lines, guardrailLines...)
	} else {
		lines = append(lines, "- no durable failure modes or guardrails recorded yet")
	}

	lines = append(lines, "", "## Architecture Overview")
	lines = append(lines, fmt.Sprintf("- indexed_files: %d", codebase.TotalFiles))
	lines = append(lines, codebase.Architecture...)

	if surveyLines := m.renderSemanticSurveySection(); len(surveyLines) > 0 {
		lines = append(lines, "", "## AI Codebase Graph")
		lines = append(lines, surveyLines...)
	}

	lines = append(lines, "", "## Critical Files")
	lines = append(lines, codebase.CriticalFiles...)

	if len(codebase.SupportingFiles) > 0 {
		lines = append(lines, "", "## Supporting Files")
		lines = append(lines, codebase.SupportingFiles...)
	}

	lines = append(lines, "", "## Provenance")
	lines = append(lines, renderProvenanceLines(sessionID, m.projectRoot, state, codebase)...)

	return strings.TrimSpace(strings.Join(lines, "\n")) + "\n", nil
}

func (m *memoryFileManager) renderSemanticSurveySection() []string {
	if m == nil {
		return nil
	}
	dataDir := filepath.Dir(filepath.Dir(m.filePath))
	manifest, ok, err := codebasesurvey.ReadManifest(dataDir)
	if err != nil || !ok {
		return nil
	}
	lines := []string{
		fmt.Sprintf("- status: %s", firstNonEmptyMemoryValue(strings.TrimSpace(manifest.Status), "ready")),
		fmt.Sprintf("- agent_count: %d", manifest.AgentCount),
		fmt.Sprintf("- total_files: %d", manifest.TotalFiles),
	}
	if !manifest.GeneratedAt.IsZero() {
		lines = append(lines, fmt.Sprintf("- generated_at: %s", manifest.GeneratedAt.UTC().Format("2006-01-02 15:04:05Z")))
	}
	if strings.TrimSpace(manifest.OverviewPath) != "" {
		lines = append(lines, fmt.Sprintf("- overview_path: %s", strings.TrimSpace(manifest.OverviewPath)))
	}
	for _, item := range manifest.Overview {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if strings.HasPrefix(item, "- ") {
			lines = append(lines, item)
			continue
		}
		lines = append(lines, "- "+item)
	}
	if len(manifest.CriticalFiles) > 0 {
		lines = append(lines, "### Semantic Critical Files")
		for _, file := range manifest.CriticalFiles {
			file = strings.TrimSpace(file)
			if file != "" {
				lines = append(lines, "- "+file)
			}
		}
	}
	if len(manifest.ShardArtifacts) > 0 {
		lines = append(lines, "### Semantic Shards")
		for _, shard := range manifest.ShardArtifacts {
			line := fmt.Sprintf("- %s [%s] files=%d", firstNonEmptyMemoryValue(shard.Label, shard.ShardID), firstNonEmptyMemoryValue(shard.Status, "unknown"), shard.FileCount)
			if strings.TrimSpace(shard.Summary) != "" {
				line += " | summary: " + strings.TrimSpace(shard.Summary)
			}
			if strings.TrimSpace(shard.ArtifactPath) != "" {
				line += " | artifact: " + strings.TrimSpace(shard.ArtifactPath)
			}
			if strings.TrimSpace(shard.Error) != "" {
				line += " | error: " + strings.TrimSpace(shard.Error)
			}
			lines = append(lines, line)
		}
	}
	return lines
}

func firstNonEmptyMemoryValue(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func (m *memoryFileManager) loadCodebaseSnapshot(rebuild bool) (memoryCodebaseSnapshot, error) {
	m.mu.Lock()
	cached := m.codebaseSnapshot
	m.mu.Unlock()

	if !rebuild && cached.TotalFiles > 0 {
		return cached, nil
	}

	snapshot, err := m.buildCodebaseSnapshot()
	if err != nil {
		return memoryCodebaseSnapshot{}, err
	}

	m.mu.Lock()
	m.codebaseSnapshot = snapshot
	m.mu.Unlock()
	return snapshot, nil
}

func (m *memoryFileManager) buildCodebaseSnapshot() (memoryCodebaseSnapshot, error) {
	var files []memoryFileSummary
	err := filepath.WalkDir(m.projectRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == m.projectRoot {
			return nil
		}

		rel, err := filepath.Rel(m.projectRoot, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if shouldSkipMemoryPath(rel, entry.IsDir()) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		summary, ok := summarizeProjectFile(path, rel)
		if !ok {
			return nil
		}
		files = append(files, memoryFileSummary{
			path:     rel,
			summary:  summary,
			priority: memoryFilePriority(rel),
		})
		return nil
	})
	if err != nil {
		return memoryCodebaseSnapshot{}, err
	}

	sort.Slice(files, func(i, j int) bool {
		if files[i].priority == files[j].priority {
			return files[i].path < files[j].path
		}
		return files[i].priority > files[j].priority
	})

	criticalLines := make([]string, 0, len(files))
	supportingLines := make([]string, 0, len(files))
	for _, file := range files {
		line := fmt.Sprintf("- %s: %s", file.path, file.summary)
		if isCriticalMemoryFile(file.path) {
			criticalLines = append(criticalLines, line)
			continue
		}
		supportingLines = append(supportingLines, line)
	}
	if len(criticalLines) == 0 && len(files) > 0 {
		criticalLines = append(criticalLines, fmt.Sprintf("- %s: %s", files[0].path, files[0].summary))
	}

	return memoryCodebaseSnapshot{
		TotalFiles:      len(files),
		Architecture:    buildArchitectureOverview(files),
		CriticalFiles:   criticalLines,
		SupportingFiles: supportingLines,
	}, nil
}

func buildArchitectureOverview(files []memoryFileSummary) []string {
	type architectureArea struct {
		prefix      string
		description string
	}

	areas := []architectureArea{
		{prefix: "main.go", description: "CLI entrypoint and top-level runtime bootstrap."},
		{prefix: "internal/cmd/", description: "Command handlers, mode selection, and terminal startup flow."},
		{prefix: "internal/app/", description: "Application container and cross-service runtime wiring."},
		{prefix: "internal/agent/", description: "Main agent loop, prompt assembly, sub-agent orchestration, and tool coordination."},
		{prefix: "internal/orchestration/db/", description: "SQLite durable state for mail, work items, dispatch, and checkpoints."},
		{prefix: "internal/memory/", description: "Persistent memory extraction, retrieval, and generated memory map support."},
		{prefix: "internal/ui/", description: "Bubble Tea terminal UI state, lists, dialogs, and chat rendering."},
		{prefix: "internal/mcp/", description: "Model Context Protocol integration and external tool inventory."},
		{prefix: "internal/lsp/", description: "Language-server based code intelligence and diagnostics."},
	}

	var lines []string
	for _, area := range areas {
		if count := countMatchingFiles(files, area.prefix); count > 0 {
			lines = append(lines, fmt.Sprintf("- %s: %s (%d tracked files)", area.prefix, area.description, count))
		}
	}
	if len(lines) == 0 {
		lines = append(lines, "- project_layout: no high-signal source files have been indexed yet")
	}
	return lines
}

func countMatchingFiles(files []memoryFileSummary, prefix string) int {
	count := 0
	for _, file := range files {
		if file.path == prefix || strings.HasPrefix(file.path, prefix) {
			count++
		}
	}
	return count
}

func trimMemoryContentForStage(content string, stage ContextLoadStage) string {
	if strings.TrimSpace(content) == "" || stage < ContextLoadStage10 {
		return ""
	}
	preamble, sections := splitMemorySections(content)
	order := memorySectionOrderForStage(stage)
	sectionMap := make(map[string][]string, len(sections))
	for _, section := range sections {
		sectionMap[section.title] = section.lines
	}

	var out []string
	out = append(out, preamble...)
	seen := make(map[string]struct{}, len(order))
	for _, title := range order {
		lines, ok := sectionMap[title]
		if !ok {
			continue
		}
		lines = trimMemorySectionForStage(title, lines, stage)
		if len(lines) == 0 {
			continue
		}
		seen[title] = struct{}{}
		out = append(out, "")
		out = append(out, lines...)
	}
	if stage >= ContextLoadStage40 {
		for _, section := range sections {
			if _, ok := seen[section.title]; ok || section.title == "Supporting Files" {
				continue
			}
			lines := trimMemorySectionForStage(section.title, section.lines, stage)
			if len(lines) == 0 {
				continue
			}
			out = append(out, "")
			out = append(out, lines...)
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func shouldSkipMemoryPath(rel string, isDir bool) bool {
	parts := strings.Split(rel, "/")
	for _, part := range parts {
		switch part {
		case ".git", ".sapphire", ".tmp", "node_modules", "dist", "build", "coverage", "tmp", "vendor":
			return true
		}
		if strings.HasPrefix(part, ".") && part != ".github" {
			return true
		}
	}
	if isDir {
		return false
	}
	switch filepath.Ext(rel) {
	case ".png", ".jpg", ".jpeg", ".gif", ".ico", ".svg", ".pdf", ".zip", ".gz", ".bin", ".so", ".dylib":
		return true
	default:
		return false
	}
}

func summarizeProjectFile(path, rel string) (string, bool) {
	ext := strings.ToLower(filepath.Ext(rel))
	switch ext {
	case ".go", ".md", ".sql", ".toml", ".yaml", ".yml", ".json", ".ts", ".tsx", ".js", ".jsx", ".css", ".sh", ".txt":
	default:
		return "", false
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "# "):
			return strings.TrimSpace(strings.TrimPrefix(line, "# ")), true
		case strings.HasPrefix(line, "//"):
			return strings.TrimSpace(strings.TrimPrefix(line, "//")), true
		case strings.HasPrefix(line, "--"):
			return strings.TrimSpace(strings.TrimPrefix(line, "--")), true
		case strings.HasPrefix(line, "/*") && strings.Contains(line, "*/"):
			return strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "/*"), "*/")), true
		}
		break
	}

	switch ext {
	case ".go":
		return "Go source file", true
	case ".md":
		return "Markdown document", true
	case ".sql":
		return "SQL migration or query file", true
	case ".toml", ".yaml", ".yml", ".json":
		return "Project configuration file", true
	case ".ts", ".tsx", ".js", ".jsx":
		return "JavaScript or TypeScript source file", true
	case ".css":
		return "Stylesheet", true
	case ".sh":
		return "Shell script", true
	default:
		return "Project file", true
	}
}

func memoryFilePriority(rel string) int {
	score := 0
	if !strings.Contains(rel, "/") {
		score += 100
	}
	switch rel {
	case "AGENTS.md", "README.md", "go.mod", "go.sum", "main.go":
		score += 200
	}
	if strings.HasPrefix(rel, "internal/agent/") {
		score += 80
	}
	if strings.HasPrefix(rel, "internal/ui/") {
		score += 70
	}
	if strings.HasPrefix(rel, "internal/cmd/") {
		score += 60
	}
	if strings.HasSuffix(rel, "_test.go") {
		score -= 25
	}
	return score
}

func isCriticalMemoryFile(rel string) bool {
	if !strings.Contains(rel, "/") {
		return true
	}
	switch rel {
	case
		"internal/agent/coordinator.go",
		"internal/agent/subagent_manager.go",
		"internal/agent/collab_tools.go",
		"internal/agent/index_codebase_tool.go",
		"internal/agent/codebase_semantic_survey.go",
		"internal/agent/templates/orchestration/30_mail_protocol.md",
		"internal/memory/memory_md.go",
		"internal/memory/system.go":
		return true
	}
	for _, prefix := range []string{
		"internal/agent/mailbox/",
		"internal/orchestration/db/",
		"internal/agent/templates/",
	} {
		if strings.HasPrefix(rel, prefix) {
			return true
		}
	}
	return false
}

type memoryMarkdownSection struct {
	title string
	lines []string
}

func renderWorkstreamLines(state sessionStateSnapshot, progress []MemoryRecord) []string {
	var lines []string
	if state.CurrentTask != "" {
		lines = append(lines, fmt.Sprintf("- active_request: %s", state.CurrentTask))
	}
	for _, prompt := range state.RecentUserPrompts {
		if strings.TrimSpace(prompt) == "" || prompt == state.CurrentTask {
			continue
		}
		lines = append(lines, fmt.Sprintf("- adjacent_request: %s", prompt))
	}
	for _, record := range progress {
		var payload TaskProgress
		if err := json.Unmarshal([]byte(record.ContentJSON), &payload); err != nil {
			continue
		}
		if strings.TrimSpace(payload.CurrentStep) != "" {
			lines = append(lines, fmt.Sprintf("- current_step: %s", strings.TrimSpace(payload.CurrentStep)))
		}
		for _, step := range payload.CompletedSteps {
			step = strings.TrimSpace(step)
			if step != "" {
				lines = append(lines, fmt.Sprintf("- completed_step: %s", step))
			}
		}
		for _, step := range payload.NextSteps {
			step = strings.TrimSpace(step)
			if step != "" {
				lines = append(lines, fmt.Sprintf("- next_step: %s", step))
			}
		}
		for _, blocker := range payload.Blockers {
			blocker = strings.TrimSpace(blocker)
			if blocker != "" {
				lines = append(lines, fmt.Sprintf("- blocker: %s", blocker))
			}
		}
	}
	if len(lines) == 0 {
		return []string{"- no active workstreams recorded yet"}
	}
	return uniqueStrings(lines)
}

func renderDecisionLines(records []MemoryRecord, state sessionStateSnapshot) []string {
	var lines []string
	for _, record := range records {
		line := formatDecisionRecord(record.ContentJSON)
		if strings.TrimSpace(line) != "" {
			lines = append(lines, "- "+line)
		}
	}
	if len(lines) == 0 && strings.TrimSpace(state.LastDecision) != "" {
		lines = append(lines, "- "+strings.TrimSpace(state.LastDecision))
	}
	return uniqueStrings(lines)
}

func formatDecisionRecord(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	var payload ArchitecturalDecision
	if err := json.Unmarshal([]byte(trimmed), &payload); err == nil && strings.TrimSpace(payload.Decision) != "" {
		line := strings.TrimSpace(payload.Decision)
		if strings.TrimSpace(payload.Rationale) != "" {
			line += " | rationale: " + strings.TrimSpace(payload.Rationale)
		}
		if len(payload.FilesAffected) > 0 {
			line += " | files: " + strings.Join(uniqueStrings(payload.FilesAffected), ", ")
		}
		return line
	}
	var generic map[string]any
	if err := json.Unmarshal([]byte(trimmed), &generic); err == nil {
		for _, key := range []string{"decision", "value", "summary"} {
			if value := strings.TrimSpace(fmt.Sprint(generic[key])); value != "" && value != "<nil>" {
				return value
			}
		}
	}
	return trimmed
}

func renderGuardrailLines(failures, constraints []MemoryRecord, state sessionStateSnapshot) []string {
	var lines []string
	for _, record := range constraints {
		line := formatConstraintRecord(record.ContentJSON)
		if strings.TrimSpace(line) != "" {
			lines = append(lines, "- guardrail: "+line)
		}
	}
	for _, record := range failures {
		line := formatFailureRecord(record.ContentJSON)
		if strings.TrimSpace(line) != "" {
			lines = append(lines, "- failure_mode: "+line)
		}
	}
	for _, failure := range state.RecentFailures {
		if strings.TrimSpace(failure) != "" {
			lines = append(lines, "- failure_signal: "+strings.TrimSpace(failure))
		}
	}
	return uniqueStrings(lines)
}

func formatConstraintRecord(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	var payload NegativeConstraint
	if err := json.Unmarshal([]byte(trimmed), &payload); err == nil && strings.TrimSpace(payload.Constraint) != "" {
		line := strings.TrimSpace(payload.Constraint)
		if strings.TrimSpace(payload.Reason) != "" {
			line += " | reason: " + strings.TrimSpace(payload.Reason)
		}
		return line
	}
	return trimmed
}

func formatFailureRecord(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	var payload FailureEncountered
	if err := json.Unmarshal([]byte(trimmed), &payload); err == nil && strings.TrimSpace(payload.WhatFailed) != "" {
		line := strings.TrimSpace(payload.WhatFailed)
		if strings.TrimSpace(payload.RootCause) != "" {
			line += " | root_cause: " + strings.TrimSpace(payload.RootCause)
		}
		if strings.TrimSpace(payload.Resolution) != "" {
			line += " | resolution: " + strings.TrimSpace(payload.Resolution)
		}
		return line
	}
	return trimmed
}

func renderProvenanceLines(sessionID, projectRoot string, state sessionStateSnapshot, codebase memoryCodebaseSnapshot) []string {
	lines := []string{
		fmt.Sprintf("- handbook_scope: session=%s repo=%s", strings.TrimSpace(sessionID), strings.TrimSpace(projectRoot)),
		"- refresh_policy: rewrite on explicit force, major achievements, or long refresh gaps",
	}
	if len(state.AchievementSignals) > 0 {
		lines = append(lines, fmt.Sprintf("- latest_refresh_signals: %s", strings.Join(state.AchievementSignals, ", ")))
	}
	lines = append(lines, fmt.Sprintf("- codebase_snapshot_files: %d", codebase.TotalFiles))
	return lines
}

func splitMemorySections(content string) ([]string, []memoryMarkdownSection) {
	lines := strings.Split(content, "\n")
	var preamble []string
	var sections []memoryMarkdownSection
	current := memoryMarkdownSection{}
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			if current.title != "" {
				sections = append(sections, current)
			}
			current = memoryMarkdownSection{
				title: strings.TrimSpace(strings.TrimPrefix(line, "## ")),
				lines: []string{line},
			}
			continue
		}
		if current.title == "" {
			preamble = append(preamble, line)
			continue
		}
		current.lines = append(current.lines, line)
	}
	if current.title != "" {
		sections = append(sections, current)
	}
	return trimTrailingBlankLines(preamble), sections
}

func trimTrailingBlankLines(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func memorySectionOrderForStage(stage ContextLoadStage) []string {
	order := []string{
		"Session Snapshot",
		"Active Workstreams",
	}
	if stage >= ContextLoadStage20 {
		order = append(order,
			"Repo Constitution",
			"Stable Decisions",
		)
	}
	if stage >= ContextLoadStage30 {
		order = append(order,
			"Failures and Guardrails",
			"Architecture Overview",
		)
	}
	if stage >= ContextLoadStage40 {
		order = append(order,
			"AI Codebase Graph",
			"Critical Files",
			"Provenance",
		)
	}
	if stage >= ContextLoadStage50 {
		order = append(order[:len(order)-2],
			"Supporting Files",
			"Critical Files",
			"Provenance",
		)
	}
	return order
}

func trimMemorySectionForStage(title string, lines []string, stage ContextLoadStage) []string {
	maxBodyLines := 0
	switch title {
	case "Session Snapshot":
		maxBodyLines = 8
	case "Active Workstreams":
		maxBodyLines = 8
	case "Repo Constitution":
		maxBodyLines = 14
	case "Stable Decisions":
		if stage >= ContextLoadStage50 {
			maxBodyLines = 8
		} else {
			maxBodyLines = 6
		}
	case "Failures and Guardrails":
		if stage >= ContextLoadStage50 {
			maxBodyLines = 6
		} else {
			maxBodyLines = 4
		}
	case "Architecture Overview":
		maxBodyLines = 8
	case "AI Codebase Graph":
		if stage >= ContextLoadStage50 {
			maxBodyLines = 10
		} else {
			maxBodyLines = 8
		}
	case "Critical Files":
		if stage >= ContextLoadStage50 {
			maxBodyLines = 6
		} else {
			maxBodyLines = 4
		}
	case "Supporting Files":
		if stage >= ContextLoadStage50 {
			maxBodyLines = 6
		}
	case "Provenance":
		maxBodyLines = 4
	default:
		maxBodyLines = 8
	}
	return capMemorySectionBody(lines, maxBodyLines)
}

func capMemorySectionBody(lines []string, maxBodyLines int) []string {
	if len(lines) == 0 || maxBodyLines <= 0 {
		return lines
	}
	if len(lines) <= maxBodyLines+1 {
		return lines
	}
	trimmed := append([]string{}, lines[:maxBodyLines+1]...)
	omitted := len(lines) - (maxBodyLines + 1)
	if omitted > 0 {
		trimmed = append(trimmed, fmt.Sprintf("- ... %d more entries retained on disk.", omitted))
	}
	return trimmed
}
