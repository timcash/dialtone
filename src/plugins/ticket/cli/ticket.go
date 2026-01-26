package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	github_cli "dialtone/cli/src/plugins/github/cli"
)

func logInfo(format string, args ...interface{}) {
	fmt.Printf("[ticket] "+format+"\n", args...)
}

func logFatal(format string, args ...interface{}) {
	fmt.Printf("[ticket] FATAL: "+format+"\n", args...)
	os.Exit(1)
}

// Run handles all ticket subcommands
func Run(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: dialtone-dev ticket <subcommand> [options]")
		fmt.Println("\nSubcommands:")
		fmt.Println("  add <name>         Add a new local ticket (scaffold only)")
		fmt.Println("  start <name>       Start a new ticket (branch + scaffold + PR)")
		fmt.Println("  done <name>        Verify and complete ticket (commit + merge)")
		fmt.Println("  validate <name>    Validate ticket.md format")
		fmt.Println("  list               List local tickets and GitHub issues")
		fmt.Println("  subtask <subcmd>   Manage subtasks in current ticket")
		fmt.Println("  create             Create a new GitHub issue")
		fmt.Println("  view <id>          View GitHub issue details")
		fmt.Println("  comment <id> <msg> Add a comment to a GitHub issue")
		fmt.Println("  close <id>         Close a GitHub issue")
		return
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "add":
		RunAdd(subArgs)
	case "start":
		RunStart(subArgs)
	case "done":
		RunDone(subArgs)
	case "validate":
		RunValidate(subArgs)
	case "subtask":
		RunSubtask(subArgs)
	case "list":
		RunList(subArgs)
	case "create", "comment", "view", "close":
		runTicketGhCommand(subcommand, subArgs)
	default:
		fmt.Printf("Unknown ticket subcommand: %s\n", subcommand)
		Run([]string{})
	}
}

// RunList lists local tickets in the tickets/ directory
func RunList(args []string) {
	files, err := os.ReadDir("tickets")
	if err != nil {
		logFatal("Failed to read tickets directory: %v", err)
	}

	fmt.Println("\nLocal Tickets:")
	fmt.Println("---------------------------------------------------")
	count := 0
	for _, f := range files {
		if f.IsDir() {
			// Check if it has a ticket.md
			ticketMd := filepath.Join("tickets", f.Name(), "ticket.md")
			if _, err := os.Stat(ticketMd); err == nil {
				fmt.Printf("- %s\n", f.Name())
				count++
			}
		}
	}
	if count == 0 {
		fmt.Println("(No local tickets found)")
	}
	fmt.Println("---------------------------------------------------")

	fmt.Println("\nRemote GitHub Issues:")
	fmt.Println("---------------------------------------------------")
	github_cli.RunGithub([]string{"issue", "list"})
}

func runTicketGhCommand(subcommand string, subArgs []string) {
	// Re-route to github plugin's issue runner
	github_cli.RunGithub(append([]string{"issue", subcommand}, subArgs...))
}

// RunAdd handles 'ticket add <ticket-name>'
func RunAdd(args []string) {
	if len(args) < 1 {
		ticketName := GetCurrentBranch()
		if ticketName == "" {
			logFatal("Usage: ticket add <ticket-name> (or run from a feature branch)")
		}
		args = []string{ticketName}
	}
	ticketName := args[0]
	ScaffoldTicket(ticketName)
	logInfo("Ticket %s added successfully", ticketName)
}

