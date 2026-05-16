package terminal

import tea "github.com/charmbracelet/bubbletea"

const responseIndent = "  "

type outputMsg string

type tuiWriter struct {
	program *tea.Program
	atStart bool
}

func (w *tuiWriter) Write(p []byte) (int, error) {
	if w.program == nil {
		return len(p), nil
	}
	s := string(p)
	var out []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\n' {
			out = append(out, c)
			w.atStart = true
			continue
		}
		if w.atStart {
			out = append(out, responseIndent...)
			w.atStart = false
		}
		out = append(out, c)
	}
	w.program.Send(outputMsg(string(out)))
	return len(p), nil
}
