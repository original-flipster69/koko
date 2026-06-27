//go:build darwin

package sandbox

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

func ExecAvailable() bool {
	_, err := exec.LookPath("sandbox-exec")
	return err == nil
}

func ExecDescription() string {
	if ExecAvailable() {
		return "sandbox-exec (macOS)"
	}
	return "none (sandbox-exec not found)"
}

func (s *Sandbox) WrapExec(ctx execContext, shellCmd string) *exec.Cmd {
	if !ExecAvailable() {
		return exec.CommandContext(ctx.Ctx, "sh", "-c", shellCmd)
	}
	kokoDir := filepath.Join(s.root, ".koko")
	profile := fmt.Sprintf(`(version 1)
(allow default)
(deny file-write*)
(allow file-write* (subpath %q))
(allow file-write* (literal "/dev/null") (literal "/dev/tty") (literal "/dev/stdout") (literal "/dev/stderr"))
(allow file-write* (subpath "/private/tmp") (subpath "/private/var/folders") (subpath "/tmp"))
(deny file-read* (subpath %q))
(deny file-write* (subpath %q))
(deny network*)
(allow network* (local ip) (remote unix-socket))
`, s.root, kokoDir, kokoDir)
	return exec.CommandContext(ctx.Ctx, "sandbox-exec", "-p", profile, "sh", "-c", shellCmd)
}
