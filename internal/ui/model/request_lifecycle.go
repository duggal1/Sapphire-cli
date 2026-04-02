package model

import tea "charm.land/bubbletea/v2"

type cancelTimerExpiredMsg struct{}

// cancelAgent interrupts the active request immediately and clears queued prompts
// in the same keypress.
func (m *UI) cancelAgent() tea.Cmd {
	if !m.hasSession() {
		return nil
	}

	coordinator := m.com.App.AgentCoordinator
	if coordinator == nil {
		return nil
	}

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
