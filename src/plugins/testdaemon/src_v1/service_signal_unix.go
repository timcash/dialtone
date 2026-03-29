//go:build !windows

package testdaemon

import (
	"os"
	"syscall"
)

func serviceSignals() []os.Signal {
	return []os.Signal{os.Interrupt, syscall.SIGTERM}
}
