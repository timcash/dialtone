package cli

import (
	"os"
	"os/exec"

	"dialtone/dev/core/logger"
)

func runBuildTests() {
	logger.LogInfo("Running Build Integration Test...")
	cmd := exec.Command("go", "run", "src/core/build/test/integration.go")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logger.LogFatal("Build tests failed: %v", err)
	}
	logger.LogInfo("Build Integration Test passed!")
}
