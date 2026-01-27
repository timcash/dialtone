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
	fmt.Println("  setup-workflows    Setup IDE agent files (default: copy)")
	fmt.Println("\nOptions:")
	fmt.Println("  --symlink          Use symlinks instead of copying")
	fmt.Println("  --copy             Use copying (default)")
	fmt.Println("  --help             Show this help message")
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
