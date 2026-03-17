// COMMENTED OUT - TODO LIST UI CODE DISABLED
// Codex update_plan tool is used for logic, but Sapphire UI/UX is restored

package chat

import (
	"slices"
	"strings"

	"github.com/charmbracelet/sapphire/internal/session"
	"github.com/charmbracelet/sapphire/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

// RESTORED FOR UI/UX - Used by pills.go
func FormatTodosList(sty *styles.Styles, todos []session.Todo, inProgressIcon string, width int) string {
	if len(todos) == 0 {
		return ""
	}

	sorted := make([]session.Todo, len(todos))
	copy(sorted, todos)
	sortTodos(sorted)

	var lines []string
	for _, todo := range sorted {
		var prefix string
		textStyle := sty.Base

		switch todo.Status {
		case session.TodoStatusCompleted:
			prefix = sty.Tool.TodoCompletedIcon.Render(styles.TodoCompletedIcon) + " "
		case session.TodoStatusInProgress:
			prefix = sty.Tool.TodoInProgressIcon.Render(inProgressIcon + " ")
		default:
			prefix = sty.Tool.TodoPendingIcon.Render(styles.TodoPendingIcon) + " "
		}

		text := todo.Content
		if todo.Status == session.TodoStatusInProgress && todo.ActiveForm != "" {
			text = todo.ActiveForm
		}
		line := prefix + textStyle.Render(text)
		line = ansi.Truncate(line, width, "…")

		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// sortTodos sorts todos by status: completed, in_progress, pending.
func sortTodos(todos []session.Todo) {
	slices.SortStableFunc(todos, func(a, b session.Todo) int {
		return statusOrder(a.Status) - statusOrder(b.Status)
	})
}

// statusOrder returns the sort order for a todo status.
func statusOrder(s session.TodoStatus) int {
	switch s {
	case session.TodoStatusCompleted:
		return 0
	case session.TodoStatusInProgress:
		return 1
	default:
		return 2
	}
}
