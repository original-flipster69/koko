package cli

import "github.com/original-flipster69/koko/internal/agent"

// Command defines the structure for a CLI command.
type Command struct {
	Desc string
	Args string
	Fn   func(input string, parts []string, a *agent.Agent) (handled bool, prompt string, output string)
}