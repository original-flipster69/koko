package terminal

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/original-flipster69/koko/internal/pushpuppet"
	"github.com/original-flipster69/koko/internal/ui"
)

type pushPuppetDoneMsg struct{ err error }
type confirmRequestMsg string
type spinnerTickMsg struct{}
type splashTickMsg struct{}
type statusTickMsg struct{}
type fadeTickMsg struct{}

const splashFrameDuration = 400 * time.Millisecond
const statusSwitchDuration = 8 * time.Second
const fadeTickDuration = 50 * time.Millisecond

var splashSequence = []int{1, 0, 1, 0, 2, 0}

func splashTickCmd() tea.Cmd {
	return tea.Tick(splashFrameDuration, func(time.Time) tea.Msg {
		return splashTickMsg{}
	})
}

func statusTickCmd() tea.Cmd {
	return tea.Tick(statusSwitchDuration, func(time.Time) tea.Msg {
		return statusTickMsg{}
	})
}

func fadeTickCmd() tea.Cmd {
	return tea.Tick(fadeTickDuration, func(time.Time) tea.Msg {
		return fadeTickMsg{}
	})
}

var (
	zodiac      = []string{"♈︎", "♉︎", "♊︎", "♋︎", "♌︎", "♍︎", "♎︎", "♏︎", "♐︎", "♑︎", "♒︎", "♓︎"}
	transitions = []string{"·", "✧", "•", "✦", "⋆", "✶", "∙", "✱"}
	beats       = []time.Duration{190 * time.Millisecond, 190 * time.Millisecond, 560 * time.Millisecond}
	dots        = []string{"   ", ".  ", ".. ", "..."}
)

type model struct {
	viewport viewport.Model
	input    textarea.Model
	content  *strings.Builder

	pushPuppet *pushpuppet.PushPuppet
	ctx        context.Context
	cancel     context.CancelFunc
	runCancel  context.CancelFunc
	kokoDir    string
	splashes   []string
	splashIdx  int
	splashHit  int
	termWidth  int
	termHeight int

	confirmCh      chan bool
	confirmMode    bool
	confirmText    string
	pushPuppetBusy bool
	ready          bool
	quitting       bool
	spinnerTick    int
	spinnerLabel   string

	cmdHandler     CmdHandler
	knownCommands  map[string]bool
	scheme         ui.Scheme
	effortLabel    string
	planModeOn     bool
	statusLabel    string
	statusFade     float64
	statusFadingIn bool
	inputRows      int
}

type CmdHandler func(input string, a *pushpuppet.PushPuppet) (handled bool, prompt string, output string)

func newModel(a *pushpuppet.PushPuppet, ctx context.Context, cancel context.CancelFunc, kokoDir string, splashes []string, cmdHandler CmdHandler, confirmCh chan bool, knownCommands []string, scheme ui.Scheme) model {
	ta := textarea.New()
	ta.Placeholder = "ask koko anything... (alt+enter or ctrl+j for newline)"
	ta.Focus()
	ta.CharLimit = 8192
	ta.SetHeight(1)
	ta.SetWidth(80)
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.KeyMap.InsertNewline.SetKeys("alt+enter", "ctrl+j", "shift+enter")

	known := make(map[string]bool, len(knownCommands))
	for _, name := range knownCommands {
		known[name] = true
	}

	m := model{
		input:      ta,
		content:    &strings.Builder{},
		pushPuppet: a,
		ctx:        ctx,
		cancel:     cancel,
		kokoDir:    kokoDir,
		splashes:   splashes,
		confirmCh:  confirmCh,
		cmdHandler: cmdHandler, knownCommands: known,
		scheme:         scheme,
		effortLabel:    a.Effort().String(),
		planModeOn:     a.PlanMode(),
		statusLabel:    fmt.Sprintf("effort: %s", a.Effort().String()),
		statusFade:     1.0,
		statusFadingIn: true,
	}
	return m
}

func (m model) recognizedCommand() (string, bool) {
	in := strings.TrimSpace(m.input.Value())
	if !strings.HasPrefix(in, ":") {
		return "", false
	}
	name := strings.Fields(in)[0]
	if m.knownCommands[name] {
		return name, true
	}
	return "", false
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{textarea.Blink}
	if len(m.splashes) > 1 {
		cmds = append(cmds, splashTickCmd())
	}
	cmds = append(cmds, statusTickCmd())
	cmds = append(cmds, fadeTickCmd())
	return tea.Batch(cmds...)
}

