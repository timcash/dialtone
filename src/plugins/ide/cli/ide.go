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
	fmt.Println("  setup-workflows    Softlink docs/workflows to .agent/workflows")
	fmt.Println("  help               Show this help message")
}

func runSetupWorkflows(args []string) {
	logger.LogInfo("Setting up workflows...")

	srcDir := filepath.Join("docs", "workflows")
	destDir := filepath.Join(".agent", "workflows")

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

		// Remove existing dest if it exists
		if _, err := os.Lstat(destPath); err == nil {
			if err := os.Remove(destPath); err != nil {
				logger.LogError("Failed to remove existing file %s: %v", destPath, err)
				continue
			}
		}

		logger.LogInfo("Linking %s -> %s", file.Name(), destPath)
		if err := os.Symlink(srcPath, destPath); err != nil {
			logger.LogError("Failed to create symlink for %s: %v", file.Name(), err)
		}
	}

	logger.LogInfo("Workflows setup complete.")
}
