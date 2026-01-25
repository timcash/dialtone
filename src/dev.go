package dialtone

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"dialtone/cli/src/core/ssh"
	build_cli "dialtone/cli/src/plugins/build/cli"
	camera_cli "dialtone/cli/src/plugins/camera/cli"
	chrome_cli "dialtone/cli/src/plugins/chrome/cli"
	deploy_cli "dialtone/cli/src/plugins/deploy/cli"
	github_cli "dialtone/cli/src/plugins/github/cli"
	install_cli "dialtone/cli/src/plugins/install/cli"
	logs_cli "dialtone/cli/src/plugins/logs/cli"
	mavlink_cli "dialtone/cli/src/plugins/mavlink/cli"
	plugin_cli "dialtone/cli/src/plugins/plugin/cli"
	test_cli "dialtone/cli/src/plugins/test/cli"
	ticket_cli "dialtone/cli/src/plugins/ticket/cli"
	ui_cli "dialtone/cli/src/plugins/ui/cli"
	www_cli "dialtone/cli/src/plugins/www/cli"
	ai_cli "dialtone/cli/src/plugins/ai/cli"

)

// ExecuteDev is the entry point for the dialtone-dev CLI
func ExecuteDev() {
	if len(os.Args) < 2 {
		printDevUsage()
		return
	}

	// Load configuration
	LoadConfig()

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "build":
		build_cli.RunBuild(args)
	case "deploy":
		deploy_cli.RunDeploy(args)
	case "ssh":
		ssh.RunSSH(args)
	case "provision":
		RunProvision(args)
	case "logs":
		logs_cli.RunLogs(args)
	case "diagnostic":
		RunDiagnostic(args)
	case "install":
		install_cli.RunInstall(args)
	case "clone":
		RunClone(args)
	case "sync-code":
		RunSyncCode(args)
	case "plan":
		runPlan(args)
	case "branch":
		runBranch(args)
	case "test":
		test_cli.RunTest(args)
	case "pull-request", "pr":
		// Delegate to github plugin
		github_cli.RunGithub(append([]string{"pull-request"}, args...))
	case "github":
		github_cli.RunGithub(args)
	case "ticket":
		runTicket(args)
	case "plugin":
		plugin_cli.RunPlugin(args)
	case "camera":
		camera_cli.RunCamera(args)
	case "chrome":
		chrome_cli.RunChrome(args)
	case "mavlink":
		mavlink_cli.RunMavlink(args)
	case "www":
		www_cli.RunWww(args)
	case "ui":
		ui_cli.Run(args)

	case "ai":
		ai_cli.RunAI(args)
	case "opencode":
		ai_cli.RunAI(append([]string{"opencode"}, args...))
	case "developer":
		ai_cli.RunAI(append([]string{"developer"}, args...))
	case "subagent":
		ai_cli.RunAI(append([]string{"subagent"}, args...))
	case "docs":
		runDocs(args)
	case "help", "-h", "--help":
		printDevUsage()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printDevUsage()
		os.Exit(1)
	}
}

func printDevUsage() {
	fmt.Println("Usage: dialtone-dev <command> [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  install [path] Install dependencies (--linux-wsl for WSL, --macos-arm for Apple Silicon)")
	fmt.Println("  build         Build web UI and binary (--local, --full, --remote, --podman, --linux-arm, --linux-arm64)")
	fmt.Println("  deploy        Deploy to remote robot")
	fmt.Println("  camera        Camera tools (snapshot, stream)")
	fmt.Println("  clone         Clone or update the repository")
	fmt.Println("  sync-code     Sync source code to remote robot")
	fmt.Println("  ssh           SSH tools (upload, download, cmd)")
	fmt.Println("  provision     Generate Tailscale Auth Key")
	fmt.Println("  logs          Tail remote logs")
	fmt.Println("  diagnostic    Run system diagnostics (local or remote)")
	fmt.Println("  plan [name]        List plans or create/view a plan file")
	fmt.Println("  branch <name>      Create or checkout a feature branch")
	fmt.Println("  ticket <subcmd>    Manage GitHub tickets (start, done, subtask, etc.)")
	fmt.Println("  plugin <subcmd>    Manage plugins (add, install, build)")
	fmt.Println("  github <subcmd>    Manage GitHub interactions (pr, check-deploy)")
	fmt.Println("  www <subcmd>       Manage public webpage (Vercel wrapper)")
	fmt.Println("  ui <subcmd>        Manage web UI (dev, build, install)")
	fmt.Println("  test <subcmd>      Run tests (ticket, plugin, tags)")

	fmt.Println("  ai <subcmd>        AI tools (opencode, developer, subagent)")
	fmt.Println("  docs               Update documentation")
	fmt.Println("  help               Show this help message")
}

