package cli

import (
	"strings"

	"github.com/original-flipster69/koko/internal/agent"
	"github.com/original-flipster69/koko/internal/ui"
)

type cmdDef interface {
	name() string
	desc() string
	args() string
	do(opts cmdOpts) (handled bool, prompt string, output string)
}

type cmdOpts struct {
	input  string
	a      *agent.Agent
	scheme ui.Scheme
}

func (c cmdOpts) parts() []string {
	return strings.Fields(c.input)
}

type command struct {
	desc string
	args string
	fn   func(opts cmdOpts) (handled bool, prompt string, output string)
}

func register(cmds map[string]command, list ...cmdDef) {
	for _, c := range list {
		cmds[":"+c.name()] = command{desc: c.desc(), args: c.args(), fn: c.do}
	}
}