func spinnerTickCmd(tick int) tea.Cmd {
	phase := tick % 3
	return tea.Tick(beats[phase], func(time.Time) tea.Msg {
		return spinnerTickMsg{}
	})
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		ui.SetTermWidth(msg.Width)
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-5)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
		}
		m.input.SetWidth(msg.Width - 6)
		m.resizeInput()
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			if m.pushPuppetBusy {
				if m.runCancel != nil {
					m.runCancel()
				}
				m.pushPuppetBusy = false
				m.appendOutput(fmt.Sprintf("\n%s%sinterrupted%s\n", ui.Dim, m.scheme.Muted, ui.Reset))
				return m, nil
			}
			m.quitting = true
			m.cancel()
			return m, tea.Quit

		case tea.KeyPgUp, tea.KeyPgDown, tea.KeyUp, tea.KeyDown:
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd

		case tea.KeyEnter:
			if m.confirmMode {
				answer := strings.TrimSpace(strings.ToLower(m.input.Value()))
				m.input.Reset()
				m.confirmMode = false
				m.confirmCh <- answer == "y" || answer == "yes"
				return m, nil
			}

			input := strings.TrimSpace(m.input.Value())
			if input == "" {
				return m, nil
			}
			m.input.Reset()
			m.resizeInput()

			if input == "exit" || input == "quit" {
				m.appendOutput("\n" + ui.Goodbye(m.scheme) + "\n")
				m.quitting = true
				m.cancel()
				return m, tea.Quit
			}

			display := "▼ " + input
			if strings.Contains(display, "\n") {
				lines := strings.Split(display, "\n")
				display = lines[0] + fmt.Sprintf(" (+%d lines)", len(lines)-1)
			}
			block := userEchoStyle.Width(m.viewport.Width - 2).Render(display)
			m.appendOutput("\n" + block + "\n\n")

			if strings.HasPrefix(input, ":") && m.cmdHandler != nil {
				handled, prompt, output := m.cmdHandler(input, m.pushPuppet)
				m.scheme = m.pushPuppet.Scheme()
				m.effortLabel = m.pushPuppet.Effort().String()
				m.planModeOn = m.pushPuppet.PlanMode()
				// Update status label if it's currently showing effort or plan
				if strings.HasPrefix(m.statusLabel, "effort:") {
					m.statusLabel = fmt.Sprintf("effort: %s", m.effortLabel)
				} else if strings.HasPrefix(m.statusLabel, "plan:") {
					m.statusLabel = fmt.Sprintf("plan: %s", map[bool]string{true: "ON", false: "OFF"}[m.planModeOn])
				}
				if output != "" {
					m.appendOutput(output + "\n")
				}
				if handled && prompt == "" {
					return m, nil
				}
				if prompt != "" {
					input = prompt
				}
			}

			m.pushPuppetBusy = true
			m.spinnerTick = 0
			m.spinnerLabel = m.pushPuppet.ThinkingVerb()
			runCtx, runCancel := context.WithCancel(m.ctx)
			m.runCancel = runCancel
			runInput := input
			return m, tea.Batch(
				func() tea.Msg {
					err := m.pushPuppet.Run(runCtx, runInput)
					runCancel()
					return pushPuppetDoneMsg{err: err}
				},
				spinnerTickCmd(0),
			)
		}
	case spinnerTickMsg:
		if m.pushPuppetBusy {
			m.spinnerTick++
			cmds = append(cmds, spinnerTickCmd(m.spinnerTick))
		}
		return m, tea.Batch(cmds...)

	case splashTickMsg:
		if m.splashHit < len(splashSequence) {
			m.splashIdx = splashSequence[m.splashHit]
			m.splashHit++
			m.syncViewport()
			if m.splashHit < len(splashSequence) {
				return m, splashTickCmd()
			}
		}
		return m, nil

	case statusTickMsg:
		// Toggle between effort and plan mode labels
		if strings.HasPrefix(m.statusLabel, "effort:") {
			m.statusLabel = fmt.Sprintf("plan: %s", map[bool]string{true: "ON", false: "OFF"}[m.planModeOn])
			m.statusFade = 0.0
			m.statusFadingIn = true
		} else {
			m.statusLabel = fmt.Sprintf("effort: %s", m.effortLabel)
			m.statusFade = 0.0
			m.statusFadingIn = true
		}
		cmds = append(cmds, statusTickCmd())
		return m, tea.Batch(cmds...)

	case fadeTickMsg:
		// Handle fade in/out animation
		if m.statusFadingIn {
			m.statusFade += 0.05
			if m.statusFade >= 1.0 {
				m.statusFade = 1.0
				m.statusFadingIn = false
			}
		} else {
			m.statusFade -= 0.05
			if m.statusFade <= 0.0 {
				m.statusFade = 0.0
				m.statusFadingIn = true
			}
		}
		cmds = append(cmds, fadeTickCmd())
		return m, tea.Batch(cmds...)

	case outputMsg:
		atBottom := m.viewport.AtBottom()
		m.content.WriteString(string(msg))
		m.syncViewport()
		if atBottom {
			m.viewport.GotoBottom()
		}
		return m, nil

	case pushPuppetDoneMsg:
		m.pushPuppetBusy = false
		if msg.err != nil {
			m.appendOutput("\n" + m.scheme.Error(msg.err.Error()) + "\n")
		} else {
			m.appendOutput(m.scheme.TokenStats(m.pushPuppet.TotalInput, m.pushPuppet.TotalOutput) + "\n")
		}
		_ = m.pushPuppet.SaveSession(m.kokoDir)
		return m, nil

	case confirmRequestMsg:
		m.confirmMode = true
		m.confirmText = string(msg)
		m.input.Reset()
		return m, nil
	}

	if !m.pushPuppetBusy || m.confirmMode {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
		m.resizeInput()
	}

	return m, tea.Batch(cmds...)
}

