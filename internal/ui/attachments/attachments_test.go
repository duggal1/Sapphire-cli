package attachments

import (
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/sapphire/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPasteBlockEditModeAndDelete(t *testing.T) {
	a := New(
		NewRenderer(lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle()),
		Keymap{
			TogglePasteEdit:     key.NewBinding(key.WithKeys("ctrl+b")),
			PastePrev:           key.NewBinding(key.WithKeys("left")),
			PasteNext:           key.NewBinding(key.WithKeys("right")),
			DeleteSelectedPaste: key.NewBinding(key.WithKeys("ctrl+delete")),
			Escape:              key.NewBinding(key.WithKeys("esc")),
		},
	)

	a.Update(message.Attachment{FileName: "paste_1.txt", MimeType: "text/plain"})
	a.Update(message.Attachment{FileName: "image.png", MimeType: "image/png"})
	a.Update(message.Attachment{FileName: "paste_2.txt", MimeType: "text/plain"})

	require.True(t, a.HasPasteBlocks())

	handled := a.Update(tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl})
	require.True(t, handled)
	assert.True(t, a.EditingPasteBlocks())
	assert.Equal(t, 3, len(a.List()))

	handled = a.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	require.True(t, handled)

	handled = a.Update(tea.KeyPressMsg{Code: tea.KeyDelete, Mod: tea.ModCtrl})
	require.True(t, handled)
	assert.Len(t, a.List(), 2)
	assert.Equal(t, "paste_1.txt", a.List()[0].FileName)
	assert.Equal(t, "image.png", a.List()[1].FileName)

	handled = a.Update(tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl})
	require.True(t, handled)
	assert.False(t, a.EditingPasteBlocks())
}

func TestPasteBlockEditModeIgnoredWithoutPasteBlocks(t *testing.T) {
	a := New(
		NewRenderer(lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle()),
		Keymap{
			TogglePasteEdit: key.NewBinding(key.WithKeys("ctrl+b")),
		},
	)

	a.Update(message.Attachment{FileName: "notes.md", MimeType: "text/markdown"})
	handled := a.Update(tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl})
	assert.False(t, handled)
	assert.False(t, a.EditingPasteBlocks())
}
