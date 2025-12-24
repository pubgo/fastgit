package tagcmd

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	semver "github.com/hashicorp/go-version"
	"github.com/pubgo/funk/v2/log"
)

const (
	envAlpha   = "alpha"
	envBeta    = "beta"
	envRelease = "release"
)

type model struct {
	cursor   int
	choices  []string
	selected string
	length   int
}

func initialModel() model {
	choices := []string{envAlpha, envBeta, envRelease}
	return model{
		choices: choices,
		length:  len(choices),
	}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyUp, tea.KeyLeft, tea.KeyDown, tea.KeyRight:
			m.cursor++
		case tea.KeyEnter:
			m.selected = m.choices[m.cursor%m.length]
			return m, tea.Quit
		default:
			log.Error().Str("key", msg.String()).Msg("unknown key")
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m model) View() string {
	s := "Please Select Pre Tag:\n"

	for i, choice := range m.choices {
		cursor := " "
		if m.cursor%m.length == i {
			cursor = ">"
		}

		s += fmt.Sprintf("%s %s\n", cursor, choice)
	}

	return s
}

type model1 struct {
	spinner  spinner.Model
	quitting bool
	err      error
}

func InitialModelNew() model1 {
	s := spinner.New()
	s.Spinner = spinner.Line
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return model1{spinner: s}
}

func (m model1) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m model1) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		default:
			return m, nil
		}

	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
}

func (m model1) View() string {

	if m.err != nil {
		return m.err.Error()
	}
	str := fmt.Sprintf("%s Preparing...", m.spinner.View())
	if m.quitting {
		return str + "\n"
	}
	return str
}

type model2 struct {
	textInput textinput.Model
	exit      bool
}

// sanitizeInput verifies that an input text string gets validated
func sanitizeInput(input string) error {
	_, err := semver.NewSemver(input)
	return err
}

func InitialTextInputModel(data string) model2 {
	ti := textinput.New()
	ti.Focus()
	ti.Prompt = ""
	ti.CharLimit = 156
	ti.Width = 20
	ti.Validate = sanitizeInput
	ti.SetValue(data)

	return model2{
		textInput: ti,
	}
}

// Init is called at the beginning of a textinput step
// and sets the cursor to blink
func (m model2) Init() tea.Cmd {
	return textinput.Blink
}

// Update is called when "things happen", it checks for the users text input,
// and for Ctrl+C or Esc to close the program.
func (m model2) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			return m, tea.Quit
		case tea.KeyCtrlC, tea.KeyEsc:
			m.exit = true
			return m, tea.Quit
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

// View is called to draw the textinput step
func (m model2) View() string {
	return fmt.Sprintf(
		"new tag: %s\n",
		m.textInput.View(),
	)
}

func (m model2) Value() string {
	return m.textInput.Value()
}
