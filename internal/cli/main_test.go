package cli

import (
	"io"
	"strings"
	"testing"
)

func TestConfirmElevated(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"y\n", true},
		{"yes\n", true},
		{"Y\n", true},
		{"YES\n", true},
		{"n\n", false},
		{"no\n", false},
		{"\n", false},
		{"", false},
		{"maybe\n", false},
	}
	for _, c := range cases {
		got := confirmElevated(strings.NewReader(c.input), io.Discard)
		if got != c.want {
			t.Errorf("input %q: got %v, want %v", c.input, got, c.want)
		}
	}
}
