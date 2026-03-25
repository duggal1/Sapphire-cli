package memory

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	appdb "github.com/duggal1/Sapphire-cli/internal/db"
	"github.com/google/uuid"
)

type repoSnapshot struct {
	RepoRoot     string
	ScopePath    string
	Branch       string
	HeadCommit   string
	Dirty        bool
	ChangedFiles []string
}

type indexedRepoFile struct {
	Path         string
	AbsolutePath string
	Language     string
	Role         string
	Status       string
	Content      string
	ContentHash  string
	ModTimeUnix  int64
	SizeBytes    int64
	Imports      []string
	Facts        map[string]any
	Symbols      []indexedRepoSymbol
	Edges        []storedRepoEdge
}

type indexedRepoSymbol struct {
	StableKey   string
	Name        string
	Kind        string
	Signature   string
	Doc         string
	StartLine   int
	EndLine     int
	Exported    bool
	Status      string
	Fingerprint string
}

type repoFileCandidate struct {
	Path         string
	AbsolutePath string
	Language     string
	Role         string
	Status       string
	ModTimeUnix  int64
	SizeBytes    int64
}

var memoryAllowedExtensions = map[string]string{
	".go":   "go",
	".md":   "markdown",
	".sql":  "sql",
	".json": "json",
	".yaml": "yaml",
	".yml":  "yaml",
	".toml": "toml",
	".ts":   "typescript",
	".tsx":  "tsx",
	".js":   "javascript",
	".jsx":  "jsx",
	".sh":   "shell",
	".txt":  "text",
}

var memoryIgnoredDirs = map[string]struct{}{
	".git":         {},
	".sapphire":    {},
	"node_modules": {},
	"vendor":       {},
	"dist":         {},
	"build":        {},
	"coverage":     {},
	".next":        {},
	".turbo":       {},
	".idea":        {},
	".vscode":      {},
}

var memoryAllowedHiddenDirs = map[string]struct{}{
	".github": {},
	".husky":  {},
}

