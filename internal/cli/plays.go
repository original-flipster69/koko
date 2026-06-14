package cli

import (
	"github.com/original-flipster69/koko/internal/agent"
	"github.com/original-flipster69/koko/internal/plays"
	"github.com/original-flipster69/koko/internal/ui"
)

// registerPlayCommands registers dynamic play commands like :slang.
func registerPlayCommands(playRegistry *plays.Registry, scheme ui.Scheme) map[string]Command {
	commands := make(map[string]Command)
	for _, p := range playRegistry.List() {
		name := ":" + p.Name
		commands[name] = Command{
			Desc: p.Description,
			Fn: func(input string, parts []string, a *agent.Agent) (bool, string, string) {
				output, err := p.Run(a, parts[1:])
				if err != nil {
					return true, "", scheme.Error(err.Error())
				}
				return true, "", output
			},
		}
	}
	return commands
}