const maxInputLines = 8

func (m *model) resizeInput() {
	m.input.SetHeight(maxInputLines)
	rows := contentRows(m.input.View())
	if rows < 1 {
		rows = 1
	}
	if rows > maxInputLines {
		rows = maxInputLines
	}
	m.inputRows = rows
	if m.termHeight > 0 {
		h := m.termHeight - 4 - rows
		if h < 1 {
			h = 1
		}
		m.viewport.Height = h
	}
	m.syncViewport()
}

func cropLines(s string, n int) string {
	if n < 1 {
		n = 1
	}
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}

func contentRows(view string) int {
	n := 0
	for i, line := range strings.Split(view, "\n") {
		if strings.TrimSpace(stripANSI(line)) != "" {
			n = i + 1
		}
	}
	return n
}

func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		if r == 0x1b {
			inEsc = true
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func (m *model) appendOutput(s string) {
	m.content.WriteString(s)
	m.syncViewport()
	m.viewport.GotoBottom()
}

func (m *model) syncViewport() {
	splash := ""
	if m.splashIdx < len(m.splashes) {
		splash = m.splashes[m.splashIdx]
	}
	content := splash + m.content.String()
	if m.viewport.Width > 0 {
		content = lipgloss.NewStyle().Width(m.viewport.Width).Render(content)
	}
	m.viewport.SetContent(content)
}

func (m model) spinnerView() string {
	i := m.spinnerTick
	phase := i % 3
	cycle := i / 3
	var frame string
	switch phase {
	case 0:
		frame = transitions[(2*cycle)%len(transitions)]
	case 1:
		frame = transitions[(2*cycle+1)%len(transitions)]
	case 2:
		frame = zodiac[cycle%len(zodiac)]
	}
	dot := dots[cycle%len(dots)]
	return fmt.Sprintf("%s%s%-2s%s %s%s%s%s", ui.Bold, m.scheme.Primary, frame, ui.Reset, m.scheme.Primary, m.spinnerLabel, dot, ui.Reset)
}

var inputBarStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderTop(true).
	BorderForeground(lipgloss.Color("135"))

var userEchoStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("234")).
	Foreground(lipgloss.Color("141")).
	Padding(0, 1)

func getStatusStyle(fade float64) lipgloss.Style {
	// Map fade value (0.0-1.0) to color intensity
	// Fade between 238 (very dim gray) and 255 (bright white)
	if fade <= 0.0 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	}
	if fade >= 1.0 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
	}
	// Interpolate between 238 and 255 based on fade
	intensity := int(238 + (255-238)*fade)
	return lipgloss.NewStyle().Foreground(lipgloss.Color(fmt.Sprintf("%d", intensity)))
}

func (m model) View() string {
	if !m.ready {
		return "loading..."
	}
	if m.quitting {
		return ""
	}

	var statusLine string
	if m.pushPuppetBusy {
		statusLine = fmt.Sprintf("  %s", m.spinnerView())
	}

	inputView := cropLines(m.input.View(), m.inputRows)

	var inputLine string
	if m.confirmMode {
		inputLine = fmt.Sprintf("  %srun:%s %s  [y/N] %s", m.scheme.Secondary, ui.Reset, m.confirmText, inputView)
	} else if name, ok := m.recognizedCommand(); ok {
		inputLine = fmt.Sprintf("%s%s▶ %s%s%s", ui.Bold, m.scheme.Label, name, ui.Reset, strings.Replace(inputView, name, "", 1))
	} else {
		inputLine = fmt.Sprintf("%s▶%s %s", m.scheme.Primary, ui.Reset, inputView)
	}

	footer := lipgloss.PlaceHorizontal(m.termWidth, lipgloss.Right,
		getStatusStyle(m.statusFade).Render(fmt.Sprintf("%s ", m.statusLabel)))

	return m.viewport.View() + "\n" + statusLine + "\n" + inputBarStyle.Render(inputLine) + "\n" + footer
}
