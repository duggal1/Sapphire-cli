package model

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/sapphire/internal/session"
	"github.com/charmbracelet/sapphire/internal/ui/chat"
)

func (m *UI) syncSessionTodoItem() tea.Cmd {
	if m.chat == nil || m.session == nil {
		return nil
	}

	itemID := chat.SessionTodoID(m.session.ID)
	if !hasRenderableSessionTodos(m.session.Todos) {
		if m.chat.MessageItem(itemID) != nil {
			m.chat.RemoveMessage(itemID)
			m.updateLayoutAndSize()
		}
		return nil
	}
	item := chat.NewSessionTodoItem(m.com.Styles, m.session.ID, m.session.Todos)

	if existing := m.chat.MessageItem(itemID); existing != nil {
		if current, ok := existing.(*chat.SessionTodoItem); ok {
			current.SetTodos(m.session.Todos)
			m.chat.InvalidateMessage(itemID)
			m.updateLayoutAndSize()
			return nil
		}
		m.chat.RemoveMessage(itemID)
	}

	m.chat.AppendMessages(item)
	m.updateLayoutAndSize()
	if m.chat.Follow() {
		return m.chat.ScrollToBottomAndAnimate()
	}
	return nil
}

func hasRenderableSessionTodos(todos []session.Todo) bool {
	for _, todo := range todos {
		if session.IsRenderableTodo(todo) {
			return true
		}
	}
	return false
}
