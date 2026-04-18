//go:build windows

package ops

import "os/exec"

func configureDetachedBackgroundProcess(_ *exec.Cmd) {}
