package cli

import (
	"fmt"
	"os"
	"path/filepath"
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
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("Unknown ide command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: dialtone-dev ide <command> [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  setup-workflows    Copy docs/workflows and docs/rules to .agent/")
	fmt.Println("  help               Show this help message")
}

func runSetupWorkflows(args []string) {
	logger.LogInfo("Setting up IDE agent files...")

	// Copy Workflows
	copyDir("docs/workflows", ".agent/workflows")
	
	// Copy Rules
	copyDir("docs/rules", ".agent/rules")

	logger.LogInfo("IDE setup complete.")
}

func copyDir(srcDir, destDir string) {
	logger.LogInfo("Copying %s -> %s", srcDir, destDir)

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

		srcPath := filepath.Join(srcDir, file.Name())
		destPath := filepath.Join(destDir, file.Name())

		// Remove existing to handle cases where it's a symlink or read-only
		if _, err := os.Lstat(destPath); err == nil {
			if err := os.Remove(destPath); err != nil {
				logger.LogError("Failed to remove existing file %s: %v", destPath, err)
				continue
			}
		}

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
