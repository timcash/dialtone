package cli

import (
	"os"
	"os/exec"

	"dialtone/cli/src/core/logger"
)

func runInstallTests() {
	logger.LogInfo("Running Install Integration Test...")
	cmd := exec.Command("go", "run", "src/core/install/test/test.go")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logger.LogFatal("Install tests failed: %v", err)
	}
	logger.LogInfo("Install Integration Test passed!")
}
