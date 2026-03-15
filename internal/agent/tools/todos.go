package tools

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/sapphire/internal/session"
)

//go:embed todos.md
var todosDescription []byte

const TodosToolName = "todos"

type TodosParams struct {
	Todos []TodoItem `json:"todos" description:"The updated todo list"`
}

type TodoItem struct {
	Content    string `json:"content" description:"What needs to be done (imperative form)"`
	Status     string `json:"status" description:"Task status: pending, in_progress, or completed"`
	ActiveForm string `json:"active_form" description:"Present continuous form (e.g., 'Running tests')"`
}

type TodosResponseMetadata struct {
	IsNew         bool           `json:"is_new"`
	Todos         []session.Todo `json:"todos"`
	JustCompleted []string       `json:"just_completed,omitempty"`
	JustStarted   string         `json:"just_started,omitempty"`
	Completed     int            `json:"completed"`
	Total         int            `json:"total"`
}

func NewTodosTool(sessions session.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		TodosToolName,
		string(todosDescription),
		func(ctx context.Context, params map[string]any, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			_ = call
			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for managing todos")
			}

			currentSession, err := sessions.Get(ctx, sessionID)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to get session: %w", err)
			}

			if params == nil {
				params = map[string]any{}
			}
			normalizeTodosInput(params)

			var typed TodosParams
			if err := decodeInto(params, &typed); err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("invalid parameters: %s", err)), nil
			}

			isNew := len(currentSession.Todos) == 0
			oldStatusByContent := make(map[string]session.TodoStatus, len(currentSession.Todos))
			for _, todo := range currentSession.Todos {
				content := strings.TrimSpace(todo.Content)
				if content == "" {
					continue
				}
				oldStatusByContent[content] = todo.Status
			}

			todos := make([]session.Todo, 0, len(typed.Todos))
			var (
				justCompleted []string
				justStarted   string
				completed     int
			)

			for _, item := range typed.Todos {
				content := strings.TrimSpace(item.Content)
				activeForm := strings.TrimSpace(item.ActiveForm)
				if content == "" && activeForm == "" {
					continue
				}

				status := normalizeTodoStatus(item.Status)
				if status == "" {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("invalid status %q for todo %q", item.Status, content)), nil
				}

				todo := session.Todo{
					Content:    content,
					Status:     status,
					ActiveForm: activeForm,
				}
				todos = append(todos, todo)

				oldStatus, existed := oldStatusByContent[content]
				if status == session.TodoStatusCompleted {
					completed++
					if existed && oldStatus != session.TodoStatusCompleted {
						justCompleted = append(justCompleted, content)
					}
				}
				if status == session.TodoStatusInProgress && (!existed || oldStatus != session.TodoStatusInProgress) {
					if activeForm != "" {
						justStarted = activeForm
					} else {
						justStarted = content
					}
				}
			}

			currentSession.Todos = todos
			if _, err := sessions.Save(ctx, currentSession); err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to save todos: %w", err)
			}

			pendingCount := 0
			inProgressCount := 0
			for _, todo := range todos {
				switch todo.Status {
				case session.TodoStatusPending:
					pendingCount++
				case session.TodoStatusInProgress:
					inProgressCount++
				}
			}

			response := "Todo list updated successfully.\n\n"
			response += fmt.Sprintf("Status: %d pending, %d in progress, %d completed\n", pendingCount, inProgressCount, completed)
			response += "Todos have been modified successfully. Ensure that you continue to use the todo list to track your progress. Please proceed with the current tasks if applicable."

			metadata := TodosResponseMetadata{
				IsNew:         isNew,
				Todos:         todos,
				JustCompleted: justCompleted,
				JustStarted:   justStarted,
				Completed:     completed,
				Total:         len(todos),
			}

			return fantasy.WithResponseMetadata(fantasy.NewTextResponse(response), metadata), nil
		},
	)
}

func normalizeTodoStatus(raw string) session.TodoStatus {
	switch strings.TrimSpace(raw) {
	case string(session.TodoStatusPending):
		return session.TodoStatusPending
	case string(session.TodoStatusInProgress):
		return session.TodoStatusInProgress
	case string(session.TodoStatusCompleted):
		return session.TodoStatusCompleted
	default:
		return ""
	}
}
