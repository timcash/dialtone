package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"dialtone/cli/src/core/config"
	"dialtone/cli/src/core/logger"
)

var labelFlags = map[string]string{
	"--p0":            "p0",
	"--p1":            "p1",
	"--bug":           "bug",
	"--ready":         "ready",
	"--ticket":        "ticket",
	"--enhancement":   "enhancement",
	"--docs":          "documentation",
	"--documentation": "documentation",
	"--perf":          "performance",
	"--performance":   "performance",
	"--security":      "security",
	"--refactor":      "refactor",
	"--test":          "test",
	"--duplicate":     "duplicate",
	"--wontfix":       "wontfix",
	"--question":      "question",
}

func findGH() string {
	depsDir := config.GetDialtoneEnv()

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
	case "issue":
		runIssue(restArgs)
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
	fmt.Println("  issue              List or sync GitHub issues to local tickets")
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
		cmdArgs = append(cmdArgs, "--merge")
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
		// Default: just close
		// cmdArgs = append(cmdArgs)
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
			ticketFile := filepath.Join("tickets", branch, "ticket.md")
			if _, statErr := os.Stat(ticketFile); statErr == nil {
				createArgs = append(createArgs, "--body-file", ticketFile)
			} else {
				createArgs = append(createArgs, "--body", fmt.Sprintf("Feature: %s", branch))
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

		// If title or body provided, OR if ticket exists (to sync), update the PR
		ticketFile := filepath.Join("tickets", branch, "ticket.md")
		hasTicket := false
		if _, err := os.Stat(ticketFile); err == nil {
			hasTicket = true
		}

		if title != "" || body != "" || (hasTicket && body == "") {
			logger.LogInfo("Updating pull request...")

			var editArgs []string
			editArgs = append(editArgs, "pr", "edit")

			if title != "" {
				editArgs = append(editArgs, "--title", title)
			}

			if body != "" {
				editArgs = append(editArgs, "--body", body)
			} else if hasTicket {
				// Read file content manually and pass as body to avoid 'gh' file access issues
				content, err := os.ReadFile(ticketFile)
				if err == nil {
					editArgs = append(editArgs, "--body", string(content))
				} else {
					logger.LogInfo("Failed to read ticket file: %v", err)
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

func runIssue(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: dialtone-dev github issue <command> [options]")
		fmt.Println("\nCommands:")
		fmt.Println("  list      List open issues")
		fmt.Println("  sync      (DEPRECATED) Sync open issues to local tickets")
		fmt.Println("  view <issue-id>    View issue details")
		fmt.Println("  edit <issue-id>... Edit an issue (add/remove labels)")
		fmt.Println("  comment <issue-id> <msg> Add a comment to an issue")
		fmt.Println("  close <issue-id>... Close specific issue(s)")
		fmt.Println("  close-all          Close all open issues")

		return
	}

	subcommand := args[0]
	restArgs := args[1:]

	// Check if subcommand is an issue number (e.g., dialtone github issue 104 --ready)
	if matched, _ := regexp.MatchString(`^[0-9]+$`, subcommand); matched {
		handleIssueDirect(subcommand, restArgs)
		return
	}

	switch subcommand {
	case "list":
		runIssueList(restArgs)
	case "sync":
		runIssueSync(restArgs)
	case "view":
		runIssueView(restArgs)
	case "edit":
		runIssueEdit(restArgs)
	case "comment":
		runIssueComment(restArgs)
	case "close":
		runIssueClose(restArgs)
	case "close-all":
		runIssueCloseAll(restArgs)
	default:
		fmt.Printf("Unknown issue command: %s (or missing issue-id)\n", subcommand)
		fmt.Println("Usage: dialtone-dev github issue <issue-id> [--ready]")

		os.Exit(1)
	}
}

type GHInfo struct {
	Number int      `json:"number"`
	Title  string   `json:"title"`
	Body   string   `json:"body"`
	Labels []GHLabel `json:"labels"`
}

type GHLabel struct {
	Name string `json:"name"`
}

func runIssueList(args []string) {
	gh := findGH()
	logger.LogInfo("Listing open issues...")

	useMarkdown := false
	var ghArgs []string
	for _, arg := range args {
		if arg == "--markdown" {
			useMarkdown = true
		} else {
			ghArgs = append(ghArgs, arg)
		}
	}

	// Always ask for labels
	cmdArgs := []string{"issue", "list", "--json", "number,title,labels"}
	cmdArgs = append(cmdArgs, ghArgs...)

	cmd := exec.Command(gh, cmdArgs...)
	if useMarkdown {
		output, err := cmd.Output()
		if err != nil {
			logger.LogFatal("Failed to list issues: %v", err)
		}

		var issues []GHInfo
		if err := json.Unmarshal(output, &issues); err != nil {
			logger.LogFatal("Failed to parse issues: %v", err)
		}

		fmt.Println("| # | Title | Labels |")
		fmt.Println("|---|-------|--------|")
		for _, issue := range issues {
			var labels []string
			for _, l := range issue.Labels {
				labels = append(labels, l.Name)
			}
			labelStr := strings.Join(labels, ", ")
			fmt.Printf("| %d | %s | %s |\n", issue.Number, issue.Title, labelStr)
		}
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			logger.LogFatal("Failed to list issues: %v", err)
		}
	}
}

func runIssueSync(args []string) {
	logger.LogWarn("The 'issue sync' command is DEPRECATED and may be removed in a future version. Please use manual ticket creation.")
	gh := findGH()
	logger.LogInfo("Syncing GitHub issues to local tickets...")

	// Get issues
	cmd := exec.Command(gh, "issue", "list", "--json", "number,title,body", "--limit", "100")
	output, err := cmd.Output()
	if err != nil {
		logger.LogFatal("Failed to fetch issues: %v", err)
	}

	var issues []GHInfo
	if err := json.Unmarshal(output, &issues); err != nil {
		logger.LogFatal("Failed to parse issues: %v", err)
	}

	templatePath := filepath.Join("tickets", "template-ticket", "ticket.md")
	templateBytes, err := os.ReadFile(templatePath)
	if err != nil {
		logger.LogFatal("Failed to read ticket template: %v", err)
	}
	template := string(templateBytes)

	for _, issue := range issues {
		slug := generateSlug(issue.Title)
		ticketDir := filepath.Join("tickets", slug)
		ticketFile := filepath.Join(ticketDir, "ticket.md")

		if _, err := os.Stat(ticketFile); err == nil {
			logger.LogInfo("Ticket already exists: %s", slug)
			continue
		}

		logger.LogInfo("Creating ticket for issue #%d: %s", issue.Number, issue.Title)

		if err := os.MkdirAll(ticketDir, 0755); err != nil {
			logger.LogFatal("Failed to create ticket directory: %v", err)
		}

		content := strings.ReplaceAll(template, "ticket-short-name", slug)
		content = strings.ReplaceAll(content, "[Ticket Title]", issue.Title)

		// Add issue body to Collaborative Notes or at the end
		if issue.Body != "" {
			bodySection := fmt.Sprintf("\n## Issue Summary\n%s\n", issue.Body)
			content = strings.ReplaceAll(content, "## Collaborative Notes", bodySection+"\n## Collaborative Notes")
		}

		if err := os.WriteFile(ticketFile, []byte(content), 0644); err != nil {
			logger.LogFatal("Failed to write ticket file: %v", err)
		}
	}
}

func runIssueView(args []string) {
	if len(args) < 1 {
		logger.LogFatal("Usage: github issue view <issue-id>")

	}
	handleIssueDirect(args[0], args[1:])
}

func handleIssueDirect(issueNum string, args []string) {
	gh := findGH()

	var labelsToAdd []string
	for _, arg := range args {
		if label, ok := labelFlags[arg]; ok {
			labelsToAdd = append(labelsToAdd, label)
		}
	}

	if len(labelsToAdd) > 0 {
		logger.LogInfo("Updating labels for issue #%s: %s", issueNum, strings.Join(labelsToAdd, ", "))
		cmdArgs := []string{"issue", "edit", issueNum}
		for _, l := range labelsToAdd {
			cmdArgs = append(cmdArgs, "--add-label", l)
		}
		cmd := exec.Command(gh, cmdArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			logger.LogFatal("Failed to update labels: %v", err)
		}
		return
	}

	// Default: view issue
	cmd := exec.Command(gh, "issue", "view", issueNum)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logger.LogFatal("Failed to view issue: %v", err)
	}
}

func runIssueEdit(args []string) {
	if len(args) < 1 {
		logger.LogFatal("Usage: dialtone-dev github issue edit <issue-id> [options]")

	}
	issueNum := args[0]
	gh := findGH()

	var editArgs []string
	editArgs = append(editArgs, "issue", "edit", issueNum)

	hasAction := false
	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--add-label":
			if i+1 < len(args) {
				editArgs = append(editArgs, "--add-label", args[i+1])
				i++
				hasAction = true
			}
		case "--remove-label":
			if i+1 < len(args) {
				editArgs = append(editArgs, "--remove-label", args[i+1])
				i++
				hasAction = true
			}
		default:
			if label, ok := labelFlags[arg]; ok {
				editArgs = append(editArgs, "--add-label", label)
				hasAction = true
			}
		}
	}

	if !hasAction {
		logger.LogFatal("No action provided for issue edit. Use --add-label, --remove-label, or a shortcut flag (--p0, --bug, etc.)")
	}

	cmd := exec.Command(gh, editArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logger.LogFatal("Failed to edit issue: %v", err)
	}
}

