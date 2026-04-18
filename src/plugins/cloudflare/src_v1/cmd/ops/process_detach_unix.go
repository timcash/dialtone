//go:build !windows

package ops

import (
	"os/exec"
	"syscall"
)

func configureDetachedBackgroundProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
