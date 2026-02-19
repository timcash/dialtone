package util

import (
	"os"
	"os/signal"
	"syscall"

	"dialtone/dev/core/logger"
)

// WaitForShutdown blocks until SIGINT or SIGTERM is received
func WaitForShutdown() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	logger.LogInfo("Received shutdown signal. Exiting gracefully...")
}
