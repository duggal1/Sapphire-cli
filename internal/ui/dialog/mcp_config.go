package dialog

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/sapphire/internal/config"
	"github.com/charmbracelet/sapphire/internal/ui/common"
	"github.com/charmbracelet/sapphire/internal/ui/util"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	uv "github.com/charmbracelet/ultraviolet"
)

// MCPConfigID is the identifier for the MCP config dialog.
const MCPConfigID = "mcp_config"

const (
	mcpConfigMaxInputWidth     = 120
	mcpConfigMinInputWidth     = 30
	mcpConfigMaxViewportHeight = 22
	mcpConfigFieldHeight       = 3
)

type mcpConfigField struct {
	ID          string
	Label       string
	Placeholder string
	Required    bool
}

// MCPConfigDialog represents a dialog for adding or editing MCP configs.
type MCPConfigDialog struct {
	com          *common.Common
	fields       []mcpConfigField
	inputs       []textinput.Model
	focused      int
	viewport     viewport.Model
	isNew        bool
	originalName string

	help  help.Model
	keyMap struct {
		Confirm  key.Binding
		Next     key.Binding
		Previous key.Binding
		Close    key.Binding
	}
}

var _ Dialog = (*MCPConfigDialog)(nil)

func NewMCPConfigDialog(com *common.Common, name string, cfg config.MCPConfig, isNew bool) *MCPConfigDialog {
	d := &MCPConfigDialog{
		com:          com,
		isNew:        isNew,
		originalName: name,
	}

	d.help = help.New()
	d.help.Styles = com.Styles.DialogHelpStyles()

	d.keyMap.Confirm = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "save"),
	)
	d.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "tab"),
		key.WithHelp("↓/tab", "next"),
	)
	d.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "shift+tab"),
		key.WithHelp("↑/shift+tab", "previous"),
	)
	d.keyMap.Close = CloseKey

	d.fields = mcpConfigFields(isNew)
	d.inputs = make([]textinput.Model, len(d.fields))
	for i, field := range d.fields {
		input := textinput.New()
		input.SetVirtualCursor(false)
		input.SetStyles(com.Styles.TextInput)
		input.Prompt = "> "
		input.Placeholder = field.Placeholder
		if i == 0 {
			input.Focus()
		} else {
			input.Blur()
		}
		d.inputs[i] = input
	}

	d.setInitialValues(name, cfg)
	return d
}

func (d *MCPConfigDialog) ID() string {
	return MCPConfigID
}

func (d *MCPConfigDialog) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, d.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, d.keyMap.Confirm):
			config, name, err := d.buildConfig()
			if err != nil {
				return ActionCmd{Cmd: util.ReportWarn(err.Error())}
			}
			return ActionSaveMCPConfig{
				Name:         name,
				OriginalName: d.originalName,
				Config:       config,
			}
		case key.Matches(msg, d.keyMap.Next):
			d.focusInput(d.focused + 1)
		case key.Matches(msg, d.keyMap.Previous):
			d.focusInput(d.focused - 1)
		default:
			var cmd tea.Cmd
			d.inputs[d.focused], cmd = d.inputs[d.focused].Update(msg)
			return ActionCmd{Cmd: cmd}
		}
	}
	return nil
}

