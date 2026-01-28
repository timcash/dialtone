package cli

import (
	"bufio"
	"context"
	"dialtone/cli/src/core/logger"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Run handles the 'ide' command
func Run(args []string) {
	if len(args) < 1 {
		printUsage()
		return
	}

	command := args[0]
	subArgs := args[1:]

	switch command {
	case "setup-workflows":
		runSetupWorkflows(subArgs)
	case "antigravity":
		runAntigravity(subArgs)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("Unknown ide command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: dialtone ide <command> [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  setup-workflows    Setup IDE agent files (soft links only)")
	fmt.Println("  antigravity        Commands for Antigravity IDE integration")
	fmt.Println("\nOptions:")
	fmt.Println("  --help             Show this help message")
}

const (
	colorReset = "\033[0m"
	colorGreen = "\033[32m"
	colorBlue  = "\033[34m"
)

func runAntigravity(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: dialtone ide antigravity logs [options]")
		fmt.Println("\nSubcommands:")
		fmt.Println("  logs    Tail Antigravity extension logs")
		fmt.Println("\nOptions:")
		fmt.Println("  --chat       Show only chat interaction logs")
		fmt.Println("  --commands   Show only terminal command logs")
		return
	}

	command := args[0]
	subArgs := args[1:]

	chat := false
	commands := false
	for _, arg := range subArgs {
		if arg == "--chat" || arg == "--clean" {
			chat = true
		}
		if arg == "--commands" {
			commands = true
		}
	}

	switch command {
	case "logs":
		runAntigravityLogs(chat, commands)
	default:
		fmt.Printf("Unknown antigravity command: %s\n", command)
		runAntigravity([]string{})
		os.Exit(1)
	}
}

func runAntigravityLogs(chat, commands bool) {
	// If no flags are provided, show both
	showAll := !chat && !commands
	if showAll {
		chat = true
		commands = true
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan bool)

	// Stream 1: Chat Logs (Proprietary .pb format)
	if chat {
		go func() {
			logger.LogInfo("Starting chat log stream...")
			StreamChatLogs(ctx, "", os.Stdout)
			done <- true
		}()
	}

	// Stream 2: System/Command Logs (Antigravity.log via tail)
	if commands {
		go func() {
			logPath := findRecentAntigravityLog()
			if logPath == "" {
				logger.LogFatal("Could not find Antigravity log file.")
			}

			logger.LogInfo("Tailing system log: %s", logPath)

			// If we are showing only commands, filter for them
			// If showing all, we still want to filter out the noise mentioned in docs
			// "Requesting planner" lines are noise if we have the real chat stream.

			cmd := exec.Command("tail", "-f", logPath)
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				logger.LogFatal("Failed to create stdout pipe: %v", err)
			}

			if err := cmd.Start(); err != nil {
				logger.LogFatal("Failed to start tail: %v", err)
			}

			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				line := scanner.Text()

				isChatNoise := strings.Contains(line, "Requesting planner with")
				isCmd := strings.Contains(line, "[Terminal]")

				// If we have chat stream enabled, suppress the noise
				if chat && isChatNoise {
					continue
				}

				if commands && !showAll && !isCmd {
					// User asked specifically for --commands, so only show commands
					continue
				}

				// Colorize
				if isCmd {
					fmt.Printf("%s[CMD ]%s %s\n", colorBlue, colorReset, line)
				} else if showAll {
					// Determine if we should show other lines?
					// For now, let's keep it clean as per "Detailed Log Filtering" docs
					// Only show if it matches high value patterns?
					// Or just print everything else as usual?
					// Let's print everything else but colored if known
					fmt.Println(line)
				}
			}

			if err := cmd.Wait(); err != nil {
				// Ignore signal exit
				if exitErr, ok := err.(*exec.ExitError); ok {
					if exitErr.ExitCode() == -1 || exitErr.ExitCode() == 130 {
						done <- true
						return
					}
				}
				logger.LogInfo("Log tail ended.")
			}
			done <- true
		}()
	}

	// Wait forever (or until one finishes)
	<-done
}

func findRecentAntigravityLog() string {
	homeDir, _ := os.UserHomeDir()
	logsRoot := filepath.Join(homeDir, "Library/Application Support/Antigravity/logs")

	// 1. Find the latest timestamped folder
	entries, err := os.ReadDir(logsRoot)
	if err != nil {
		return ""
	}

	var latestFolder string
	var latestTime int64
	for _, entry := range entries {
		if entry.IsDir() {
			info, _ := entry.Info()
			if info.ModTime().Unix() > latestTime {
				latestTime = info.ModTime().Unix()
				latestFolder = entry.Name()
			}
		}
	}

	if latestFolder == "" {
		return ""
	}

	// 2. Find the window folder with the latest modified google.antigravity/Antigravity.log
	windowPathPattern := filepath.Join(logsRoot, latestFolder, "window*", "exthost", "google.antigravity", "Antigravity.log")
	matches, _ := filepath.Glob(windowPathPattern)

	var bestLog string
	var bestTime int64
	for _, match := range matches {
		info, err := os.Stat(match)
		if err == nil {
			if info.ModTime().Unix() > bestTime {
				bestTime = info.ModTime().Unix()
				bestLog = match
			}
		}
	}

	return bestLog
}

func runSetupWorkflows(args []string) {
	logger.LogInfo("Setting up IDE agent files (soft links only)...")

	// Setup Workflows
	setupDir("docs/workflows", ".agent/workflows")

	// Setup Rules
	setupDir("docs/rules", ".agent/rules")

	logger.LogInfo("IDE setup complete.")
}

func setupDir(srcDir, destDir string) {
	logger.LogInfo("Processing %s -> %s", srcDir, destDir)

	// Ensure destination directory exists
	if _, err := os.Stat(destDir); os.IsNotExist(err) {
		if err := os.MkdirAll(destDir, 0755); err != nil {
			logger.LogFatal("Failed to create destination directory %s: %v", destDir, err)
		}
	}

	// Read files from source directory
	files, err := os.ReadDir(srcDir)
	if err != nil {
		logger.LogFatal("Failed to read source directory %s: %v", srcDir, err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		srcPath, _ := filepath.Abs(filepath.Join(srcDir, file.Name()))
		destPath, _ := filepath.Abs(filepath.Join(destDir, file.Name()))

		// Fail if destination exists
		if _, err := os.Lstat(destPath); err == nil {
			logger.LogFatal("File already exists: %s. Refusing to overwrite.", destPath)
		}

		logger.LogInfo("Linking %s", file.Name())
		if err := os.Symlink(srcPath, destPath); err != nil {
			logger.LogFatal("Failed to create symlink for %s: %v", file.Name(), err)
		}
	}
}