func (c *Compiler) ensureIndexedScope(ctx context.Context, workingDir string) (storedRepoScope, error) {
	snapshot, err := captureRepoSnapshot(ctx, workingDir)
	if err != nil {
		return storedRepoScope{}, err
	}
	candidates, err := scanRepoFileCandidates(snapshot.ScopePath)
	if err != nil {
		return storedRepoScope{}, err
	}
	existing, err := c.loadExistingScope(ctx, snapshot)
	if err != nil && !errors.Is(err, errNoScope) {
		return storedRepoScope{}, err
	}

	existingFiles := make(map[string]storedRepoFile)
	if existing.ID != "" {
		existingFiles, err = c.loadExistingFiles(ctx, existing.ID)
		if err != nil {
			return storedRepoScope{}, err
		}
	}
	existingSymbolsByFile := map[string][]appdb.MemoryRepoSymbol{}
	existingEdges := []appdb.MemoryRepoEdge{}
	if existing.ID != "" {
		if symbols, err := c.q.ListMemoryRepoSymbolsByScope(ctx, existing.ID); err == nil {
			for _, symbol := range symbols {
				existingSymbolsByFile[symbol.FileID] = append(existingSymbolsByFile[symbol.FileID], symbol)
			}
		}
		if edges, err := c.q.ListMemoryRepoEdgesByScope(ctx, existing.ID); err == nil {
			existingEdges = edges
		}
	}

	changedPaths := make([]string, 0, len(candidates))
	currentByPath := make(map[string]repoFileCandidate, len(candidates))
	for _, candidate := range candidates {
		currentByPath[candidate.Path] = candidate
		prev, ok := existingFiles[candidate.Path]
		if !ok || prev.ModTimeUnix != candidate.ModTimeUnix || prev.SizeBytes != candidate.SizeBytes || prev.Language != candidate.Language || prev.Role != candidate.Role || prev.Status != candidate.Status {
			changedPaths = append(changedPaths, candidate.Path)
		}
	}
	removedPaths := make([]string, 0)
	for path := range existingFiles {
		if _, ok := currentByPath[path]; !ok {
			removedPaths = append(removedPaths, path)
		}
	}
	sort.Strings(changedPaths)
	sort.Strings(removedPaths)

	scopeID := existing.ID
	if scopeID == "" {
		scopeID = uuid.NewString()
	}
	nowUnix := c.now().Unix()
	epoch := existing.LatestEpoch
	if epoch == 0 {
		epoch = 1
	} else if len(changedPaths) > 0 || len(removedPaths) > 0 || snapshot.HeadCommit != existing.HeadCommit || snapshot.Dirty != existing.Dirty {
		epoch++
	}

	tx, err := c.conn.BeginTx(ctx, nil)
	if err != nil {
		return storedRepoScope{}, err
	}
	defer tx.Rollback() //nolint:errcheck
	qtx := appdb.New(tx)

	if err := qtx.UpsertMemoryRepoScope(ctx, appdb.UpsertMemoryRepoScopeParams{
		ID:            scopeID,
		RepoRoot:      snapshot.RepoRoot,
		ScopePath:     snapshot.ScopePath,
		Branch:        snapshot.Branch,
		HeadCommit:    snapshot.HeadCommit,
		Dirty:         snapshot.Dirty,
		ChangedFiles:  snapshot.ChangedFiles,
		LatestEpoch:   epoch,
		LastIndexedAt: nowUnix,
		CreatedAt:     coalesceInt64(existing.LastIndexedAt, nowUnix),
		UpdatedAt:     nowUnix,
	}); err != nil {
		return storedRepoScope{}, err
	}

	if len(removedPaths) > 0 {
		for _, path := range removedPaths {
			if file, ok := existingFiles[path]; ok {
				_ = qtx.DeleteMemoryFactProvenanceByFact(ctx, "repo_file", file.ID)
				_ = qtx.DeleteMemoryFactProvenanceByFacts(ctx, "repo_symbol", extractMemorySymbolIDs(existingSymbolsByFile[file.ID]))
			}
			if err := qtx.DeleteMemoryRepoFileByScopeAndPath(ctx, scopeID, path); err != nil {
				return storedRepoScope{}, err
			}
		}
	}

	parsedChanged := make([]indexedRepoFile, 0, len(changedPaths))
	for _, path := range changedPaths {
		candidate := currentByPath[path]
		file, ok, err := loadIndexedRepoFile(snapshot.ScopePath, candidate.AbsolutePath)
		if err != nil {
			return storedRepoScope{}, err
		}
		if !ok {
			continue
		}
		parsedChanged = append(parsedChanged, file)
	}

	if len(parsedChanged) > 0 || existing.ID == "" {
		for _, file := range parsedChanged {
			fileID := existingFiles[file.Path].ID
			if fileID == "" {
				fileID = uuid.NewString()
			}
			if err := qtx.UpsertMemoryRepoFile(ctx, appdb.UpsertMemoryRepoFileParams{
				ID:          fileID,
				ScopeID:     scopeID,
				Path:        file.Path,
				Language:    file.Language,
				Role:        file.Role,
				Status:      file.Status,
				ContentHash: file.ContentHash,
				ModTimeUnix: file.ModTimeUnix,
				SizeBytes:   file.SizeBytes,
				SymbolCount: len(file.Symbols),
				Imports:     file.Imports,
				Facts:       file.Facts,
				UpdatedAt:   nowUnix,
				CreatedAt:   nowUnix,
			}); err != nil {
				return storedRepoScope{}, err
			}
			_ = qtx.DeleteMemoryFactProvenanceByFact(ctx, "repo_file", fileID)
			if err := qtx.DeleteMemoryRepoSymbolsByFile(ctx, scopeID, fileID); err != nil {
				return storedRepoScope{}, err
			}
			_ = qtx.DeleteMemoryFactProvenanceByFacts(ctx, "repo_symbol", extractMemorySymbolIDs(existingSymbolsByFile[fileID]))
			fileProvID, err := c.createTxProvenance(ctx, qtx, appdb.InsertMemoryProvenanceParams{
				ID:          uuid.NewString(),
				RepoScopeID: scopeID,
				SourceKind:  "repo_file",
				FilePath:    file.Path,
				HeadCommit:  snapshot.HeadCommit,
				IndexEpoch:  epoch,
				Metadata: map[string]any{
					"content_hash": file.ContentHash,
					"mod_time":     file.ModTimeUnix,
				},
				CreatedAt: nowUnix,
			})
			if err == nil {
				_ = qtx.LinkMemoryFactProvenance(ctx, appdb.LinkMemoryFactProvenanceParams{
					FactKind:     "repo_file",
					FactID:       fileID,
					ProvenanceID: fileProvID,
					CreatedAt:    nowUnix,
				})
			}
			for _, symbol := range file.Symbols {
				symbolID := hashText(scopeID, symbol.StableKey)
				if err := qtx.InsertMemoryRepoSymbol(ctx, appdb.InsertMemoryRepoSymbolParams{
					ID:          symbolID,
					ScopeID:     scopeID,
					FileID:      fileID,
					StableKey:   symbol.StableKey,
					Name:        symbol.Name,
					Kind:        symbol.Kind,
					Signature:   symbol.Signature,
					Doc:         symbol.Doc,
					StartLine:   symbol.StartLine,
					EndLine:     symbol.EndLine,
					Exported:    symbol.Exported,
					Status:      symbol.Status,
					Fingerprint: symbol.Fingerprint,
					UpdatedAt:   nowUnix,
					CreatedAt:   nowUnix,
				}); err != nil {
					return storedRepoScope{}, err
				}
				if fileProvID != "" {
					_ = qtx.LinkMemoryFactProvenance(ctx, appdb.LinkMemoryFactProvenanceParams{
						FactKind:     "repo_symbol",
						FactID:       symbolID,
						ProvenanceID: fileProvID,
						CreatedAt:    nowUnix,
					})
				}
			}
		}
	}

	impactedPaths := append(append([]string{}, changedPaths...), removedPaths...)
	if err := qtx.DeleteMemoryRepoEdgesForPaths(ctx, scopeID, impactedPaths); err != nil {
		return storedRepoScope{}, err
	}
	_ = qtx.DeleteMemoryFactProvenanceByFacts(ctx, "repo_edge", extractMemoryEdgeIDs(existingEdges, impactedPaths))
	for _, file := range parsedChanged {
		for _, edge := range file.Edges {
			edgeID := hashText(scopeID, edge.FromFile, edge.FromSymbol, edge.Type, edge.ToFile, edge.ToSymbol, edge.ToSymbolKey)
			if err := qtx.InsertMemoryRepoEdge(ctx, appdb.InsertMemoryRepoEdgeParams{
				ID:          edgeID,
				ScopeID:     scopeID,
				FromFile:    edge.FromFile,
				FromSymbol:  edge.FromSymbol,
				EdgeType:    edge.Type,
				ToFile:      edge.ToFile,
				ToSymbol:    edge.ToSymbol,
				ToSymbolKey: edge.ToSymbolKey,
				Metadata:    edge.Metadata,
				UpdatedAt:   nowUnix,
				CreatedAt:   nowUnix,
			}); err != nil {
				return storedRepoScope{}, err
			}
			fileProvID, _ := c.createTxProvenance(ctx, qtx, appdb.InsertMemoryProvenanceParams{
				ID:          uuid.NewString(),
				RepoScopeID: scopeID,
				SourceKind:  "repo_edge",
				FilePath:    edge.FromFile,
				SymbolKey:   edge.FromSymbol,
				HeadCommit:  snapshot.HeadCommit,
				IndexEpoch:  epoch,
				Metadata:    edge.Metadata,
				CreatedAt:   nowUnix,
			})
			if fileProvID != "" {
				_ = qtx.LinkMemoryFactProvenance(ctx, appdb.LinkMemoryFactProvenanceParams{
					FactKind:     "repo_edge",
					FactID:       edgeID,
					ProvenanceID: fileProvID,
					CreatedAt:    nowUnix,
				})
			}
		}
	}
	if err := qtx.RelinkMemoryRepoEdgesToChangedFiles(ctx, scopeID, changedPaths); err != nil {
		return storedRepoScope{}, err
	}

	if len(changedPaths) > 0 || len(removedPaths) > 0 || existing.ID == "" || snapshot.HeadCommit != existing.HeadCommit || snapshot.Dirty != existing.Dirty {
		if err := qtx.InsertMemoryIndexEpoch(ctx, appdb.InsertMemoryIndexEpochParams{
			ID:           uuid.NewString(),
			ScopeID:      scopeID,
			Epoch:        epoch,
			HeadCommit:   snapshot.HeadCommit,
			ChangedFiles: changedPaths,
			RemovedFiles: removedPaths,
			FileCount:    len(candidates),
			Status:       "ready",
			CreatedAt:    nowUnix,
			CompletedAt:  nowUnix,
		}); err != nil {
			return storedRepoScope{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return storedRepoScope{}, err
	}
	return storedRepoScope{
		ID:            scopeID,
		RepoRoot:      snapshot.RepoRoot,
		ScopePath:     snapshot.ScopePath,
		Branch:        snapshot.Branch,
		HeadCommit:    snapshot.HeadCommit,
		Dirty:         snapshot.Dirty,
		ChangedFiles:  snapshot.ChangedFiles,
		LatestEpoch:   epoch,
		LastIndexedAt: nowUnix,
	}, nil
}

func (c *Compiler) loadExistingScope(ctx context.Context, snapshot repoSnapshot) (storedRepoScope, error) {
	item, err := c.q.GetMemoryRepoScope(ctx, snapshot.RepoRoot, snapshot.ScopePath, snapshot.Branch)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return storedRepoScope{}, errNoScope
		}
		return storedRepoScope{}, err
	}
	return storedRepoScope{
		ID:            item.ID,
		RepoRoot:      item.RepoRoot,
		ScopePath:     item.ScopePath,
		Branch:        item.Branch,
		HeadCommit:    item.HeadCommit,
		Dirty:         item.Dirty,
		ChangedFiles:  item.ChangedFiles,
		LatestEpoch:   item.LatestEpoch,
		LastIndexedAt: item.LastIndexedAt,
	}, nil
}

