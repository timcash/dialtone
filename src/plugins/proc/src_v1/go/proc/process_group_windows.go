//go:build windows

package proc

import (
	"os/exec"

	gopsprocess "github.com/shirou/gopsutil/v3/process"
)

func configureManagedCommand(cmd *exec.Cmd) {
}

func killManagedPID(pid int) error {
	p, err := gopsprocess.NewProcess(int32(pid))
	if err != nil {
		return err
	}
	return p.Kill()
}
