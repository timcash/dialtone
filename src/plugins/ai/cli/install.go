package cli

import (
	"dialtone/cli/src/core/logger"
	"os"
	"os/exec"

	"github.com/joho/godotenv"
)

// RunAIInstall handles the installation steps for the AI plugin
func RunAIInstall(args []string) {
	logger.LogInfo("AI Plugin: Checking dependencies...")

	// Load .env to get DIALTONE_ENV
	if err := godotenv.Load(); err != nil {
		logger.LogDebug("AI Plugin: No .env file found, using defaults")
	}

	dialtoneEnv := os.Getenv("DIALTONE_ENV")
	if dialtoneEnv == "" {
		logger.LogFatal("DIALTONE_ENV is not set. Please add it to your .env file.")
	}

	// Check if npm is installed
	_, err := exec.LookPath("npm")
	if err != nil {
		logger.LogError("AI Plugin: npm not found. Please install Node.js and npm to use the Gemini CLI features.")
		return
	}

	logger.LogInfo("AI Plugin: Installing @google/gemini-cli to %s...", dialtoneEnv)

	// Create the directory if it doesn't exist
	if err := os.MkdirAll(dialtoneEnv, 0755); err != nil {
		logger.LogError("AI Plugin: Failed to create dependency directory: %v", err)
		return
	}

	// Install locally using --prefix to dialtoneEnv
	cmd := exec.Command("npm", "install", "--prefix", dialtoneEnv, "@google/gemini-cli")
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.LogError("AI Plugin: Failed to install @google/gemini-cli: %v", err)
		logger.LogDebug("npm output: %s", string(output))
		return
	}

	logger.LogInfo("AI Plugin: @google/gemini-cli installed successfully in %s.", dialtoneEnv)
	logger.LogInfo("AI Plugin: Installation complete.")
}
