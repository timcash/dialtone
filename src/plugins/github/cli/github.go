package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dialtone/cli/src/core/logger"
)

func findGH() string {
	depsDir := os.Getenv("DIALTONE_ENV")
	if depsDir == "" {
		// Fallback to home if not set, match config.go logic
		home, _ := os.UserHomeDir()
		depsDir = filepath.Join(home, ".dialtone_env")
	}
	
	ghPath := filepath.Join(depsDir, "gh", "bin", "gh")
	if _, err := os.Stat(ghPath); err == nil {
		return ghPath
	}
	
	// Fallback to system PATH
	if p, err := exec.LookPath("gh"); err == nil {
		return p
	}
	
	return "gh"
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
	fmt.Println("  pull-request       Create, update, merge, or close a pull request")
	fmt.Println("  check-deploy       Check Vercel deployment status for current branch")
	fmt.Println("  help               Show this help message")
}


// runMerge merges the current pull request
func runMerge(args []string) {
	gh := findGH()
	logger.LogInfo("Merging pull request...")

	// Default args: merge current PR, use merge commit, delete branch
	// We allow user args to override or append?
	// gh pr merge [number | url | branch] [flags]
	// If no arg provided, it uses current branch.
	
	cmdArgs := []string{"pr", "merge"}
	
	// If user provided args, pass them. If not, default to --merge --delete-branch
	if len(args) > 0 {
		cmdArgs = append(cmdArgs, args...)
	} else {
		cmdArgs = append(cmdArgs, "--merge", "--delete-branch")
	}

	cmd := exec.Command(gh, cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// gh pr merge might be interactive if not enough info/flags, but we inherit proper stdio so it should be fine.
	
	if err := cmd.Run(); err != nil {
		logger.LogFatal("Failed to merge PR: %v", err)
	}
	logger.LogInfo("Pull request merged successfully.")
}

func runClose(args []string) {
	gh := findGH()
	logger.LogInfo("Closing pull request...")
	
	cmdArgs := []string{"pr", "close"}
	
	// If user provided args, pass them. If not, default to --delete-branch (if user deletes branch local, gh pr close --delete-branch deletes remote?)
	// gh pr close [number | url | branch] [flags]
	// --delete-branch: Delete the local and remote branch after close.
	
	if len(args) > 0 {
		cmdArgs = append(cmdArgs, args...)
	} else {
		cmdArgs = append(cmdArgs, "--delete-branch")
	}
	
	cmd := exec.Command(gh, cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Run(); err != nil {
		logger.LogFatal("Failed to close PR: %v", err)
	}
	logger.LogInfo("Pull request closed successfully.")
}

// runPullRequest handles the pull-request command
// Migrated from src/dev.go
func runPullRequest(args []string) {
	gh := findGH()

	// Check for subcommands
	if len(args) > 0 {
		switch args[0] {
		case "merge":
			runMerge(args[1:])
			return
		case "close":
			runClose(args[1:])
			return
		case "help", "-h", "--help":
			printGithubUsage()
			return
		}
		
		// Also scan for help flag anywhere if not subcommand
		for _, arg := range args {
			if arg == "--help" || arg == "-h" {
				printGithubUsage()
				return
			}
		}
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
		logger.LogFatal("Failed to get current branch: %v", err)
	}
	branch := strings.TrimSpace(string(output))

	if branch == "main" || branch == "master" {
		logger.LogFatal("Cannot create PR from main/master branch. Create a feature branch first.")
	}

	// Check if PR already exists
	checkCmd := exec.Command(gh, "pr", "view", "--json", "number,title,url")
	prOutput, prErr := checkCmd.Output()
	prExists := prErr == nil

	if !prExists {
		// PR doesn't exist, create it
		logger.LogInfo("Creating new pull request for branch: %s", branch)

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

		cmd = exec.Command(gh, createArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			logger.LogFatal("Failed to create PR: %v", err)
		}
	} else {
		// PR exists
		logger.LogInfo("Pull request exists for branch: %s", branch)

		// If title or body provided, update the PR
		if title != "" || body != "" {
			logger.LogInfo("Updating pull request...")

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

			cmd = exec.Command(gh, editArgs...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				logger.LogFatal("Failed to update PR: %v", err)
			}
			logger.LogInfo("Pull request updated successfully")
		}

		// Mark as ready for review if --ready flag
		if ready {
			logger.LogInfo("Marking pull request as ready for review...")
			cmd = exec.Command(gh, "pr", "ready")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				logger.LogFatal("Failed to mark PR as ready: %v", err)
			}
			logger.LogInfo("Pull request is now ready for review")
		}

		// Show PR info
		fmt.Printf("%s\n", string(prOutput))

		// Open in browser if --view flag
		if view {
			logger.LogInfo("Opening in browser...")
			cmd = exec.Command(gh, "pr", "view", "--web")
			cmd.Run()
		}
	}
}

// runCheckDeploy checks the status of the Vercel deployment for the current branch
func runCheckDeploy(args []string) {
	logger.LogInfo("Checking Vercel deployment status...")

	// 1. Find Vercel CLI
	homeDir, _ := os.UserHomeDir()
	vercelPath := filepath.Join(homeDir, ".dialtone_env", "node", "bin", "vercel")
	if _, err := os.Stat(vercelPath); os.IsNotExist(err) {
		if p, err := exec.LookPath("vercel"); err == nil {
			vercelPath = p
		} else {
			logger.LogFatal("Vercel CLI not found. Run 'dialtone install' to install dependencies.")
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
	
	logger.LogInfo("Running: vercel list (in %s)", webDir)
	if err := cmd.Run(); err != nil {
		logger.LogFatal("Failed to check deployments: %v", err)
	}
}
