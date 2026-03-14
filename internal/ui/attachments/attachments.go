package attachments

import (
	"fmt"
	"image/color"
	"math"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/sapphire/internal/message"
	"github.com/charmbracelet/x/ansi"
)

const maxFilename = 15
const pasteTextIcon = "≡"

// greenBadge is a pre-styled green background for the hamburger icon
var greenBadge = lipgloss.NewStyle().Background(lipgloss.Color("#22c55e")).Foreground(lipgloss.Color("#ffffff")).Padding(0, 1)

type Keymap struct {
	DeleteMode,
	DeleteAll,
	TogglePasteEdit,
	PastePrev,
	PasteNext,
	DeleteSelectedPaste,
	Escape key.Binding
}

func New(renderer *Renderer, keyMap Keymap) *Attachments {
	return &Attachments{
		keyMap:   keyMap,
		renderer: renderer,
	}
}

type Attachments struct {
	renderer *Renderer
	keyMap   Keymap
	list     []message.Attachment
	deleting bool
	editing  bool
	selected int
}

func (m *Attachments) List() []message.Attachment { return m.list }
func (m *Attachments) Reset() {
	m.list = nil
	m.deleting = false
	m.editing = false
	m.selected = -1
}

func (m *Attachments) HasPasteBlocks() bool {
	return len(m.pasteIndices()) > 0
}

func (m *Attachments) EditingPasteBlocks() bool {
	return m.editing
}

func (m *Attachments) Update(msg tea.Msg) bool {
	switch msg := msg.(type) {
	case message.Attachment:
		m.list = append(m.list, msg)
		m.reconcileSelection()
		return true
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.TogglePasteEdit):
			if !m.HasPasteBlocks() {
				return false
			}
			m.deleting = false
			m.editing = !m.editing
			if m.editing {
				m.reconcileSelection()
			} else {
				m.selected = -1
			}
			return true
		case m.editing && key.Matches(msg, m.keyMap.Escape):
			m.editing = false
			m.selected = -1
			return true
		case m.editing && key.Matches(msg, m.keyMap.PastePrev):
			m.moveSelection(-1)
			return true
		case m.editing && key.Matches(msg, m.keyMap.PasteNext):
			m.moveSelection(1)
			return true
		case m.editing && key.Matches(msg, m.keyMap.DeleteSelectedPaste):
			if m.selected >= 0 && m.selected < len(m.list) {
				m.list = slices.Delete(m.list, m.selected, m.selected+1)
				m.reconcileSelection()
			}
			return true
		case key.Matches(msg, m.keyMap.DeleteMode):
			if len(m.list) > 0 {
				m.deleting = true
			}
			return true
		case m.deleting && key.Matches(msg, m.keyMap.Escape):
			m.deleting = false
			return true
		case m.deleting && key.Matches(msg, m.keyMap.DeleteAll):
			m.deleting = false
			m.list = nil
			return true
		case m.deleting:
			// Handle digit keys for individual attachment deletion.
			r := msg.Code
			if r >= '0' && r <= '9' {
				num := int(r - '0')
				if num < len(m.list) {
					m.list = slices.Delete(m.list, num, num+1)
				}
				m.deleting = false
			}
			return true
		}
	}
	return false
}

func (m *Attachments) Render(width int) string {
	return m.renderer.Render(m.list, m.deleting, m.editing, m.selected, width)
}

func NewRenderer(
	normalStyle, deletingStyle, imageStyle, textStyle, pasteStyle, pasteSelectedStyle lipgloss.Style,
	pastePalette, pasteSelectedPalette []color.Color,
) *Renderer {
	if len(pastePalette) == 0 {
		pastePalette = defaultPastePalette()
	}
	if len(pasteSelectedPalette) == 0 {
		pasteSelectedPalette = defaultPasteSelectedPalette()
	}

	return &Renderer{
		normalStyle:          normalStyle,
		textStyle:            textStyle,
		imageStyle:           imageStyle,
		deletingStyle:        deletingStyle,
		pasteStyle:           pasteStyle,
		pasteSelectedStyle:   pasteSelectedStyle,
		pastePalette:         pastePalette,
		pasteSelectedPalette: pasteSelectedPalette,
	}
}

