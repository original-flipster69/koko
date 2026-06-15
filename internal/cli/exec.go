package cli

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/original-flipster69/koko/internal/agent"
	"github.com/original-flipster69/koko/internal/sandbox"
	"github.com/original-flipster69/koko/internal/ui"
)

type run struct{ sb *sandbox.Sandbox }

func (r run) name() string { return "run" }
func (r run) desc() string { return "Run a shell command directly" }
func (r run) args() string { return "<cmd>" }
func (r run) do(input string, parts []string, a *agent.Agent, scheme ui.Scheme) (bool, string, string) {
	if len(parts) < 2 {
		return true, "", scheme.Error("usage: :run <command>")
	}
	cmdStr := strings.TrimPrefix(input, ":run ")
	return true, "", runShell(r.sb, cmdStr, scheme)
}

type execCmd struct{ sb *sandbox.Sandbox }

func (e execCmd) name() string { return "exec" }
func (e execCmd) desc() string { return "Execute a command with approval" }
func (e execCmd) args() string { return "<cmd>" }
func (e execCmd) do(input string, parts []string, a *agent.Agent, scheme ui.Scheme) (bool, string, string) {
	if len(parts) < 2 {
		return true, "", scheme.Error("usage: :exec <command>")
	}
	cmdStr := strings.TrimPrefix(input, ":exec ")
	if !a.Confirm(fmt.Sprintf("Execute command: %s", cmdStr)) {
		return true, "", scheme.Info("exec", "cancelled")
	}
	return true, "", runShell(e.sb, cmdStr, scheme)
}

func runShell(sb *sandbox.Sandbox, cmdStr string, scheme ui.Scheme) string {
	runCmd := exec.Command("sh", "-c", cmdStr)
	runCmd.Dir = sb.Root()
	output, err := runCmd.CombinedOutput()
	text := strings.TrimRight(string(output), "\n")
	if err != nil {
		return scheme.Error(text)
	}
	return text
}
