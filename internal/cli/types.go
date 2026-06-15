package cli

import (
	"github.com/original-flipster69/koko/internal/agent"
	"github.com/original-flipster69/koko/internal/ui"
)

type cmdDef interface {
	name() string
	desc() string
	args() string
	do(input string, parts []string, a *agent.Agent, scheme ui.Scheme) (handled bool, prompt string, output string)
}

type command struct {
	desc string
	args string
	fn   func(input string, parts []string, a *agent.Agent, scheme ui.Scheme) (handled bool, prompt string, output string)
}

func register(cmds map[string]command, list ...cmdDef) {
	for _, c := range list {
		cmds[":"+c.name()] = command{desc: c.desc(), args: c.args(), fn: c.do}
	}
}