func (c *Compiler) loadExistingFiles(ctx context.Context, scopeID string) (map[string]storedRepoFile, error) {
	rows, err := c.q.ListMemoryRepoFilesByScope(ctx, scopeID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]storedRepoFile, len(rows))
	for _, item := range rows {
		out[item.Path] = storedRepoFile{
			ID:          item.ID,
			ScopeID:     item.ScopeID,
			Path:        item.Path,
			Language:    item.Language,
			Role:        item.Role,
			Status:      item.Status,
			ContentHash: item.ContentHash,
			ModTimeUnix: item.ModTimeUnix,
			SizeBytes:   item.SizeBytes,
			SymbolCount: int(item.SymbolCount),
			Imports:     item.Imports,
			Facts:       item.Facts,
		}
	}
	return out, nil
}

func (c *Compiler) loadScopeGraph(ctx context.Context, scopeID string) (compiledGraph, error) {
	graph := compiledGraph{
		files:       map[string]storedRepoFile{},
		symbols:     map[string]storedRepoSymbol{},
		symbolsBy:   map[string][]storedRepoSymbol{},
		fileSymbols: map[string][]storedRepoSymbol{},
	}

	fileRows, err := c.q.ListMemoryRepoFilesByScope(ctx, scopeID)
	if err != nil {
		return graph, err
	}
	fileByID := make(map[string]string)
	for _, item := range fileRows {
		file := storedRepoFile{
			ID:          item.ID,
			ScopeID:     item.ScopeID,
			Path:        item.Path,
			Language:    item.Language,
			Role:        item.Role,
			Status:      item.Status,
			ContentHash: item.ContentHash,
			ModTimeUnix: item.ModTimeUnix,
			SizeBytes:   item.SizeBytes,
			SymbolCount: int(item.SymbolCount),
			Imports:     item.Imports,
			Facts:       item.Facts,
		}
		graph.files[file.Path] = file
		fileByID[file.ID] = file.Path
	}

	symbolRows, err := c.q.ListMemoryRepoSymbolsByScope(ctx, scopeID)
	if err != nil {
		return graph, err
	}
	for _, item := range symbolRows {
		symbol := storedRepoSymbol{
			ID:          item.ID,
			ScopeID:     item.ScopeID,
			FileID:      item.FileID,
			StableKey:   item.StableKey,
			Name:        item.Name,
			Kind:        item.Kind,
			Signature:   item.Signature,
			Doc:         item.Doc,
			StartLine:   int(item.StartLine),
			EndLine:     int(item.EndLine),
			Exported:    item.Exported,
			Status:      item.Status,
			Fingerprint: item.Fingerprint,
		}
		symbol.FilePath = fileByID[symbol.FileID]
		graph.symbols[symbol.StableKey] = symbol
		graph.symbolsBy[strings.ToLower(symbol.Name)] = append(graph.symbolsBy[strings.ToLower(symbol.Name)], symbol)
		graph.fileSymbols[symbol.FilePath] = append(graph.fileSymbols[symbol.FilePath], symbol)
	}

	edgeRows, err := c.q.ListMemoryRepoEdgesByScope(ctx, scopeID)
	if err != nil {
		return graph, err
	}
	for _, item := range edgeRows {
		edge := storedRepoEdge{
			Type:        item.EdgeType,
			FromFile:    item.FromFile,
			FromSymbol:  item.FromSymbol,
			ToFile:      item.ToFile,
			ToSymbol:    item.ToSymbol,
			ToSymbolKey: item.ToSymbolKey,
			Metadata:    item.Metadata,
		}
		graph.edges = append(graph.edges, edge)
	}
	return graph, nil
}