// RunStart handles 'ticket start <ticket-name>'
func RunStart(args []string) {
	if len(args) < 1 {
		logFatal("Usage: ticket start <ticket-name> [--plugin <plugin-name>]")
	}

	arg := args[0]

	ticketName := GetTicketName(arg)

	// 1. Create/Switch Git Branch
	// Check if branch exists
	cmd := exec.Command("git", "branch", "--list", ticketName)
	output, err := cmd.Output()
	if err != nil {
		logFatal("Failed to check git branches: %v", err)
	}

	if strings.TrimSpace(string(output)) != "" {
		logInfo("Switching to existing branch: %s", ticketName)
		if err := exec.Command("git", "checkout", ticketName).Run(); err != nil {
			logFatal("Failed to checkout branch: %v", err)
		}
	} else {
		logInfo("Creating new branch: %s", ticketName)
		if err := exec.Command("git", "checkout", "-b", ticketName).Run(); err != nil {
			logFatal("Failed to create branch: %v", err)
		}
	}

	// 2. Ticket Scaffolding
	ScaffoldTicket(ticketName)

	// 2.5 Audit & Commit
	logInfo("Committing scaffolding...")
	if err := exec.Command("git", "add", ".").Run(); err != nil {
		logFatal("Failed to git add: %v", err)
	}
	commitMsg := fmt.Sprintf("chore: start ticket %s", ticketName)
	if err := exec.Command("git", "commit", "-m", commitMsg).Run(); err != nil {
		logFatal("Failed to commit: %v", err)
	}

	// 3. Git Push & PR
	logInfo("Pushing branch to origin...")
	pushCmd := exec.Command("git", "push", "-u", "origin", ticketName)
	pushCmd.Stdout = os.Stdout
	pushCmd.Stderr = os.Stderr
	if err := pushCmd.Run(); err != nil {
		logFatal("Failed to push branch: %v", err)
	}

	// Wait for GitHub to sync (exponential backoff)
	logInfo("Verifying branch on remote...")
	backoff := 100 * time.Millisecond
	maxRetries := 10
	for i := 0; i < maxRetries; i++ {
		// Check if branch exists on remote
		checkCmd := exec.Command("git", "ls-remote", "--exit-code", "--heads", "origin", ticketName)
		if err := checkCmd.Run(); err == nil {
			logInfo("Branch verified on remote.")
			break
		}

		if i == maxRetries-1 {
			logFatal("Timed out waiting for branch to appear on remote.")
		}

		time.Sleep(backoff)
		backoff *= 2
	}

	logInfo("Creating Pull Request...")
	prCmd := exec.Command("./dialtone.sh", "github", "pr")
	prCmd.Stdout = os.Stdout
	prCmd.Stderr = os.Stderr
	if err := prCmd.Run(); err != nil {
		logFatal("Failed to create PR: %v", err)
	}

	logInfo("Ticket %s started successfully", ticketName)
	logReminder(ticketName)
}

func ScaffoldTicket(ticketName string) {
	ticketDir := filepath.Join("tickets", ticketName)
	ensureDir(ticketDir)
	ensureDir(filepath.Join(ticketDir, "code"))
	ensureDir(filepath.Join(ticketDir, "test"))

	// Create test templates
	createTestTemplates(filepath.Join(ticketDir, "test"), ticketName)

	// Copy ticket.md template
	ticketMd := filepath.Join(ticketDir, "ticket.md")
	if _, err := os.Stat(ticketMd); os.IsNotExist(err) {
		templatePath := filepath.Join("docs", "ticket-template.md")
		content, err := os.ReadFile(templatePath)
		if err == nil {
			// Replace placeholders
			newContent := strings.ReplaceAll(string(content), "<branch-name>", ticketName)
			newContent = strings.ReplaceAll(newContent, "<ticket-name>", ticketName)

			if err := os.WriteFile(ticketMd, []byte(newContent), 0644); err != nil {
				logFatal("Failed to write ticket.md: %v", err)
			}
			logInfo("Created %s from %s", ticketMd, templatePath)
		} else {
			logInfo("Warning: Template %s not found, skipping ticket.md creation.", templatePath)
		}
	}

	progressTxt := filepath.Join(ticketDir, "progress.txt")
	if _, err := os.Stat(progressTxt); os.IsNotExist(err) {
		content := fmt.Sprintf("Progress log for %s\n\n", ticketName)
		if err := os.WriteFile(progressTxt, []byte(content), 0644); err != nil {
			logFatal("Failed to create progress.txt: %v", err)
		}
		logInfo("Created %s", progressTxt)
	}
}

