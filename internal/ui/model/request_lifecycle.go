package model

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

const cancelConfirmationWindow = 2 * time.Second

type requestCancelExpiredMsg struct {
	token uint64
}

type requestLifecycle struct {
	startedAt   time.Time
	completedAt time.Time
	cancelArmed bool
	cancelToken uint64
}

func (m *UI) requestTiming() (time.Time, time.Time) {
	return m.requestLifecycle.startedAt, m.requestLifecycle.completedAt
}

func (m *UI) resetRequestTimer() {
	m.requestLifecycle.startedAt = time.Time{}
	m.requestLifecycle.completedAt = time.Time{}
	m.syncRequestTiming()
}

func (m *UI) startRequestTimer() {
	if !m.requestLifecycle.startedAt.IsZero() && m.requestLifecycle.completedAt.IsZero() {
		return
	}
	m.requestLifecycle.startedAt = time.Now()
	m.requestLifecycle.completedAt = time.Time{}
	m.syncRequestTiming()
}

func (m *UI) completeRequestTimer() {
	if m.requestLifecycle.startedAt.IsZero() || !m.requestLifecycle.completedAt.IsZero() {
		return
	}
	m.requestLifecycle.completedAt = time.Now()
	m.syncRequestTiming()
}

func (m *UI) syncRequestTiming() {
	if m.assistantFooter == nil {
		return
	}
	m.assistantFooter.SetRequestTiming(m.requestLifecycle.startedAt, m.requestLifecycle.completedAt)
}

func (m *UI) armCancelConfirmation() tea.Cmd {
	m.requestLifecycle.cancelArmed = true
	m.requestLifecycle.cancelToken++
	token := m.requestLifecycle.cancelToken
	return tea.Tick(cancelConfirmationWindow, func(time.Time) tea.Msg {
		return requestCancelExpiredMsg{token: token}
	})
}

func (m *UI) clearCancelConfirmation() {
	m.requestLifecycle.cancelArmed = false
	m.requestLifecycle.cancelToken++
}

func (m *UI) handleCancelExpiry(msg requestCancelExpiredMsg) {
	if msg.token != m.requestLifecycle.cancelToken {
		return
	}
	m.requestLifecycle.cancelArmed = false
}

func (m *UI) cancelAgent() tea.Cmd {
	if !m.hasSession() {
		return nil
	}

	coordinator := m.com.App.AgentCoordinator
	if coordinator == nil {
		return nil
	}

	if m.requestLifecycle.cancelArmed {
		m.clearCancelConfirmation()
		coordinator.Cancel(m.session.ID)
		m.completeRequestTimer()
		m.todoIsSpinning = false
		m.renderPills()
		return nil
	}

	if coordinator.QueuedPrompts(m.session.ID) > 0 {
		coordinator.ClearQueue(m.session.ID)
		return nil
	}

	return m.armCancelConfirmation()
}
