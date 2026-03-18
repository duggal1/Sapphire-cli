package chat

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// TreeNode represents a generic renderable tree node.
type TreeNode struct {
	Label    string
	Children []*TreeNode
}

// renderTreeWithRoot renders a root label without a branch prefix, then its children.
func renderTreeWithRoot(root *TreeNode, width int) []string {
	if root == nil {
		return nil
	}
	var lines []string
	rootLines := strings.Split(root.Label, "\n")
	for _, line := range rootLines {
		line = ansi.Truncate(line, max(0, width), "…")
		lines = append(lines, line)
	}
	if len(root.Children) > 0 {
		lines = append(lines, renderTreeLines(root.Children, "", width)...)
	}
	return lines
}

// renderTreeLines renders nodes using a consistent tree style.
func renderTreeLines(nodes []*TreeNode, prefix string, width int) []string {
	var lines []string
	for i, node := range nodes {
		isLast := i == len(nodes)-1
		branch := "├── "
		if isLast {
			branch = "└── "
		}

		labelLines := strings.Split(node.Label, "\n")
		if len(labelLines) == 0 {
			continue
		}

		line := prefix + branch + labelLines[0]
		line = ansi.Truncate(line, max(0, width), "…")
		lines = append(lines, line)

		continuationPrefix := prefix + "│   "
		if isLast {
			continuationPrefix = prefix + "    "
		}

		if len(labelLines) > 1 {
			indent := continuationPrefix + "    "
			for _, extra := range labelLines[1:] {
				extraLine := indent + extra
				extraLine = ansi.Truncate(extraLine, max(0, width), "…")
				lines = append(lines, extraLine)
			}
		}

		if len(node.Children) > 0 {
			lines = append(lines, renderTreeLines(node.Children, continuationPrefix, width)...)
		}
	}
	return lines
}