// RunDone handles 'ticket done <ticket-name>'
// RunDone handles 'ticket done <ticket-name>'
func RunDone(args []string) {
	if len(args) < 1 {
		ticketName := GetCurrentBranch()
		if ticketName == "" {
			logFatal("Usage: ticket done <ticket-name> (or run from a feature branch)")
		}
		args = []string{ticketName}
	}
	ticketName := args[0]

	// 1. Verify all subtasks are done (except 'ticket-done')
	subtasks, err := parseSubtasks(ticketName)
	if err != nil {
		logFatal("Failed to parse subtasks: %v", err)
	}
	for _, st := range subtasks {
		if st.Name != "ticket-done" && st.Status != "done" {
			logFatal("Subtask '%s' is not done (status: %s). All subtasks must be done before completing the ticket.", st.Name, st.Status)
		}
	}
	logInfo("All subtasks verified as done (excluding 'ticket-done').")

	// 3. Verify git status
	cmd := exec.Command("git", "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		logFatal("Failed to run git status: %v", err)
	}
	if len(strings.TrimSpace(string(output))) > 0 {
		logFatal("Uncommitted changes detected. Please commit or stash them before running ticket done.\n%s", string(output))
	}
	logInfo("Git status clean.")

	// 4. Push local changes
	logInfo("Pushing latest changes to origin...")
	pushCmd := exec.Command("git", "push")
	pushCmd.Stdout = os.Stdout
	pushCmd.Stderr = os.Stderr
	if err := pushCmd.Run(); err != nil {
		logFatal("Failed to push changes: %v", err)
	}

	// 5. GitHub PR
	logInfo("Updating Pull Request and setting to ready...")
	prCmd := exec.Command("./dialtone.sh", "github", "pr", "--ready")
	prCmd.Stdout = os.Stdout
	prCmd.Stderr = os.Stderr
	if err := prCmd.Run(); err != nil {
		logFatal("Failed to update PR: %v", err)
	}

	// 5. Mark 'ticket-done' as done
	// We'll proceed even if it doesn't exist, but if it does, it will now be marked done.
	// Since parseSubtasks worked, we know if it exists.
	for _, st := range subtasks {
		if st.Name == "ticket-done" {
			RunSubtaskDone(ticketName, "ticket-done")
			logInfo("Marked 'ticket-done' subtask as done.")
			break
		}
	}

	logInfo("Ticket %s done setup complete.", ticketName)
	logReminder(ticketName)
}

func logReminder(ticketName string) {
	fmt.Printf("\nREMINDER: remember to update tickets/%s/progress.txt with important notes\n", ticketName)
}

func ensureDir(path string) {
	if err := os.MkdirAll(path, 0755); err != nil {
		logFatal("Failed to create directory %s: %v", path, err)
	}
}

// GetTicketName parses the ticket name from an argument (name or file path)
func GetTicketName(arg string) string {
	if strings.HasSuffix(arg, ".md") {
		if _, err := os.Stat(arg); err == nil {
			// It's a file, try to parse Branch
			content, err := os.ReadFile(arg)
			if err != nil {
				// Should not happen if Stat succeeded, but log just in case
				logInfo("Failed to read ticket file: %v", err)
				return ""
			}
			lines := strings.Split(string(content), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "# Branch:") {
					name := strings.TrimSpace(strings.TrimPrefix(line, "# Branch:"))
					logInfo("Parsed ticket name from file: %s", name)
					return name
				}
			}

			logInfo("No '# Branch:' found in %s, using filename as ticket name", arg)
			base := filepath.Base(arg)
			return strings.TrimSuffix(base, filepath.Ext(base))
		} else {
			// Ends in .md but doesn't exist
			logInfo("Ticket name '%s' ends in .md but file not found; treating as ticket name.", arg)
			return arg
		}
	}
	return arg
}

// GetCurrentBranch returns the name of the current git branch, or empty if on main/master or error
func GetCurrentBranch() string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	name := strings.TrimSpace(string(output))
	if name == "main" || name == "master" || name == "HEAD" || name == "" {
		return ""
	}
	return name
}

func createTestTemplates(testDir, ticketName string) {
	fullPath := filepath.Join(testDir, "test.go")
	if _, err := os.Stat(fullPath); !os.IsNotExist(err) {
		return
	}

	content := fmt.Sprintf(`package test

import (
	"fmt"
	"dialtone/cli/src/core/test"
	"dialtone/cli/src/core/logger"
)

func init() {
	// Register subtask tests here: test.Register("<subtask-name>", "<ticket-name>", []string{"<tag1>"}, Run<SubtaskName>)
	test.Register("example-subtask", "%s", []string{"example"}, RunExample)
}

// RunAll is the standard entry point required by project rules.
// It uses the registry to find and run all tests for this ticket.
func RunAll() error {
	logger.LogInfo("Running %s suite...")
	return test.RunTicket("%s")
}

func RunExample() error {
	fmt.Println("PASS: [example] Subtask logic verified")
	return nil
}
`, ticketName, ticketName, ticketName)

	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		logFatal("Failed to create test file %s: %v", fullPath, err)
	}
	logInfo("Created test template: %s", fullPath)
}
