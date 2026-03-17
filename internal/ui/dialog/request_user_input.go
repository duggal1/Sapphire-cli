package dialog

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/sapphire/internal/planmode"
	"github.com/charmbracelet/sapphire/internal/ui/common"
	"github.com/charmbracelet/sapphire/internal/ui/list"
	"github.com/charmbracelet/sapphire/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/sahilm/fuzzy"
)

const (
	RequestUserInputID              = "request-user-input"
	requestUserInputDialogMaxWidth  = 80
	requestUserInputDialogMaxHeight = 18
	requestOtherOptionLabel         = "Other"
)

type RequestUserInput struct {
	com       *common.Common
	requestID string
	questions []planmode.Question
	index     int
	answers   map[string]planmode.Answer

	help  help.Model
	list  *list.FilterableList
	input textinput.Model

	enteringOther bool

	keyMap struct {
		Select   key.Binding
		Next     key.Binding
		Previous key.Binding
		UpDown   key.Binding
		Close    key.Binding
	}
}

type requestUserInputOptionItem struct {
	option  planmode.QuestionOption
	other   bool
	title   string
	t       *styles.Styles
	m       fuzzy.Match
	cache   map[int]string
	focused bool
}

var (
	_ Dialog   = (*RequestUserInput)(nil)
	_ ListItem = (*requestUserInputOptionItem)(nil)
)

func NewRequestUserInput(com *common.Common, req planmode.Request) (*RequestUserInput, error) {
	d := &RequestUserInput{
		com:       com,
		requestID: req.ID,
		questions: req.Questions,
		answers:   make(map[string]planmode.Answer, len(req.Questions)),
	}

	helpModel := help.New()
	helpModel.Styles = com.Styles.DialogHelpStyles()
	d.help = helpModel

	d.list = list.NewFilterableList()
	d.list.Focus()

	d.input = textinput.New()
	d.input.SetVirtualCursor(false)
	d.input.Placeholder = "Type an answer"
	d.input.SetStyles(com.Styles.TextInput)

	d.keyMap.Select = key.NewBinding(key.WithKeys("enter", "ctrl+y"), key.WithHelp("enter", "confirm"))
	d.keyMap.Next = key.NewBinding(key.WithKeys("down", "ctrl+n"), key.WithHelp("↓", "next item"))
	d.keyMap.Previous = key.NewBinding(key.WithKeys("up", "ctrl+p"), key.WithHelp("↑", "previous item"))
	d.keyMap.UpDown = key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑/↓", "choose"))
	d.keyMap.Close = CloseKey

	d.rebuildOptions()
	return d, nil
}

func (d *RequestUserInput) ID() string { return RequestUserInputID }

func (d *RequestUserInput) RequestID() string { return d.requestID }

func (d *RequestUserInput) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, d.keyMap.Close):
			return ActionClose{}
		case d.enteringOther:
			switch msg.String() {
			case "enter":
				text := strings.TrimSpace(d.input.Value())
				if text == "" {
					return nil
				}
				current := d.questions[d.index]
				d.answers[current.ID] = planmode.Answer{Answers: []string{"user_note: " + text}}
				d.enteringOther = false
				d.input.SetValue("")
				if d.index == len(d.questions)-1 {
					return ActionRespondUserInput{RequestID: d.requestID, Response: planmode.Response{Answers: d.answers}}
				}
				d.index++
				d.rebuildOptions()
			default:
				var cmd tea.Cmd
				d.input, cmd = d.input.Update(msg)
				if cmd != nil {
					return ActionCmd{Cmd: cmd}
				}
			}
		case key.Matches(msg, d.keyMap.Previous):
			if d.list.IsSelectedFirst() {
				d.list.SelectLast()
				d.list.ScrollToBottom()
				break
			}
			d.list.SelectPrev()
			d.list.ScrollToSelected()
		case key.Matches(msg, d.keyMap.Next):
			if d.list.IsSelectedLast() {
				d.list.SelectFirst()
				d.list.ScrollToTop()
				break
			}
			d.list.SelectNext()
			d.list.ScrollToSelected()
		case key.Matches(msg, d.keyMap.Select):
			selected := d.list.SelectedItem()
			if selected == nil {
				break
			}
			item, ok := selected.(*requestUserInputOptionItem)
			if !ok {
				break
			}
			if item.other {
				d.enteringOther = true
				d.input.SetValue("")
				d.input.Placeholder = "Type a custom answer"
				d.input.Focus()
				return nil
			}
			current := d.questions[d.index]
			d.answers[current.ID] = planmode.Answer{Answers: []string{item.option.Label}}
			if d.index == len(d.questions)-1 {
				return ActionRespondUserInput{RequestID: d.requestID, Response: planmode.Response{Answers: d.answers}}
			}
			d.index++
			d.rebuildOptions()
		default:
			var cmd tea.Cmd
			d.input, cmd = d.input.Update(msg)
			d.list.SetFilter(d.input.Value())
			d.list.ScrollToTop()
			d.list.SetSelected(0)
			return ActionCmd{Cmd: cmd}
		}
	}
	return nil
}

