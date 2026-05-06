package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/meeseeks/koko/internal/agent"
	"github.com/meeseeks/koko/internal/ui"
)

func Run(
	a *agent.Agent,
	providerName string,
	kokoDir string,
	splash string,
	slashHandler SlashHandler,
) error {
	confirmCh := make(chan bool, 1)
	w := &tuiWriter{atStart: true}

	a.SetOutput(w)
	a.SetConfirm(func(action string) bool {
		w.program.Send(confirmRequestMsg(action))
		return <-confirmCh
	})
	a.SetSuppressSpinner(true)

	if providerName == "ollama" {
		splash += ui.Dim + ui.Gray + "  note: tool support depends on model (llama3.1+, mistral, command-r)" + ui.Reset + "\n\n"
	}

	ctx, cancel := context.WithCancel(context.Background())
	m := newModel(a, ctx, cancel, kokoDir, splash, slashHandler, confirmCh)

	p := tea.NewProgram(m, tea.WithAltScreen())
	w.program = p

	if _, err := p.Run(); err != nil {
		cancel()
		return err
	}
	cancel()
	return nil
}