func captureRepoSnapshot(ctx context.Context, workingDir string) (repoSnapshot, error) {
	scopePath := strings.TrimSpace(workingDir)
	if scopePath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return repoSnapshot{}, err
		}
		scopePath = cwd
	}
	scopePath, _ = filepath.Abs(scopePath)
	scopePath = filepath.Clean(scopePath)

	repoRoot := gitOutput(ctx, scopePath, "rev-parse", "--show-toplevel")
	if strings.TrimSpace(repoRoot) == "" {
		repoRoot = scopePath
	}
	repoRoot = filepath.Clean(strings.TrimSpace(repoRoot))
	branch := strings.TrimSpace(gitOutput(ctx, scopePath, "rev-parse", "--abbrev-ref", "HEAD"))
	headCommit := strings.TrimSpace(gitOutput(ctx, scopePath, "rev-parse", "HEAD"))
	status := strings.TrimSpace(gitOutput(ctx, scopePath, "status", "--porcelain"))
	var changed []string
	if status != "" {
		for _, line := range strings.Split(status, "\n") {
			line = strings.TrimSpace(line)
			if len(line) < 4 {
				continue
			}
			path := strings.TrimSpace(line[3:])
			if idx := strings.Index(path, " -> "); idx >= 0 {
				path = path[idx+4:]
			}
			if path != "" {
				changed = append(changed, filepath.ToSlash(path))
			}
		}
	}
	return repoSnapshot{
		RepoRoot:     repoRoot,
		ScopePath:    scopePath,
		Branch:       branch,
		HeadCommit:   headCommit,
		Dirty:        status != "",
		ChangedFiles: uniqueSortedStrings(changed),
	}, nil
}

