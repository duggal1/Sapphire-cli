package tools

import (
	"cmp"
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/duggal1/Sapphire-cli/internal/filepathext"
	"github.com/duggal1/Sapphire-cli/internal/fsext"
	"github.com/duggal1/Sapphire-cli/internal/permission"
)

type LSParams struct {
	Path   string   `json:"path,omitempty" description:"The path to the directory to list (defaults to current working directory)"`
	Paths  []string `json:"paths,omitempty" description:"Optional list of directory paths to list in one parallel call"`
	Ignore []string `json:"ignore,omitempty" description:"List of glob patterns to ignore"`
	Depth  int      `json:"depth,omitempty" description:"The maximum depth to traverse"`
}

type LSPermissionsParams struct {
	Path   string   `json:"path,omitempty"`
	Paths  []string `json:"paths,omitempty"`
	Ignore []string `json:"ignore"`
	Depth  int      `json:"depth"`
}

type NodeType string

const (
	NodeTypeFile      NodeType = "file"
	NodeTypeDirectory NodeType = "directory"
)

type TreeNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	Type     NodeType    `json:"type"`
	Children []*TreeNode `json:"children,omitempty"`
}

type LSResponseMetadata struct {
	NumberOfFiles int  `json:"number_of_files"`
	NumberOfRoots int  `json:"number_of_roots,omitempty"`
	Truncated     bool `json:"truncated"`
}

const (
	LSToolName = "ls"
	maxLSFiles = 1000
	lsTimeout  = 5 * time.Second
)

//go:embed ls.md
var lsDescription []byte

// NewLsTool creates a tool for listing files and subdirectories in a tree structure.
func NewLsTool(permissions permission.Service, workingDir string, lsConfig config.ToolLs) fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(
		LSToolName,
		string(lsDescription),
		func(ctx context.Context, params LSParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if ctx.Err() != nil {
				return fantasy.ToolResponse{}, ctx.Err()
			}

			toolCtx, cancel := context.WithTimeout(ctx, lsTimeout)
			defer cancel()
			ctx = toolCtx

			// Military-grade safeguard: immediate exit if context cancelled
			if ctx.Err() != nil {
				return fantasy.ToolResponse{}, ctx.Err()
			}

			absWorkingDir, err := filepath.Abs(workingDir)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("error resolving working directory: %v", err)), nil
			}
			searchPaths, err := resolveLSTargets(params, workingDir)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			for _, searchPath := range searchPaths {
				absSearchPath, err := filepath.Abs(searchPath)
				if err != nil {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("error resolving search path: %v", err)), nil
				}

				relPath, err := filepath.Rel(absWorkingDir, absSearchPath)
				if err != nil || strings.HasPrefix(relPath, "..") {
					sessionID := GetSessionFromContext(ctx)
					if sessionID == "" {
						return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for accessing directories outside working directory")
					}

					granted, err := permissions.Request(ctx,
						permission.CreatePermissionRequest{
							SessionID:   sessionID,
							Path:        absSearchPath,
							ToolCallID:  call.ID,
							ToolName:    LSToolName,
							Action:      "list",
							Description: fmt.Sprintf("List directory outside working directory: %s", absSearchPath),
							Params: LSPermissionsParams{
								Path:   params.Path,
								Paths:  append([]string{}, params.Paths...),
								Ignore: append([]string{}, params.Ignore...),
								Depth:  params.Depth,
							},
						},
					)
					if err != nil {
						return fantasy.ToolResponse{}, err
					}
					if !granted {
						return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
					}
				}
			}

			output, metadata, err := ListDirectoryTrees(ctx, searchPaths, params, lsConfig)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}

			return fantasy.WithResponseMetadata(
				fantasy.NewTextResponse(output),
				metadata,
			), nil
		})
}

func resolveLSTargets(params LSParams, workingDir string) ([]string, error) {
	targets := normalizeBatchTargets(params.Path, params.Paths, workingDir)
	resolved := make([]string, 0, len(targets))
	for _, target := range targets {
		expanded, err := fsext.Expand(target)
		if err != nil {
			return nil, fmt.Errorf("error expanding path %q: %w", target, err)
		}
		resolved = append(resolved, filepathext.SmartJoin(workingDir, expanded))
	}
	return resolved, nil
}

