package cli

import (
	"dialtone/cli/src/core/logger"
	"os"
	"os/exec"
	"path/filepath"

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

	// Determine npm path
	npmPath := "npm" // default to system npm
	localNpm := filepath.Join(dialtoneEnv, "node", "bin", "npm")
	if _, err := os.Stat(localNpm); err == nil {
		npmPath = localNpm
		logger.LogDebug("AI Plugin: Using local npm at %s", npmPath)
	} else {
		// Check if npm is installed globally if local one isn't found
		_, err := exec.LookPath("npm")
		if err != nil {
			logger.LogError("AI Plugin: npm not found. Please install Node.js and npm to use the Gemini CLI features.")
			return
		}
	}

	logger.LogInfo("AI Plugin: Installing @google/gemini-cli...")

	// Create the directory if it doesn't exist
	if err := os.MkdirAll(dialtoneEnv, 0755); err != nil {
		logger.LogError("AI Plugin: Failed to create dependency directory: %v", err)
		return
	}

	// Install using local npm prefix if using local node
	var cmd *exec.Cmd
	if npmPath == localNpm {
		// Install into the node directory so binary is in node/bin
		nodeDir := filepath.Join(dialtoneEnv, "node")
		logger.LogDebug("AI Plugin: Installing with --prefix %s -g", nodeDir)
		cmd = exec.Command(npmPath, "install", "-g", "--prefix", nodeDir, "@google/gemini-cli")
	} else {
		logger.LogDebug("AI Plugin: Installing with --prefix %s", dialtoneEnv)
		cmd = exec.Command(npmPath, "install", "--prefix", dialtoneEnv, "@google/gemini-cli")
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.LogError("AI Plugin: Failed to install @google/gemini-cli: %v", err)
		logger.LogDebug("npm output: %s", string(output))
		return
	}

	logger.LogInfo("AI Plugin: @google/gemini-cli installed successfully.")
	logger.LogInfo("AI Plugin: Installation complete.")
}
