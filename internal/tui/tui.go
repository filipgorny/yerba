// Package tui is yerba's terminal UI (Bubble Tea), consuming a session's message
// stream. It is the only place coupled to the TUI library.
package tui

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	session "github.com/filipgorny/agent-session"
	"github.com/filipgorny/agent/stream"

	"github.com/filipgorny/yerba/internal/config"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true)
	userStyle     = lipgloss.NewStyle().Bold(true)
	answerStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	askStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	toolStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	resultStyle   = lipgloss.NewStyle().Faint(true)
	errStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	rootStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("13"))
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	hintStyle     = lipgloss.NewStyle().Faint(true)
)

type inputMode int

const (
	modeNormal inputMode = iota
	modeFreeAnswer
	modeChoice
)

type streamMsg stream.Message

// Run starts the TUI over the given session.
func Run(s *session.Session, ui config.UIConfig) error {
	_, err := tea.NewProgram(newModel(s, ui), tea.WithAltScreen()).Run()

	return err
}

type model struct {
	session   *session.Session
	show      map[string]bool
	input     textinput.Model
	lines     []string
	mode      inputMode
	choices   []string
	choiceIdx int
}

func newModel(s *session.Session, ui config.UIConfig) model {
	in := textinput.New()
	in.Placeholder = "Type a message… (esc to quit)"
	in.Focus()

	show := map[string]bool{}

	for _, st := range ui.LogSubtypes {
		show[st] = true
	}

	return model{session: s, show: show, input: in, mode: modeNormal}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, waitMsg(m.session.Messages()))
}

func waitMsg(ch <-chan stream.Message) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch

		if !ok {
			return nil
		}

		return streamMsg(msg)
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case streamMsg:
		(&m).handleStream(stream.Message(msg))

		return m, waitMsg(m.session.Messages())

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	var cmd tea.Cmd

	m.input, cmd = m.input.Update(msg)

	return m, cmd
}

func (m *model) handleStream(sm stream.Message) {
	switch sm.Type {

	case stream.TypeAnswerUser:
		m.lines = append(m.lines, answerStyle.Render("● ")+asText(sm.Payload))
		m.mode = modeNormal

	case stream.TypeAskUser:
		q, choices := askPayload(sm.Payload)
		m.lines = append(m.lines, askStyle.Render("? "+q))

		if sm.Subtype == stream.SubtypeChoice && len(choices) > 0 {
			m.choices = choices
			m.choiceIdx = 0
			m.mode = modeChoice
		} else {
			m.mode = modeFreeAnswer
		}

	case stream.TypeChangeRoot:
		m.lines = append(m.lines, rootStyle.Render("📁 root: "+asText(sm.Payload)))

	case stream.TypeLog:

		if m.show[sm.Subtype] {
			m.lines = append(m.lines, logLine(sm))
		}
	}
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {

	case tea.KeyCtrlC, tea.KeyEsc:
		m.session.Close()

		return m, tea.Quit
	}

	if m.mode == modeChoice {
		return m.handleChoiceKey(msg)
	}

	if msg.Type == tea.KeyEnter {
		text := strings.TrimSpace(m.input.Value())

		if text == "" {
			return m, nil
		}

		m.input.SetValue("")
		m.lines = append(m.lines, userStyle.Render("› "+text))

		if m.mode == modeFreeAnswer {
			m.session.Answer(text)
			m.mode = modeNormal
		} else {
			m.session.Send(context.Background(), text)
		}

		return m, nil
	}

	var cmd tea.Cmd

	m.input, cmd = m.input.Update(msg)

	return m, cmd
}

func (m model) handleChoiceKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {

	case tea.KeyUp:

		if m.choiceIdx > 0 {
			m.choiceIdx--
		}

	case tea.KeyDown:

		if m.choiceIdx < len(m.choices)-1 {
			m.choiceIdx++
		}

	case tea.KeyEnter:
		choice := m.choices[m.choiceIdx]
		m.lines = append(m.lines, userStyle.Render("› "+choice))
		m.session.Answer(choice)
		m.mode = modeNormal
		m.choices = nil
	}

	return m, nil
}

func (m model) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("yerba"))
	b.WriteString("\n\n")
	b.WriteString(strings.Join(m.lines, "\n"))
	b.WriteString("\n\n")

	if m.mode == modeChoice {
		for i, c := range m.choices {
			cursor := "  "

			if i == m.choiceIdx {
				cursor = "▸ "
			}

			line := cursor + c

			if i == m.choiceIdx {
				line = selectedStyle.Render(line)
			}

			b.WriteString(line + "\n")
		}

		b.WriteString(hintStyle.Render("↑/↓ select · enter confirm · esc quit"))

		return b.String()
	}

	b.WriteString(m.input.View())

	return b.String()
}

// --- helpers ---

func logLine(sm stream.Message) string {
	icon, style := logStyle(sm.Subtype)

	return style.Render(icon + " " + sm.Subtype + ": " + truncate(asText(sm.Payload), 120))
}

func logStyle(subtype string) (string, lipgloss.Style) {
	switch subtype {

	case stream.LogToolCall:
		return "⚙", toolStyle

	case stream.LogToolResult:
		return "↳", resultStyle

	case stream.LogError:
		return "✗", errStyle

	default:
		return "•", resultStyle
	}
}

func asText(p any) string {
	switch v := p.(type) {

	case nil:
		return ""

	case string:
		return v

	default:
		b, _ := json.Marshal(v)

		return string(b)
	}
}

func askPayload(p any) (question string, choices []string) {
	m, ok := p.(map[string]any)

	if !ok {
		return asText(p), nil
	}

	question, _ = m["question"].(string)

	switch c := m["choices"].(type) {

	case []string:
		choices = c

	case []any:
		for _, e := range c {
			if s, ok := e.(string); ok {
				choices = append(choices, s)
			}
		}
	}

	return question, choices
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}

	return s[:n] + "…"
}
