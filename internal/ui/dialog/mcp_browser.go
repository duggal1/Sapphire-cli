package dialog

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/duggal1/Sapphire-cli/internal/agent/tools/mcp"
	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/duggal1/Sapphire-cli/internal/ui/common"
	"github.com/duggal1/Sapphire-cli/internal/ui/list"
	"github.com/duggal1/Sapphire-cli/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// MCPBrowserID is the identifier for the MCP browser dialog.
const MCPBrowserID = "mcp_browser"

const (
	mcpBrowserMaxWidth  = 96
	mcpBrowserMaxHeight = 26
)

type mcpBrowserMode uint8

const (
	mcpBrowserModeList mcpBrowserMode = iota
	mcpBrowserModeDetail
)

type mcpRegistryLoadedMsg struct {
	defs []config.RegistryMCPDefinition
	err  error
}

type shimmerTickMsg struct{}

// MCPBrowser represents the MCP registry browser dialog.
type MCPBrowser struct {
	com          *common.Common
	list         *list.FilterableList
	input        textinput.Model
	help         help.Model
	mode         mcpBrowserMode
	loading      bool
	loadErr      string
	shimmerFrame int
	defs         []config.RegistryMCPDefinition
	defsByName   map[string]config.RegistryMCPDefinition
	selectedName string

	keyMap struct {
		Select  key.Binding
		Next    key.Binding
		Prev    key.Binding
		UpDown  key.Binding
		Install key.Binding
		Delete  key.Binding
		Back    key.Binding
		Close   key.Binding
	}
}

var _ Dialog = (*MCPBrowser)(nil)
var _ LoadingDialog = (*MCPBrowser)(nil)

// NewMCPBrowser creates a new MCP browser dialog.
func NewMCPBrowser(com *common.Common, selected string) (*MCPBrowser, error) {
	b := &MCPBrowser{
		com:          com,
		mode:         mcpBrowserModeList,
		defsByName:   map[string]config.RegistryMCPDefinition{},
		selectedName: selected,
	}

	b.help = help.New()
	b.help.Styles = com.Styles.DialogHelpStyles()

	b.list = list.NewFilterableList()
	b.list.Focus()
	b.list.SetSelected(0)

	b.input = textinput.New()
	b.input.SetVirtualCursor(false)
	b.input.Placeholder = "Search MCP registry"
	b.input.SetStyles(com.Styles.TextInput)
	b.input.Focus()

	b.keyMap.Select = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "view"),
	)
	b.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("/", "choose"),
	)
	b.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("", "next"),
	)
	b.keyMap.Prev = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("", "previous"),
	)
	b.keyMap.Install = key.NewBinding(
		key.WithKeys("i"),
		key.WithHelp("i", "install"),
	)
	b.keyMap.Delete = key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "remove"),
	)
	b.keyMap.Back = key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	)
	b.keyMap.Close = CloseKey

	return b, nil
}

// ID implements Dialog.
func (b *MCPBrowser) ID() string {
	return MCPBrowserID
}

// StartLoading implements LoadingDialog.
func (b *MCPBrowser) StartLoading() tea.Cmd {
	if b.loading {
		return nil
	}
	b.loading = true
	return tea.Batch(
		b.shimmerTick(),
		func() tea.Msg {
			defs, err := config.FetchRegistryDefinitions(context.Background())
			return mcpRegistryLoadedMsg{defs: defs, err: err}
		},
	)
}

// StopLoading implements LoadingDialog.
func (b *MCPBrowser) StopLoading() {
	b.loading = false
}

