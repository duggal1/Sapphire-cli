package session

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/sapphire/internal/message"
)

// ExportSessionToMarkdown exports a session and its messages to a Markdown string.
func ExportSessionToMarkdown(ctx context.Context, s Session, messages []message.Message) (string, error) {
	var sb strings.Builder

	// Header
	fmt.Fprintf(&sb, "# %s\n\n", s.Title)
	fmt.Fprintf(&sb, "- **Session ID**: `%s`\n", s.ID)
	fmt.Fprintf(&sb, "- **Created At**: %s\n", time.Unix(s.CreatedAt, 0).Format(time.RFC3339))
	fmt.Fprintf(&sb, "- **Tokens**: %d prompt / %d completion (%d total)\n", s.PromptTokens, s.CompletionTokens, s.PromptTokens+s.CompletionTokens)
	fmt.Fprintf(&sb, "- **Cost**: $%.4f\n\n", s.Cost)

	// Todos
	if len(s.Todos) > 0 {
		sb.WriteString("## Todo List\n\n")
		for _, t := range s.Todos {
			if !IsRenderableTodo(t) {
				continue
			}
			status := " "
			if t.Status == TodoStatusCompleted {
				status = "x"
			}
			fmt.Fprintf(&sb, "- [%s] %s", status, t.Content)
			if t.ActiveForm != "" {
				fmt.Fprintf(&sb, " — *%s*", t.ActiveForm)
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Messages
	sb.WriteString("## Conversation\n\n")
	for _, m := range messages {
		if m.IsSummaryMessage {
			sb.WriteString("--- \n\n> **Summary Message**\n\n")
		}

		role := strings.Title(string(m.Role))
		fmt.Fprintf(&sb, "### %s\n\n", role)

		// Reasoning
		if reasoning := m.ReasoningContent().Thinking; reasoning != "" {
			sb.WriteString("<details>\n<summary>Reasoning</summary>\n\n")
			sb.WriteString(reasoning)
			sb.WriteString("\n\n</details>\n\n")
		}

		// Content
		if text := m.Content().Text; text != "" {
			sb.WriteString(text)
			sb.WriteString("\n\n")
		}

		// Tool Calls & Results
		// Since results are separate messages in the list, we'll render tool calls when we see them,
		// and the results will follow as their own "### Tool" sections.
		// However, to satisfy "preserve ordering exactly" and "include tool calls, tool results",
		// we should handle the Tool role specially if we want it nested, 
		// but since they are chronological in the slice, flat rendering is most accurate to the DB.
		
		toolCalls := m.ToolCalls()
		if len(toolCalls) > 0 {
			for _, tc := range toolCalls {
				fmt.Fprintf(&sb, "> **Tool Call**: `%s`\n", tc.Name)
				if tc.Input != "" && tc.Input != "{}" {
					sb.WriteString("> ```json\n> ")
					sb.WriteString(strings.ReplaceAll(tc.Input, "\n", "\n> "))
					sb.WriteString("\n> ```\n")
				}
				sb.WriteString("\n")
			}
		}

		if m.Role == message.Tool {
			for _, tr := range m.ToolResults() {
				fmt.Fprintf(&sb, "> **Tool Result** (`%s`):\n", tr.Name)
				if tr.IsError {
					sb.WriteString("> **Error**: ")
				}
				content := tr.Content
				if content == "" && tr.Data != "" {
					content = fmt.Sprintf("[Binary Data: %s]", tr.MIMEType)
				}
				
				if strings.Contains(content, "\n") {
					sb.WriteString("> ```\n> ")
					sb.WriteString(strings.ReplaceAll(content, "\n", "\n> "))
					sb.WriteString("\n> ```\n")
				} else {
					fmt.Fprintf(&sb, "> %s\n", content)
				}
				sb.WriteString("\n")
			}
		}

		// Attachments (BinaryContent)
		binaries := m.BinaryContent()
		if len(binaries) > 0 {
			sb.WriteString("**Attachments**:\n")
			for _, b := range binaries {
				fmt.Fprintf(&sb, "- `%s` (%s)\n", b.Path, b.MIMEType)
			}
			sb.WriteString("\n")
		}
	}

	return sb.String(), nil
}
