package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/original-flipster69/koko/internal/provider"
)

type fakeCounter struct {
	n      int
	err    error
	called bool
}

func (f *fakeCounter) CountTokens(ctx context.Context, msgs []provider.Msg, tools []provider.ToolDef) (int, error) {
	f.called = true
	return f.n, f.err
}

func TestMeasureTokensUsesCounter(t *testing.T) {
	fc := &fakeCounter{n: 4242}
	a := &Agent{counter: fc, history: []provider.Msg{
		{Role: provider.System, Content: "s"},
		{Role: provider.User, Content: "hi"},
	}}
	if got := a.measureTokens(context.Background()); got != 4242 {
		t.Errorf("got %d, want 4242", got)
	}
	if !fc.called {
		t.Error("token counter was not used")
	}
}

func TestMeasureTokensFallsBackOnError(t *testing.T) {
	fc := &fakeCounter{err: errors.New("boom")}
	a := &Agent{counter: fc, history: []provider.Msg{{Role: provider.User, Content: "hello world"}}}
	got := a.measureTokens(context.Background())
	if want := estimateMessagesTokens(a.history); got != want {
		t.Errorf("got %d, want fallback %d", got, want)
	}
}

func TestMeasureTokensFallsBackWhenNoCounter(t *testing.T) {
	a := &Agent{history: []provider.Msg{{Role: provider.User, Content: "hello world"}}}
	got := a.measureTokens(context.Background())
	if want := estimateMessagesTokens(a.history); got != want {
		t.Errorf("got %d, want fallback %d", got, want)
	}
}
