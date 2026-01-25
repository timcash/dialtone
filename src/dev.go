package dialtone

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"dialtone/cli/src/core/config"
	"dialtone/cli/src/core/logger"
	"dialtone/cli/src/core/ssh"
	ai_cli "dialtone/cli/src/plugins/ai/cli"
	build_cli "dialtone/cli/src/plugins/build/cli"
	camera_cli "dialtone/cli/src/plugins/camera/cli"
	chrome_cli "dialtone/cli/src/plugins/chrome/cli"
	deploy_cli "dialtone/cli/src/plugins/deploy/cli"
	diagnostic_cli "dialtone/cli/src/plugins/diagnostic/cli"
	github_cli "dialtone/cli/src/plugins/github/cli"
	install_cli "dialtone/cli/src/plugins/install/cli"
	logs_cli "dialtone/cli/src/plugins/logs/cli"
	mavlink_cli "dialtone/cli/src/plugins/mavlink/cli"
	plugin_cli "dialtone/cli/src/plugins/plugin/cli"
	test_cli "dialtone/cli/src/plugins/test/cli"
	ticket_cli "dialtone/cli/src/plugins/ticket/cli"
	ui_cli "dialtone/cli/src/plugins/ui/cli"
	vpn_cli "dialtone/cli/src/plugins/vpn/cli"
	www_cli "dialtone/cli/src/plugins/www/cli"
)

// ExecuteDev is the entry point for the dialtone-dev CLI
func ExecuteDev() {
	if len(os.Args) < 2 {
		printDevUsage()
		return
	}

	// Load configuration
	config.LoadConfig()

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "build":
		build_cli.RunBuild(args)
	case "deploy":
		deploy_cli.RunDeploy(args)
	case "ssh":
		ssh.RunSSH(args)
	case "vpn":
		// User requested: dialtone vpn ...
		// run provision logic or subcommands
		vpn_cli.RunProvision(args)
	case "provision":
		// Keep legacy provision command mapped to vpn plugin for now?
		// Or remove it? User said "make ... a cli sub command on a plugin called vpn".
		// I will redirect provision to vpn for backward compat or just remove it.
		// Let's redirect it.
		vpn_cli.RunProvision(args)
	case "logs":
		logs_cli.RunLogs(args)
	case "diagnostic":
		diagnostic_cli.RunDiagnostic(args)
	case "install":
		install_cli.RunInstall(args)
	case "clone":
		RunClone(args)
	case "sync-code":
		deploy_cli.RunSyncCode(args)
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
	fmt.Println("  branch <name>      Create or checkout a feature branch")
	fmt.Println("  ticket <subcmd>    Manage GitHub tickets (start, done, subtask, etc.)")
	fmt.Println("  plugin <subcmd>    Manage plugins (add, install, build)")
	fmt.Println("  github <subcmd>    Manage GitHub interactions (pr, check-deploy)")
	fmt.Println("  www <subcmd>       Manage public webpage (Vercel wrapper)")
	fmt.Println("  ui <subcmd>        Manage web UI (dev, build, install)")
	fmt.Println("  test <subcmd>      Run tests (ticket, plugin, tags)")

	fmt.Println("  ai <subcmd>        AI tools (opencode, developer, subagent)")
	fmt.Println("  help               Show this help message")
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
		logger.LogFatal("Failed to check branches: %v", err)
	}

	if strings.TrimSpace(string(output)) != "" {
		// Branch exists, checkout
		logger.LogInfo("Branch '%s' exists, checking out...", branchName)
		cmd = exec.Command("git", "checkout", branchName)
	} else {
		// Branch doesn't exist, create
		logger.LogInfo("Creating new branch '%s'...", branchName)
		cmd = exec.Command("git", "checkout", "-b", branchName)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logger.LogFatal("Git operation failed: %v", err)
	}

	logger.LogInfo("Now on branch: %s", branchName)
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
			logger.LogFatal("GitHub CLI (gh) not found. Install it from: https://cli.github.com/")
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
			logger.LogFatal("Failed to list tickets: %v", err)
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
			logger.LogFatal("Failed to create ticket: %v", err)
		}

	case "comment":
		if len(subArgs) < 2 {
			logger.LogFatal("Usage: dialtone-dev ticket comment <ticket-id> <message>")
		}
		ticketID := subArgs[0]
		message := subArgs[1]
		cmd := exec.Command("gh", "ticket", "comment", ticketID, "--body", message)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			logger.LogFatal("Failed to add comment: %v", err)
		}

	case "view":
		if len(subArgs) < 1 {
			logger.LogFatal("Usage: dialtone-dev ticket view <ticket-id>")
		}
		ticketID := subArgs[0]
		cmd := exec.Command("gh", "ticket", "view", ticketID)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			logger.LogFatal("Failed to view ticket: %v", err)
		}

	case "close":
		if len(subArgs) < 1 {
			logger.LogFatal("Usage: dialtone-dev ticket close <ticket-id>")
		}
		ticketID := subArgs[0]
		logger.LogInfo("Closing ticket #%s...", ticketID)
		cmd := exec.Command("gh", "ticket", "close", ticketID)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			logger.LogFatal("Failed to close ticket: %v", err)
		}

	default:
		fmt.Printf("Unknown ticket subcommand: %s\n", subcommand)
		runTicket([]string{}) // Show usage
	}
}
