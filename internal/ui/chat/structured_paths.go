package chat

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
)

type fileContextEntry struct {
	Path      string
	LineStart int
	LineEnd   int
}

type fileContextNode struct {
	name      string
	path      string
	isFile    bool
	lineStart int
	lineEnd   int
	children  []*fileContextNode
}

func buildFileContextRoot(sty *styles.Styles, rootLabel string, entries []fileContextEntry) *TreeNode {
	children := buildFileContextNodes(sty, entries)
	if len(children) == 0 {
		return nil
	}
	return &TreeNode{
		Label:    sty.Tool.ListRoot.Render(rootLabel),
		Children: children,
	}
}

func buildFileContextNodes(sty *styles.Styles, entries []fileContextEntry) []*TreeNode {
	root := &fileContextNode{}

	for _, entry := range entries {
		path := formatRelativePath(strings.TrimSpace(entry.Path))
		if path == "" {
			continue
		}

		parts := strings.Split(path, "/")
		current := root
		currentPath := ""
		for i, part := range parts {
			if part == "" {
				continue
			}
			if currentPath != "" {
				currentPath += "/"
			}
			currentPath += part

			isLeaf := i == len(parts)-1
			child := findFileContextChild(current, part, isLeaf)
			if child == nil {
				child = &fileContextNode{
					name:     part,
					path:     currentPath,
					isFile:   isLeaf,
					children: []*fileContextNode{},
				}
				current.children = append(current.children, child)
			}
			if isLeaf {
				child.isFile = true
				child.lineStart = entry.LineStart
				child.lineEnd = entry.LineEnd
			}
			current = child
		}
	}

	nodes := make([]*TreeNode, 0, len(root.children))
	for _, child := range root.children {
		nodes = append(nodes, fileContextNodeToRenderNode(sty, child))
	}
	return nodes
}

func findFileContextChild(parent *fileContextNode, name string, isLeaf bool) *fileContextNode {
	for _, child := range parent.children {
		if child.name == name && child.isFile == isLeaf {
			return child
		}
	}
	return nil
}

func fileContextNodeToRenderNode(sty *styles.Styles, node *fileContextNode) *TreeNode {
	label := sty.Tool.ListDirectory.Render(node.name)
	if node.isFile {
		label = sty.Tool.ListFile.Bold(true).Render(node.name)
		if node.lineStart > 0 && node.lineEnd >= node.lineStart {
			label += sty.Tool.ListMeta.Render(fmt.Sprintf(" L%d-L%d", node.lineStart, node.lineEnd))
		}
	}

	children := make([]*TreeNode, 0, len(node.children))
	for _, child := range node.children {
		children = append(children, fileContextNodeToRenderNode(sty, child))
	}

	return &TreeNode{
		Label:    label,
		Children: children,
	}
}

func parseAnnotatedFileContext(raw string) fileContextEntry {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fileContextEntry{}
	}

	entry := fileContextEntry{Path: raw}
	idx := strings.LastIndex(raw, " L")
	if idx == -1 {
		return entry
	}

	pathPart := strings.TrimSpace(raw[:idx])
	linePart := strings.TrimSpace(raw[idx+1:])
	linePart = strings.TrimPrefix(linePart, "L")
	startText, endText, ok := strings.Cut(linePart, "-")
	if !ok {
		start, err := strconv.Atoi(strings.TrimPrefix(linePart, "L"))
		if err != nil {
			return entry
		}
		entry.Path = pathPart
		entry.LineStart = start
		entry.LineEnd = start
		return entry
	}

	start, errStart := strconv.Atoi(strings.TrimPrefix(startText, "L"))
	end, errEnd := strconv.Atoi(strings.TrimPrefix(endText, "L"))
	if errStart != nil || errEnd != nil {
		return entry
	}

	entry.Path = pathPart
	entry.LineStart = start
	entry.LineEnd = end
	return entry
}

func fileContextEntriesFromPaths(paths []string) []fileContextEntry {
	entries := make([]fileContextEntry, 0, len(paths))
	for _, path := range paths {
		entry := parseAnnotatedFileContext(path)
		if entry.Path == "" {
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}

func rootDirectoryLabel(path string) string {
	path = formatRelativePath(path)
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		return ""
	}
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		return dir
	}
	return filepath.Base(path)
}