func scanRepoFileCandidates(root string) ([]repoFileCandidate, error) {
	paths := make([]repoFileCandidate, 0, 512)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		name := d.Name()
		if d.IsDir() {
			if _, skip := memoryIgnoredDirs[name]; skip {
				return filepath.SkipDir
			}
			if strings.HasPrefix(name, ".") && path != root {
				if _, allow := memoryAllowedHiddenDirs[name]; allow {
					return nil
				}
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if _, ok := memoryAllowedExtensions[ext]; !ok && name != "AGENTS.md" && name != "agent.md" {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() == 0 || info.Size() > 2*1024*1024 {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		paths = append(paths, repoFileCandidate{
			Path:         rel,
			AbsolutePath: path,
			Language:     memoryDetectLanguage(path),
			Role:         inferFileRole(rel),
			Status:       inferPathStatus(rel),
			ModTimeUnix:  info.ModTime().Unix(),
			SizeBytes:    info.Size(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(paths, func(i, j int) bool { return paths[i].Path < paths[j].Path })
	return paths, nil
}

func gitOutput(ctx context.Context, dir string, args ...string) string {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func discoverRepoFiles(root string) ([]indexedRepoFile, error) {
	paths := make([]string, 0, 512)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		name := d.Name()
		if d.IsDir() {
			if _, skip := memoryIgnoredDirs[name]; skip {
				return filepath.SkipDir
			}
			if strings.HasPrefix(name, ".") && path != root {
				if _, allow := memoryAllowedHiddenDirs[name]; allow {
					return nil
				}
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if _, ok := memoryAllowedExtensions[ext]; !ok && name != "AGENTS.md" && name != "agent.md" {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	out := make([]indexedRepoFile, 0, len(paths))
	for _, path := range paths {
		file, ok, err := loadIndexedRepoFile(root, path)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, file)
		}
	}
	return out, nil
}

func (c *Compiler) createTxProvenance(ctx context.Context, qtx *appdb.Queries, arg appdb.InsertMemoryProvenanceParams) (string, error) {
	if strings.TrimSpace(arg.ID) == "" {
		arg.ID = uuid.NewString()
	}
	if err := qtx.InsertMemoryProvenance(ctx, arg); err != nil {
		return "", err
	}
	return arg.ID, nil
}

func extractMemorySymbolIDs(items []appdb.MemoryRepoSymbol) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.ID)
	}
	return out
}

func extractMemoryEdgeIDs(items []appdb.MemoryRepoEdge, impactedPaths []string) []string {
	impacted := make(map[string]struct{}, len(impactedPaths))
	for _, path := range impactedPaths {
		impacted[path] = struct{}{}
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if _, ok := impacted[item.FromFile]; ok {
			out = append(out, item.ID)
			continue
		}
		if _, ok := impacted[item.ToFile]; ok {
			out = append(out, item.ID)
		}
	}
	return out
}

func inferPathStatus(path string) string {
	path = strings.ToLower(filepath.ToSlash(path))
	if strings.Contains(path, "deprecated") || strings.Contains(path, "legacy") || strings.Contains(path, "archive") {
		return "deprecated"
	}
	return "active"
}

func loadIndexedRepoFile(root, path string) (indexedRepoFile, bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return indexedRepoFile{}, false, err
	}
	if info.Size() == 0 || info.Size() > 2*1024*1024 {
		return indexedRepoFile{}, false, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return indexedRepoFile{}, false, err
	}
	if !memoryIsTextFile(raw) {
		return indexedRepoFile{}, false, nil
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return indexedRepoFile{}, false, err
	}
	rel = filepath.ToSlash(rel)
	file := indexedRepoFile{
		Path:         rel,
		AbsolutePath: path,
		Language:     memoryDetectLanguage(path),
		Role:         inferFileRole(rel),
		Status:       inferFileStatus(rel, string(raw)),
		Content:      sanitizeText(raw),
		ContentHash:  hashBytes(raw),
		ModTimeUnix:  info.ModTime().Unix(),
		SizeBytes:    info.Size(),
		Facts: map[string]any{
			"line_count": strings.Count(string(raw), "\n") + 1,
		},
	}
	if file.Language == "go" {
		extractGoFacts(&file)
	}
	if strings.HasSuffix(file.Path, "go.mod") {
		file.Facts["module"] = firstLineWithPrefix(file.Content, "module ")
	}
	return file, true, nil
}

func extractGoFacts(file *indexedRepoFile) {
	if file == nil || file.Language != "go" {
		return
	}
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file.Path, file.Content, parser.ParseComments)
	if err != nil {
		file.Facts["parse_error"] = err.Error()
		return
	}
	lines := strings.Split(file.Content, "\n")
	file.Facts["package"] = parsed.Name.Name
	imports := make([]string, 0, len(parsed.Imports))
	nameToKey := make(map[string]string)

	for _, imp := range parsed.Imports {
		pathValue := strings.Trim(imp.Path.Value, `"`)
		imports = append(imports, pathValue)
		file.Edges = append(file.Edges, storedRepoEdge{
			Type:     "imports",
			FromFile: file.Path,
			ToFile:   pathValue,
			Metadata: map[string]any{"import": pathValue},
		})
	}
	file.Imports = uniqueSortedStrings(imports)

	ast.Inspect(parsed, func(node ast.Node) bool {
		switch decl := node.(type) {
		case *ast.FuncDecl:
			start := fset.Position(decl.Pos()).Line
			end := fset.Position(decl.End()).Line
			kind := "function"
			receiver := ""
			if decl.Recv != nil && len(decl.Recv.List) > 0 {
				kind = "method"
				receiver = receiverName(decl.Recv.List[0].Type)
			}
			stableKey := stableSymbolKey(file.Path, receiver, decl.Name.Name, kind)
			signature := renderFuncSignature(decl)
			doc := ""
			if decl.Doc != nil {
				doc = strings.TrimSpace(decl.Doc.Text())
			}
			status := "active"
			if strings.HasPrefix(doc, "Deprecated:") {
				status = "deprecated"
			}
			body := sliceLines(lines, start, end)
			file.Symbols = append(file.Symbols, indexedRepoSymbol{
				StableKey:   stableKey,
				Name:        decl.Name.Name,
				Kind:        kind,
				Signature:   signature,
				Doc:         compactText(doc, 240),
				StartLine:   start,
				EndLine:     end,
				Exported:    ast.IsExported(decl.Name.Name),
				Status:      status,
				Fingerprint: hashText(body),
			})
			nameToKey[decl.Name.Name] = stableKey
			file.Edges = append(file.Edges, storedRepoEdge{
				Type:        "defines",
				FromFile:    file.Path,
				ToSymbol:    decl.Name.Name,
				ToSymbolKey: stableKey,
				Metadata:    map[string]any{"evidence": fmt.Sprintf("%s:%d-%d", file.Path, start, end)},
			})
			if strings.HasSuffix(file.Path, "_test.go") && strings.HasPrefix(decl.Name.Name, "Test") {
				target := strings.TrimPrefix(decl.Name.Name, "Test")
				if target != "" {
					file.Edges = append(file.Edges, storedRepoEdge{
						Type:       "test_covers",
						FromFile:   file.Path,
						FromSymbol: stableKey,
						ToSymbol:   target,
						Metadata:   map[string]any{"evidence": decl.Name.Name},
					})
				}
			}
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				switch typed := spec.(type) {
				case *ast.TypeSpec:
					start := fset.Position(typed.Pos()).Line
					end := fset.Position(typed.End()).Line
					kind := "type"
					stableKey := stableSymbolKey(file.Path, "", typed.Name.Name, kind)
					doc := ""
					if decl.Doc != nil {
						doc = strings.TrimSpace(decl.Doc.Text())
					}
					status := "active"
					if strings.HasPrefix(doc, "Deprecated:") {
						status = "deprecated"
					}
					file.Symbols = append(file.Symbols, indexedRepoSymbol{
						StableKey:   stableKey,
						Name:        typed.Name.Name,
						Kind:        kind,
						Signature:   renderTypeSignature(typed),
						Doc:         compactText(doc, 240),
						StartLine:   start,
						EndLine:     end,
						Exported:    ast.IsExported(typed.Name.Name),
						Status:      status,
						Fingerprint: hashText(sliceLines(lines, start, end)),
					})
					nameToKey[typed.Name.Name] = stableKey
					file.Edges = append(file.Edges, storedRepoEdge{
						Type:        "defines",
						FromFile:    file.Path,
						ToSymbol:    typed.Name.Name,
						ToSymbolKey: stableKey,
						Metadata:    map[string]any{"evidence": fmt.Sprintf("%s:%d-%d", file.Path, start, end)},
					})
					switch typed.Type.(type) {
					case *ast.InterfaceType:
						file.Facts["has_interfaces"] = true
					}
				case *ast.ValueSpec:
					for _, name := range typed.Names {
						start := fset.Position(name.Pos()).Line
						stableKey := stableSymbolKey(file.Path, "", name.Name, strings.ToLower(decl.Tok.String()))
						file.Symbols = append(file.Symbols, indexedRepoSymbol{
							StableKey:   stableKey,
							Name:        name.Name,
							Kind:        strings.ToLower(decl.Tok.String()),
							StartLine:   start,
							EndLine:     start,
							Exported:    ast.IsExported(name.Name),
							Status:      "active",
							Fingerprint: hashText(name.Name, strconv.Itoa(start)),
						})
						nameToKey[name.Name] = stableKey
					}
				}
			}
		}
		return true
	})

	for _, symbol := range file.Symbols {
		for _, call := range discoverCallsForSymbol(parsed, fset, symbol) {
			edge := storedRepoEdge{
				Type:       "calls",
				FromFile:   file.Path,
				FromSymbol: symbol.StableKey,
				ToSymbol:   call,
				Metadata:   map[string]any{"evidence": call},
			}
			if key, ok := nameToKey[call]; ok {
				edge.ToSymbolKey = key
			}
			file.Edges = append(file.Edges, edge)
		}
	}
}

func discoverCallsForSymbol(file *ast.File, fset *token.FileSet, symbol indexedRepoSymbol) []string {
	calls := make([]string, 0, 8)
	seen := make(map[string]struct{})
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		pos := fset.Position(call.Pos()).Line
		if pos < symbol.StartLine || pos > symbol.EndLine {
			return true
		}
		name := calledFunctionName(call.Fun)
		if name == "" {
			return true
		}
		if _, ok := seen[name]; ok {
			return true
		}
		seen[name] = struct{}{}
		calls = append(calls, name)
		return true
	})
	sort.Strings(calls)
	return calls
}

func calledFunctionName(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.SelectorExpr:
		return typed.Sel.Name
	default:
		return ""
	}
}

