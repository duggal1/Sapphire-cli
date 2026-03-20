package memory

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const memoryFileName = "memory.md"

type memoryFileManager struct {
	filePath    string
	projectRoot string

	mu               sync.Mutex
	lastRefreshTurn  map[string]uint64
	lastRefreshState map[string]string
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
	stateKey := strings.Join([]string{state.CurrentTask, state.LastDecision, state.LastModifiedFile}, "|")

	m.mu.Lock()
	shouldRefresh := force
	if !shouldRefresh {
		lastTurn := m.lastRefreshTurn[sessionID]
		lastState := m.lastRefreshState[sessionID]
		if lastTurn == 0 {
			shouldRefresh = true
		} else if currentTurn >= lastTurn+50 {
			shouldRefresh = true
		} else if stateKey != lastState && currentTurn > 0 {
			shouldRefresh = true
		}
	}
	m.mu.Unlock()

	if !shouldRefresh {
		return nil
	}

	content, err := m.buildContent(ctx, sessionID, state, store)
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

func (m *memoryFileManager) buildContent(ctx context.Context, sessionID string, state sessionStateSnapshot, store *Store) (string, error) {
	fileLines, totalFiles, err := m.buildCodebaseMap()
	if err != nil {
		return "", err
	}

	var latestDecision string
	if store != nil {
		if records, err := store.QueryRecordsBySession(ctx, sessionID, "architectural", 1); err == nil && len(records) > 0 {
			latestDecision = truncate(records[0].ContentJSON, 220)
		}
	}
	if latestDecision == "" {
		latestDecision = state.LastDecision
	}

	var lines []string
	lines = append(lines,
		"# Sapphire Memory",
		"",
		"Concise project and session memory. Use it as a map, not as the full transcript.",
		"",
		"## Session State",
	)
	lines = append(lines, fmt.Sprintf("- session: %s", strings.TrimSpace(sessionID)))
	if state.CurrentTask != "" {
		lines = append(lines, fmt.Sprintf("- current_task: %s", state.CurrentTask))
	}
	if latestDecision != "" {
		lines = append(lines, fmt.Sprintf("- last_decision: %s", latestDecision))
	}
	if state.LastModifiedFile != "" {
		lines = append(lines, fmt.Sprintf("- last_modified_file: %s", state.LastModifiedFile))
	}
	if len(state.RecentTools) > 0 {
		lines = append(lines, fmt.Sprintf("- recent_tools: %s", strings.Join(state.RecentTools, ", ")))
	}

	lines = append(lines, "", "## Codebase Map")
	lines = append(lines, fmt.Sprintf("- project_root: %s", m.projectRoot))
	lines = append(lines, fmt.Sprintf("- indexed_files: %d", totalFiles))
	lines = append(lines, fileLines...)

	return strings.TrimSpace(strings.Join(lines, "\n")) + "\n", nil
}

func (m *memoryFileManager) buildCodebaseMap() ([]string, int, error) {
	type fileSummary struct {
		path     string
		summary  string
		priority int
	}

	var files []fileSummary
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
		files = append(files, fileSummary{
			path:     rel,
			summary:  summary,
			priority: memoryFilePriority(rel),
		})
		return nil
	})
	if err != nil {
		return nil, 0, err
	}

	sort.Slice(files, func(i, j int) bool {
		if files[i].priority == files[j].priority {
			return files[i].path < files[j].path
		}
		return files[i].priority > files[j].priority
	})

	targetEntries := dynamicMemoryEntryBudget(len(files))
	lines := make([]string, 0, targetEntries+1)
	for i, file := range files {
		if i >= targetEntries {
			break
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", file.path, file.summary))
	}
	if omitted := len(files) - len(lines); omitted > 0 {
		lines = append(lines, fmt.Sprintf("- ... %d more files omitted to keep this memory concise.", omitted))
	}
	return lines, len(files), nil
}

func dynamicMemoryEntryBudget(fileCount int) int {
	switch {
	case fileCount <= 80:
		return fileCount
	case fileCount <= 250:
		return 110
	case fileCount <= 700:
		return 180
	case fileCount <= 1500:
		return 260
	default:
		return 340
	}
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
			return truncate(strings.TrimPrefix(line, "# "), 110), true
		case strings.HasPrefix(line, "//"):
			return truncate(strings.TrimSpace(strings.TrimPrefix(line, "//")), 110), true
		case strings.HasPrefix(line, "--"):
			return truncate(strings.TrimSpace(strings.TrimPrefix(line, "--")), 110), true
		case strings.HasPrefix(line, "/*") && strings.Contains(line, "*/"):
			return truncate(strings.TrimSuffix(strings.TrimPrefix(line, "/*"), "*/"), 110), true
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
