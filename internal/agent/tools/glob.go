package tools

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/fsext"
)

const (
	GlobToolName = "glob"
	globTimeout  = 5 * time.Second
)

//go:embed glob.md
var globDescription []byte

type GlobParams struct {
	Pattern string `json:"pattern" description:"The glob pattern to match files against"`
	Path    string `json:"path,omitempty" description:"The directory to search in. Defaults to the current working directory."`
	Paths   []string `json:"paths,omitempty" description:"Optional list of directories to search in one parallel call"`
}

type GlobResponseMetadata struct {
	NumberOfFiles int  `json:"number_of_files"`
	NumberOfRoots int  `json:"number_of_roots,omitempty"`
	Truncated     bool `json:"truncated"`
}

// NewGlobTool creates a new tool for finding files that match specific glob patterns.
func NewGlobTool(workingDir string) fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(
		GlobToolName,
		string(globDescription),
		func(ctx context.Context, params GlobParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if ctx.Err() != nil {
				return fantasy.ToolResponse{}, ctx.Err()
			}

			toolCtx, cancel := context.WithTimeout(ctx, globTimeout)
			defer cancel()
			ctx = toolCtx

			// Military-grade safeguard: immediate exit if context cancelled
			if ctx.Err() != nil {
				return fantasy.ToolResponse{}, ctx.Err()
			}

			if params.Pattern == "" {
				return fantasy.NewTextErrorResponse("pattern is required"), nil
			}

			searchPaths := normalizeBatchTargets(params.Path, params.Paths, workingDir)
			files, truncated, errors := globFilesAcrossRoots(ctx, params.Pattern, searchPaths, 100)

			var output string
			if len(files) == 0 {
				output = "No files found"
			} else {
				normalizeFilePaths(files)
				if len(searchPaths) > 1 {
					output = fmt.Sprintf("Searched %d roots in parallel.\n%s", len(searchPaths), strings.Join(files, "\n"))
				} else {
					output = strings.Join(files, "\n")
				}
				if truncated {
					output += "\n\n(Results are truncated. Consider using a more specific path or pattern.)"
				}
			}
			if len(errors) > 0 {
				if output != "" {
					output += "\n\n"
				}
				output += "Errors:\n" + strings.Join(errors, "\n")
			}
			rootCount := len(searchPaths)
			if rootCount == 0 {
				rootCount = 1
			}

			return fantasy.WithResponseMetadata(
				fantasy.NewTextResponse(output),
				GlobResponseMetadata{
					NumberOfFiles: len(files),
					NumberOfRoots: rootCount,
					Truncated:     truncated,
				},
			), nil
		})
}

func globFilesAcrossRoots(ctx context.Context, pattern string, searchPaths []string, limit int) ([]string, bool, []string) {
	if len(searchPaths) == 0 {
		searchPaths = []string{"."}
	}
	if len(searchPaths) == 1 {
		files, truncated, err := globFiles(ctx, pattern, searchPaths[0], limit)
		if err != nil {
			return nil, false, []string{fmt.Sprintf("- %s: %v", filepath.ToSlash(searchPaths[0]), err)}
		}
		return files, truncated, nil
	}

	type rootResult struct {
		root      string
		files     []string
		truncated bool
		err       error
	}

	results := make([]rootResult, len(searchPaths))
	var wg sync.WaitGroup
	sem := make(chan struct{}, boundedParallelism(len(searchPaths), 8))
	for i, searchPath := range searchPaths {
		wg.Add(1)
		go func(index int, root string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				results[index] = rootResult{root: root, err: ctx.Err()}
				return
			}
			defer func() { <-sem }()
			files, truncated, err := globFiles(ctx, pattern, root, limit)
			results[index] = rootResult{
				root:      root,
				files:     files,
				truncated: truncated,
				err:       err,
			}
		}(i, searchPath)
	}
	wg.Wait()

	combined := make([]string, 0, len(searchPaths)*limit)
	errors := make([]string, 0)
	seen := make(map[string]struct{})
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
	if limit > 0 && len(combined) > limit {
		combined = combined[:limit]
		truncated = true
	}
	return combined, truncated, errors
}

func globFiles(ctx context.Context, pattern, searchPath string, limit int) ([]string, bool, error) {
	cmdRg := getRgCmd(ctx, pattern)
	if cmdRg != nil {
		cmdRg.Dir = searchPath
		matches, err := runRipgrep(cmdRg, searchPath, limit)
		if err == nil {
			return matches, len(matches) >= limit && limit > 0, nil
		}
		slog.Warn("Ripgrep execution failed, falling back to doublestar", "error", err)
	}

	return fsext.GlobGitignoreAware(ctx, pattern, searchPath, limit)
}

func runRipgrep(cmd *exec.Cmd, searchRoot string, limit int) ([]string, error) {
	out, err := cmd.CombinedOutput()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("ripgrep: %w\n%s", err, out)
	}

	var matches []string
	for p := range bytes.SplitSeq(out, []byte{0}) {
		if len(p) == 0 {
			continue
		}
		absPath := string(p)
		if !filepath.IsAbs(absPath) {
			absPath = filepath.Join(searchRoot, absPath)
		}
		if fsext.SkipHidden(absPath) {
			continue
		}
		matches = append(matches, absPath)
	}

	sort.SliceStable(matches, func(i, j int) bool {
		return len(matches[i]) < len(matches[j])
	})

	if limit > 0 && len(matches) > limit {
		matches = matches[:limit]
	}
	return matches, nil
}

func normalizeFilePaths(paths []string) {
	for i, p := range paths {
		paths[i] = filepath.ToSlash(p)
	}
}
