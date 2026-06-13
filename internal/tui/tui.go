// Package tui is yerba's terminal UI (Bubble Tea), consuming a session's message
// stream. It is the only place coupled to the TUI library.
package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	session "github.com/filipgorny/agent-session"
	"github.com/filipgorny/agent/stream"

	"github.com/filipgorny/yerba/internal/config"
)

// Palette.
const (
	colAccent  = lipgloss.Color("13") // magenta — brand / focus
	colAccent2 = lipgloss.Color("14") // cyan — selection
	colAnswer  = lipgloss.Color("10") // green
	colAsk     = lipgloss.Color("11") // yellow
	colTool    = lipgloss.Color("12") // blue
	colErr     = lipgloss.Color("9")  // red
	colFaint   = lipgloss.Color("8")  // grey
	colFg      = lipgloss.Color("15") // white
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(colFg).Background(colAccent).Padding(0, 1)
	brandStyle = lipgloss.NewStyle().Faint(true).Italic(true)

	userStyle   = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	answerStyle = lipgloss.NewStyle().Foreground(colAnswer)
	askStyle    = lipgloss.NewStyle().Foreground(colAsk).Bold(true)
	toolStyle   = lipgloss.NewStyle().Foreground(colTool)
	resultStyle = lipgloss.NewStyle().Foreground(colFaint)
	errStyle    = lipgloss.NewStyle().Foreground(colErr).Bold(true)
	rootStyle   = lipgloss.NewStyle().Foreground(colAccent2)

	waitStyle     = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(colAccent2)
	hintStyle     = lipgloss.NewStyle().Faint(true)

	inputBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colAccent).
			Padding(0, 1)

	choiceBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colAccent2).
			Padding(0, 1)
)

// maxInputLines caps how tall the input grows before it scrolls internally.
const maxInputLines = 6

type inputMode int

const (
	modeNormal inputMode = iota
	modeFreeAnswer
	modeChoice
)

type streamMsg stream.Record

// block is one rendered transcript entry; style is applied (and wrapped to the
// current width) at render time so resizing reflows cleanly.
type block struct {
	style lipgloss.Style
	body  string
}

// Run starts the TUI over the given session.
func Run(s *session.Session, ui config.UIConfig) error {
	_, err := tea.NewProgram(newModel(s, ui), tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()

	return err
}

type model struct {
	session *session.Session
	show    map[string]bool

	width  int
	height int
	ready  bool

	viewport viewport.Model
	input    textarea.Model
	spin     spinner.Model

	blocks  []block
	waiting bool

	mode      inputMode
	choices   []string
	choiceIdx int
}

func newModel(s *session.Session, ui config.UIConfig) model {
	in := textarea.New()
	in.Placeholder = "Message yerba…"
	in.Prompt = ""
	in.ShowLineNumbers = false
	in.CharLimit = 0
	in.SetHeight(1)
	in.MaxHeight = maxInputLines
	in.Focus()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = waitStyle

	show := map[string]bool{}

	for _, st := range ui.LogSubtypes {
		show[st] = true
	}

	return model{session: s, show: show, input: in, spin: sp, mode: modeNormal}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.spin.Tick, waitMsg(m.session.Stream()))
}

func waitMsg(ch <-chan stream.Record) tea.Cmd {
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
		(&m).handleStream(stream.Record(msg))

		return m, waitMsg(m.session.Stream())

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		var cmd tea.Cmd

		m.viewport, cmd = m.viewport.Update(msg)

		return m, cmd

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		if !m.ready {
			m.viewport = viewport.New(msg.Width, 1)
			m.ready = true
		}

		(&m).sync()

		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd

		m.spin, cmd = m.spin.Update(msg)

		return m, cmd
	}

	var cmd tea.Cmd

	m.input, cmd = m.input.Update(msg)
	(&m).sync()

	return m, cmd
}

