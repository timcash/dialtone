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
	rawMode := false
	var filteredArgs []string
	for _, arg := range args {
		if arg == "--raw" {
			rawMode = true
		} else {
			filteredArgs = append(filteredArgs, arg)
		}
	}
	args = filteredArgs

	if len(args) == 0 {
		if rawMode {
			logger.LogRaw("Gemini: Please provide a prompt.")
		} else {
			logger.LogInfo("Gemini: Please provide a prompt. Usage: dialtone ai gemini \"prompt\"")
		}
		return
	}

	// Load .env to get DIALTONE_ENV and GOOGLE_API_KEY
	if err := godotenv.Load(); err != nil {
		if !rawMode {
			logger.LogDebug("Gemini: No .env file found")
		}
	}

	dialtoneEnv := os.Getenv("DIALTONE_ENV")
	if dialtoneEnv == "" {
		logger.LogFatal("DIALTONE_ENV is not set. Please add it to your .env file.")
	}

	// Check for API Key
	googleKey := os.Getenv("GOOGLE_API_KEY")

	if googleKey == "" {
		if rawMode {
			logger.LogRaw("Gemini: Authentication failed. No GOOGLE_API_KEY found.")
		} else {
			logger.LogError("Gemini: Authentication failed. No GOOGLE_API_KEY found.")
			logger.LogInfo("Please run 'dialtone ai auth' for instructions on how to set up your API key.")
		}
		return
	}

	// @google/gemini-cli specifically looks for GEMINI_API_KEY
	os.Setenv("GEMINI_API_KEY", googleKey)

	// The gemini executable should be in node/bin/gemini inside dialtoneEnv if installed via local npm
	localGemini := filepath.Join(dialtoneEnv, "node", "bin", "gemini")
	geminiPath := filepath.Join(dialtoneEnv, "node_modules", ".bin", "gemini")

	if _, err := os.Stat(localGemini); err == nil {
		if !rawMode {
			logger.LogDebug("Gemini: Using local binary at %s", localGemini)
		}
		geminiPath = localGemini
	} else if _, err := os.Stat(geminiPath); os.IsNotExist(err) {
		if !rawMode {
			logger.LogDebug("Gemini: CLI not found in %s or %s, checking PATH...", localGemini, geminiPath)
		}
		p, err := exec.LookPath("gemini")
		if err != nil {
			if rawMode {
				logger.LogRaw("Gemini: CLI not found. Please run 'dialtone ai install' first.")
			} else {
				logger.LogError("Gemini: CLI not found. Please run 'dialtone ai install' first.")
			}
			return
		}
		geminiPath = p
	}

	prompt := strings.Join(args, " ")
	if !rawMode {
		logger.LogInfo("Gemini: Sending prompt '%s' to CLI...", prompt)
	}

	// Execute gemini CLI
	// gemini takes the prompt as arguments
	cmdArgs := append([]string{"chat"}, args...)
	cmd := exec.Command(geminiPath, cmdArgs...)
	// Create pipes to capture stdout/stderr
	prOut, pwOut := io.Pipe()
	prErr, pwErr := io.Pipe()

	// If raw mode, don't use MultiWriter with os.Stdout/Stderr to avoid duplicates
	// since LogRaw will print to terminal.
	if rawMode {
		cmd.Stdout = pwOut
		cmd.Stderr = pwErr
	} else {
		cmd.Stdout = io.MultiWriter(os.Stdout, pwOut)
		cmd.Stderr = io.MultiWriter(os.Stderr, pwErr)
	}
	cmd.Stdin = os.Stdin // Allow interactive chat if needed

	// Start goroutines to scan and log output
	go func() {
		scanner := bufio.NewScanner(prOut)
		for scanner.Scan() {
			if rawMode {
				logger.LogRaw("%s", scanner.Text())
			} else {
				logger.LogInfo("[Gemini] %s", scanner.Text())
			}
		}
	}()

	go func() {
		scanner := bufio.NewScanner(prErr)
		for scanner.Scan() {
			text := scanner.Text()
			if rawMode {
				logger.LogRaw("%s", text)
			} else if strings.Contains(text, "[DEBUG]") {
				logger.LogDebug("[Gemini] %s", text)
			} else {
				logger.LogError("[Gemini] %s", text)
			}
		}
	}()

	// Ensure pipes are closed after command finishes
	defer pwOut.Close()
	defer pwErr.Close()

	if err := cmd.Run(); err != nil {
		if !rawMode {
			logger.LogError("Gemini: CLI execution failed: %v", err)
		}
		return
	}
}
