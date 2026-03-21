package codeindex

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
)

const (
	singleFileChunkBytes = 12 * 1024
	maxChunkBytes        = 6400
	minChunkBytes        = 1200
	maxSnippetRunes      = 220
	maxBatchEmbeds       = 64
)

type discoveredFile struct {
	AbsolutePath string
	RelativePath string
	Language     string
	Content      string
	ContentHash  string
	ModTimeUnix  int64
	Size         int64
}

type indexedChunk struct {
	ID            string
	Path          string
	Language      string
	ChunkIndex    int
	Kind          string
	StartLine     int
	EndLine       int
	Content       string
	SearchText    string
	ContentHash   string
	TokenEstimate int
	Embedding     []float32
}

type indexedFile struct {
	Path        string
	Language    string
	ContentHash string
	ModTimeUnix int64
	Size        int64
	NeedsDelete bool
	Chunks      []indexedChunk
}

var allowedFileExtensions = map[string]string{
	".go":    "go",
	".ts":    "typescript",
	".tsx":   "tsx",
	".js":    "javascript",
	".jsx":   "jsx",
	".mjs":   "javascript",
	".cjs":   "javascript",
	".py":    "python",
	".rs":    "rust",
	".java":  "java",
	".kt":    "kotlin",
	".rb":    "ruby",
	".php":   "php",
	".swift": "swift",
	".css":   "css",
	".scss":  "scss",
	".html":  "html",
	".json":  "json",
	".yaml":  "yaml",
	".yml":   "yaml",
	".toml":  "toml",
	".md":    "markdown",
	".txt":   "text",
	".sql":   "sql",
	".sh":    "shell",
}

var allowedNamedFiles = map[string]string{
	"dockerfile":        "docker",
	"makefile":          "make",
	"go.mod":            "gomod",
	"go.sum":            "gosum",
	"readme":            "markdown",
	"readme.md":         "markdown",
	"package.json":      "json",
	"package-lock.json": "json",
	"tsconfig.json":     "json",
	".gitignore":        "text",
	".editorconfig":     "text",
	".npmrc":            "text",
	".env.example":      "text",
}

var ignoredDirs = map[string]struct{}{
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

var allowedHiddenDirs = map[string]struct{}{
	".github": {},
	".husky":  {},
}

func discoverFiles(root string) ([]discoveredFile, error) {
	paths := make([]string, 0, 256)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		name := d.Name()
		if d.IsDir() {
			if _, skip := ignoredDirs[name]; skip {
				return filepath.SkipDir
			}
			if strings.HasPrefix(name, ".") && path != root {
				if _, allow := allowedHiddenDirs[name]; allow {
					return nil
				}
				return filepath.SkipDir
			}
			return nil
		}
		if !shouldIndexPath(path, name) {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	files, err := loadDiscoveredFiles(root, paths)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(files, func(a, b discoveredFile) int {
		return strings.Compare(a.RelativePath, b.RelativePath)
	})
	return files, nil
}

func loadDiscoveredFiles(root string, paths []string) ([]discoveredFile, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	workers := discoveryWorkerCount(len(paths))
	type result struct {
		file discoveredFile
		err  error
	}
	workCh := make(chan string)
	resultCh := make(chan result, len(paths))
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range workCh {
				file, err := loadDiscoveredFile(root, path)
				resultCh <- result{file: file, err: err}
			}
		}()
	}
	for _, path := range paths {
		workCh <- path
	}
	close(workCh)
	wg.Wait()
	close(resultCh)

	files := make([]discoveredFile, 0, len(paths))
	for result := range resultCh {
		if result.err != nil {
			return nil, result.err
		}
		if result.file.RelativePath == "" {
			continue
		}
		files = append(files, result.file)
	}
	return files, nil
}

func loadDiscoveredFile(root, path string) (discoveredFile, error) {
	info, err := os.Stat(path)
	if err != nil {
		return discoveredFile{}, err
	}
	if info.Size() == 0 || info.Size() > 2*1024*1024 {
		return discoveredFile{}, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return discoveredFile{}, err
	}
	if !isTextFile(raw) {
		return discoveredFile{}, nil
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return discoveredFile{}, err
	}
	rel = filepath.ToSlash(rel)
	name := filepath.Base(path)
	return discoveredFile{
		AbsolutePath: path,
		RelativePath: rel,
		Language:     detectLanguage(path, name),
		Content:      string(raw),
		ContentHash:  hashBytes(raw),
		ModTimeUnix:  info.ModTime().Unix(),
		Size:         info.Size(),
	}, nil
}

func shouldIndexPath(path, name string) bool {
	if _, ok := allowedNamedFiles[strings.ToLower(name)]; ok {
		return true
	}
	if strings.HasPrefix(name, ".") && filepath.Ext(name) == "" {
		return false
	}
	ext := strings.ToLower(filepath.Ext(path))
	_, ok := allowedFileExtensions[ext]
	return ok
}

func detectLanguage(path, name string) string {
	if lang, ok := allowedNamedFiles[strings.ToLower(name)]; ok {
		return lang
	}
	if lang, ok := allowedFileExtensions[strings.ToLower(filepath.Ext(path))]; ok {
		return lang
	}
	return "text"
}

func isTextFile(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return false
	}
	return true
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func buildIndexedFile(file discoveredFile) indexedFile {
	chunks := chunkFile(file)
	return indexedFile{
		Path:        file.RelativePath,
		Language:    file.Language,
		ContentHash: file.ContentHash,
		ModTimeUnix: file.ModTimeUnix,
		Size:        file.Size,
		Chunks:      chunks,
	}
}

