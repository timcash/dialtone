package dialtone

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	browser_cli "dialtone/cli/src/core/browser/cli"
	build_cli "dialtone/cli/src/core/build/cli"
	"dialtone/cli/src/core/config"
	config_core_cli "dialtone/cli/src/core/config/cli"
	earth_cli "dialtone/cli/src/core/earth/cli"
	install_cli "dialtone/cli/src/core/install/cli"
	"dialtone/cli/src/core/logger"
	logger_cli "dialtone/cli/src/core/logger/cli"
	mock_cli "dialtone/cli/src/core/mock/cli"
	ssh_cli "dialtone/cli/src/core/ssh/cli"
	util_cli "dialtone/cli/src/core/util/cli"
	web_cli "dialtone/cli/src/core/web/cli"

	camera_cli "dialtone/cli/src/plugins/camera/cli"
	chrome_cli "dialtone/cli/src/plugins/chrome/cli"
	cloudflare_cli "dialtone/cli/src/plugins/cloudflare/cli"
	deploy_cli "dialtone/cli/src/plugins/deploy/cli"
	diagnostic_cli "dialtone/cli/src/plugins/diagnostic/cli"
	github_cli "dialtone/cli/src/plugins/github/cli"
	go_cli "dialtone/cli/src/plugins/go/cli"
	ide_cli "dialtone/cli/src/plugins/ide/cli"
	jax_demo_cli "dialtone/cli/src/plugins/jax-demo/cli"
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
	case "start":
		runStart(args)
	case "build":
		build_cli.Run(args)
	case "deploy":
		deploy_cli.RunDeploy(args)
	case "ssh":
		ssh_cli.Run(args)
	case "vpn":
		runVPN(args)
	case "vpn-provision", "provision":
		vpn_cli.RunProvision(args)
	case "logs":
		logs_cli.RunLogs(args)
	case "diagnostic":
		diagnostic_cli.RunDiagnostic(args)
	case "install":
		install_cli.Run(args)
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
		ticket_cli.Run(args)
	case "plugin":
		plugin_cli.RunPlugin(args)
	case "cloudflare":
		cloudflare_cli.RunCloudflare(args)
	case "ide":
		ide_cli.Run(args)
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
	case "go":
		go_cli.RunGo(args)
	case "jax-demo":
		jax_demo_cli.Run(args)

	// Core Module Specific Commands
	case "browser":
		browser_cli.Run(args)
	case "config":
		config_core_cli.Run(args)
	case "earth":
		earth_cli.Run(args)
	case "logger":
		logger_cli.Run(args)
	case "mock":
		mock_cli.Run(args)
	case "util":
		util_cli.Run(args)
	case "web":
		web_cli.Run(args)

	case "ai", "opencode", "developer", "subagent":
		// Delegate to plugin command to remove static dependency on AI from core
		plugin_cli.RunPlugin(append([]string{command}, args...))
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
	fmt.Println("  start         Start the NATS and Web server")
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
	fmt.Println("  ticket <subcmd>    Manage GitHub tickets (start, next, done, etc.)")
	fmt.Println("  plugin <subcmd>    Manage plugins (add, install, build)")
	fmt.Println("  ide <subcmd>       IDE tools (setup-workflows)")
	fmt.Println("  github <subcmd>    Manage GitHub interactions (pr, check-deploy)")
	fmt.Println("  www <subcmd>       Manage public webpage (Vercel wrapper)")
	fmt.Println("  ui <subcmd>        Manage web UI (dev, build, install)")
	fmt.Println("  test <subcmd>      Run tests (ticket, plugin, tags)")

	fmt.Println("  ai <subcmd>        AI tools (opencode, developer, subagent)")
	fmt.Println("  go <subcmd>        Go toolchain tools (install, lint)")
	fmt.Println("  jax-demo           JAX Demo Plugin")
	fmt.Println("\nCore Modules:")
	fmt.Println("  browser, config, earth, logger, mock, util, web")

	fmt.Println("\nHelp:")
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
