package cli

import (
	"os"
	"regexp"
	"testing"
)

// TestReadmeCoversCommands guards against shipping a colon command that isn't
// documented in the README. It enumerates the canonical command list (plus the
// separately-registered :help) and checks each `:name` appears.
func TestReadmeCoversCommands(t *testing.T) {
	readme, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatalf("reading README: %v", err)
	}
	doc := string(readme)

	defs := append(commandList(nil, nil, nil, nil, "", "", "", Flags{}, nil), help{})
	for _, d := range defs {
		name := ":" + d.name()
		if !regexp.MustCompile(regexp.QuoteMeta(name) + `\b`).MatchString(doc) {
			t.Errorf("command %q is not documented in README.md", name)
		}
	}
}

// TestReadmeNoStaleCommands flags colon-prefixed tokens in the README's command
// table that no longer map to a real command, so docs don't drift the other way.
func TestReadmeNoStaleCommands(t *testing.T) {
	readme, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatalf("reading README: %v", err)
	}

	known := map[string]bool{":help": true, ":<play>": true, ":<name>": true, ":review": true}
	for _, d := range commandList(nil, nil, nil, nil, "", "", "", Flags{}, nil) {
		known[":"+d.name()] = true
	}

	rowRe := regexp.MustCompile("(?m)^\\| `(:[a-z]+)")
	for _, m := range rowRe.FindAllStringSubmatch(string(readme), -1) {
		if !known[m[1]] {
			t.Errorf("README documents %q which is not a registered command", m[1])
		}
	}
}
