package cli

import (
	"dialtone/dev/config"
	"dialtone/dev/plugins/logs/src_v1/go"
	"os"
	"os/exec"
	"path/filepath"
)

// RunAIInstall handles the installation steps for the AI plugin
func RunAIInstall(args []string) {
	logs.Info("AI Plugin: Checking dependencies...")

	dialtoneEnv := config.GetDialtoneEnv()
	if dialtoneEnv == "" {
		logs.Fatal("DIALTONE_ENV is not set.")
	}

	// Determine npm path
	npmPath := "npm" // default to system npm
	localNpm := filepath.Join(dialtoneEnv, "node", "bin", "npm")
	if _, err := os.Stat(localNpm); err == nil {
		npmPath = localNpm
		logs.Debug("AI Plugin: Using local npm at %s", npmPath)
	} else {
		// Check if npm is installed globally if local one isn't found
		_, err := exec.LookPath("npm")
		if err != nil {
			logs.Error("AI Plugin: npm not found. Please install Node.js and npm to use the Gemini CLI features.")
			return
		}
	}

	// Check if already installed
	geminiCliBin := filepath.Join(dialtoneEnv, "node", "bin", "gemini")
	if npmPath != localNpm {
		geminiCliBin = filepath.Join(dialtoneEnv, "bin", "gemini")
	}
	if _, err := os.Stat(geminiCliBin); err == nil {
		logs.Info("AI Plugin: @google/gemini-cli is already installed.")
		return
	}

	logs.Info("AI Plugin: Installing @google/gemini-cli...")

	// Create the directory if it doesn't exist
	if err := os.MkdirAll(dialtoneEnv, 0755); err != nil {
		logs.Error("AI Plugin: Failed to create dependency directory: %v", err)
		return
	}

	// Install using local npm prefix if using local node
	var cmd *exec.Cmd
	if npmPath == localNpm {
		// Install into the node directory so binary is in node/bin
		nodeDir := filepath.Join(dialtoneEnv, "node")
		logs.Info("AI Plugin: Running npm install -g --prefix %s @google/gemini-cli", nodeDir)
		cmd = exec.Command(npmPath, "install", "-g", "--prefix", nodeDir, "@google/gemini-cli")
	} else {
		logs.Info("AI Plugin: Running npm install --prefix %s @google/gemini-cli", dialtoneEnv)
		cmd = exec.Command(npmPath, "install", "--prefix", dialtoneEnv, "@google/gemini-cli")
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		logs.Error("AI Plugin: Failed to install @google/gemini-cli: %v", err)
		return
	}

	logs.Info("AI Plugin: @google/gemini-cli installed successfully.")
	logs.Info("AI Plugin: Installation complete.")
}