// runDocs handles the docs command
func runDocs(args []string) {
	LogInfo("Updating documentation...")

	// 1. Capture dialtone-dev help output
	// We need to re-run the current binary with "help" argument
	// However, we are running as "go run ...", so os.Args[0] is a temporary binary.
	// That's fine for capturing output.
	cmd := exec.Command(os.Args[0], "help")
	output, err := cmd.Output()
	if err != nil {
		LogFatal("Failed to run help command: %v", err)
	}

	helpOutput := string(output)

	// 2. Parse help output to extract commands
	lines := strings.Split(helpOutput, "\n")
	var commands []string
	capture := false
	for _, line := range lines {
		if strings.Contains(line, "Commands:") {
			capture = true
			continue
		}
		if capture && strings.TrimSpace(line) != "" {
			commands = append(commands, strings.TrimSpace(line))
		}
	}

	// 3. Format as markdown list
	var markdownLines []string
	markdownLines = append(markdownLines, "### Development CLI (`dialtone.sh`)")
	markdownLines = append(markdownLines, "")

	for i, cmdLine := range commands {
		parts := strings.Fields(cmdLine)
		if len(parts) >= 2 {
			cmdName := parts[0]
			desc := strings.Join(parts[1:], " ")
			markdownLines = append(markdownLines, fmt.Sprintf("%d. `./dialtone.sh %s` — %s", i+1, cmdName, desc))

			// Add examples based on command name
			example := ""
			switch cmdName {
			case "install":
				example = "./dialtone.sh install --linux-wsl"
			case "build":
				example = "./dialtone.sh build --local"
			case "deploy":
				example = "./dialtone.sh deploy"
			case "clone":
				example = "./dialtone.sh clone ./dialtone"
			case "sync-code":
				example = "./dialtone.sh sync-code"
			case "ssh":
				example = "./dialtone.sh ssh download /tmp/log.txt"
			case "provision":
				example = "./dialtone.sh provision"
			case "logs":
				example = "./dialtone.sh logs"
			case "diagnostic":
				example = "./dialtone.sh diagnostic --remote"
			case "branch":
				example = "./dialtone.sh branch my-feature"
			case "plan":
				example = "./dialtone.sh plan my-feature"
			case "test":
				example = "./dialtone.sh test my-feature"
			case "pull-request":
				example = "./dialtone.sh pull-request --draft"
			case "ticket":
				example = "./dialtone.sh ticket add my-feature"
			case "github":
				example = "./dialtone.sh github pull-request --draft"
			case "www":
				example = "./dialtone.sh www publish"
			case "ui":
				example = "./dialtone.sh ui dev"
			case "ai":
				example = "./dialtone.sh ai developer --dry-run"
			case "docs":
				example = "./dialtone.sh docs"
			}

			if example != "" {
				markdownLines = append(markdownLines, fmt.Sprintf("   - Example: `%s`", example))
			}
		}
	}

	newContent := strings.Join(markdownLines, "\n")

	// 4. Update AGENT.md
	agentMdPath := "AGENT.md"
	content, err := os.ReadFile(agentMdPath)
	if err != nil {
		LogFatal("Failed to read AGENT.md: %v", err)
	}

	text := string(content)

	// Regex to find the section
	// We want to replace everything from "### Development CLI (`dialtone-dev.go`)" up to the next "---"
	re := regexp.MustCompile(`(?s)### Development CLI \(` + "`" + `dialtone-dev\.go` + "`" + `\).*?(---)`)

	if !re.MatchString(text) {
		LogFatal("Could not find Development CLI section in AGENT.md")
	}

	// Replace content, keeping the trailing separator
	updatedText := re.ReplaceAllString(text, newContent+"\n\n$1")

	if err := os.WriteFile(agentMdPath, []byte(updatedText), 0644); err != nil {
		LogFatal("Failed to write AGENT.md: %v", err)
	}

	LogInfo("AGENT.md updated successfully!")
}

