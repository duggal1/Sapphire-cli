package chat

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/sapphire/internal/session"
	"github.com/charmbracelet/sapphire/internal/ui/styles"
)

// SessionTodoID returns the stable ID for the session-scoped todo item.
func SessionTodoID(sessionID string) string {
	return fmt.Sprintf("%s:session-todos", sessionID)
}

// SessionTodoItem renders the current session todo list as a single persistent item.
type SessionTodoItem struct {
	*cachedMessageItem

	id    string
	sty   *styles.Styles
	todos []session.Todo
}

// NewSessionTodoItem creates a session-scoped todo item.
func NewSessionTodoItem(sty *styles.Styles, sessionID string, todos []session.Todo) MessageItem {
	item := &SessionTodoItem{
		cachedMessageItem: &cachedMessageItem{},
		id:                SessionTodoID(sessionID),
		sty:               sty,
	}
	item.SetTodos(todos)
	return item
}

// ID implements MessageItem.
func (t *SessionTodoItem) ID() string {
	return t.id
}

// SetTodos updates the rendered todo list.
func (t *SessionTodoItem) SetTodos(todos []session.Todo) {
	t.todos = sanitizedRenderableTodos(todos)
	t.clearCache()
}

// RawRender implements MessageItem.
func (t *SessionTodoItem) RawRender(width int) string {
	innerWidth := max(0, width-MessageLeftPaddingTotal)
	content, _, ok := t.getCachedRender(innerWidth)
	if ok {
		return content
	}

	content = t.renderContent(innerWidth)
	t.setCachedRender(content, innerWidth, strings.Count(content, "\n")+1)
	return content
}

// Render implements MessageItem.
func (t *SessionTodoItem) Render(width int) string {
	raw := t.RawRender(width)
	if raw == "" {
		return ""
	}
	prefix := t.sty.Chat.Message.SectionHeader.Render()
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func (t *SessionTodoItem) renderContent(width int) string {
	if len(t.todos) == 0 {
		return ""
	}

	total := len(t.todos)
	resolved := 0
	inProgressText := ""
	for _, todo := range t.todos {
		switch todo.Status {
		case session.TodoStatusCompleted, session.TodoStatusFailed, session.TodoStatusCanceled:
			resolved++
		case session.TodoStatusInProgress:
			if inProgressText == "" {
				inProgressText = strings.TrimSpace(todo.ActiveForm)
				if inProgressText == "" {
					inProgressText = strings.TrimSpace(todo.Content)
				}
			}
		}
	}

	headerText := t.sty.Tool.TodoRatio.Render(fmt.Sprintf("%d/%d", resolved, total))
	if inProgressText != "" {
		headerText = fmt.Sprintf("%s · %s", headerText, inProgressText)
	}

	body := FormatTodosList(t.sty, t.todos, styles.ArrowRightIcon, cappedMessageWidth(width))
	status := ToolStatusRunning
	if resolved == total {
		status = ToolStatusSuccess
	}
	header := toolHeader(t.sty, status, "To-Do", cappedMessageWidth(width), false, headerText)
	if body == "" {
		return header
	}
	return joinToolParts(header, t.sty.Tool.Body.Render(body))
}

func sanitizedRenderableTodos(todos []session.Todo) []session.Todo {
	if len(todos) == 0 {
		return nil
	}

	out := make([]session.Todo, 0, len(todos))
	for _, todo := range todos {
		todo.Content = strings.TrimSpace(todo.Content)
		todo.ActiveForm = strings.TrimSpace(todo.ActiveForm)
		if !session.IsRenderableTodo(todo) {
			continue
		}
		out = append(out, todo)
	}
	return out
}