// HandleMsg implements Dialog.
func (b *MCPBrowser) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case mcpRegistryLoadedMsg:
		b.loading = false
		if msg.err != nil {
			b.loadErr = msg.err.Error()
			return nil
		}
		b.loadErr = ""
		b.defs = msg.defs
		b.defsByName = map[string]config.RegistryMCPDefinition{}
		for _, def := range msg.defs {
			b.defsByName[def.Name] = def
		}
		b.refreshItems()
		return nil
	case shimmerTickMsg:
		if b.loading {
			b.shimmerFrame++
			return ActionCmd{Cmd: b.shimmerTick()}
		}
		return nil
	case tea.KeyPressMsg:
		switch b.mode {
		case mcpBrowserModeDetail:
			switch {
			case key.Matches(msg, b.keyMap.Back, b.keyMap.Close):
				b.mode = mcpBrowserModeList
				return nil
			case key.Matches(msg, b.keyMap.Install):
				if def, ok := b.defsByName[b.selectedName]; ok {
					return ActionInstallMCPFromRegistry{Definition: def}
				}
			case key.Matches(msg, b.keyMap.Delete):
				if b.selectedName != "" {
					return ActionDeleteMCPConfig{Name: b.selectedName}
				}
			}
		default:
			switch {
			case key.Matches(msg, b.keyMap.Close):
				return ActionClose{}
			case key.Matches(msg, b.keyMap.Select):
				b.selectedName = b.selectedID()
				if b.selectedName != "" {
					b.mode = mcpBrowserModeDetail
				}
			case key.Matches(msg, b.keyMap.Prev):
				b.list.Focus()
				if b.list.IsSelectedFirst() {
					b.list.SelectLast()
				} else {
					b.list.SelectPrev()
				}
				b.list.ScrollToSelected()
			case key.Matches(msg, b.keyMap.Next):
				b.list.Focus()
				if b.list.IsSelectedLast() {
					b.list.SelectFirst()
				} else {
					b.list.SelectNext()
				}
				b.list.ScrollToSelected()
			default:
				var cmd tea.Cmd
				b.input, cmd = b.input.Update(msg)
				b.list.SetFilter(b.input.Value())
				b.list.ScrollToTop()
				b.list.SetSelected(0)
				return ActionCmd{Cmd: cmd}
			}
		}
	}
	return nil
}

func (b *MCPBrowser) shimmerTick() tea.Cmd {
	return tea.Tick(33*time.Millisecond, func(time.Time) tea.Msg {
		return shimmerTickMsg{}
	})
}

