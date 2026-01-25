package dialtone

import (
	"dialtone/cli/src/core/logger"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// RunClone handles the 'clone' command
func RunClone(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: dialtone clone <directory>")
		fmt.Println("\nOptional: dialtone clone <directory> <repo-url>")
		os.Exit(1)
	}

	targetDir := args[0]
	repoUrl := "https://github.com/timcash/dialtone.git" // Default repo

	if len(args) > 1 {
		repoUrl = args[1]
	}

	logger.LogInfo("Cloning repository %s into %s...", repoUrl, targetDir)

	// Check if directory already exists
	if _, err := os.Stat(targetDir); err == nil {
		logger.LogInfo("Directory %s already exists. Checking for updates...", targetDir)

		// If it's a git repo, pull
		if _, err := os.Stat(filepath.Join(targetDir, ".git")); err == nil {
			cmd := exec.Command("git", "pull")
			cmd.Dir = targetDir
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				logger.LogFatal("Failed to update repository: %v", err)
			}
			logger.LogInfo("Repository updated successfully.")
			return
		} else {
			logger.LogFatal("Directory %s exists but is not a git repository.", targetDir)
		}
	}

	// Clone
	cmd := exec.Command("git", "clone", repoUrl, targetDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logger.LogFatal("Failed to clone repository: %v", err)
	}

	logger.LogInfo("Repository cloned successfully into %s", targetDir)
}

func RunSyncCodeCopy(args []string) {
	// Local copy for testing/dev
	src := "."
	dst := args[0]
	if dst == "" {
		logger.LogFatal("Destination required for local sync")
	}

	logger.LogInfo("Copying code to %s...", dst)
	cmd := exec.Command("cp", "-r", src+"/.", dst)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logger.LogFatal("Failed to copy: %v", err)
	}
}
