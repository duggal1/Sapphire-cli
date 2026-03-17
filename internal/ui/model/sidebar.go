package model

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/sapphire/internal/config"
	"github.com/charmbracelet/sapphire/internal/ui/common"
	"github.com/charmbracelet/sapphire/internal/ui/logo"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/ultraviolet/layout"
)

// formatReasoningEffortBrackets formats a reasoning effort level for display.
func formatReasoningEffortBrackets(effort string) string {
	switch effort {
	case "xhigh":
		return "X-High"
	case "high":
		return "High"
	case "medium":
		return "Medium"
	case "low":
		return "Low"
	case "minimal":
		return "minimal"
	case "thinking_on":
		return "Thinking On"
	case "thinking_off":
		return "Thinking Off"
	default:
		return effort
	}
}

// modelInfo renders current model reasoning settings and context usage for the sidebar.
func (m *UI) modelInfo(width int) string {
	model := m.selectedLargeModel()
	reasoningInfo := ""
	providerName := ""

	if model != nil {
		// Get provider name first
		providerConfig, ok := m.com.Config().Providers.Get(model.ModelCfg.Provider)
		if ok {
			providerName = providerConfig.Name

			// Only check reasoning if model can reason
			if model.CatwalkCfg.CanReason {
				reasoningChoices := config.ReasoningChoicesForModel(&model.CatwalkCfg)
				if len(reasoningChoices) == 0 {
					if model.ModelCfg.Think {
						reasoningInfo = "Thinking On"
					} else {
						reasoningInfo = "Thinking Off"
					}
				} else {
					reasoningEffort := config.CurrentReasoningSelection(&model.CatwalkCfg, model.ModelCfg)
					// Use bracket format for Gemini models
					isGemini := strings.Contains(strings.ToLower(model.CatwalkCfg.ID), "gemini")
					if isGemini {
						reasoningInfo = formatReasoningEffortBrackets(reasoningEffort)
					} else {
						reasoningInfo = fmt.Sprintf("Reasoning %s", common.FormatReasoningEffort(reasoningEffort))
					}
				}
			}
		}
	}

	var modelContext *common.ModelContextInfo
	if model != nil && m.session != nil {
		modelContext = &common.ModelContextInfo{
			ContextUsed:  m.session.CompletionTokens + m.session.PromptTokens,
			Cost:         m.session.Cost,
			ModelContext: model.CatwalkCfg.ContextWindow,
		}
	}
	return common.ModelInfo(m.com.Styles, model.CatwalkCfg.Name, providerName, reasoningInfo, modelContext, width)
}

// getDynamicHeightLimits will give us the num of items to show in each section based on the hight
// some items are more important than others.
func getDynamicHeightLimits(availableHeight int) (maxFiles, maxLSPs, maxMCPs int) {
	const (
		minItemsPerSection      = 2
		defaultMaxFilesShown    = 10
		defaultMaxLSPsShown     = 8
		defaultMaxMCPsShown     = 8
		minAvailableHeightLimit = 10
	)

	// If we have very little space, use minimum values
	if availableHeight < minAvailableHeightLimit {
		return minItemsPerSection, minItemsPerSection, minItemsPerSection
	}

	// Distribute available height among the three sections
	// Give priority to files, then LSPs, then MCPs
	totalSections := 3
	heightPerSection := availableHeight / totalSections

	// Calculate limits for each section, ensuring minimums
	maxFiles = max(minItemsPerSection, min(defaultMaxFilesShown, heightPerSection))
	maxLSPs = max(minItemsPerSection, min(defaultMaxLSPsShown, heightPerSection))
	maxMCPs = max(minItemsPerSection, min(defaultMaxMCPsShown, heightPerSection))

	// If we have extra space, give it to files first
	remainingHeight := availableHeight - (maxFiles + maxLSPs + maxMCPs)
	if remainingHeight > 0 {
		extraForFiles := min(remainingHeight, defaultMaxFilesShown-maxFiles)
		maxFiles += extraForFiles
		remainingHeight -= extraForFiles

		if remainingHeight > 0 {
			extraForLSPs := min(remainingHeight, defaultMaxLSPsShown-maxLSPs)
			maxLSPs += extraForLSPs
			remainingHeight -= extraForLSPs

			if remainingHeight > 0 {
				maxMCPs += min(remainingHeight, defaultMaxMCPsShown-maxMCPs)
			}
		}
	}

	return maxFiles, maxLSPs, maxMCPs
}

// sidebar renders the chat sidebar containing session title, working
// directory, model info, file list, LSP status, and MCP status.
func (m *UI) drawSidebar(scr uv.Screen, area uv.Rectangle) {
	if m.session == nil {
		return
	}

	const logoHeightBreakpoint = 30

	t := m.com.Styles
	width := area.Dx()
	height := area.Dy()

	title := t.Muted.Width(width).MaxHeight(2).Render(m.session.Title)
	cwd := common.PrettyPath(t, m.com.Config().WorkingDir(), width)
	sidebarLogo := m.sidebarLogo
	if height < logoHeightBreakpoint {
		sidebarLogo = logo.SmallRender(m.com.Styles, width)
	}
	blocks := []string{
		sidebarLogo,
		title,
		"",
		cwd,
		"",
		m.modelInfo(width),
		"",
	}

	sidebarHeader := lipgloss.JoinVertical(
		lipgloss.Left,
		blocks...,
	)

	_, remainingHeightArea := layout.SplitVertical(m.layout.sidebar, layout.Fixed(lipgloss.Height(sidebarHeader)))
	remainingHeight := remainingHeightArea.Dy() - 10
	maxFiles, maxLSPs, maxMCPs := getDynamicHeightLimits(remainingHeight)

	lspSection := m.lspInfo(width, maxLSPs, true)
	mcpSection := m.mcpInfo(width, maxMCPs, true)
	filesSection := m.filesInfo(m.com.Config().WorkingDir(), width, maxFiles, true)

	uv.NewStyledString(
		lipgloss.NewStyle().
			MaxWidth(width).
			MaxHeight(height).
			Render(
				lipgloss.JoinVertical(
					lipgloss.Left,
					sidebarHeader,
					filesSection,
					"",
					lspSection,
					"",
					mcpSection,
				),
			),
	).Draw(scr, area)
}
