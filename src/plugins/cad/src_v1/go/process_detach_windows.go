//go:build windows

package cad

import "os/exec"

func configureDetachedProcess(cmd *exec.Cmd) {
	_ = cmd
}