func (d *MCPConfigDialog) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	s := d.com.Styles
	contentStyle := s.Dialog.Arguments.Content
	possibleWidth := area.Dx() - s.Dialog.View.GetHorizontalFrameSize() - contentStyle.GetHorizontalFrameSize()

	caser := cases.Title(language.English)

	var fields []string
	for i, field := range d.fields {
		isFocused := i == d.focused

		labelText := field.Label
		if labelText == "" {
			labelText = strings.ReplaceAll(field.ID, "_", " ")
			labelText = strings.ReplaceAll(labelText, "-", " ")
			labelText = caser.String(strings.ToLower(labelText))
		}

		markRequiredStyle := s.Dialog.Arguments.InputRequiredMarkBlurred
		labelStyle := s.Dialog.Arguments.InputLabelBlurred
		if isFocused {
			labelStyle = s.Dialog.Arguments.InputLabelFocused
			markRequiredStyle = s.Dialog.Arguments.InputRequiredMarkFocused
		}
		if field.Required {
			labelText += markRequiredStyle.String()
		}
		label := labelStyle.Render(labelText)

		labelWidth := lipgloss.Width(labelText)
		placeholderWidth := lipgloss.Width(d.inputs[i].Placeholder)

		inputWidth := max(placeholderWidth, labelWidth, mcpConfigMinInputWidth)
		inputWidth = min(inputWidth, min(possibleWidth, mcpConfigMaxInputWidth))
		d.inputs[i].SetWidth(inputWidth)

		inputLine := d.inputs[i].View()
		fieldView := lipgloss.JoinVertical(lipgloss.Left, label, inputLine, "")
		fields = append(fields, fieldView)
	}

	renderedFields := lipgloss.JoinVertical(lipgloss.Left, fields...)
	width := lipgloss.Width(renderedFields)
	height := lipgloss.Height(renderedFields)

	titleText := cmp.Or("MCP Configuration", "MCP Configuration")
	header := common.DialogTitle(s, titleText, width, s.Primary, s.Secondary)

	helpView := s.Dialog.HelpView.Width(width).Render(d.help.View(d))

	availableHeight := area.Dy() - s.Dialog.View.GetVerticalFrameSize() - contentStyle.GetVerticalFrameSize() - lipgloss.Height(header) - lipgloss.Height(helpView) - 2
	viewportHeight := min(height, mcpConfigMaxViewportHeight, availableHeight)

	d.viewport.SetWidth(width)
	d.viewport.SetHeight(viewportHeight)
	d.viewport.SetContent(renderedFields)

	scrollbar := common.Scrollbar(s, viewportHeight, d.viewport.TotalLineCount(), viewportHeight, d.viewport.YOffset())
	content := d.viewport.View()
	if scrollbar != "" {
		content = lipgloss.JoinHorizontal(lipgloss.Top, content, scrollbar)
	}

	view := lipgloss.JoinVertical(
		lipgloss.Left,
		s.Dialog.Title.Render(header),
		contentStyle.Render(content),
		helpView,
	)
	dialogView := s.Dialog.View.Render(view)
	cur := d.cursor()
	DrawCenterCursor(scr, area, dialogView, cur)
	return cur
}

func (d *MCPConfigDialog) cursor() *tea.Cursor {
	cursor := InputCursor(d.com.Styles, d.inputs[d.focused].Cursor())
	if cursor == nil {
		return nil
	}
	cursor.Y += d.focused*mcpConfigFieldHeight - d.viewport.YOffset() + 1
	return cursor
}

func (d *MCPConfigDialog) ShortHelp() []key.Binding {
	return []key.Binding{d.keyMap.Confirm, d.keyMap.Next, d.keyMap.Close}
}

func (d *MCPConfigDialog) FullHelp() [][]key.Binding {
	return [][]key.Binding{{d.keyMap.Confirm, d.keyMap.Next, d.keyMap.Previous}, {d.keyMap.Close}}
}

func (d *MCPConfigDialog) focusInput(newIndex int) {
	d.inputs[d.focused].Blur()
	n := len(d.inputs)
	d.focused = ((newIndex % n) + n) % n
	d.inputs[d.focused].Focus()
	d.ensureFieldVisible(d.focused)
}

func (d *MCPConfigDialog) ensureFieldVisible(fieldIndex int) {
	fieldStart := fieldIndex * mcpConfigFieldHeight
	fieldEnd := fieldStart + mcpConfigFieldHeight - 1
	viewportTop := d.viewport.YOffset()
	viewportBottom := viewportTop + d.viewport.Height() - 1

	if fieldStart >= viewportTop && fieldEnd <= viewportBottom {
		return
	}

	if fieldStart < viewportTop {
		d.viewport.SetYOffset(fieldStart)
		return
	}
	if fieldEnd > viewportBottom {
		d.viewport.SetYOffset(fieldEnd - d.viewport.Height() + 1)
	}
}

func (d *MCPConfigDialog) setInitialValues(name string, cfg config.MCPConfig) {
	values := map[string]string{
		"name":            name,
		"type":            string(cmp.Or(cfg.Type, config.MCPHttp)),
		"url":             cfg.URL,
		"command":         cfg.Command,
		"args":            strings.Join(cfg.Args, ", "),
		"env":             renderKeyValue(cfg.Env),
		"headers":         renderKeyValue(cfg.Headers),
		"timeout":         formatTimeout(cfg.Timeout),
		"disabled":        formatBool(cfg.Disabled),
		"disabled_tools":  strings.Join(cfg.DisabledTools, ", "),
	}

	for i, field := range d.fields {
		if value, ok := values[field.ID]; ok {
			d.inputs[i].SetValue(value)
		}
	}
}

