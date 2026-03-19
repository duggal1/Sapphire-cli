package common

import (
	"cmp"
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/duggal1/Sapphire-cli/internal/home"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// PrettyPath formats a file path with home directory shortening and applies muted styling.
func PrettyPath(t *styles.Styles, path string, width int) string {
	formatted := home.Short(path)
	return t.Muted.Width(width).Render(formatted)
}

// FormatReasoningEffort formats a reasoning effort level for display.
func FormatReasoningEffort(effort string) string {
	if effort == "xhigh" {
		return "X-High"
	}
	switch effort {
	case "thinking_on":
		return "Thinking On"
	case "thinking_off":
		return "Thinking Off"
	}
	return cases.Title(language.English).String(effort)
}

// ModelContextInfo contains token usage and cost information for a model.
type ModelContextInfo struct {
	ContextUsed  int64
	ModelContext int64
	Cost         float64
}

// ModelInfo renders model information including name, reasoning
// settings, and optional context usage/cost.
func ModelInfo(t *styles.Styles, modelName, providerName, reasoningInfo string, context *ModelContextInfo, width int) string {
	modelIcon := t.Subtle.Render(styles.ModelIcon)
	modelName = t.Base.Foreground(t.Primary).Render(modelName)

	// Build first line with model name only (provider removed for minimal footer)
	firstLine := fmt.Sprintf("%s %s", modelIcon, modelName)
	parts := []string{firstLine}

	if reasoningInfo != "" {
		parts = append(parts, t.Subtle.PaddingLeft(2).Render(reasoningInfo))
	}

	if context != nil {
		formattedInfo := formatTokensAndCost(t, context.ContextUsed, context.ModelContext, context.Cost)
		parts = append(parts, lipgloss.NewStyle().PaddingLeft(2).Render(formattedInfo))
	}

	return lipgloss.NewStyle().Width(width).Render(
		lipgloss.JoinVertical(lipgloss.Left, parts...),
	)
}

// formatTokensAndCost formats token usage and cost with appropriate units
// (K/M) and percentage of context window.
func formatTokensAndCost(t *styles.Styles, tokens, contextWindow int64, cost float64) string {
	// Format tokens with comma separators
	formattedTokens := formatNumberWithCommas(tokens)

	percentage := (float64(tokens) / float64(contextWindow)) * 100

	// Format cost in green to stand out
	formattedCost := t.Base.Foreground(t.Green).Render(fmt.Sprintf("$%.2f", cost))

	formattedTokens = t.Base.Foreground(t.Tertiary).Render(fmt.Sprintf("(%s)", formattedTokens))
	formattedPercentage := t.Muted.Render(fmt.Sprintf("%d%%", int(percentage)))
	formattedTokens = fmt.Sprintf("%s %s", formattedPercentage, formattedTokens)
	if percentage > 80 {
		formattedTokens = fmt.Sprintf("%s %s", styles.LSPWarningIcon, formattedTokens)
	}

	return fmt.Sprintf("%s %s", formattedTokens, formattedCost)
}

// formatNumberWithCommas formats an integer with comma separators (e.g., 65,000).
func formatNumberWithCommas(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}

	// Handle negative numbers
	negative := n < 0
	if negative {
		n = -n
	}

	var result []byte
	for i, digit := range fmt.Sprintf("%d", n) {
		if i > 0 && (len(fmt.Sprintf("%d", n))-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(digit))
	}

	if negative {
		return "-" + string(result)
	}
	return string(result)
}

// StatusOpts defines options for rendering a status line with icon, title,
// description, and optional extra content.
type StatusOpts struct {
	Icon             string // if empty no icon will be shown
	Title            string
	TitleColor       color.Color
	Description      string
	DescriptionColor color.Color
	ExtraContent     string // additional content to append after the description
}

// Status renders a status line with icon, title, description, and extra
// content. The description is truncated if it exceeds the available width.
func Status(t *styles.Styles, opts StatusOpts, width int) string {
	icon := opts.Icon
	title := opts.Title
	description := opts.Description

	titleColor := cmp.Or(opts.TitleColor, t.Muted.GetForeground())
	descriptionColor := cmp.Or(opts.DescriptionColor, t.Subtle.GetForeground())

	title = t.Base.Foreground(titleColor).Render(title)

	if description != "" {
		extraContentWidth := lipgloss.Width(opts.ExtraContent)
		if extraContentWidth > 0 {
			extraContentWidth += 1
		}
		description = ansi.Truncate(description, width-lipgloss.Width(icon)-lipgloss.Width(title)-2-extraContentWidth, "…")
		description = t.Base.Foreground(descriptionColor).Render(description)
	}

	var content []string
	if icon != "" {
		content = append(content, icon)
	}
	content = append(content, title)
	if description != "" {
		content = append(content, description)
	}
	if opts.ExtraContent != "" {
		content = append(content, opts.ExtraContent)
	}

	return strings.Join(content, " ")
}

// Section renders a section header with a title and a horizontal line filling
// the remaining width.
func Section(t *styles.Styles, text string, width int, info ...string) string {
	char := styles.SectionSeparator
	length := lipgloss.Width(text) + 1
	remainingWidth := width - length

	var infoText string
	if len(info) > 0 {
		infoText = strings.Join(info, " ")
		if len(infoText) > 0 {
			infoText = " " + infoText
			remainingWidth -= lipgloss.Width(infoText)
		}
	}

	text = t.Section.Title.Render(text)
	if remainingWidth > 0 {
		text = text + " " + t.Section.Line.Render(strings.Repeat(char, remainingWidth)) + infoText
	}
	return text
}

// DialogTitle renders a dialog title with a decorative line filling the
// remaining width.
func DialogTitle(t *styles.Styles, title string, width int, fromColor, toColor color.Color) string {
	char := "╱"
	length := lipgloss.Width(title) + 1
	remainingWidth := width - length
	if remainingWidth > 0 {
		lines := strings.Repeat(char, remainingWidth)
		lines = styles.ApplyForegroundGrad(t, lines, fromColor, toColor)
		title = title + " " + lines
	}
	return title
}
