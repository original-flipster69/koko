package provider

import (
	"context"
	"errors"
	"time"
)

// FIXME: might have be increased... maybe also expose to config?
const streamIdleTimeout = 300 * time.Second

var errStreamStalled = errors.New("stream stalled: no data received within timeout")

type stallGuard struct {
	timer *time.Timer
	idle  time.Duration
}

func newStallGuard(ctx context.Context, idle time.Duration) (context.Context, *stallGuard) {
	cctx, cancel := context.WithCancelCause(ctx)
	g := &stallGuard{idle: idle}
	g.timer = time.AfterFunc(idle, func() { cancel(errStreamStalled) })
	context.AfterFunc(cctx, func() { g.timer.Stop() })
	return cctx, g
}

func (g *stallGuard) progress() {
	g.timer.Reset(g.idle)
}

func (g *stallGuard) done() {
	g.timer.Stop()
}

func streamErr(streamCtx context.Context, scanErr error) error {
	if cause := context.Cause(streamCtx); cause != nil && !errors.Is(cause, context.Canceled) {
		return cause
	}
	return scanErr
}
