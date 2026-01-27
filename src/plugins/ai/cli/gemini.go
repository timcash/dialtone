package cli

import (
	"bufio"
	"dialtone/cli/src/core/logger"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

// RunGemini handles the --gemini flag functionality by proxying to @google/gemini-cli
func RunGemini(args []string) {
	// If no args provided, we might want to default to interactive mode or show help.
	// But let's at least check if we have args. 
	// Actually, gemini CLI might handle empty args by showing help or interactive.
	// Let's pass it through.
	if len(args) == 0 {
		// Example: dialtone ai --gemini 
		// This should probably launch interactive mode if supported, or just show help from gemini.
		// For now, let's allow it and let gemini decide.
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

	if googleKey == "" {
		logger.LogError("Gemini: Authentication failed. No GOOGLE_API_KEY found.")
		logger.LogInfo("Please run 'dialtone ai auth' for instructions on how to set up your API key.")
		return
	}

	// @google/gemini-cli specifically looks for GEMINI_API_KEY
	os.Setenv("GEMINI_API_KEY", googleKey)

	// The gemini executable should be in node/bin/gemini inside dialtoneEnv if installed via local npm
	localGemini := filepath.Join(dialtoneEnv, "node", "bin", "gemini")
	geminiPath := filepath.Join(dialtoneEnv, "node_modules", ".bin", "gemini")

	if _, err := os.Stat(localGemini); err == nil {
		logger.LogDebug("Gemini: Using local binary at %s", localGemini)
		geminiPath = localGemini
	} else if _, err := os.Stat(geminiPath); os.IsNotExist(err) {
		logger.LogDebug("Gemini: CLI not found in %s or %s, checking PATH...", localGemini, geminiPath)
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
	// Create pipes to capture stdout/stderr
	prOut, pwOut := io.Pipe()
	prErr, pwErr := io.Pipe()

	// MultiWriter allows writing to both original output and our pipe
	cmd.Stdout = io.MultiWriter(os.Stdout, pwOut)
	cmd.Stderr = io.MultiWriter(os.Stderr, pwErr)
	cmd.Stdin = os.Stdin // Allow interactive chat if needed

	// Start goroutines to scan and log output
	go func() {
		scanner := bufio.NewScanner(prOut)
		for scanner.Scan() {
			logger.LogInfo("[Gemini] %s", scanner.Text())
		}
	}()

	go func() {
		scanner := bufio.NewScanner(prErr)
		for scanner.Scan() {
			logger.LogError("[Gemini] %s", scanner.Text())
		}
	}()

	// Ensure pipes are closed after command finishes
	defer pwOut.Close()
	defer pwErr.Close()

	if err := cmd.Run(); err != nil {
		logger.LogError("Gemini: CLI execution failed: %v", err)
		return
	}
}
