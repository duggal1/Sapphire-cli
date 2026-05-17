package format

import (
	"context"
	"errors"
	"fmt"
	"os"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Spinner wraps the bubbles spinner for non-interactive mode
type Spinner struct {
	done chan struct{}
	prog *tea.Program
}

type spinnerLabelMsg string

type model struct {
	cancel context.CancelFunc
	label  string
	spin   spinner.Model
}

func (m model) Init() tea.Cmd { return m.spin.Tick }

func (m model) View() tea.View {
	return tea.NewView(m.spin.View() + " " + m.label)
}

// Update implements tea.Model.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancel()
			return m, tea.Quit
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	case spinnerLabelMsg:
		m.label = string(msg)
		return m, nil
	}
	return m, nil
}

// NewSpinner creates a new spinner with the given message
func NewSpinner(ctx context.Context, cancel context.CancelFunc, label string, style lipgloss.Style) *Spinner {
	spin := spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(style),
	)
	m := model{
		cancel: cancel,
		label:  label,
		spin:   spin,
	}

	p := tea.NewProgram(m, tea.WithOutput(os.Stderr), tea.WithContext(ctx))

	return &Spinner{
		prog: p,
		done: make(chan struct{}, 1),
	}
}

// Start begins the spinner animation
func (s *Spinner) Start() {
	go func() {
		defer close(s.done)
		_, err := s.prog.Run()
		// ensures line is cleared
		fmt.Fprint(os.Stderr, ansi.EraseEntireLine)
		if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, tea.ErrInterrupted) {
			fmt.Fprintf(os.Stderr, "Error running spinner: %v\n", err)
		}
	}()
}

// Stop ends the spinner animation
func (s *Spinner) Stop() {
	s.prog.Quit()
	<-s.done
}

// SetLabel updates the visible spinner label without restarting the program.
func (s *Spinner) SetLabel(label string) {
	if s == nil {
		return
	}
	s.prog.Send(spinnerLabelMsg(label))
}