func runIssueComment(args []string) {
	if len(args) < 2 {
		logger.LogFatal("Usage: github issue comment <issue-id> <message>")

	}
	gh := findGH()
	cmd := exec.Command(gh, "issue", "comment", args[0], "--body", args[1])
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logger.LogFatal("Failed to add comment: %v", err)
	}
}

func runIssueClose(args []string) {
	if len(args) == 0 {
		logger.LogFatal("Usage: dialtone-dev github issue close <issue-id>...")

	}

	gh := findGH()
	for _, num := range args {
		logger.LogInfo("Closing issue #%s...", num)
		cmd := exec.Command(gh, "issue", "close", num)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			logger.LogError("Failed to close issue #%s: %v", num, err)
		}
	}
}

func runIssueCloseAll(args []string) {
	gh := findGH()
	logger.LogInfo("Fetching all open issues...")

	cmd := exec.Command(gh, "issue", "list", "--json", "number", "--limit", "100")
	output, err := cmd.Output()
	if err != nil {
		logger.LogFatal("Failed to fetch issues: %v", err)
	}

	var issues []GHInfo
	if err := json.Unmarshal(output, &issues); err != nil {
		logger.LogFatal("Failed to parse issues: %v", err)
	}

	if len(issues) == 0 {
		logger.LogInfo("No open issues found.")
		return
	}

	logger.LogInfo("Closing %d open issues...", len(issues))
	for _, issue := range issues {
		logger.LogInfo("Closing issue #%d...", issue.Number)
		closeCmd := exec.Command(gh, "issue", "close", fmt.Sprintf("%d", issue.Number))
		closeCmd.Stdout = os.Stdout
		closeCmd.Stderr = os.Stderr
		if err := closeCmd.Run(); err != nil {
			logger.LogError("Failed to close issue #%d: %v", issue.Number, err)
		}
	}
	logger.LogInfo("All open issues closed.")
}

func generateSlug(title string) string {
	slug := strings.ToLower(title)
	slug = strings.ReplaceAll(slug, " ", "-")

	// Remove special characters
	reg := regexp.MustCompile("[^a-z0-9-]+")
	slug = reg.ReplaceAllString(slug, "")

	// Remove double dashes
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}

	return strings.Trim(slug, "-")
}
