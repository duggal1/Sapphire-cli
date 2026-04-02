package model

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/duggal1/Sapphire-cli/internal/ui/chat"
)

const cancelConfirmationWindow = 2 * time.Second

type cancelTimerExpiredMsg struct{}

// cancelAgent handles the cancel key press. The first press sets isCanceling to true
// and starts a timer. The second press (before the timer expires) actually
// cancels the agent.
func (m *UI) cancelAgent() tea.Cmd {
	if !m.hasSession() {
		return nil
	}

	coordinator := m.com.App.AgentCoordinator
	if coordinator == nil {
		return nil
	}

	if m.isCanceling {
		// Second escape press - actually cancel the agent.
		m.isCanceling = false
		m.clearDeepPlanningState(false)
		m.fixedTailNotice = nil
		coordinator.Cancel(m.session.ID)
		// Stop the spinning todo indicator.
		m.todoIsSpinning = false
		m.renderPills()
		m.updateLayoutAndSize()
		return nil
	}

	// Check if there are queued prompts - if so, clear the queue.
	if coordinator.QueuedPrompts(m.session.ID) > 0 {
		coordinator.ClearQueue(m.session.ID)
		return nil
	}

	// First escape press - set canceling state and start timer.
	m.isCanceling = true
	m.fixedTailNotice = chat.NewInterruptNoticeMessageItem(m.com.Styles)
	m.updateLayoutAndSize()
	return tea.Batch(
		m.chat.ScrollToBottomAndAnimate(),
		tea.Tick(cancelConfirmationWindow, func(time.Time) tea.Msg {
			return cancelTimerExpiredMsg{}
		}),
	)
}
