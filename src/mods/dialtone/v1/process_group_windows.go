//go:build windows

package main

import "os/exec"

func setDetachedProcessGroup(cmd *exec.Cmd) {}