type Renderer struct {
	normalStyle, textStyle, imageStyle, deletingStyle lipgloss.Style
	pasteStyle, pasteSelectedStyle                    lipgloss.Style
	pastePalette, pasteSelectedPalette                []color.Color
}

func (r *Renderer) Render(attachments []message.Attachment, deleting, editing bool, selected, width int) string {
	var chips []string

	maxItemWidth := lipgloss.Width(r.imageStyle.String() + r.normalStyle.Render(strings.Repeat("x", maxFilename)))
	fits := int(math.Floor(float64(width)/float64(maxItemWidth))) - 1

	pasteIndex := 0

	for i, att := range attachments {
		filename := filepath.Base(att.FileName)
		// Truncate if needed.
		if ansi.StringWidth(filename) > maxFilename {
			filename = ansi.Truncate(filename, maxFilename, "…")
		}

		if deleting {
			chips = append(
				chips,
				r.deletingStyle.Render(fmt.Sprintf("%d", i)),
				r.normalStyle.Render(filename),
			)
		} else if isPasteBlock(att) {
			// Render paste block with green badge icon on left, filename on right
			iconBadge := greenBadge.Render(pasteTextIcon)
			label := fmt.Sprintf(" %s", filename)
			style := r.pasteStyle.Copy()
			color := r.pastePalette[pasteIndex%len(r.pastePalette)]
			style = style.Background(color)
			if editing && selected == i {
				style = r.pasteSelectedStyle.Copy()
				selectedColor := r.pasteSelectedPalette[pasteIndex%len(r.pasteSelectedPalette)]
				style = style.Background(selectedColor)
			}
			chips = append(chips, style.Render(iconBadge+label))
			pasteIndex++
		} else {
			chips = append(
				chips,
				r.icon(att).String(),
				r.normalStyle.Render(filename),
			)
		}

		if i == fits && len(attachments) > i {
			chips = append(chips, lipgloss.NewStyle().Width(maxItemWidth).Render(fmt.Sprintf("%d more…", len(attachments)-fits)))
			break
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Left, chips...)
}

func (r *Renderer) icon(a message.Attachment) lipgloss.Style {
	if a.IsImage() {
		return r.imageStyle
	}
	return r.textStyle
}

var pasteBlockNameRE = regexp.MustCompile(`^paste_\d+\.txt$`)

func isPasteBlock(att message.Attachment) bool {
	return att.IsText() && pasteBlockNameRE.MatchString(filepath.Base(att.FileName))
}

func defaultPastePalette() []color.Color {
	return []color.Color{
		lipgloss.Color("#1f2b27"),
		lipgloss.Color("#232534"),
		lipgloss.Color("#2c1f2c"),
		lipgloss.Color("#1f2a30"),
		lipgloss.Color("#253128"),
	}
}

func defaultPasteSelectedPalette() []color.Color {
	return []color.Color{
		lipgloss.Color("#315449"),
		lipgloss.Color("#3b3549"),
		lipgloss.Color("#44293c"),
		lipgloss.Color("#2d3f3b"),
		lipgloss.Color("#344d3b"),
	}
}

func (m *Attachments) pasteIndices() []int {
	var indices []int
	for i, att := range m.list {
		if isPasteBlock(att) {
			indices = append(indices, i)
		}
	}
	return indices
}

func (m *Attachments) reconcileSelection() {
	pasteIndices := m.pasteIndices()
	if len(pasteIndices) == 0 {
		m.editing = false
		m.selected = -1
		return
	}
	if !m.editing {
		m.selected = -1
		return
	}
	for _, idx := range pasteIndices {
		if idx == m.selected {
			return
		}
	}
	m.selected = pasteIndices[0]
}

func (m *Attachments) moveSelection(delta int) {
	pasteIndices := m.pasteIndices()
	if len(pasteIndices) == 0 {
		m.selected = -1
		m.editing = false
		return
	}
	if m.selected == -1 {
		m.selected = pasteIndices[0]
		return
	}
	currentPos := 0
	for i, idx := range pasteIndices {
		if idx == m.selected {
			currentPos = i
			break
		}
	}
	next := (currentPos + delta + len(pasteIndices)) % len(pasteIndices)
	m.selected = pasteIndices[next]
}
