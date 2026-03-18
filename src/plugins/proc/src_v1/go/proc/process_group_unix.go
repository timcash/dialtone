//go:build !windows

package proc

import (
	"os/exec"
	"syscall"
	"time"

	gopsprocess "github.com/shirou/gopsutil/v3/process"
)

func configureManagedCommand(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killManagedPID(pid int) error {
	if pid <= 0 {
		return syscall.ESRCH
	}
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil && err != syscall.ESRCH {
		return err
	}
	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		running, err := managedProcessRunning(pid)
		if err == syscall.ESRCH || !running {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
		return err
	}
	deadline = time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		running, err := managedProcessRunning(pid)
		if err == syscall.ESRCH || !running {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

func managedProcessRunning(pid int) (bool, error) {
	p, err := gopsprocess.NewProcess(int32(pid))
	if err != nil {
		return false, err
	}
	running, err := p.IsRunning()
	if err != nil {
		return false, err
	}
	return running, nil
}