func renderFuncSignature(decl *ast.FuncDecl) string {
	if decl == nil || decl.Type == nil {
		return ""
	}
	var parts []string
	if decl.Recv != nil && len(decl.Recv.List) > 0 {
		parts = append(parts, "method")
	}
	params := make([]string, 0, len(decl.Type.Params.List))
	if decl.Type.Params != nil {
		for _, item := range decl.Type.Params.List {
			typeText := renderExpr(item.Type)
			if len(item.Names) == 0 {
				params = append(params, typeText)
				continue
			}
			for _, name := range item.Names {
				params = append(params, name.Name+" "+typeText)
			}
		}
	}
	results := make([]string, 0)
	if decl.Type.Results != nil {
		for _, item := range decl.Type.Results.List {
			typeText := renderExpr(item.Type)
			if len(item.Names) == 0 {
				results = append(results, typeText)
				continue
			}
			for _, name := range item.Names {
				results = append(results, name.Name+" "+typeText)
			}
		}
	}
	signature := decl.Name.Name + "(" + strings.Join(params, ", ") + ")"
	if len(results) > 0 {
		signature += " (" + strings.Join(results, ", ") + ")"
	}
	if len(parts) > 0 {
		return strings.Join(parts, " ") + " " + signature
	}
	return signature
}

