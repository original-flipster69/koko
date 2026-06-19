package provider

import (
	"context"
	"errors"
	"time"
)

const streamIdleTimeout = 30 * time.Second

var errStreamStalled = errors.New("stream stalled: no data received within timeout")

type stallGuard struct {
	reset chan struct{}
	stop  chan struct{}
}

func newStallGuard(ctx context.Context, idle time.Duration) (context.Context, *stallGuard) {
	cctx, cancel := context.WithCancelCause(ctx)
	g := &stallGuard{
		reset: make(chan struct{}, 1),
		stop:  make(chan struct{}),
	}
	go func() {
		timer := time.NewTimer(idle)
		defer timer.Stop()
		for {
			select {
			case <-g.stop:
				return
			case <-cctx.Done():
				return
			case <-g.reset:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(idle)
			case <-timer.C:
				cancel(errStreamStalled)
				return
			}
		}
	}()
	return cctx, g
}

func (g *stallGuard) progress() {
	select {
	case g.reset <- struct{}{}:
	default:
	}
}

func (g *stallGuard) done() {
	close(g.stop)
}

func streamErr(streamCtx context.Context, scanErr error) error {
	if cause := context.Cause(streamCtx); cause != nil && !errors.Is(cause, context.Canceled) {
		return cause
	}
	return scanErr
}
