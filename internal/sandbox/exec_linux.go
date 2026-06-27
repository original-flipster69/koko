//go:build linux

package sandbox

import (
	"os/exec"
	"path/filepath"
)

func ExecAvailable() bool {
	_, err := exec.LookPath("bwrap")
	return err == nil
}

func ExecDescription() string {
	if ExecAvailable() {
		return "bubblewrap (bwrap)"
	}
	return "none (bwrap not found)"
}

func (s *Sandbox) WrapExec(ctx execContext, shellCmd string) *exec.Cmd {
	if !ExecAvailable() {
		return exec.CommandContext(ctx.Ctx, "sh", "-c", shellCmd)
	}
	args := []string{
		"--ro-bind", "/usr", "/usr",
		"--ro-bind", "/bin", "/bin",
		"--ro-bind", "/lib", "/lib",
		"--ro-bind", "/lib64", "/lib64",
		"--ro-bind", "/etc", "/etc",
		"--bind", s.root, s.root,
		"--tmpfs", filepath.Join(s.root, ".koko"),
		"--bind", "/tmp", "/tmp",
		"--proc", "/proc",
		"--dev", "/dev",
		"--unshare-all",
		"--share-net",
		"--die-with-parent",
		"--chdir", s.root,
		"sh", "-c", shellCmd,
	}
	return exec.CommandContext(ctx.Ctx, "bwrap", args...)
}
