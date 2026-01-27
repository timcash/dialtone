package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"dialtone/cli/src/core/logger"
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
	fmt.Println("  setup-workflows    Setup IDE agent files (default: copy)")
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
	logPath := findRecentAntigravityLog()
	if logPath == "" {
		logger.LogFatal("Could not find Antigravity log file.")
	}

	logger.LogInfo("Tailing Antigravity log: %s", logPath)
	
	filtering := chat || commands
	if filtering {
		logger.LogInfo("Filtering enabled: chat=%v, commands=%v", chat, commands)
	}

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
		
		isChat := strings.Contains(line, "Requesting planner with")
		isCmd := strings.Contains(line, "[Terminal]")

		// If filtering is on, only show the enabled types
		if filtering {
			if chat && isChat {
				fmt.Printf("%s[CHAT]%s %s\n", colorGreen, colorReset, line)
			} else if commands && isCmd {
				fmt.Printf("%s[CMD ]%s %s\n", colorBlue, colorReset, line)
			}
			continue
		}

		// Show all if no filtering, but still color known types
		if isChat {
			fmt.Printf("%s[CHAT]%s %s\n", colorGreen, colorReset, line)
		} else if isCmd {
			fmt.Printf("%s[CMD ]%s %s\n", colorBlue, colorReset, line)
		} else {
			fmt.Println(line)
		}
	}

	if err := cmd.Wait(); err != nil {
		logger.LogFatal("Tail process exited with error: %v", err)
	}
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
	useSymlink := false
	for _, arg := range args {
		if arg == "--symlink" {
			useSymlink = true
		}
	}

	mode := "copying"
	if useSymlink {
		mode = "symlinking"
	}
	logger.LogInfo("Setting up IDE agent files (mode: %s)...", mode)

	// Setup Workflows
	setupDir("docs/workflows", ".agent/workflows", useSymlink)
	
	// Setup Rules
	setupDir("docs/rules", ".agent/rules", useSymlink)

	logger.LogInfo("IDE setup complete.")
}

func setupDir(srcDir, destDir string, useSymlink bool) {
	logger.LogInfo("Processing %s -> %s", srcDir, destDir)

	// Ensure destination directory exists
	if err := os.MkdirAll(destDir, 0755); err != nil {
		logger.LogFatal("Failed to create destination directory %s: %v", destDir, err)
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

		// Remove existing to handle cases where it's a symlink or read-only
		if _, err := os.Lstat(destPath); err == nil {
			if err := os.Remove(destPath); err != nil {
				logger.LogError("Failed to remove existing file %s: %v", destPath, err)
				continue
			}
		}

		if useSymlink {
			logger.LogInfo("Linking %s", file.Name())
			if err := os.Symlink(srcPath, destPath); err != nil {
				logger.LogError("Failed to create symlink for %s: %v", file.Name(), err)
			}
		} else {
			logger.LogInfo("Copying %s", file.Name())
			content, err := os.ReadFile(srcPath)
			if err != nil {
				logger.LogError("Failed to read source file %s: %v", srcPath, err)
				continue
			}
			if err := os.WriteFile(destPath, content, 0644); err != nil {
				logger.LogError("Failed to write destination file %s: %v", destPath, err)
				continue
			}
		}
	}
}
