//go:build !windows

package repl

import (
	"os/exec"
	"syscall"
)

func configureDetachedBackgroundProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
