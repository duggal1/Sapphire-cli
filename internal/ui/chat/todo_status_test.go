package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/sapphire/internal/session"
	"github.com/charmbracelet/sapphire/internal/ui/styles"
)

func TestSessionTodoItemRendersRealTodoText(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	item := NewSessionTodoItem(&sty, "session-1", []session.Todo{
		{Content: "Read codebase", Status: session.TodoStatusCompleted},
		{Content: "Run tests", Status: session.TodoStatusInProgress, ActiveForm: "Running tests"},
		{Content: "Fix timeout", Status: session.TodoStatusPending},
	})

	rendered := item.Render(100)
	requireContainsAll(t, rendered, "To-Do", "1/3", "Running tests", "Fix timeout")
}

func TestSessionTodoItemOmitsBlankRows(t *testing.T) {
	t.Parallel()

	sty := styles.DefaultStyles(false)
	item := NewSessionTodoItem(&sty, "session-2", []session.Todo{
		{Content: "", ActiveForm: "", Status: session.TodoStatusPending},
	})

	rendered := item.Render(100)
	if strings.TrimSpace(rendered) != "" {
		t.Fatalf("expected blank todos to render nothing, got %q", rendered)
	}
}
