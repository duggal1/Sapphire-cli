package chat

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/sapphire/internal/agent/tools"
	"github.com/charmbracelet/sapphire/internal/message"
	"github.com/charmbracelet/sapphire/internal/session"
	"github.com/charmbracelet/sapphire/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

// -----------------------------------------------------------------------------
// Todos Tool
// -----------------------------------------------------------------------------

// TodosToolMessageItem is a message item that represents a todos tool call.
type TodosToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*TodosToolMessageItem)(nil)

// NewTodosToolMessageItem creates a new [TodosToolMessageItem].
func NewTodosToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &TodosToolRenderContext{}, canceled)
}

// TodosToolRenderContext renders todos tool messages.
type TodosToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (t *TodosToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if opts.IsPending() {
		return pendingTool(sty, "To-Do", opts.Anim)
	}

	var params tools.TodosParams
	var meta tools.TodosResponseMetadata
	var headerText string
	var body string
	var items []tools.TodoItem
	metaOK := false

	if opts.HasResult() && opts.Result.Metadata != "" {
		metaOK = json.Unmarshal([]byte(opts.Result.Metadata), &meta) == nil
	}

	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err == nil {
		resolvedCount := 0
		inProgressTask := ""
		items = displayTodoItems(params)
		for _, todo := range items {
			if session.IsTodoTerminalStatus(session.TodoStatus(todo.Status)) {
				resolvedCount++
			}
			if todo.Status == "in_progress" {
				if todo.ActiveForm != "" {
					inProgressTask = todo.ActiveForm
				} else {
					inProgressTask = todo.Content
				}
			}
		}

		if !metaOK && len(items) > 0 {
			ratio := sty.Tool.TodoRatio.Render(fmt.Sprintf("%d/%d", resolvedCount, len(items)))
			headerText = ratio
			if inProgressTask != "" {
				headerText = fmt.Sprintf("%s · %s", ratio, inProgressTask)
			}
		}
	}

	if metaOK {
		if meta.IsNew {
			if meta.JustStarted != "" {
				headerText = fmt.Sprintf("created %d todos, starting first", meta.Total)
			} else {
				headerText = fmt.Sprintf("created %d todos", meta.Total)
			}
			body = FormatTodosList(sty, meta.Todos, styles.ArrowRightIcon, cappedWidth)
		} else {
			// Build header based on what changed.
			hasCompleted := len(meta.JustCompleted) > 0
			hasStarted := meta.JustStarted != ""
			allResolved := meta.Resolved == meta.Total && meta.Total > 0

			ratio := sty.Tool.TodoRatio.Render(fmt.Sprintf("%d/%d", meta.Resolved, meta.Total))
			if hasCompleted && hasStarted {
				text := sty.Subtle.Render(fmt.Sprintf(" · completed %d, starting next", len(meta.JustCompleted)))
				headerText = fmt.Sprintf("%s%s", ratio, text)
			} else if hasCompleted {
				text := sty.Subtle.Render(fmt.Sprintf(" · completed %d", len(meta.JustCompleted)))
				if allResolved {
					text = sty.Subtle.Render(" · resolved all")
				}
				headerText = fmt.Sprintf("%s%s", ratio, text)
			} else if hasStarted {
				headerText = fmt.Sprintf("%s%s", ratio, sty.Subtle.Render(" · starting task"))
			} else {
				headerText = ratio
			}

			if allResolved {
				body = FormatTodosList(sty, meta.Todos, styles.ArrowRightIcon, cappedWidth)
			} else if meta.JustStarted != "" {
				body = sty.Tool.TodoInProgressIcon.Render(styles.ArrowRightIcon+" ") +
					sty.Base.Render(meta.JustStarted)
			}
		}
	}

	if body == "" && len(items) > 0 {
		body = FormatTodosList(sty, todosFromItems(items), styles.ArrowRightIcon, cappedWidth)
	}

	toolParams := []string{headerText}
	header := toolHeader(sty, opts.Status, "To-Do", cappedWidth, opts.Compact, toolParams...)
	if opts.Compact {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(header, earlyState)
	}

	if body == "" {
		return header
	}

	return joinToolParts(header, sty.Tool.Body.Render(body))
}

// FormatTodosList formats a list of todos for display.
func FormatTodosList(sty *styles.Styles, todos []session.Todo, inProgressIcon string, width int) string {
	if len(todos) == 0 {
		return ""
	}

	sorted := make([]session.Todo, len(todos))
	copy(sorted, todos)
	sortTodos(sorted)

	var lines []string
	for _, todo := range sorted {
		if !session.IsRenderableTodo(todo) {
			continue
		}
		var prefix string
		textStyle := sty.Base

		switch todo.Status {
		case session.TodoStatusCompleted:
			prefix = sty.Tool.TodoCompletedIcon.Render(styles.TodoCompletedIcon) + " "
			textStyle = sty.Muted
		case session.TodoStatusInProgress:
			prefix = sty.Tool.TodoInProgressIcon.Render(inProgressIcon + " ")
		case session.TodoStatusFailed:
			prefix = sty.Tool.TodoFailedIcon.Render("× ")
			textStyle = sty.Subtle
		case session.TodoStatusCanceled:
			prefix = sty.Tool.TodoCanceledIcon.Render("− ")
			textStyle = sty.Subtle
		default:
			prefix = sty.Tool.TodoPendingIcon.Render(styles.TodoPendingIcon) + " "
			textStyle = sty.Subtle
		}

		text := todo.Content
		if todo.Status == session.TodoStatusInProgress && todo.ActiveForm != "" {
			text = todo.ActiveForm
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		line := prefix + textStyle.Render(text)
		line = ansi.Truncate(line, width, "…")

		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return ""
	}

	return strings.Join(lines, "\n")
}

func todosFromItems(items []tools.TodoItem) []session.Todo {
	todos := make([]session.Todo, 0, len(items))
	for _, item := range items {
		content := strings.TrimSpace(item.Content)
		activeForm := strings.TrimSpace(item.ActiveForm)
		if content == "" && activeForm == "" {
			continue
		}
		status := session.TodoStatus(item.Status)
		if status == "" {
			status = session.TodoStatusPending
		}
		todos = append(todos, session.Todo{
			Content:    content,
			Status:     status,
			ActiveForm: activeForm,
		})
	}
	return todos
}

func displayTodoItems(params tools.TodosParams) []tools.TodoItem {
	switch {
	case len(params.Tasks) > 0:
		return nonEmptyTodoItems(params.Tasks)
	case len(params.Todos) > 0:
		return nonEmptyTodoItems(params.Todos)
	case params.Task != nil:
		return nonEmptyTodoItems([]tools.TodoItem{*params.Task})
	default:
		return nil
	}
}

func nonEmptyTodoItems(items []tools.TodoItem) []tools.TodoItem {
	out := make([]tools.TodoItem, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Content) == "" && strings.TrimSpace(item.ActiveForm) == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}

// sortTodos sorts todos by status: in_progress, pending, failed/cancelled, completed.
func sortTodos(todos []session.Todo) {
	slices.SortStableFunc(todos, func(a, b session.Todo) int {
		return statusOrder(a.Status) - statusOrder(b.Status)
	})
}

// statusOrder returns the sort order for a todo status.
func statusOrder(s session.TodoStatus) int {
	switch s {
	case session.TodoStatusInProgress:
		return 0
	case session.TodoStatusPending:
		return 1
	case session.TodoStatusFailed:
		return 2
	case session.TodoStatusCanceled:
		return 3
	case session.TodoStatusCompleted:
		return 4
	default:
		return 5
	}
}