func (m *model) handleStream(sm stream.Record) {
	switch sm.Type {

	case stream.TypeStatus:

		switch sm.Subtype {

		case stream.StatusInput, stream.StatusLLMRequest:
			m.waiting = true

		case stream.StatusLLMResponse:
			m.waiting = false
		}

	case stream.TypeAnswerUser:
		m.waiting = false
		m.appendBlock(block{answerStyle, "● " + asText(sm.Payload)})
		m.mode = modeNormal

	case stream.TypeAskUser:
		m.waiting = false
		q, choices := askPayload(sm.Payload)
		m.appendBlock(block{askStyle, "? " + q})

		if sm.Subtype == stream.SubtypeChoice && len(choices) > 0 {
			m.choices = choices
			m.choiceIdx = 0
			m.mode = modeChoice
		} else {
			m.mode = modeFreeAnswer
		}

	case stream.TypeChangeRoot:
		m.appendBlock(block{rootStyle, "📁 root: " + asText(sm.Payload)})

	case stream.TypeLog:

		if m.show[sm.Subtype] {
			m.appendBlock(logBlock(sm))
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

	switch msg.Type {

	case tea.KeyPgUp, tea.KeyPgDown, tea.KeyHome, tea.KeyEnd:
		var cmd tea.Cmd

		m.viewport, cmd = m.viewport.Update(msg)

		return m, cmd
	}

	if msg.Type == tea.KeyEnter {
		text := strings.TrimSpace(m.input.Value())

		if text == "" {
			return m, nil
		}

		m.input.Reset()
		(&m).appendBlock(block{userStyle, "› " + text})
		m.waiting = true

		if m.mode == modeFreeAnswer {
			m.session.Answer(text)
			m.mode = modeNormal
		} else {
			m.session.Send(context.Background(), text)
		}

		(&m).sync()

		return m, nil
	}

	var cmd tea.Cmd

	m.input, cmd = m.input.Update(msg)
	(&m).sync()

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
		(&m).appendBlock(block{userStyle, "› " + choice})
		m.waiting = true
		m.session.Answer(choice)
		m.mode = modeNormal
		m.choices = nil
		(&m).sync()
	}

	return m, nil
}

// sync recomputes the layout: it sizes the growable input, then sizes the
// transcript viewport to the space left over and (re)flows its content.
func (m *model) sync() {
	if !m.ready {
		return
	}

	// contentW is the box's inner text area: a lipgloss border (1 each side) plus
	// horizontal padding (1 each side) take 4 columns off the full width, so the
	// box style itself is sized to m.width-2 (see renderBottom) and the text
	// inside it to m.width-4.
	contentW := m.width - 4

	if contentW < 1 {
		contentW = 1
	}

	m.input.SetWidth(contentW)

	// Grow the input only when the text actually wraps past one line. We measure
	// the real wrapped height of the value at the input's width (word wrapping,
	// same as the textarea), so a line that still fits stays a single row.
	rows := lipgloss.Height(lipgloss.NewStyle().Width(contentW).Render(m.input.Value()))

	if rows < 1 {
		rows = 1
	}

	if rows > maxInputLines {
		rows = maxInputLines
	}

	m.input.SetHeight(rows)

	chrome := lipgloss.Height(m.renderHeader()) +
		lipgloss.Height(m.renderStatus()) +
		lipgloss.Height(m.renderBottom()) +
		lipgloss.Height(m.renderHint())

	vpH := m.height - chrome

	if vpH < 1 {
		vpH = 1
	}

	m.viewport.Width = m.width
	m.viewport.Height = vpH
	m.viewport.SetContent(m.transcript())
}

func (m *model) appendBlock(b block) {
	m.blocks = append(m.blocks, b)

	if m.ready {
		// Follow new output only when already at the bottom, so scrolling up to
		// read history isn't interrupted by incoming messages.
		stick := m.viewport.AtBottom()
		m.viewport.SetContent(m.transcript())

		if stick {
			m.viewport.GotoBottom()
		}
	}
}

func (m model) View() string {
	if !m.ready {
		return "starting yerba…"
	}

	view := lipgloss.JoinVertical(
		lipgloss.Left,
		m.renderHeader(),
		m.viewport.View(),
		m.renderStatus(),
		m.renderBottom(),
		m.renderHint(),
	)

	// Clamp to the terminal so an over-wide line (e.g. a long hint) can never
	// wrap and visually push the layout taller than the screen.
	return lipgloss.NewStyle().MaxWidth(m.width).MaxHeight(m.height).Render(view)
}

// --- rendering ---

func (m model) renderHeader() string {
	bar := titleStyle.Render("yerba") + "  " + brandStyle.Render("agent terminal")

	return bar + "\n"
}

func (m model) renderStatus() string {
	if m.waiting {
		return waitStyle.Render(m.spin.View() + " Waiting for llm…")
	}

	return " "
}

func (m model) renderBottom() string {
	if m.mode == modeChoice {
		var b strings.Builder

		for i, c := range m.choices {
			cursor := "  "

			if i == m.choiceIdx {
				cursor = "▸ "
			}

			line := cursor + c

			if i == m.choiceIdx {
				line = selectedStyle.Render(line)
			}

			b.WriteString(line)

			if i < len(m.choices)-1 {
				b.WriteString("\n")
			}
		}

		return choiceBoxStyle.Width(m.width - 2).Render(b.String())
	}

	return inputBoxStyle.Width(m.width - 2).Render(m.input.View())
}

func (m model) renderHint() string {
	var hint string

	switch m.mode {

	case modeChoice:
		hint = "↑/↓ select · enter confirm · esc quit"

	case modeFreeAnswer:
		hint = "enter answer · pgup/pgdn scroll · esc quit"

	default:
		hint = "enter send · pgup/pgdn scroll · esc quit"
	}

	line := hintStyle.Render(hint)

	if m.ready && !m.viewport.AtBottom() {
		line += "  " + hintStyle.Render(fmt.Sprintf("· %d%% ↓ more below", int(m.viewport.ScrollPercent()*100)))
	}

	return line
}

// transcript renders every block, wrapped to the viewport width, with a blank
// line between entries for breathing room.
func (m model) transcript() string {
	w := m.width

	if w < 1 {
		w = 80
	}

	rendered := make([]string, len(m.blocks))

	for i, blk := range m.blocks {
		rendered[i] = blk.style.Width(w).Render(blk.body)
	}

	return strings.Join(rendered, "\n\n")
}

// --- helpers ---

func logBlock(sm stream.Record) block {
	icon, style := logStyle(sm.Subtype)

	return block{style, icon + " " + logBody(sm)}
}

// logBody renders a LOG record as a human-readable line. TOOL_CALL/TOOL_RESULT
// payloads carry {"action":…, "params"/"result":…}, which we unpack into
// "tool: <name>, params: …" and "tool <name> result: …".
func logBody(sm stream.Record) string {
	fields, _ := sm.Payload.(map[string]any)

	switch sm.Subtype {

	case stream.LogToolCall:
		return "tool: " + asText(fields["action"]) + ", params: " + asText(fields["params"])

	case stream.LogToolResult:
		return "tool " + asText(fields["action"]) + " result: " + truncate(asText(fields["result"]), 400)

	case stream.LogError:
		return "error: " + asText(sm.Payload)

	default:
		return sm.Subtype + ": " + truncate(asText(sm.Payload), 400)
	}
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
