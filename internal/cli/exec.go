package cli

import (
	"os/exec"
	"strings"

	"github.com/original-flipster69/koko/internal/sandbox"
	"github.com/original-flipster69/koko/internal/ui"
)

type run struct{ sb *sandbox.Sandbox }

func (r run) name() string { return "run" }
func (r run) desc() string { return "Run a shell command directly" }
func (r run) args() string { return "<cmd>" }
func (r run) do(opts cmdOpts) (bool, string, string) {
	if len(opts.parts()) < 2 {
		return true, "", opts.scheme.Error("usage: :run <command>")
	}
	cmdStr := strings.TrimPrefix(opts.input, ":run ")
	return true, "", runShell(r.sb, cmdStr, opts.scheme)
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
