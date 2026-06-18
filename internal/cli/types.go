package cli

import (
	"strings"

	"github.com/original-flipster69/koko/internal/pushpuppet"
	"github.com/original-flipster69/koko/internal/ui"
)

type cmdDef interface {
	name() string
	desc() string
	args() string
	do(opts cmdOpts) string
}

type cmdOpts struct {
	input string
	a     *pushpuppet.PushPuppet
}

func (c cmdOpts) parts() []string {
	return strings.Fields(c.input)
}

func (c cmdOpts) scheme() ui.Scheme {
	return c.a.Scheme()
}

type command struct {
	desc string
	args string
	fn   func(opts cmdOpts) string
}
