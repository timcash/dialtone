package cli

import (
	"dialtone/cli/src/core/logger"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

// RunGemini handles the --gemini flag functionality by proxying to @google/gemini-cli
func RunGemini(args []string) {
	if len(args) == 0 {
		logger.LogInfo("Gemini: Please provide a prompt. Usage: dialtone ai --gemini \"prompt\"")
		return
	}

	// Load .env to get DIALTONE_ENV and GOOGLE_API_KEY
	if err := godotenv.Load(); err != nil {
		logger.LogDebug("Gemini: No .env file found")
	}

	dialtoneEnv := os.Getenv("DIALTONE_ENV")
	if dialtoneEnv == "" {
		logger.LogFatal("DIALTONE_ENV is not set. Please add it to your .env file.")
	}

	// Check for API Key
	googleKey := os.Getenv("GOOGLE_API_KEY")
	geminiKey := os.Getenv("GEMINI_API_KEY")

	if googleKey == "" && geminiKey == "" {
		logger.LogInfo("Gemini: GOOGLE_API_KEY is not set. You can get one at: https://aistudio.google.com/app/apikey")
		logger.LogInfo("Please add GOOGLE_API_KEY=your_key to your .env file.")
		return
	}

	// gemini-cli specifically looks for GEMINI_API_KEY
	if geminiKey == "" && googleKey != "" {
		os.Setenv("GEMINI_API_KEY", googleKey)
	}

	// The gemini executable should be in node_modules/.bin/gemini inside dialtoneEnv
	geminiPath := filepath.Join(dialtoneEnv, "node_modules", ".bin", "gemini")

	// If it doesn't exist there, check if it's in the PATH (for global install)
	if _, err := os.Stat(geminiPath); os.IsNotExist(err) {
		logger.LogDebug("Gemini: CLI not found in %s, checking PATH...", geminiPath)
		p, err := exec.LookPath("gemini")
		if err != nil {
			logger.LogError("Gemini: CLI not found. Please run 'dialtone ai install' first.")
			return
		}
		geminiPath = p
	}

	prompt := strings.Join(args, " ")
	logger.LogInfo("Gemini: Sending prompt '%s' to CLI...", prompt)

	// Execute gemini CLI
	// gemini takes the prompt as arguments
	cmdArgs := append([]string{"chat"}, args...)
	cmd := exec.Command(geminiPath, cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin // Allow interactive chat if needed

	if err := cmd.Run(); err != nil {
		logger.LogError("Gemini: CLI execution failed: %v", err)
		return
	}
}