func (d *RequestUserInput) Cursor() *tea.Cursor {
	if !d.enteringOther {
		return nil
	}
	return InputCursor(d.com.Styles, d.input.Cursor())
}

func (d *RequestUserInput) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := d.com.Styles
	width := max(0, min(requestUserInputDialogMaxWidth, area.Dx()))
	height := max(0, min(requestUserInputDialogMaxHeight, area.Dy()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.InputPrompt.GetVerticalFrameSize() + inputContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()

	d.input.SetWidth(innerWidth - t.Dialog.InputPrompt.GetHorizontalFrameSize() - 1)
	d.list.SetSize(innerWidth, height-heightOffset-4)
	d.help.SetWidth(innerWidth)

	rc := NewRenderContext(t, width)
	rc.Title = d.dialogTitle()
	question := d.currentQuestion()
	rc.AddPart(t.Base.Bold(true).Render(question.Question))
	if d.enteringOther {
		rc.AddPart(t.Dialog.InputPrompt.Render(d.input.View()))
	} else {
		rc.AddPart(t.Dialog.InputPrompt.Render(d.input.View()))
		visibleCount := len(d.list.FilteredItems())
		if d.list.Height() >= visibleCount {
			d.list.ScrollToTop()
		} else {
			d.list.ScrollToSelected()
		}
		rc.AddPart(t.Dialog.List.Height(d.list.Height()).Render(d.list.Render()))
	}
	rc.Help = d.help.View(d)

	view := rc.Render()
	cur := d.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

func (d *RequestUserInput) ShortHelp() []key.Binding {
	if d.enteringOther {
		return []key.Binding{d.keyMap.Select, d.keyMap.Close}
	}
	return []key.Binding{d.keyMap.UpDown, d.keyMap.Select, d.keyMap.Close}
}

func (d *RequestUserInput) FullHelp() [][]key.Binding {
	return [][]key.Binding{{d.keyMap.Select, d.keyMap.Next, d.keyMap.Previous, d.keyMap.Close}}
}

func (d *RequestUserInput) currentQuestion() planmode.Question {
	return d.questions[d.index]
}

func (d *RequestUserInput) dialogTitle() string {
	current := d.currentQuestion()
	return fmt.Sprintf("%s · %d/%d", current.Header, d.index+1, len(d.questions))
}

func (d *RequestUserInput) rebuildOptions() {
	current := d.currentQuestion()
	items := make([]list.FilterableItem, 0, len(current.Options)+1)
	for _, option := range current.Options {
		items = append(items, &requestUserInputOptionItem{
			option: option,
			title:  option.Label,
			t:      d.com.Styles,
		})
	}
	items = append(items, &requestUserInputOptionItem{
		other: true,
		title: requestOtherOptionLabel,
		t:     d.com.Styles,
	})
	d.input.SetValue("")
	d.input.Placeholder = "Type to filter"
	d.list.SetItems(items...)
	d.list.SetSelected(0)
	d.list.SetFilter("")
}

func (i *requestUserInputOptionItem) Filter() string { return i.title }
func (i *requestUserInputOptionItem) ID() string {
	if i.other {
		return "other"
	}
	return i.option.Label
}

func (i *requestUserInputOptionItem) SetFocused(focused bool) {
	if i.focused != focused {
		i.cache = nil
	}
	i.focused = focused
}

func (i *requestUserInputOptionItem) SetMatch(m fuzzy.Match) {
	i.cache = nil
	i.m = m
}

func (i *requestUserInputOptionItem) Render(width int) string {
	info := i.option.Description
	if i.other {
		info = "Provide a custom answer"
	}
	itemStyles := ListItemStyles{
		ItemBlurred:     i.t.Dialog.NormalItem,
		ItemFocused:     i.t.Dialog.SelectedItem,
		InfoTextBlurred: i.t.Subtle,
		InfoTextFocused: i.t.Base,
	}
	if i.other {
		itemStyles.ItemBlurred = lipgloss.NewStyle().Foreground(lipgloss.Color("#ea8eed"))
		itemStyles.ItemFocused = i.t.Dialog.SelectedItem.Foreground(lipgloss.Color("#ea8eed"))
	}
	return renderItem(itemStyles, i.title, info, i.focused, width, i.cache, &i.m)
}
