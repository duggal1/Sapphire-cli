// Package chat provides UI components and message items for the chat interface.
package chat

// GenericToolMessageItem provides a fallback renderer for unknown tool types.

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/duggal1/Sapphire-cli/internal/message"
	"github.com/duggal1/Sapphire-cli/internal/stringext"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
)

// GenericToolMessageItem is a message item that represents an unknown tool call.
type GenericToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*GenericToolMessageItem)(nil)

// NewGenericToolMessageItem creates a new [GenericToolMessageItem].
func NewGenericToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &GenericToolRenderContext{}, canceled)
}

// GenericToolRenderContext renders unknown/generic tool messages.
type GenericToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (g *GenericToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	name := genericPrettyName(opts.ToolCall.Name)
	if opts.ToolCall.Name == "orchestrate_worktrees" {
		return renderOrchestrateWorktreesTool(sty, cappedWidth, opts, name)
	}

	if opts.IsPending() {
		return pendingTool(sty, name)
	}

	var params map[string]any
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid parameters"}, cappedWidth)
	}

	var toolParams []string
	if len(params) > 0 {
		parsed, _ := json.Marshal(params)
		toolParams = append(toolParams, string(parsed))
	}

	header := toolHeader(sty, opts.Status, name, cappedWidth, opts.Compact, toolParams...)
	if opts.Compact {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(header, earlyState)
	}

	if !opts.HasResult() || opts.Result.Content == "" {
		return header
	}

	bodyWidth := cappedWidth - toolBodyLeftPaddingTotal

	// Handle image data.
	if opts.Result.Data != "" && strings.HasPrefix(opts.Result.MIMEType, "image/") {
		body := sty.Tool.Body.Render(toolOutputImageContent(sty, opts.Result.Data, opts.Result.MIMEType))
		return joinToolParts(header, body)
	}

	// Try to parse result as JSON for pretty display.
	var result json.RawMessage
	var body string
	if err := json.Unmarshal([]byte(opts.Result.Content), &result); err == nil {
		prettyResult, err := json.MarshalIndent(result, "", "  ")
		if err == nil {
			body = toolOutputCodeContent(sty, "result.json", string(prettyResult), 0, bodyWidth, opts.ExpandedContent)
		} else {
			body = sty.Tool.Body.Render(toolOutputPlainContent(sty, opts.Result.Content, bodyWidth, opts.ExpandedContent))
		}
	} else if looksLikeMarkdown(opts.Result.Content) {
		body = sty.Tool.Body.Render(toolOutputCollapsedMarkdownContent(sty, opts.Result.Content, bodyWidth))
	} else {
		body = sty.Tool.Body.Render(toolOutputPlainContent(sty, opts.Result.Content, bodyWidth, opts.ExpandedContent))
	}

	return joinToolParts(header, body)
}

// genericPrettyName converts a snake_case or kebab-case tool name to a
// human-readable title case name.
func genericPrettyName(name string) string {
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "-", " ")
	return stringext.Capitalize(name)
}

type orchestrationWorktreeParams struct {
	Tasks             []orchestrationWorktreeTask `json:"tasks"`
	TestCommand       string                      `json:"test_command,omitempty"`
	IntegrationPrompt string                      `json:"integration_prompt,omitempty"`
	IntegrationBranch string                      `json:"integration_branch,omitempty"`
}

type orchestrationWorktreeTask struct {
	Agent            string   `json:"agent,omitempty"`
	Branch           string   `json:"branch,omitempty"`
	WorktreePath     string   `json:"worktree_path,omitempty"`
	DefinitionOfDone string   `json:"definition_of_done,omitempty"`
	Message          string   `json:"message,omitempty"`
	Task             string   `json:"task,omitempty"`
	Title            string   `json:"title,omitempty"`
	WriteManifest    []string `json:"write_manifest,omitempty"`
}

func renderOrchestrateWorktreesTool(sty *styles.Styles, width int, opts *ToolRenderOpts, name string) string {
	header := toolHeader(sty, opts.Status, name, width, opts.Compact)
	if opts.Compact {
		return header
	}

	var params orchestrationWorktreeParams
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		return header
	}

	body := renderOrchestrationWorktreeBody(sty, params, width-toolBodyLeftPaddingTotal)
	if body == "" {
		return header
	}
	return joinToolParts(header, sty.Tool.Body.Render(body))
}

func renderOrchestrationWorktreeBody(sty *styles.Styles, params orchestrationWorktreeParams, width int) string {
	if len(params.Tasks) == 0 {
		return ""
	}

	root := &TreeNode{
		Label: sty.Tool.ListRoot.Render("Worktree Plan"),
	}

	taskNodes := make([]*TreeNode, 0, len(params.Tasks))
	for i, task := range params.Tasks {
		taskLabel := strings.TrimSpace(task.Title)
		if taskLabel == "" {
			taskLabel = strings.TrimSpace(task.Branch)
		}
		if taskLabel == "" {
			taskLabel = strings.TrimSpace(task.Task)
		}
		if taskLabel == "" {
			taskLabel = fmt.Sprintf("Task %d", i+1)
		}

		node := &TreeNode{
			Label: sty.Tool.ListDirectory.Render(taskLabel),
		}
		if agent := strings.TrimSpace(task.Agent); agent != "" {
			node.Children = append(node.Children, &TreeNode{Label: subAgentKVLabel("Agent", agent)})
		}
		if branch := strings.TrimSpace(task.Branch); branch != "" {
			node.Children = append(node.Children, &TreeNode{Label: subAgentKVLabel("Branch", branch)})
		}
		if worktree := strings.TrimSpace(task.WorktreePath); worktree != "" {
			node.Children = append(node.Children, &TreeNode{Label: subAgentKVLabel("Worktree", formatRelativePath(worktree))})
		}
		if taskText := firstNonEmptyOrchestrationValue(task.Task, task.Message); taskText != "" {
			node.Children = append(node.Children, &TreeNode{Label: subAgentKVLabel("Task", oneLine(taskText))})
		}
		if done := strings.TrimSpace(task.DefinitionOfDone); done != "" {
			node.Children = append(node.Children, &TreeNode{Label: subAgentKVLabel("Done", oneLine(done))})
		}
		if len(task.WriteManifest) > 0 {
			writeRoot := &TreeNode{Label: "Write Scope"}
			for _, path := range task.WriteManifest {
				path = strings.TrimSpace(path)
				if path == "" {
					continue
				}
				writeRoot.Children = append(writeRoot.Children, &TreeNode{Label: formatRelativePath(path)})
			}
			if len(writeRoot.Children) > 0 {
				node.Children = append(node.Children, writeRoot)
			}
		}
		taskNodes = append(taskNodes, node)
	}

	root.Children = append(root.Children, &TreeNode{Label: "Tasks", Children: taskNodes})
	if branch := strings.TrimSpace(params.IntegrationBranch); branch != "" {
		root.Children = append(root.Children, &TreeNode{Label: subAgentKVLabel("Integration Branch", branch)})
	}
	if testCmd := strings.TrimSpace(params.TestCommand); testCmd != "" {
		root.Children = append(root.Children, &TreeNode{Label: subAgentKVLabel("Test Command", oneLine(testCmd))})
	}
	if prompt := strings.TrimSpace(params.IntegrationPrompt); prompt != "" {
		root.Children = append(root.Children, &TreeNode{Label: subAgentKVLabel("Integration", oneLine(prompt))})
	}

	return strings.Join(renderTreeWithRoot(root, width), "\n")
}

func firstNonEmptyOrchestrationValue(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
