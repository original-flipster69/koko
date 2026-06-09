package terminal

import (
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
)

func newTestModel(known []string) model {
	set := make(map[string]bool, len(known))
	for _, n := range known {
		set[n] = true
	}
	ta := textarea.New()
	return model{input: ta, knownCommands: set}
}

func TestRecognizedCommand(t *testing.T) {
	m := newTestModel([]string{":help", ":model", ":review"})
	cases := []struct {
		input    string
		wantName string
		wantOK   bool
	}{
		{":help", ":help", true},
		{":model gpt", ":model", true},
		{":review focus on auth", ":review", true},
		{"  :help  ", ":help", true},
		{":unknown", "", false},
		{"hello world", "", false},
		{":", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		m.input.SetValue(c.input)
		name, ok := m.recognizedCommand()
		if ok != c.wantOK || name != c.wantName {
			t.Errorf("input %q: got (%q,%v), want (%q,%v)", c.input, name, ok, c.wantName, c.wantOK)
		}
	}
}
