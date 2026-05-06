package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/meeseeks/koko/internal/agent"
	"github.com/meeseeks/koko/internal/audit"
	"github.com/meeseeks/koko/internal/config"
	"github.com/meeseeks/koko/internal/memory"
	"github.com/meeseeks/koko/internal/plays"
	"github.com/meeseeks/koko/internal/policy"
	"github.com/meeseeks/koko/internal/provider"
	"github.com/meeseeks/koko/internal/sandbox"
	"github.com/meeseeks/koko/internal/ui"
)

func Run(
	cfg *config.Config,
	llm provider.Provider,
	sb *sandbox.Sandbox,
	auditLog *audit.Log,
	memoryStore *memory.Store,
	cmdPolicy *policy.CommandPolicy,
	playRegistry *plays.Registry,
	projectContext string,
	kokoDir string,
	splash string,
	slashHandler SlashHandler,
) error {
	confirmCh := make(chan bool, 1)
	w := &tuiWriter{atStart: true}

	confirm := func(action string) bool {
		w.program.Send(confirmRequestMsg(action))
		return <-confirmCh
	}

	a := agent.New(llm, sb, w, confirm, auditLog, projectContext)
	a.SetThinkingVerbs(cfg.ThinkingVerbs)
	a.SetMemory(memoryStore)
	a.SetCommandPolicy(cmdPolicy)
	a.SetLimits(cfg.MaxToolCalls, cfg.MaxSessionTokens)
	a.SetScrubPII(cfg.ScrubPII)
	a.SetQuietTools(cfg.QuietToolOutputs)
	a.SetExecLimits(cfg.ExecCPUSeconds, cfg.ExecMemoryMB, cfg.ExecMaxFileMB)
	a.SetSuppressSpinner(true)

	if llm.Name() == "ollama" {
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
