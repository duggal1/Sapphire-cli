package tools

import (
	"context"
	_ "embed"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/fantasy"
)

const RGFilesToolName = "rg_files"

type RGFilesParams struct {
	Query string   `json:"query" description:"Filename/path query or glob pattern to match against rg --files output"`
	Path  string   `json:"path,omitempty" description:"The directory to search in. Defaults to the current working directory."`
	Paths []string `json:"paths,omitempty" description:"Optional list of directories to search in one parallel call"`
	Limit int      `json:"limit,omitempty" description:"Maximum number of file paths to return"`
}

//go:embed rg_files.md
var rgFilesDescription []byte

func NewRGFilesTool(workingDir string) fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(
		RGFilesToolName,
		string(rgFilesDescription),
		func(ctx context.Context, params RGFilesParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if ctx.Err() != nil {
				return fantasy.ToolResponse{}, ctx.Err()
			}
			query := strings.TrimSpace(params.Query)
			if query == "" {
				return fantasy.NewTextErrorResponse("query is required"), nil
			}

			searchPaths := normalizeBatchTargets(params.Path, params.Paths, workingDir)
			limit := params.Limit
			if limit <= 0 {
				limit = 100
			}

			files, truncated, errors := rgFilesAcrossRoots(ctx, query, searchPaths, limit)
			rootCount := len(searchPaths)
			if rootCount == 0 {
				rootCount = 1
			}

			var output string
			if len(files) == 0 {
				output = "No files found"
			} else {
				if rootCount > 1 {
					output = fmt.Sprintf("Searched %d roots in parallel.\n%s", rootCount, strings.Join(files, "\n"))
				} else {
					output = strings.Join(files, "\n")
				}
				if truncated {
					output += "\n\n(Results are truncated. Consider making the filename query more specific.)"
				}
			}
			if len(errors) > 0 {
				if output != "" {
					output += "\n\n"
				}
				output += "Errors:\n" + strings.Join(errors, "\n")
			}

			return fantasy.WithResponseMetadata(
				fantasy.NewTextResponse(output),
				GlobResponseMetadata{
					NumberOfFiles: len(files),
					NumberOfRoots: rootCount,
					Truncated:     truncated,
				},
			), nil
		},
	)
}

func rgFilesAcrossRoots(ctx context.Context, query string, rootPaths []string, limit int) ([]string, bool, []string) {
	if len(rootPaths) == 0 {
		rootPaths = []string{"."}
	}
	if len(rootPaths) == 1 {
		files, truncated, err := rgFiles(rootPaths[0], query, limit, ctx)
		if err != nil {
			return nil, false, []string{fmt.Sprintf("- %s: %v", filepath.ToSlash(rootPaths[0]), err)}
		}
		return files, truncated, nil
	}

	type rootResult struct {
		root      string
		files     []string
		truncated bool
		err       error
	}

	results := make([]rootResult, len(rootPaths))
	runRGParallelRoots(ctx, rootPaths, func(index int, root string) {
		files, truncated, err := rgFiles(root, query, limit, ctx)
		results[index] = rootResult{
			root:      root,
			files:     files,
			truncated: truncated,
			err:       err,
		}
	})

	combined := make([]string, 0, len(rootPaths)*limit)
	errors := make([]string, 0)
	seen := map[string]struct{}{}
	truncated := false
	for _, result := range results {
		if result.err != nil {
			errors = append(errors, fmt.Sprintf("- %s: %v", filepath.ToSlash(result.root), result.err))
			continue
		}
		truncated = truncated || result.truncated
		for _, file := range result.files {
			if _, ok := seen[file]; ok {
				continue
			}
			seen[file] = struct{}{}
			combined = append(combined, file)
		}
	}

	sort.SliceStable(combined, func(i, j int) bool {
		if len(combined[i]) == len(combined[j]) {
			return combined[i] < combined[j]
		}
		return len(combined[i]) < len(combined[j])
	})
	if len(combined) > limit {
		combined = combined[:limit]
		truncated = true
	}
	return combined, truncated, errors
}

func rgFiles(rootPath, query string, limit int, ctx context.Context) ([]string, bool, error) {
	globQuery := ""
	if looksLikeGlobQuery(query) {
		globQuery = query
	}
	cmd := getRgCmd(ctx, globQuery)
	if cmd == nil {
		return nil, false, fmt.Errorf("ripgrep not found in $PATH")
	}
	cmd.Dir = rootPath
	allFiles, err := runRipgrep(cmd, rootPath, 0)
	if err != nil {
		return nil, false, err
	}
	results := rankRGFileQuery(allFiles, query)
	truncated := false
	if limit > 0 && len(results) > limit {
		results = results[:limit]
		truncated = true
	}
	return results, truncated, nil
}

func looksLikeGlobQuery(query string) bool {
	return strings.ContainsAny(query, "*?[]{}")
}

