package app

import (
	"fmt"
	"os"
	"os/exec"

	"dialtone/dev/logger"
)

// RunOpencodeServer starts the opencode AI assistant server
func RunOpencodeServer(port int) {
	opencodePath := os.ExpandEnv("$HOME/.opencode/bin/opencode")
	if _, err := os.Stat(opencodePath); os.IsNotExist(err) {
		logger.LogInfo("opencode binary not found at %s, skipping...", opencodePath)
		return
	}

	logger.LogInfo("Starting opencode server on port %d...", port)
	cmd := exec.Command(opencodePath, "--port", fmt.Sprintf("%d", port))

	// Create log file
	logFile, err := os.OpenFile("opencode.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		logger.LogInfo("Failed to create opencode log file: %v", err)
		return
	}
	defer logFile.Close()

	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Run(); err != nil {
		logger.LogInfo("opencode server exited: %v", err)
	}
}
