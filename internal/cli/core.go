package cli

import (
	"fmt"
	"strings"

	"github.com/original-flipster69/koko/internal/agent"
	"github.com/original-flipster69/koko/internal/ui"
)

// registerCoreCommands registers core commands like :koko, :clear, etc.
func registerCoreCommands(a *agent.Agent, scheme ui.Scheme) map[string]Command {
	return map[string]Command{
		":koko": {
			Desc: "Print the koko mascot",
			Fn: func(_ string, _ []string, _ *agent.Agent) (bool, string, string) {
				return true, "", "\n" + ui.Mascot(scheme)
			},
		},
		":clear": {
			Desc: "Reset conversation history",
			Fn: func(_ string, _ []string, a *agent.Agent) (bool, string, string) {
				a.ClearHistory()
				return true, "", scheme.Info("cleared", "conversation history reset")
			},
		},
		":history": {
			Desc: "Show message count",
			Fn: func(_ string, _ []string, a *agent.Agent) (bool, string, string) {
				return true, "", scheme.Info("messages", fmt.Sprintf("%d", a.HistoryLen()))
			},
		},
		":undo": {
			Desc: "Revert last file change",
			Fn: func(_ string, _ []string, a *agent.Agent) (bool, string, string) {
				path, err := a.Undo()
				switch {
				case err != nil:
					return true, "", scheme.Error(fmt.Sprintf("undo failed: %v", err))
				case path == "":
					return true, "", scheme.Info("undo", "nothing to undo")
				default:
					return true, "", scheme.Info("undo", fmt.Sprintf("reverted %s", path))
				}
			},
		},
		":tokens": {
			Desc: "Show token usage stats",
			Fn: func(_ string, _ []string, a *agent.Agent) (bool, string, string) {
				var b strings.Builder
				b.WriteString(scheme.Info("input   ", fmt.Sprintf("%d tokens", a.TotalInput)) + "\n")
				b.WriteString(scheme.Info("output  ", fmt.Sprintf("%d tokens", a.TotalOutput)) + "\n")
				b.WriteString(scheme.Info("total   ", fmt.Sprintf("%d tokens", a.TotalInput+a.TotalOutput)) + "\n")
				b.WriteString(scheme.Info("messages", fmt.Sprintf("%d", a.HistoryLen())))
				return true, "", b.String()
			},
		},
	}
}