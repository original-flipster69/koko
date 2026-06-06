package agent

import (
	"context"
	"io"
	"testing"

	"github.com/original-flipster69/koko/internal/provider"
)

type fakeProvider struct {
	name   string
	called bool
}

func (f *fakeProvider) ChatStream(ctx context.Context, msgs []provider.Msg, tools []provider.ToolDef, onDelta func(provider.StreamDelta)) (*provider.Response, error) {
	f.called = true
	return &provider.Response{Content: "done"}, nil
}

func (f *fakeProvider) Name() string    { return f.name }
func (f *fakeProvider) Model() string   { return f.name }
func (f *fakeProvider) SetModel(string) {}

func TestNextProviderIsOneShot(t *testing.T) {
	base := &fakeProvider{name: "base"}
	oneShot := &fakeProvider{name: "oneshot"}
	a := New(base, nil, io.Discard, nil, nil, Options{})
	a.SetSuppressSpinner(true)

	a.SetNextProvider(oneShot)
	if err := a.Run(context.Background(), "hi"); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !oneShot.called {
		t.Error("one-shot provider was not used for the play run")
	}
	if base.called {
		t.Error("base provider should not have been used during the one-shot run")
	}
	if a.provider != base {
		t.Error("provider did not revert to base after the run")
	}

	base.called, oneShot.called = false, false
	if err := a.Run(context.Background(), "again"); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !base.called {
		t.Error("base provider should be used on the next run")
	}
	if oneShot.called {
		t.Error("one-shot provider used twice; must apply to a single run only")
	}
}
