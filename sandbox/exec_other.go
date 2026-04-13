//go:build !darwin && !linux

package sandbox

import (
	"os/exec"
)

func ExecAvailable() bool {
	return false
}

func ExecDescription() string {
	return "none (unsupported platform)"
}

func (s *Sandbox) WrapExec(ctx execContext, shellCmd string) *exec.Cmd {
	return exec.CommandContext(ctx.Ctx, "sh", "-c", shellCmd)
}
