package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)



func logInfo(format string, args ...interface{}) {
	fmt.Printf("[github] "+format+"\n", args...)
}

func logFatal(format string, args ...interface{}) {
	fmt.Printf("[github] FATAL: "+format+"\n", args...)
	os.Exit(1)
}

// RunGithub handles 'github <subcommand>'
func RunGithub(args []string) {
	if len(args) == 0 {
		printGithubUsage()
		return
	}

	subcommand := args[0]
	restArgs := args[1:]

	switch subcommand {
	case "pull-request", "pr":
		runPullRequest(restArgs)
	case "check-deploy":
		runCheckDeploy(restArgs)
	case "help", "-h", "--help":
		printGithubUsage()
	default:
		fmt.Printf("Unknown github command: %s\n", subcommand)
		printGithubUsage()
		os.Exit(1)
	}
}

func printGithubUsage() {
	fmt.Println("Usage: dialtone-dev github <command> [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  pull-request       Create or update a pull request (wrapper around gh CLI)")
	fmt.Println("  check-deploy       Check Vercel deployment status for current branch")
	fmt.Println("  help               Show this help message")
}

// runPullRequest handles the pull-request command
// Migrated from src/dev.go
func runPullRequest(args []string) {
	// Check if gh CLI is available
	if _, err := exec.LookPath("gh"); err != nil {
		logFatal("GitHub CLI (gh) not found. Install it from: https://cli.github.com/")
	}

	// Parse flags and capture positional arguments
	var title, body string
	var draft, ready, view bool
	var positional []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--title", "-t":
			if i+1 < len(args) {
				title = args[i+1]
				i++
			}
		case "--body", "-b":
			if i+1 < len(args) {
				body = args[i+1]
				i++
			}
		case "--draft", "-d":
			draft = true
		case "--ready", "-r":
			ready = true
		case "--view", "-v":
			view = true
		default:
			// Capture positional arguments (not starting with -)
			if !strings.HasPrefix(args[i], "-") {
				positional = append(positional, args[i])
			}
		}
	}

	// Example: dialtone-dev pull-request linux-wsl-camera-support "Added V4L2 support"
	if len(positional) >= 1 && title == "" {
		// Use first positional as title (could be branch name)
		title = positional[0]
	}
	if len(positional) >= 2 && body == "" {
		// Use second positional as body
		body = positional[1]
	}

	// Get current branch name
	cmd := exec.Command("git", "branch", "--show-current")
	output, err := cmd.Output()
	if err != nil {
		logFatal("Failed to get current branch: %v", err)
	}
	branch := strings.TrimSpace(string(output))

	if branch == "main" || branch == "master" {
		logFatal("Cannot create PR from main/master branch. Create a feature branch first.")
	}

	// Check if PR already exists
	checkCmd := exec.Command("gh", "pr", "view", "--json", "number,title,url")
	prOutput, prErr := checkCmd.Output()
	prExists := prErr == nil

	if !prExists {
		// PR doesn't exist, create it
		logInfo("Creating new pull request for branch: %s", branch)

		var createArgs []string
		createArgs = append(createArgs, "pr", "create")

		// Use provided title or default to branch name
		if title != "" {
			createArgs = append(createArgs, "--title", title)
		} else {
			createArgs = append(createArgs, "--title", branch)
		}

		// Use provided body, or plan file, or default message
		if body != "" {
			createArgs = append(createArgs, "--body", body)
		} else {
			planFile := filepath.Join("plan", fmt.Sprintf("plan-%s.md", branch))
			if _, statErr := os.Stat(planFile); statErr == nil {
				createArgs = append(createArgs, "--body-file", planFile)
			} else {
				createArgs = append(createArgs, "--body", fmt.Sprintf("Feature: %s\n\nSee plan file for details.", branch))
			}
		}

		// Add draft flag if specified
		if draft {
			createArgs = append(createArgs, "--draft")
		}

		cmd = exec.Command("gh", createArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			logFatal("Failed to create PR: %v", err)
		}
	} else {
		// PR exists
		logInfo("Pull request exists for branch: %s", branch)

		// If title or body provided, update the PR
		if title != "" || body != "" {
			logInfo("Updating pull request...")

			var editArgs []string
			editArgs = append(editArgs, "pr", "edit")

			if title != "" {
				editArgs = append(editArgs, "--title", title)
			}

			if body != "" {
				editArgs = append(editArgs, "--body", body)
			} else {
				planFile := filepath.Join("plan", fmt.Sprintf("plan-%s.md", branch))
				if _, statErr := os.Stat(planFile); statErr == nil {
					editArgs = append(editArgs, "--body-file", planFile)
				}
			}

			cmd = exec.Command("gh", editArgs...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				logFatal("Failed to update PR: %v", err)
			}
			logInfo("Pull request updated successfully")
		}

		// Mark as ready for review if --ready flag
		if ready {
			logInfo("Marking pull request as ready for review...")
			cmd = exec.Command("gh", "pr", "ready")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				logFatal("Failed to mark PR as ready: %v", err)
			}
			logInfo("Pull request is now ready for review")
		}

		// Show PR info
		fmt.Printf("%s\n", string(prOutput))

		// Open in browser if --view flag
		if view {
			logInfo("Opening in browser...")
			cmd = exec.Command("gh", "pr", "view", "--web")
			cmd.Run()
		}
	}
}

// runCheckDeploy checks the status of the Vercel deployment for the current branch
func runCheckDeploy(args []string) {
	logInfo("Checking Vercel deployment status...")

	// 1. Find Vercel CLI
	homeDir, _ := os.UserHomeDir()
	vercelPath := filepath.Join(homeDir, ".dialtone_env", "node", "bin", "vercel")
	if _, err := os.Stat(vercelPath); os.IsNotExist(err) {
		if p, err := exec.LookPath("vercel"); err == nil {
			vercelPath = p
		} else {
			logFatal("Vercel CLI not found. Run 'dialtone install' to install dependencies.")
		}
	}

	// 2. Determine web dir
	webDir := filepath.Join("src", "plugins", "www", "app")

	// 3. Run vercel list
	// We pass args to allow filtering if the user wants, e.g. dialtone-dev github check-deploy <project>
	vArgs := append([]string{"list"}, args...)
	
	cmd := exec.Command(vercelPath, vArgs...)
	cmd.Dir = webDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	logInfo("Running: vercel list (in %s)", webDir)
	if err := cmd.Run(); err != nil {
		logFatal("Failed to check deployments: %v", err)
	}
}
