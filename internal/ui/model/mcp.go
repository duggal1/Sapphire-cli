package model

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/sapphire/internal/agent/tools/mcp"
	"github.com/charmbracelet/sapphire/internal/config"
	"github.com/charmbracelet/sapphire/internal/ui/common"
	"github.com/charmbracelet/sapphire/internal/ui/styles"
)

// mcpInfo renders the MCP status section showing active MCP clients and their
// tool/prompt counts.
func (m *UI) mcpInfo(width, maxItems int, isSection bool) string {
	var mcps []mcp.ClientInfo
	t := m.com.Styles
	active := 0

	for _, entry := range m.com.Config().MCP.Sorted() {
		if state, ok := m.mcpStates[entry.Name]; ok {
			if state.State == mcp.StateConnected {
				active++
			}
			mcps = append(mcps, state)
			continue
		}
		status := mcp.StateDisabled
		if entry.MCP.Disabled {
			status = mcp.StateDisabled
		}
		mcps = append(mcps, mcp.ClientInfo{
			Name:   entry.Name,
			State:  status,
			Counts: mcp.Counts{},
		})
	}

	title := t.ResourceGroupTitle.Render("MCPs")
	if isSection {
		title = common.Section(t, title, width)
	}
	totalKnown := len(config.RegistryMCPDefinitions)
	installed := len(mcps)
	summary := renderMCPSummary(t, width, active, installed, totalKnown, mcpVisibleCount(installed, maxItems))
	list := t.ResourceAdditionalText.Render("No MCP servers installed")
	if len(mcps) > 0 {
		list = mcpList(t, mcps, width, maxItems)
	}

	return lipgloss.NewStyle().Width(width).Render(fmt.Sprintf("%s\n\n%s\n\n%s", title, summary, list))
}

// mcpCounts formats tool, prompt, and resource counts for display.
func mcpCounts(t *styles.Styles, counts mcp.Counts) string {
	var parts []string
	if counts.Tools > 0 {
		parts = append(parts, t.Subtle.Render(fmt.Sprintf("%d tools", counts.Tools)))
	}
	if counts.Prompts > 0 {
		parts = append(parts, t.Subtle.Render(fmt.Sprintf("%d prompts", counts.Prompts)))
	}
	if counts.Resources > 0 {
		parts = append(parts, t.Subtle.Render(fmt.Sprintf("%d resources", counts.Resources)))
	}
	return strings.Join(parts, " ")
}

// mcpList renders a list of MCP clients with their status and counts,
// showing at most five items and summarizing the remainder.
func mcpList(t *styles.Styles, mcps []mcp.ClientInfo, width, maxItems int) string {
	if maxItems <= 0 {
		return ""
	}
	slices.SortFunc(mcps, func(a, b mcp.ClientInfo) int {
		if pa := mcpStatePriority(a.State); pa != mcpStatePriority(b.State) {
			return mcpStatePriority(a.State) - mcpStatePriority(b.State)
		}
		return strings.Compare(a.Name, b.Name)
	})

	maxVisible := mcpVisibleCount(len(mcps), maxItems)
	var renderedMcps []string

	for i, m := range mcps {
		if i >= maxVisible {
			break
		}
		var icon string
		title := t.ResourceName.Render(m.Name)
		var description string
		var extraContent string

		switch m.State {
		case mcp.StateStarting:
			icon = t.ResourceBusyIcon.String()
			description = t.ResourceStatus.Render("starting")
		case mcp.StateConnected:
			icon = t.ResourceOnlineIcon.String()
			extraContent = mcpCounts(t, m.Counts)
		case mcp.StateError:
			icon = t.ResourceErrorIcon.String()
			description = t.ResourceStatus.Render("error")
			if m.Error != nil {
				description = t.ResourceStatus.Render(fmt.Sprintf("error: %s", m.Error.Error()))
			}
		case mcp.StateDisabled:
			icon = t.ResourceOfflineIcon.String()
			description = t.ResourceStatus.Render("disconnected")
		default:
			icon = t.ResourceOfflineIcon.String()
			description = t.ResourceStatus.Render("disconnected")
		}

		renderedMcps = append(renderedMcps, common.Status(t, common.StatusOpts{
			Icon:         icon,
			Title:        title,
			Description:  description,
			ExtraContent: extraContent,
		}, width))
	}

	remaining := len(mcps) - maxVisible
	if remaining > 0 {
		renderedMcps = append(renderedMcps, t.ResourceAdditionalText.Render(fmt.Sprintf("%d more hidden", remaining)))
	}
	return lipgloss.JoinVertical(lipgloss.Left, renderedMcps...)
}

func renderMCPSummary(t *styles.Styles, width, active, installed, total, visible int) string {
	metric := func(label string, value int) string {
		return fmt.Sprintf(
			"%s %s",
			t.ResourceStatus.Render(label+":"),
			t.Base.Foreground(t.Tertiary).Render(fmt.Sprintf("%d", value)),
		)
	}

	visibleSummary := fmt.Sprintf(
		"%s %s",
		t.ResourceStatus.Render("MCPs:"),
		t.Base.Foreground(t.Tertiary).Render(fmt.Sprintf("%d shown", visible)),
	)
	if hidden := installed - visible; hidden > 0 {
		visibleSummary += t.ResourceAdditionalText.Render(fmt.Sprintf(" · %d more", hidden))
	}

	top := strings.Join([]string{
		metric("Active", active),
		metric("Installed", installed),
		metric("Total", total),
	}, t.ResourceAdditionalText.Render(" · "))

	return lipgloss.NewStyle().Width(width).Render(
		lipgloss.JoinVertical(lipgloss.Left, top, visibleSummary),
	)
}

func mcpVisibleCount(total, maxItems int) int {
	if total <= 0 || maxItems <= 0 {
		return 0
	}
	return minInt(total, minInt(5, maxItems))
}

func mcpStatePriority(state mcp.State) int {
	switch state {
	case mcp.StateConnected:
		return 0
	case mcp.StateStarting:
		return 1
	case mcp.StateError:
		return 2
	default:
		return 3
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