// runPlan handles the plan command
func runPlan(args []string) {
	planDir := "plan"

	// Ensure plan directory exists
	if err := os.MkdirAll(planDir, 0755); err != nil {
		LogFatal("Failed to create plan directory: %v", err)
	}

	// No args: list all plans
	if len(args) == 0 {
		listPlans(planDir)
		return
	}

	// With name: create or show plan
	name := args[0]
	planFile := filepath.Join(planDir, fmt.Sprintf("plan-%s.md", name))

	if _, err := os.Stat(planFile); os.IsNotExist(err) {
		// Create new plan from template
		createPlan(planFile, name)
	} else {
		// Show existing plan
		showPlan(planFile)
	}
}

// listPlans lists all plan files with their completion status
func listPlans(planDir string) {
	entries, err := os.ReadDir(planDir)
	if err != nil {
		LogFatal("Failed to read plan directory: %v", err)
	}

	fmt.Println("Plan Files:")
	fmt.Println("===========")

	planFound := false
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "plan-") && strings.HasSuffix(entry.Name(), ".md") {
			planFound = true
			planPath := filepath.Join(planDir, entry.Name())
			completed, total := countProgress(planPath)

			// Extract feature name from filename
			name := strings.TrimPrefix(entry.Name(), "plan-")
			name = strings.TrimSuffix(name, ".md")

			status := "[ ]"
			if total > 0 {
				if completed == total {
					status = "[x]"
				} else if completed > 0 {
					status = "[~]"
				}
			}

			fmt.Printf("  %s %s [%d/%d] %s\n", status, name, completed, total, entry.Name())
		}
	}

	if !planFound {
		fmt.Println("  No plan files found.")
		fmt.Println("\nCreate a new plan with: dialtone-dev plan <feature-name>")
	}
}

// countProgress counts completed items (- [x]) vs total items (- [ ] or - [x])
func countProgress(planPath string) (completed, total int) {
	content, err := os.ReadFile(planPath)
	if err != nil {
		return 0, 0
	}

	lines := strings.Split(string(content), "\n")
	checkboxPattern := regexp.MustCompile(`^- \[([ xX])\]`)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		matches := checkboxPattern.FindStringSubmatch(trimmed)
		if len(matches) > 1 {
			total++
			if matches[1] == "x" || matches[1] == "X" {
				completed++
			}
		}
	}

	return completed, total
}

// createPlan creates a new plan file from template
func createPlan(planPath, name string) {
	template := fmt.Sprintf(`# Plan: %s

## Goal
[Describe the goal of this feature]

## Tests
- [ ] test_example_1: [Description of first test]
- [ ] test_example_2: [Description of second test]

## Notes
- [Add any relevant notes]

## Blocking Tickets
- None

## Progress Log
- %s: Created plan file
`, name, time.Now().Format("2006-01-02"))

	if err := os.WriteFile(planPath, []byte(template), 0644); err != nil {
		LogFatal("Failed to create plan file: %v", err)
	}

	LogInfo("Created plan file: %s", planPath)
	fmt.Println("\nPlan Template Created:")
	fmt.Println("======================")
	fmt.Println(template)
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Edit the plan file to define your goal and tests")
	fmt.Println("  2. Create a branch: dialtone-dev branch", name)
	fmt.Println("  3. Start implementing tests from the plan")
}

// showPlan displays the contents of a plan file
func showPlan(planPath string) {
	content, err := os.ReadFile(planPath)
	if err != nil {
		LogFatal("Failed to read plan file: %v", err)
	}

	completed, total := countProgress(planPath)

	fmt.Println("Plan File:", planPath)
	fmt.Printf("Progress: %d/%d tests completed\n", completed, total)
	fmt.Println("======================")
	fmt.Println(string(content))
}

// runBranch handles the branch command
func runBranch(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: dialtone-dev branch <name>")
		fmt.Println("\nThis command creates or checks out a feature branch.")
		os.Exit(1)
	}

	branchName := args[0]

	// Check if branch exists
	cmd := exec.Command("git", "branch", "--list", branchName)
	output, err := cmd.Output()
	if err != nil {
		LogFatal("Failed to check branches: %v", err)
	}

	if strings.TrimSpace(string(output)) != "" {
		// Branch exists, checkout
		LogInfo("Branch '%s' exists, checking out...", branchName)
		cmd = exec.Command("git", "checkout", branchName)
	} else {
		// Branch doesn't exist, create
		LogInfo("Creating new branch '%s'...", branchName)
		cmd = exec.Command("git", "checkout", "-b", branchName)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		LogFatal("Git operation failed: %v", err)
	}

	LogInfo("Now on branch: %s", branchName)
}



