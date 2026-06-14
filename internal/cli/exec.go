package cli

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/original-flipster69/koko/internal/agent"
	"github.com/original-flipster69/koko/internal/sandbox"
	"github.com/original-flipster69/koko/internal/ui"
)

// registerExecCommands registers exec operations like :run, :exec, etc.
func registerExecCommands(sb *sandbox.Sandbox, scheme ui.Scheme) map[string]Command {
	return map[string]Command{
		":run": {
			Desc: "Run a shell command directly",
			Args: "<cmd>",
			Fn: func(input string, parts []string, _ *agent.Agent) (bool, string, string) {
				if len(parts) < 2 {
					return true, "", scheme.Error("usage: :run <command>")
				}
				cmdStr := strings.TrimPrefix(input, ":run ")
				runCmd := exec.Command("sh", "-c", cmdStr)
				runCmd.Dir = sb.Root()
				output, err := runCmd.CombinedOutput()
				text := strings.TrimRight(string(output), "\n")
				if err != nil {
					return true, "", scheme.Error(text)
				}
				return true, "", text
			},
		},
		":exec": {
			Desc: "Execute a command with approval",
			Args: "<cmd>",
			Fn: func(input string, parts []string, a *agent.Agent) (bool, string, string) {
				if len(parts) < 2 {
					return true, "", scheme.Error("usage: :exec <command>")
				}
				cmdStr := strings.TrimPrefix(input, ":exec ")
				if !a.Confirm(fmt.Sprintf("Execute command: %s", cmdStr)) {
					return true, "", scheme.Info("exec", "cancelled")
				}
				runCmd := exec.Command("sh", "-c", cmdStr)
				runCmd.Dir = sb.Root()
				output, err := runCmd.CombinedOutput()
				text := strings.TrimRight(string(output), "\n")
				if err != nil {
					return true, "", scheme.Error(text)
				}
				return true, "", text
			},
		},
	}
}