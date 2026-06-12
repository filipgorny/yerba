// Package tui holds yerba's terminal UI logic (a Bubble Tea chat over an agent).
package tui

import (
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/filipgorny/agent"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true)
	userStyle  = lipgloss.NewStyle().Bold(true)
	hintStyle  = lipgloss.NewStyle().Faint(true)
)

// Model is the Bubble Tea model: an input box and a running transcript driven by
// the agent.
type Model struct {
	agent   *agent.Agent
	input   textinput.Model
	history []string
	waiting bool
}

// replyMsg carries the agent's answer back into the update loop.
type replyMsg struct {
	text string
	err  error
}

// New builds the TUI model for the given agent.
func New(a *agent.Agent) Model {
	in := textinput.New()
	in.Placeholder = "Ask the agent… (esc to quit)"
	in.Focus()

	return Model{agent: a, input: in}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:

		switch msg.Type {

		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit

		case tea.KeyEnter:

			if m.waiting {
				return m, nil
			}

			q := strings.TrimSpace(m.input.Value())

			if q == "" {
				return m, nil
			}

			m.history = append(m.history, userStyle.Render("› "+q))
			m.input.SetValue("")
			m.waiting = true

			return m, m.ask(q)
		}

	case replyMsg:
		m.waiting = false

		if msg.err != nil {
			m.history = append(m.history, "error: "+msg.err.Error())
		} else {
			m.history = append(m.history, msg.text)
		}

		return m, nil
	}

	var cmd tea.Cmd

	m.input, cmd = m.input.Update(msg)

	return m, cmd
}

// ask runs the agent off the UI goroutine and reports the result as a message.
func (m Model) ask(q string) tea.Cmd {
	a := m.agent

	return func() tea.Msg {
		out, err := a.Ask(context.Background(), q)

		return replyMsg{text: out, err: err}
	}
}

func (m Model) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("yerba"))
	b.WriteString("\n\n")
	b.WriteString(strings.Join(m.history, "\n\n"))

	if m.waiting {
		b.WriteString("\n\n")
		b.WriteString(hintStyle.Render("…thinking"))
	}

	b.WriteString("\n\n")
	b.WriteString(m.input.View())

	return b.String()
}