// runTicket handles the ticket command
func runTicket(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: dialtone-dev ticket <subcommand> [options]")
		fmt.Println("\nSubcommands:")
		fmt.Println("  add <name>         Add a new local ticket (scaffold only)")
		fmt.Println("  start <name>       Start a new ticket (branch + scaffold)")
		fmt.Println("  done <name>        Verify ticket completion")
		fmt.Println("  subtask <subcmd>   Manage ticket subtasks")
		fmt.Println("  list [N]           List the top N tickets (GH)")
		fmt.Println("  create             Create a new GitHub ticket (GH)")
		fmt.Println("  comment <id> <msg> Add a comment to a ticket (GH)")
		fmt.Println("  view <id>          View ticket details (GH)")
		fmt.Println("  close <id>         Close a GitHub ticket (GH)")
		return
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "add":
		ticket_cli.RunAdd(subArgs)
	case "start":
		ticket_cli.RunStart(subArgs)
	case "done":
		ticket_cli.RunDone(subArgs)
	case "subtask":
		ticket_cli.RunSubtask(subArgs)
	// Fallback to legacy GitHub CLI commands for everything else
	// But first check if they exist to provide better error if not
	case "list", "create", "comment", "view", "close":
		// Check if gh CLI is available for these commands
		if _, err := exec.LookPath("gh"); err != nil {
			LogFatal("GitHub CLI (gh) not found. Install it from: https://cli.github.com/")
		}
		// Continue to switch block below (or reuse logic)
		// Since we can't easily jump into existing switch case from here without code duplication or goto,
		// I will reimplement the dispatcher to be cleaner.
	}

	// Re-implement the switch for legacy or just handle legacy logic here
	switch subcommand {
	case "add", "start", "test", "done", "subtask":
		return // Already handled
	case "list":

		limit := "10"
		if len(subArgs) > 0 {
			limit = subArgs[0]
		}
		cmd := exec.Command("gh", "ticket", "list", "-L", limit)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			LogFatal("Failed to list tickets: %v", err)
		}

	case "create":
		var title, body string
		var passedArgs []string

		// Simple flag parsing for title and body
		for i := 0; i < len(subArgs); i++ {
			switch subArgs[i] {
			case "--title", "-t":
				if i+1 < len(subArgs) {
					title = subArgs[i+1]
					passedArgs = append(passedArgs, "--title", title)
					i++
				}
			case "--body", "-b":
				if i+1 < len(subArgs) {
					body = subArgs[i+1]
					passedArgs = append(passedArgs, "--body", body)
					i++
				}
			case "--label", "-l":
				if i+1 < len(subArgs) {
					passedArgs = append(passedArgs, "--label", subArgs[i+1])
					i++
				}
			}
		}

		args := []string{"ticket", "create"}
		args = append(args, passedArgs...)

		cmd := exec.Command("gh", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		// Only attach Stdin if interactive (no title provided)
		if title == "" {
			cmd.Stdin = os.Stdin
		}

		if err := cmd.Run(); err != nil {
			LogFatal("Failed to create ticket: %v", err)
		}

	case "comment":
		if len(subArgs) < 2 {
			LogFatal("Usage: dialtone-dev ticket comment <ticket-id> <message>")
		}
		ticketID := subArgs[0]
		message := subArgs[1]
		cmd := exec.Command("gh", "ticket", "comment", ticketID, "--body", message)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			LogFatal("Failed to add comment: %v", err)
		}

	case "view":
		if len(subArgs) < 1 {
			LogFatal("Usage: dialtone-dev ticket view <ticket-id>")
		}
		ticketID := subArgs[0]
		cmd := exec.Command("gh", "ticket", "view", ticketID)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			LogFatal("Failed to view ticket: %v", err)
		}

	case "close":
		if len(subArgs) < 1 {
			LogFatal("Usage: dialtone-dev ticket close <ticket-id>")
		}
		ticketID := subArgs[0]
		LogInfo("Closing ticket #%s...", ticketID)
		cmd := exec.Command("gh", "ticket", "close", ticketID)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			LogFatal("Failed to close ticket: %v", err)
		}

	default:
		fmt.Printf("Unknown ticket subcommand: %s\n", subcommand)
		runTicket([]string{}) // Show usage
	}
}