// Draw implements Dialog.
func (b *MCPBrowser) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := b.com.Styles
	width := max(0, min(mcpBrowserMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	height := max(0, min(mcpBrowserMaxHeight, area.Dy()-t.Dialog.View.GetVerticalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()

	rc := NewRenderContext(t, width)
	rc.Title = "MCP Browser"
	rc.TitleInfo = b.titleInfo(t)

	switch b.mode {
	case mcpBrowserModeDetail:
		rc.AddPart(t.Dialog.ContentPanel.Width(innerWidth).Height(height).Render(b.renderDetail(innerWidth)))
		rc.Help = b.help.View(b)
		view := rc.Render()
		DrawCenter(scr, area, view)
		return nil
	default:
		inputWidth := max(0, innerWidth-t.Dialog.InputPrompt.GetHorizontalFrameSize()-1)
		b.input.SetWidth(inputWidth)

		heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
			t.Dialog.InputPrompt.GetVerticalFrameSize() + inputContentHeight +
			t.Dialog.HelpView.GetVerticalFrameSize() +
			t.Dialog.View.GetVerticalFrameSize()
		listHeight := max(0, height-heightOffset)

		leftWidth := max(28, innerWidth/2-1)
		rightWidth := max(0, innerWidth-leftWidth-1)

		b.list.SetSize(leftWidth, listHeight)

		inputView := t.Dialog.InputPrompt.Render(b.input.View())
		rc.AddPart(inputView)

		listView := t.Dialog.List.Height(listHeight).Width(leftWidth).Render(b.list.Render())
		detailView := t.Dialog.ContentPanel.Width(rightWidth).Height(listHeight).Render(b.renderOverview(rightWidth))
		body := lipgloss.JoinHorizontal(lipgloss.Top, listView, " ", detailView)
		body = lipgloss.NewStyle().Width(innerWidth).Render(body)
		rc.AddPart(body)

		rc.Help = b.help.View(b)

		view := rc.Render()
		cur := InputCursor(b.com.Styles, b.input.Cursor())
		DrawCenterCursor(scr, area, view, cur)
		return cur
	}
}

func (b *MCPBrowser) titleInfo(t *styles.Styles) string {
	if b.loading {
		return styles.ShimmerText(t, "Loading registry...", b.shimmerFrame)
	}
	count := len(b.defs)
	label := fmt.Sprintf("%d servers", count)
	if count == 1 {
		label = "1 server"
	}
	return t.HalfMuted.Render(label)
}

func (b *MCPBrowser) renderOverview(width int) string {
	if b.loading {
		label := ansi.Truncate("Loading MCP registry...", width, "…")
		return styles.ShimmerText(b.com.Styles, label, b.shimmerFrame)
	}
	if b.loadErr != "" {
		return ansi.Truncate("Registry error: "+b.loadErr, width, "…")
	}
	name := b.selectedID()
	if name == "" {
		return ansi.Truncate("Select an MCP server", width, "…")
	}
	return ansi.Truncate(b.summaryFor(name), width, "…")
}

func (b *MCPBrowser) renderDetail(width int) string {
	if b.loading {
		label := ansi.Truncate("Loading MCP registry...", width, "…")
		return styles.ShimmerText(b.com.Styles, label, b.shimmerFrame)
	}
	if b.selectedName == "" {
		return ansi.Truncate("Select an MCP server", width, "…")
	}
	def, ok := b.defsByName[b.selectedName]
	if !ok {
		return ansi.Truncate("Unknown MCP server", width, "…")
	}

	status := b.registryStatus(b.selectedName)
	lines := []string{
		strings.ToUpper(def.Name),
		strings.TrimSpace(def.Description),
		"",
		fmt.Sprintf("Status: %s", status),
		fmt.Sprintf("Type: %s", def.Type),
		fmt.Sprintf("Command: %s %s", def.Command, strings.Join(def.Args, " ")),
	}
	if len(def.EnvKeys) > 0 {
		lines = append(lines, fmt.Sprintf("Env: %s", strings.Join(def.EnvKeys, ", ")))
	}

	toolNames := b.toolNames(def.Name)
	if len(toolNames) > 0 {
		lines = append(lines, "", "Tools:")
		for _, name := range toolNames {
			lines = append(lines, "  "+name)
		}
	}

	content := strings.Join(lines, "\n")
	return ansi.Truncate(content, width, "…")
}

func (b *MCPBrowser) summaryFor(name string) string {
	def, ok := b.defsByName[name]
	if !ok {
		return ""
	}
	return fmt.Sprintf("%s\n%s\nStatus: %s", def.Name, strings.TrimSpace(def.Description), b.registryStatus(name))
}

func (b *MCPBrowser) registryStatus(name string) string {
	cfg, ok := b.com.Config().MCP[name]
	if !ok {
		return "available"
	}
	if cfg.Disabled {
		return "disabled"
	}
	state, hasState := mcp.GetState(name)
	if hasState && state.State == mcp.StateConnected {
		return "connected"
	}
	return "installed"
}

func (b *MCPBrowser) toolNames(name string) []string {
	var tools []string
	for mcpName, list := range mcp.Tools() {
		if mcpName != name {
			continue
		}
		for _, tool := range list {
			tools = append(tools, tool.Name)
		}
	}
	slices.Sort(tools)
	return tools
}

func (b *MCPBrowser) refreshItems() {
	items := make([]list.FilterableItem, 0, len(b.defs))
	for _, def := range b.defs {
		items = append(items, &MCPItem{
			name:   def.Name,
			status: b.registryStatus(def.Name),
			t:      b.com.Styles,
		})
	}
	b.list.SetItems(items...)
	b.list.SetFilter(b.input.Value())
	b.list.ScrollToTop()
	b.selectByName(b.selectedName)
}

func (b *MCPBrowser) selectByName(name string) {
	if name == "" {
		b.list.SetSelected(0)
		return
	}
	for i, item := range b.list.FilteredItems() {
		if mcpItem, ok := item.(*MCPItem); ok && mcpItem.ID() == name {
			b.list.SetSelected(i)
			b.list.ScrollToSelected()
			return
		}
	}
	b.list.SetSelected(0)
}

func (b *MCPBrowser) selectedID() string {
	item := b.list.SelectedItem()
	if item == nil {
		return ""
	}
	if mcpItem, ok := item.(*MCPItem); ok {
		return mcpItem.ID()
	}
	return ""
}

func (b *MCPBrowser) ShortHelp() []key.Binding {
	if b.mode == mcpBrowserModeDetail {
		return []key.Binding{b.keyMap.Install, b.keyMap.Delete, b.keyMap.Back}
	}
	return []key.Binding{b.keyMap.Select, b.keyMap.Install, b.keyMap.Delete, b.keyMap.Close}
}

func (b *MCPBrowser) FullHelp() [][]key.Binding {
	if b.mode == mcpBrowserModeDetail {
		return [][]key.Binding{{b.keyMap.Install, b.keyMap.Delete, b.keyMap.Back}}
	}
	return [][]key.Binding{{b.keyMap.UpDown, b.keyMap.Select, b.keyMap.Close}, {b.keyMap.Install, b.keyMap.Delete}}
}

// ActionInstallMCPFromRegistry installs an MCP entry from the registry list.
type ActionInstallMCPFromRegistry struct {
	Definition config.RegistryMCPDefinition
}

func buildEnvPlaceholder(keys []string) (map[string]string, []string) {
	if len(keys) == 0 {
		return nil, nil
	}
	missing := []string{}
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		out[key] = "$" + key
		if _, ok := os.LookupEnv(key); !ok {
			missing = append(missing, key)
		}
	}
	slices.Sort(missing)
	return out, missing
}

func (b *MCPBrowser) Refresh() {
	b.refreshItems()
}
