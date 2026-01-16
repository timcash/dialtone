package dialtone

import (
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

	LogInfo("Cloning repository %s into %s...", repoUrl, targetDir)

	// Check if directory already exists
	if _, err := os.Stat(targetDir); err == nil {
		LogInfo("Directory %s already exists. Checking for updates...", targetDir)

		// If it's a git repo, pull
		if _, err := os.Stat(filepath.Join(targetDir, ".git")); err == nil {
			cmd := exec.Command("git", "pull")
			cmd.Dir = targetDir
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				LogFatal("Failed to update repository: %v", err)
			}
			LogInfo("Repository updated successfully.")
			return
		} else {
			LogFatal("Directory %s exists but is not a git repository.", targetDir)
		}
	}

	// Clone
	cmd := exec.Command("git", "clone", repoUrl, targetDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		LogFatal("Failed to clone repository: %v", err)
	}

	LogInfo("Repository cloned successfully into %s", targetDir)
}
