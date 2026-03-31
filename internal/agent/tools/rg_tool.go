package tools

import (
	"context"
	_ "embed"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"charm.land/fantasy"
)

const RGToolName = "rg"

type RGParams struct {
	Pattern       string   `json:"pattern" description:"The ripgrep pattern to search for in file contents"`
	Path          string   `json:"path,omitempty" description:"The directory to search in. Defaults to the current working directory."`
	Paths         []string `json:"paths,omitempty" description:"Optional list of directories to search in one parallel call"`
	Include       string   `json:"include,omitempty" description:"Optional glob used to restrict searched files"`
	LiteralText   bool     `json:"literal_text,omitempty" description:"If true, treat pattern as exact text instead of regex"`
	CaseSensitive bool     `json:"case_sensitive,omitempty" description:"If true, keep the search case-sensitive. Default is false."`
	Limit         int      `json:"limit,omitempty" description:"Maximum number of matches to return"`
}

//go:embed rg.md
var rgDescription []byte

func NewRGTool(workingDir string) fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(
		RGToolName,
		string(rgDescription),
		func(ctx context.Context, params RGParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if ctx.Err() != nil {
				return fantasy.ToolResponse{}, ctx.Err()
			}
			if strings.TrimSpace(params.Pattern) == "" {
				return fantasy.NewTextErrorResponse("pattern is required"), nil
			}

			searchPaths := normalizeBatchTargets(params.Path, params.Paths, workingDir)
			limit := params.Limit
			if limit <= 0 {
				limit = 100
			}

			matches, truncated, errors := rgSearchAcrossRoots(ctx, params, searchPaths, limit)
			rootCount := len(searchPaths)
			if rootCount == 0 {
				rootCount = 1
			}

			var output strings.Builder
			if len(matches) == 0 {
				output.WriteString("No matches found")
			} else {
				if rootCount > 1 {
					fmt.Fprintf(&output, "Searched %d roots in parallel. Found %d matches\n", rootCount, len(matches))
				} else {
					fmt.Fprintf(&output, "Found %d matches\n", len(matches))
				}
				currentFile := ""
				for _, match := range matches {
					if currentFile != match.path {
						if currentFile != "" {
							output.WriteString("\n")
						}
						currentFile = match.path
						fmt.Fprintf(&output, "%s:\n", filepath.ToSlash(match.path))
					}
					lineText := match.lineText
					if len(lineText) > maxGrepContentWidth {
						lineText = lineText[:maxGrepContentWidth] + "..."
					}
					if match.charNum > 0 {
						fmt.Fprintf(&output, "  Line %d, Char %d: %s\n", match.lineNum, match.charNum, lineText)
					} else {
						fmt.Fprintf(&output, "  Line %d: %s\n", match.lineNum, lineText)
					}
				}
				if truncated {
					output.WriteString("\n(Results are truncated. Consider narrowing the query or include pattern.)")
				}
			}
			if len(errors) > 0 {
				if output.Len() > 0 {
					output.WriteString("\n\n")
				}
				output.WriteString("Errors:\n")
				output.WriteString(strings.Join(errors, "\n"))
			}

			return fantasy.WithResponseMetadata(
				fantasy.NewTextResponse(strings.TrimSpace(output.String())),
				GrepResponseMetadata{
					NumberOfMatches: len(matches),
					NumberOfRoots:   rootCount,
					Truncated:       truncated,
				},
			), nil
		},
	)
}

func rgSearchAcrossRoots(ctx context.Context, params RGParams, rootPaths []string, limit int) ([]grepMatch, bool, []string) {
	if len(rootPaths) == 0 {
		rootPaths = []string{"."}
	}
	if len(rootPaths) == 1 {
		matches, err := searchWithRipgrepConfig(ctx, params.Pattern, rootPaths[0], params.Include, params.LiteralText, params.CaseSensitive)
		if err != nil {
			return nil, false, []string{fmt.Sprintf("- %s: %v", filepath.ToSlash(rootPaths[0]), err)}
		}
		if len(matches) > limit {
			return matches[:limit], true, nil
		}
		return matches, false, nil
	}

	type rootResult struct {
		root    string
		matches []grepMatch
		err     error
	}

	results := make([]rootResult, len(rootPaths))
	runRGParallelRoots(ctx, rootPaths, func(index int, root string) {
		matches, err := searchWithRipgrepConfig(ctx, params.Pattern, root, params.Include, params.LiteralText, params.CaseSensitive)
		results[index] = rootResult{root: root, matches: matches, err: err}
	})

	combined := make([]grepMatch, 0, len(rootPaths)*limit)
	errors := make([]string, 0)
	seen := map[string]struct{}{}
	for _, result := range results {
		if result.err != nil {
			errors = append(errors, fmt.Sprintf("- %s: %v", filepath.ToSlash(result.root), result.err))
			continue
		}
		for _, match := range result.matches {
			key := fmt.Sprintf("%s:%d:%d:%s", match.path, match.lineNum, match.charNum, match.lineText)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			combined = append(combined, match)
		}
	}

	sort.Slice(combined, func(i, j int) bool {
		return combined[i].modTime.After(combined[j].modTime)
	})
	truncated := false
	if len(combined) > limit {
		combined = combined[:limit]
		truncated = true
	}
	return combined, truncated, errors
}

func searchWithRipgrepConfig(ctx context.Context, pattern, path, include string, literalText, caseSensitive bool) ([]grepMatch, error) {
	if literalText {
		pattern = escapeRegexPattern(pattern)
	}
	if !caseSensitive {
		pattern = "(?i)" + pattern
	}
	return searchWithRipgrep(ctx, pattern, path, include)
}

func runRGParallelRoots(ctx context.Context, roots []string, run func(index int, root string)) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, boundedParallelism(len(roots), 8))
	for i, root := range roots {
		wg.Add(1)
		go func(index int, currentRoot string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()
			run(index, currentRoot)
		}(i, root)
	}
	wg.Wait()
}