func rankRGFileQuery(paths []string, query string) []string {
	query = strings.TrimSpace(strings.ToLower(query))
	if query == "" {
		return nil
	}
	terms := toolSearchScoreTerms(query)
	type scoredPath struct {
		path  string
		score int
	}
	scored := make([]scoredPath, 0, len(paths))
	for _, path := range paths {
		score := scoreFilePathQuery(path, query, terms)
		if score <= 0 {
			continue
		}
		scored = append(scored, scoredPath{path: filepath.ToSlash(path), score: score})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			if len(scored[i].path) == len(scored[j].path) {
				return scored[i].path < scored[j].path
			}
			return len(scored[i].path) < len(scored[j].path)
		}
		return scored[i].score > scored[j].score
	})
	out := make([]string, 0, len(scored))
	for _, item := range scored {
		out = append(out, item.path)
	}
	return out
}

func scoreFilePathQuery(path, query string, terms []string) int {
	pathLower := strings.ToLower(filepath.ToSlash(path))
	baseLower := strings.ToLower(filepath.Base(path))
	stemLower := strings.TrimSuffix(baseLower, filepath.Ext(baseLower))
	score := 0
	switch {
	case baseLower == query:
		score += 260
	case stemLower == query:
		score += 240
	case pathLower == query:
		score += 220
	}
	if strings.HasPrefix(baseLower, query) {
		score += 150
	}
	if strings.HasPrefix(stemLower, query) {
		score += 140
	}
	if strings.HasPrefix(pathLower, query) {
		score += 120
	}
	if strings.Contains(baseLower, query) {
		score += 120
	}
	if strings.Contains(pathLower, query) {
		score += 90
	}
	coveredTerms := 0
	for _, term := range terms {
		if term == "" {
			continue
		}
		matched := false
		switch {
		case stemLower == term:
			score += 48
			matched = true
		case strings.HasPrefix(stemLower, term):
			score += 42
			matched = true
		case containsRGFilePathSegment(pathLower, term):
			score += 36
			matched = true
		case strings.Contains(baseLower, term):
			score += 35
			matched = true
		case strings.Contains(pathLower, term):
			score += 20
			matched = true
		}
		if matched {
			coveredTerms++
		}
	}
	score += coveredTerms * 18
	if coveredTerms > 1 && coveredTerms == len(terms) {
		score += 30
	}
	score -= rgFilePathNoisePenalty(pathLower, terms)
	score -= min(max(strings.Count(pathLower, "/")-8, 0)*3, 18)
	if score < 0 {
		return 0
	}
	return score
}

func containsRGFilePathSegment(path, term string) bool {
	if term == "" {
		return false
	}
	for _, segment := range strings.Split(path, "/") {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		stem := strings.TrimSuffix(segment, filepath.Ext(segment))
		if segment == term || strings.Contains(segment, term) || stem == term || strings.Contains(stem, term) {
			return true
		}
	}
	return false
}

func rgFilePathNoisePenalty(path string, terms []string) int {
	switch {
	case strings.Contains(path, "/node_modules/"), strings.Contains(path, "/vendor/"), strings.Contains(path, "/third_party/"):
		return 80
	case strings.Contains(path, "/dist/"), strings.Contains(path, "/build/"), strings.Contains(path, "/out/"), strings.Contains(path, "/coverage/"), strings.Contains(path, "/target/"), strings.Contains(path, "/.next/"), strings.Contains(path, "/.turbo/"):
		return 44
	case strings.Contains(path, ".pb.go"), strings.Contains(path, "/generated/"), strings.Contains(path, "_generated"), strings.Contains(path, ".gen."), strings.Contains(path, "/gen/"):
		if toolSearchPathNeedsGeneratedAssets(terms) {
			return 8
		}
		return 28
	case strings.Contains(path, "/testdata/"), strings.Contains(path, "/fixtures/"), strings.Contains(path, "/fixture/"), strings.Contains(path, "/mock/"), strings.Contains(path, "/mocks/"):
		if toolSearchPathNeedsTestAssets(terms) {
			return 4
		}
		return 18
	default:
		return 0
	}
}

func toolSearchPathNeedsGeneratedAssets(terms []string) bool {
	return toolSearchPathHasAnyTerm(terms, "generated", "generator", "proto", "protobuf", "pb", "codegen")
}

func toolSearchPathNeedsTestAssets(terms []string) bool {
	return toolSearchPathHasAnyTerm(terms, "test", "tests", "spec", "coverage", "mock", "fixture")
}

func toolSearchPathHasAnyTerm(terms []string, candidates ...string) bool {
	if len(terms) == 0 || len(candidates) == 0 {
		return false
	}
	wanted := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		wanted[candidate] = struct{}{}
	}
	for _, term := range terms {
		if _, ok := wanted[term]; ok {
			return true
		}
	}
	return false
}

func toolSearchScoreTerms(query string) []string {
	fields := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		switch {
		case r >= 'a' && r <= 'z':
			return false
		case r >= '0' && r <= '9':
			return false
		default:
			return true
		}
	})
	out := make([]string, 0, len(fields))
	seen := map[string]struct{}{}
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if len(field) < 2 {
			continue
		}
		if _, ok := seen[field]; ok {
			continue
		}
		seen[field] = struct{}{}
		out = append(out, field)
	}
	return out
}
