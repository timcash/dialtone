//go:build windows

package repl

import "os/exec"

func configureDetachedBackgroundProcess(_ *exec.Cmd) {}