func (d *MCPConfigDialog) buildConfig() (config.MCPConfig, string, error) {
	values := make(map[string]string)
	for i, field := range d.fields {
		values[field.ID] = strings.TrimSpace(d.inputs[i].Value())
		if field.Required && values[field.ID] == "" {
			return config.MCPConfig{}, "", fmt.Errorf("%s is required", field.Label)
		}
	}

	name := values["name"]
	mcpType, err := parseMCPType(values["type"])
	if err != nil {
		return config.MCPConfig{}, "", err
	}

	cfg := config.MCPConfig{
		Type:     mcpType,
		URL:      values["url"],
		Command:  values["command"],
		Args:     parseCSVList(values["args"]),
		Env:      parseKeyValue(values["env"]),
		Headers:  parseKeyValue(values["headers"]),
		Disabled: parseBool(values["disabled"]),
	}

	if values["disabled_tools"] != "" {
		cfg.DisabledTools = parseCSVList(values["disabled_tools"])
	}

	if values["timeout"] != "" {
		timeout, err := strconv.Atoi(values["timeout"])
		if err != nil || timeout <= 0 {
			return config.MCPConfig{}, "", fmt.Errorf("timeout must be a positive number of seconds")
		}
		cfg.Timeout = timeout
	}

	if cfg.Type == config.MCPStdio && strings.TrimSpace(cfg.Command) == "" {
		return config.MCPConfig{}, "", fmt.Errorf("command is required for stdio MCP")
	}
	if (cfg.Type == config.MCPHttp || cfg.Type == config.MCPSSE) && strings.TrimSpace(cfg.URL) == "" {
		return config.MCPConfig{}, "", fmt.Errorf("url is required for http/sse MCP")
	}

	return cfg, name, nil
}

func mcpConfigFields(isNew bool) []mcpConfigField {
	fields := []mcpConfigField{
		{ID: "name", Label: "Name", Placeholder: "supabase", Required: true},
		{ID: "type", Label: "Type", Placeholder: "http", Required: true},
		{ID: "url", Label: "URL", Placeholder: "https://example.com/mcp"},
		{ID: "command", Label: "Command", Placeholder: "npx"},
		{ID: "args", Label: "Args", Placeholder: "comma,separated,args"},
		{ID: "env", Label: "Env", Placeholder: "KEY=VALUE,KEY2=VALUE2"},
		{ID: "headers", Label: "Headers", Placeholder: "Authorization=Bearer ${TOKEN}"},
		{ID: "timeout", Label: "Timeout", Placeholder: "15"},
		{ID: "disabled", Label: "Disabled", Placeholder: "false"},
		{ID: "disabled_tools", Label: "Disabled Tools", Placeholder: "tool_a,tool_b"},
	}
	if !isNew {
		fields[0].Placeholder = ""
	}
	return fields
}

func parseMCPType(value string) (config.MCPType, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case string(config.MCPStdio):
		return config.MCPStdio, nil
	case string(config.MCPHttp):
		return config.MCPHttp, nil
	case string(config.MCPSSE):
		return config.MCPSSE, nil
	default:
		return "", fmt.Errorf("type must be stdio, http, or sse")
	}
}

func parseCSVList(value string) []string {
	parts := splitCSV(value)
	if len(parts) == 0 {
		return nil
	}
	return parts
}

func parseKeyValue(value string) map[string]string {
	parts := splitCSV(value)
	if len(parts) == 0 {
		return nil
	}
	result := make(map[string]string)
	for _, part := range parts {
		if !strings.Contains(part, "=") {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		if key == "" {
			continue
		}
		result[key] = val
	}
	return result
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n'
	})
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		cleaned = append(cleaned, part)
	}
	slices.Sort(cleaned)
	return cleaned
}

func formatTimeout(timeout int) string {
	if timeout <= 0 {
		return ""
	}
	return strconv.Itoa(timeout)
}

func formatBool(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func parseBool(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	return value == "true" || value == "1" || value == "yes" || value == "on"
}