func chunkFile(file discoveredFile) []indexedChunk {
	if shouldUseSingleChunk(file) {
		return []indexedChunk{newChunk(file, 0, "file", 1, max(1, len(strings.Split(file.Content, "\n"))), filepath.Base(file.RelativePath), strings.TrimSpace(file.Content))}
	}
	if file.Language == "go" {
		if chunks := chunkGoFile(file); len(chunks) > 0 {
			return chunks
		}
	}
	return chunkTextFile(file)
}

func shouldUseSingleChunk(file discoveredFile) bool {
	if file.Size <= 0 {
		return false
	}
	if file.Size > singleFileChunkBytes {
		return false
	}
	return estimateTokens(file.Content) <= 2200
}

func chunkGoFile(file discoveredFile) []indexedChunk {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file.RelativePath, file.Content, parser.ParseComments)
	if err != nil {
		return nil
	}
	lines := strings.Split(file.Content, "\n")
	chunks := make([]indexedChunk, 0, len(parsed.Decls))
	for idx, decl := range parsed.Decls {
		if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.IMPORT {
			continue
		}
		start := fset.Position(decl.Pos()).Line
		end := fset.Position(decl.End()).Line
		if start <= 0 || end < start || start > len(lines) {
			continue
		}
		if end > len(lines) {
			end = len(lines)
		}
		text := strings.Join(lines[start-1:end], "\n")
		text = strings.TrimSpace(text)
		if text == "" || len(text) < 120 {
			continue
		}
		kind, name := describeGoDecl(decl)
		chunks = append(chunks, newChunk(file, idx, kind, start, end, name, text))
	}
	if len(chunks) == 0 {
		return nil
	}
	return chunks
}

func describeGoDecl(decl ast.Decl) (string, string) {
	switch node := decl.(type) {
	case *ast.FuncDecl:
		if node.Recv != nil {
			return "method", node.Name.Name
		}
		return "function", node.Name.Name
	case *ast.GenDecl:
		if len(node.Specs) == 0 {
			return "declaration", ""
		}
		switch spec := node.Specs[0].(type) {
		case *ast.TypeSpec:
			return "type", spec.Name.Name
		case *ast.ValueSpec:
			if len(spec.Names) > 0 {
				return strings.ToLower(node.Tok.String()), spec.Names[0].Name
			}
		}
		return strings.ToLower(node.Tok.String()), ""
	default:
		return "declaration", ""
	}
}

func chunkTextFile(file discoveredFile) []indexedChunk {
	lines := strings.Split(file.Content, "\n")
	chunks := make([]indexedChunk, 0, max(1, len(lines)/40))
	var (
		startLine int
		builder   strings.Builder
		chunkIdx  int
	)
	flush := func(endLine int) {
		text := strings.TrimSpace(builder.String())
		if text == "" {
			builder.Reset()
			startLine = 0
			return
		}
		chunks = append(chunks, newChunk(file, chunkIdx, "block", startLine, endLine, "", text))
		chunkIdx++
		builder.Reset()
		startLine = 0
	}

	for i, line := range lines {
		lineNo := i + 1
		trimmed := strings.TrimSpace(line)
		if startLine == 0 {
			startLine = lineNo
		}
		if builder.Len() > 0 {
			builder.WriteByte('\n')
		}
		builder.WriteString(line)

		isBoundary := strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "##") || strings.HasPrefix(trimmed, "func ") || strings.HasPrefix(trimmed, "type ")
		if builder.Len() >= maxChunkBytes || (builder.Len() >= minChunkBytes && isBoundary) {
			flush(lineNo)
		}
	}
	if builder.Len() > 0 {
		flush(len(lines))
	}
	return chunks
}

func newChunk(file discoveredFile, chunkIndex int, kind string, startLine, endLine int, name, text string) indexedChunk {
	searchText := text
	if name != "" {
		searchText = fmt.Sprintf("%s\n%s\n%s", file.RelativePath, name, text)
	} else {
		searchText = fmt.Sprintf("%s\n%s", file.RelativePath, text)
	}
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%s", file.ContentHash, chunkIndex, searchText)))
	return indexedChunk{
		ID:            uuidFromHash(hash),
		Path:          file.RelativePath,
		Language:      file.Language,
		ChunkIndex:    chunkIndex,
		Kind:          kind,
		StartLine:     startLine,
		EndLine:       endLine,
		Content:       text,
		SearchText:    searchText,
		ContentHash:   hex.EncodeToString(hash[:]),
		TokenEstimate: estimateTokens(searchText),
	}
}

func uuidFromHash(hash [32]byte) string {
	buf := hash
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	raw := hex.EncodeToString(buf[:16])
	return fmt.Sprintf("%s-%s-%s-%s-%s", raw[0:8], raw[8:12], raw[12:16], raw[16:20], raw[20:32])
}

func estimateTokens(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	runes := len([]rune(text))
	return max(1, runes/4)
}

func snippet(text string) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\n", " "))
	runes := []rune(text)
	if len(runes) <= maxSnippetRunes {
		return text
	}
	return string(runes[:maxSnippetRunes]) + "…"
}

func discoveryWorkerCount(totalPaths int) int {
	workers := runtime.GOMAXPROCS(0) * 2
	if workers < 4 {
		workers = 4
	}
	if workers > 32 {
		workers = 32
	}
	if totalPaths < workers {
		return totalPaths
	}
	return workers
}
