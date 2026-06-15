package cli

import (
	"fmt"
	"strings"

	"github.com/original-flipster69/koko/internal/agent"
	"github.com/original-flipster69/koko/internal/plays"
	"github.com/original-flipster69/koko/internal/ui"
)

type playsCmd struct{ registry *plays.Registry }

func (p playsCmd) name() string { return "plays" }
func (p playsCmd) desc() string { return "List installed plays" }
func (p playsCmd) args() string { return "" }
func (p playsCmd) do(input string, parts []string, a *agent.Agent, scheme ui.Scheme) (bool, string, string) {
	list := p.registry.List()
	if len(list) == 0 {
		return true, "", scheme.Info("plays", fmt.Sprintf("none installed — add *.md files in %s", p.registry.Dir()))
	}
	var b strings.Builder
	for _, pl := range list {
		desc := pl.Description
		if desc == "" {
			desc = "(no description)"
		}
		b.WriteString(scheme.Info(pl.Name, desc) + "\n")
	}
	return true, "", strings.TrimRight(b.String(), "\n")
}
