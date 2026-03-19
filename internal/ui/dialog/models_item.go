package dialog

import (
	"fmt"
	"strings"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/lipgloss/v2"
	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// ModelGroup represents a group of model items.
type ModelGroup struct {
	Title      string
	Items      []*ModelItem
	configured bool
	t          *styles.Styles
}

// NewModelGroup creates a new ModelGroup.
func NewModelGroup(t *styles.Styles, title string, configured bool, items ...*ModelItem) ModelGroup {
	return ModelGroup{
		Title:      title,
		Items:      items,
		configured: configured,
		t:          t,
	}
}

// AppendItems appends [ModelItem]s to the group.
func (m *ModelGroup) AppendItems(items ...*ModelItem) {
	m.Items = append(m.Items, items...)
}

// Render implements [list.Item].
func (m *ModelGroup) Render(width int) string {
	var configured string
	if m.configured {
		configuredIcon := lipgloss.NewStyle().
			Foreground(m.t.GreenLight).
			Background(m.t.BgBaseLighter).
			SetString(styles.ToolSuccess).
			Render()
		configuredText := m.t.HalfMuted.Background(m.t.BgBaseLighter).Render("Configured")
		configured = configuredIcon + " " + configuredText
	}

	title := " " + m.Title + " "
	title = ansi.Truncate(title, max(0, width-lipgloss.Width(configured)-1), "…")

	return renderModelGroupHeader(m.t, title, width, configured)
}

func renderModelGroupHeader(t *styles.Styles, text string, width int, info ...string) string {
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

	titleStyle := t.Section.Title.Background(t.BgBaseLighter)
	lineStyle := t.Section.Line.Foreground(t.Primary).Background(t.BgBaseLighter)
	text = titleStyle.Render(text)
	if remainingWidth > 0 {
		text = text + " " + lineStyle.Render(strings.Repeat(char, remainingWidth)) + infoText
	}
	return text
}

// ModelItem represents a list item for a model type.
type ModelItem struct {
	prov      catwalk.Provider
	model     catwalk.Model
	modelType ModelType

	cache        map[int]string
	t            *styles.Styles
	m            fuzzy.Match
	focused      bool
	showProvider bool
}

// SelectedModel returns this model item as a [config.SelectedModel] instance.
func (m *ModelItem) SelectedModel() config.SelectedModel {
	return config.SelectedModel{
		Model:           m.model.ID,
		Provider:        string(m.prov.ID),
		ReasoningEffort: m.model.DefaultReasoningEffort,
		MaxTokens:       m.model.DefaultMaxTokens,
	}
}

// SelectedModelType returns the type of model represented by this item.
func (m *ModelItem) SelectedModelType() config.SelectedModelType {
	return m.modelType.Config()
}

var _ ListItem = &ModelItem{}

// NewModelItem creates a new ModelItem.
func NewModelItem(t *styles.Styles, prov catwalk.Provider, model catwalk.Model, typ ModelType, showProvider bool) *ModelItem {
	return &ModelItem{
		prov:         prov,
		model:        model,
		modelType:    typ,
		t:            t,
		cache:        make(map[int]string),
		showProvider: showProvider,
	}
}

// Filter implements ListItem.
func (m *ModelItem) Filter() string {
	return m.model.Name
}

// ID implements ListItem.
func (m *ModelItem) ID() string {
	return modelKey(string(m.prov.ID), m.model.ID)
}

// Render implements ListItem.
func (m *ModelItem) Render(width int) string {
	var providerInfo string
	if m.showProvider {
		providerInfo = string(m.prov.Name)
	}

	// Add reasoning level indicator for models that support reasoning
	reasoningIndicator := m.getReasoningIndicator()

	styles := ListItemStyles{
		ItemBlurred:     m.t.Dialog.NormalItem,
		ItemFocused:     m.t.Dialog.SelectedItem,
		InfoTextBlurred: m.t.Base,
		InfoTextFocused: m.t.Base,
	}
	return renderItemWithReasoning(styles, m.model.Name, providerInfo, reasoningIndicator, m.focused, width, m.cache, &m.m)
}

// getReasoningIndicator returns the reasoning level indicator for Gemini and other reasoning models.
func (m *ModelItem) getReasoningIndicator() string {
	// Check if this is a Gemini model
	isGemini := strings.Contains(strings.ToLower(m.model.ID), "gemini")

	if isGemini && m.model.DefaultReasoningEffort != "" {
		effort := formatReasoningEffort(m.model.DefaultReasoningEffort)
		return fmt.Sprintf("[%s]", effort)
	}

	// Also check for other models with reasoning capability
	if m.model.CanReason && m.model.DefaultReasoningEffort != "" {
		effort := formatReasoningEffort(m.model.DefaultReasoningEffort)
		return fmt.Sprintf("[%s]", effort)
	}

	return ""
}

// formatReasoningEffort formats a reasoning effort level for display.
func formatReasoningEffort(effort string) string {
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
		return "Minimal"
	case "thinking_on":
		return "Thinking On"
	case "thinking_off":
		return "Thinking Off"
	default:
		return cases.Title(language.English).String(effort)
	}
}

// SetFocused implements ListItem.
func (m *ModelItem) SetFocused(focused bool) {
	if m.focused != focused {
		m.cache = nil
	}
	m.focused = focused
}

// SetMatch implements ListItem.
func (m *ModelItem) SetMatch(fm fuzzy.Match) {
	m.cache = nil
	m.m = fm
}

// renderItemWithReasoning renders a list item with an optional reasoning indicator.
func renderItemWithReasoning(t ListItemStyles, title string, info string, reasoningIndicator string, focused bool, width int, cache map[int]string, m *fuzzy.Match) string {
	if cache == nil {
		cache = make(map[int]string)
	}

	cached, ok := cache[width]
	if ok {
		return cached
	}

	style := t.ItemBlurred
	if focused {
		style = t.ItemFocused
	}

	var infoText string
	var infoWidth int
	lineWidth := width

	// Add reasoning indicator to info if present
	if reasoningIndicator != "" {
		if info != "" {
			info = info + " " + reasoningIndicator
		} else {
			info = reasoningIndicator
		}
	}

	if len(info) > 0 {
		infoText = fmt.Sprintf(" %s ", info)
		if focused {
			infoText = t.InfoTextFocused.Render(infoText)
		} else {
			infoText = t.InfoTextBlurred.Render(infoText)
		}

		infoWidth = lipgloss.Width(infoText)
	}

	title = ansi.Truncate(title, max(0, lineWidth-infoWidth), "…")
	titleWidth := lipgloss.Width(title)
	gap := strings.Repeat(" ", max(0, lineWidth-titleWidth-infoWidth))
	content := title
	if m != nil && len(m.MatchedIndexes) > 0 {
		var lastPos int
		parts := make([]string, 0)
		ranges := matchedRanges(m.MatchedIndexes)
		for _, rng := range ranges {
			start, stop := bytePosToVisibleCharPos(title, rng)
			if start > lastPos {
				parts = append(parts, ansi.Cut(title, lastPos, start))
			}
			parts = append(parts,
				ansi.NewStyle().Underline(true).String(),
				ansi.Cut(title, start, stop+1),
				ansi.NewStyle().Underline(false).String(),
			)
			lastPos = stop + 1
		}
		if lastPos < ansi.StringWidth(title) {
			parts = append(parts, ansi.Cut(title, lastPos, ansi.StringWidth(title)))
		}

		content = strings.Join(parts, "")
	}

	content = style.Render(content + gap + infoText)
	cache[width] = content
	return content
}