func ListDirectoryTrees(ctx context.Context, searchPaths []string, params LSParams, lsConfig config.ToolLs) (string, LSResponseMetadata, error) {
	if len(searchPaths) == 0 {
		searchPaths = []string{"."}
	}
	if len(searchPaths) == 1 {
		output, metadata, err := ListDirectoryTree(ctx, searchPaths[0], params, lsConfig)
		if metadata.NumberOfRoots == 0 {
			metadata.NumberOfRoots = 1
		}
		return output, metadata, err
	}

	type listResult struct {
		path     string
		output   string
		metadata LSResponseMetadata
		err      error
	}

	results := make([]listResult, len(searchPaths))
	var wg sync.WaitGroup
	sem := make(chan struct{}, boundedParallelism(len(searchPaths), 8))
	for i, searchPath := range searchPaths {
		wg.Add(1)
		go func(index int, path string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				results[index] = listResult{path: path, err: ctx.Err()}
				return
			}
			defer func() { <-sem }()
			output, metadata, err := ListDirectoryTree(ctx, path, params, lsConfig)
			results[index] = listResult{
				path:     path,
				output:   output,
				metadata: metadata,
				err:      err,
			}
		}(i, searchPath)
	}
	wg.Wait()

	sort.Slice(results, func(i, j int) bool {
		return results[i].path < results[j].path
	})

	metadata := LSResponseMetadata{NumberOfRoots: len(searchPaths)}
	sections := make([]string, 0, len(results))
	errors := make([]string, 0)
	for _, result := range results {
		if result.err != nil {
			errors = append(errors, fmt.Sprintf("- %s: %v", filepath.ToSlash(result.path), result.err))
			continue
		}
		metadata.NumberOfFiles += result.metadata.NumberOfFiles
		metadata.Truncated = metadata.Truncated || result.metadata.Truncated
		sections = append(sections, fmt.Sprintf("Path: %s\n%s", filepath.ToSlash(result.path), strings.TrimSpace(result.output)))
	}
	if len(sections) == 0 && len(errors) > 0 {
		return "", metadata, fmt.Errorf("error listing directories:\n%s", strings.Join(errors, "\n"))
	}
	output := fmt.Sprintf("Listed %d directories in parallel.\n\n%s", len(sections), strings.Join(sections, "\n\n"))
	if len(errors) > 0 {
		output += "\n\nErrors:\n" + strings.Join(errors, "\n")
	}
	return output, metadata, nil
}

func ListDirectoryTree(ctx context.Context, searchPath string, params LSParams, lsConfig config.ToolLs) (string, LSResponseMetadata, error) {
	if _, err := os.Stat(searchPath); os.IsNotExist(err) {
		return "", LSResponseMetadata{}, fmt.Errorf("path does not exist: %s", searchPath)
	}

	depth, limit := lsConfig.Limits()
	maxFiles := cmp.Or(limit, maxLSFiles)
	files, truncated, err := fsext.ListDirectory(
		ctx,
		searchPath,
		params.Ignore,
		cmp.Or(params.Depth, depth),
		maxFiles,
	)
	if err != nil {
		return "", LSResponseMetadata{}, fmt.Errorf("error listing directory: %w", err)
	}

	metadata := LSResponseMetadata{
		NumberOfFiles: len(files),
		NumberOfRoots: 1,
		Truncated:     truncated,
	}
	tree := createFileTree(files, searchPath)

	var output string
	if truncated {
		output = fmt.Sprintf("There are more than %d files in the directory. Use a more specific path or use the Glob tool to find specific files. The first %[1]d files and directories are included below.\n", maxFiles)
	}
	if depth > 0 {
		output = fmt.Sprintf("The directory tree is shown up to a depth of %d. Use a higher depth and a specific path to see more levels.\n", cmp.Or(params.Depth, depth))
	}
	return output + "\n" + printTree(tree, searchPath), metadata, nil
}

func createFileTree(sortedPaths []string, rootPath string) []*TreeNode {
	root := []*TreeNode{}
	pathMap := make(map[string]*TreeNode)

	for _, path := range sortedPaths {
		relativePath := strings.TrimPrefix(path, rootPath)
		parts := strings.Split(relativePath, string(filepath.Separator))
		currentPath := ""
		var parentPath string

		var cleanParts []string
		for _, part := range parts {
			if part != "" {
				cleanParts = append(cleanParts, part)
			}
		}
		parts = cleanParts

		if len(parts) == 0 {
			continue
		}

		for i, part := range parts {
			if currentPath == "" {
				currentPath = part
			} else {
				currentPath = filepath.Join(currentPath, part)
			}

			if _, exists := pathMap[currentPath]; exists {
				parentPath = currentPath
				continue
			}

			isLastPart := i == len(parts)-1
			isDir := !isLastPart || strings.HasSuffix(relativePath, string(filepath.Separator))
			nodeType := NodeTypeFile
			if isDir {
				nodeType = NodeTypeDirectory
			}
			newNode := &TreeNode{
				Name:     part,
				Path:     currentPath,
				Type:     nodeType,
				Children: []*TreeNode{},
			}

			pathMap[currentPath] = newNode

			if i > 0 && parentPath != "" {
				if parent, ok := pathMap[parentPath]; ok {
					parent.Children = append(parent.Children, newNode)
				}
			} else {
				root = append(root, newNode)
			}

			parentPath = currentPath
		}
	}

	return root
}

func printTree(tree []*TreeNode, rootPath string) string {
	var result strings.Builder

	result.WriteString("- ")
	result.WriteString(filepath.ToSlash(rootPath))
	if rootPath[len(rootPath)-1] != '/' {
		result.WriteByte('/')
	}
	result.WriteByte('\n')

	for _, node := range tree {
		printNode(&result, node, 1)
	}

	return result.String()
}

func printNode(builder *strings.Builder, node *TreeNode, level int) {
	indent := strings.Repeat("  ", level)

	nodeName := node.Name
	if node.Type == NodeTypeDirectory {
		nodeName = nodeName + "/"
	}

	fmt.Fprintf(builder, "%s- %s\n", indent, nodeName)

	if node.Type == NodeTypeDirectory && len(node.Children) > 0 {
		for _, child := range node.Children {
			printNode(builder, child, level+1)
		}
	}
}
