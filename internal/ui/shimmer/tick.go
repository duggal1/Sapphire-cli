package shimmer

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

type ShimmerTickMsg time.Time

const shimmerTickInterval = time.Second / 80

func ShimmerTickCmd() tea.Cmd {
	return tea.Tick(shimmerTickInterval, func(t time.Time) tea.Msg {
		return ShimmerTickMsg(t)
	})
}
