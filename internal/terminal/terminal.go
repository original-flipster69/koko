package terminal

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/original-flipster69/koko/internal/agent"
	"github.com/original-flipster69/koko/internal/ui"
)

func Run(
	a *agent.Agent,
	providerName string,
	kokoDir string,
	splashes []string,
	slashHandler CmdHandler,
	knownCommands []string,
) error {
	ctx, cancel := context.WithCancel(context.Background())
	confirmCh := make(chan bool, 1)
	w := &tuiWriter{atStart: true}

	a.SetOutput(w)
	a.SetConfirm(func(action string) bool {
		w.program.Send(confirmRequestMsg(action))
		select {
		case ok := <-confirmCh:
			return ok
		case <-ctx.Done():
			return false
		}
	})
	a.SetSuppressSpinner(true)

	if providerName == "ollama" {
		suffix := ui.Dim + ui.Muted + "  note: tool support depends on model (llama3.1+, mistral, command-r)" + ui.Reset + "\n\n"
		for i := range splashes {
			splashes[i] += suffix
		}
	}

	m := newModel(a, ctx, cancel, kokoDir, splashes, slashHandler, confirmCh, knownCommands)

	p := tea.NewProgram(m, tea.WithAltScreen())
	w.program = p

	if _, err := p.Run(); err != nil {
		cancel()
		return err
	}
	cancel()
	return nil
}