func renderTypeSignature(spec *ast.TypeSpec) string {
	if spec == nil {
		return ""
	}
	return spec.Name.Name + " " + renderExpr(spec.Type)
}

func renderExpr(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.SelectorExpr:
		return renderExpr(typed.X) + "." + typed.Sel.Name
	case *ast.StarExpr:
		return "*" + renderExpr(typed.X)
	case *ast.ArrayType:
		return "[]" + renderExpr(typed.Elt)
	case *ast.InterfaceType:
		return "interface"
	case *ast.StructType:
		return "struct"
	case *ast.MapType:
		return "map[" + renderExpr(typed.Key) + "]" + renderExpr(typed.Value)
	case *ast.FuncType:
		return "func"
	case *ast.IndexExpr:
		return renderExpr(typed.X)
	default:
		return ""
	}
}

func receiverName(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.StarExpr:
		return receiverName(typed.X)
	default:
		return ""
	}
}

func stableSymbolKey(path, receiver, name, kind string) string {
	parts := []string{filepath.ToSlash(path), kind}
	if receiver != "" {
		parts = append(parts, receiver)
	}
	parts = append(parts, name)
	return strings.Join(parts, "::")
}

func inferFileRole(path string) string {
	path = filepath.ToSlash(path)
	switch {
	case strings.HasSuffix(path, "_test.go"):
		return "test"
	case strings.Contains(path, "/cmd/"):
		return "entrypoint"
	case strings.Contains(path, "/config/"), strings.HasSuffix(path, ".json"), strings.HasSuffix(path, ".yaml"), strings.HasSuffix(path, ".yml"), strings.HasSuffix(path, ".toml"):
		return "config"
	case strings.Contains(path, "/internal/agent/"):
		return "orchestration"
	case strings.Contains(path, "/internal/ui/"):
		return "ui"
	case strings.HasSuffix(path, "AGENTS.md") || strings.HasSuffix(path, "agent.md"):
		return "docs"
	default:
		return "code"
	}
}

func inferFileStatus(path, content string) string {
	path = strings.ToLower(filepath.ToSlash(path))
	if strings.Contains(path, "deprecated") || strings.Contains(path, "legacy") || strings.Contains(path, "archive") || strings.Contains(content, "Deprecated:") {
		return "deprecated"
	}
	return "active"
}

func memoryDetectLanguage(path string) string {
	if filepath.Base(path) == "AGENTS.md" || filepath.Base(path) == "agent.md" {
		return "markdown"
	}
	if lang, ok := memoryAllowedExtensions[strings.ToLower(filepath.Ext(path))]; ok {
		return lang
	}
	return "text"
}

func memoryIsTextFile(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return false
	}
	return utf8.Valid(data)
}

func sanitizeText(raw []byte) string {
	return strings.ReplaceAll(string(raw), "\r\n", "\n")
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func sliceLines(lines []string, start, end int) string {
	if start <= 0 || end < start || start > len(lines) {
		return ""
	}
	if end > len(lines) {
		end = len(lines)
	}
	return strings.Join(lines[start-1:end], "\n")
}

func firstLineWithPrefix(content, prefix string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func coalesceInt64(value, fallback int64) int64 {
	if value != 0 {
		return value
	}
	return fallback
}
