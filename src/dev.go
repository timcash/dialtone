//go:build !no_duckdb

package dialtone

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	build_cli "dialtone/cli/src/core/build/cli"
	"dialtone/cli/src/core/config"
	format_cli "dialtone/cli/src/core/format/cli"
	"dialtone/cli/src/core/install"
	"dialtone/cli/src/core/logger"
	"dialtone/cli/src/core/ssh"
	test_cli "dialtone/cli/src/core/test/cli"
	ai_cli "dialtone/cli/src/plugins/ai/cli"
	bun_cli "dialtone/cli/src/plugins/bun/cli"
	cad_cli "dialtone/cli/src/plugins/cad/cli"
	camera_cli "dialtone/cli/src/plugins/camera/cli"
	chrome_cli "dialtone/cli/src/plugins/chrome/cli"
	cloudflare_cli "dialtone/cli/src/plugins/cloudflare/cli"
	dag_cli "dialtone/cli/src/plugins/dag/cli"
	deploy_cli "dialtone/cli/src/plugins/deploy/cli"
	diagnostic_cli "dialtone/cli/src/plugins/diagnostic/cli"
	github_cli "dialtone/cli/src/plugins/github/cli"
	go_cli "dialtone/cli/src/plugins/go/cli"
	ide_cli "dialtone/cli/src/plugins/ide/cli"
	logs_cli "dialtone/cli/src/plugins/logs/cli"
	mavlink_cli "dialtone/cli/src/plugins/mavlink/cli"
	nix_cli "dialtone/cli/src/plugins/nix/cli"
	swarm_cli "dialtone/cli/src/plugins/swarm/cli"
	template_cli "dialtone/cli/src/plugins/template/cli"
	ui_cli "dialtone/cli/src/plugins/ui/cli"
	vpn_cli "dialtone/cli/src/plugins/vpn/cli"
	wsl_cli "dialtone/cli/src/plugins/wsl/cli"

	task_cli "dialtone/cli/src/plugins/task/cli"
	www_cli "dialtone/cli/src/plugins/www/cli"
)

// ExecuteDev is the entry point for the dialtone CLI
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
	case "format":
		format_cli.Run(args)
	case "ssh":
		ssh.RunSSH(args)
	case "vpn", "vpn-provision", "provision", "vpn-test":
		vpn_cli.RunVPN(append([]string{command}, args...))
	case "logs":
		logs_cli.RunLogs(args)
	case "diagnostic":
		diagnostic_cli.RunDiagnostic(args)
	case "install":
		install.RunInstall(args)
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
	case "swarm":
		swarm_cli.RunSwarm(args)
	case "cloudflare":
		cloudflare_cli.RunCloudflare(args)
	case "ide":
		ide_cli.Run(args)
	case "camera":
		camera_cli.RunCamera(args)
	case "bun":
		bun_cli.RunBun(args)
	case "chrome":
		chrome_cli.RunChrome(args)
	case "mavlink":
		mavlink_cli.RunMavlink(args)
	case "www":
		www_cli.RunWww(args)
	case "ui":
		ui_cli.Run(args)
	case "task":
		task_cli.Run(args)
	case "nix":
		if err := nix_cli.Run(args); err != nil {
			fmt.Printf("Nix command error: %v\n", err)
			os.Exit(1)
		}
	case "dag":
		if err := dag_cli.Run(args); err != nil {
			fmt.Printf("DAG command error: %v\n", err)
			os.Exit(1)
		}
	case "wsl":
		if err := wsl_cli.Run(args); err != nil {
			fmt.Printf("WSL command error: %v\n", err)
			os.Exit(1)
		}
	case "template":
		if err := template_cli.Run(args); err != nil {
			fmt.Printf("Template command error: %v\n", err)
			os.Exit(1)
		}
	case "go":

		go_cli.RunGo(args)
	case "cad":
		cad_cli.RunCad(args)

	case "ai", "opencode", "developer", "subagent":
		if command == "ai" {
			ai_cli.RunAI(args)
		} else {
			ai_cli.RunAI(append([]string{command}, args...))
		}
	case "help", "-h", "--help":
		printDevUsage()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printDevUsage()
		os.Exit(1)
	}
}

func printDevUsage() {
	script := "./dialtone.sh"
	if runtime.GOOS == "windows" {
		script = ".\\dialtone.cmd"
	}
	fmt.Printf("Usage: %s <command> [options]\n", script)
	fmt.Println("\nCommands:")
	fmt.Println("  start         Start the NATS and Web server")
	fmt.Println("  install [path] Install dependencies (--linux-wsl for WSL, --macos-arm for Apple Silicon)")
	fmt.Println("  build         Build web UI and binary (--local, --full, --remote, --podman, --linux-arm, --linux-arm64, --linux-amd64)")
	fmt.Println("  deploy        Deploy to remote robot")
	fmt.Println("  format        Format Go code across the repo")
	fmt.Println("  camera        Camera tools (snapshot, stream)")
	fmt.Println("  bun <subcmd>       Bun toolchain tools (exec, run, x)")
	fmt.Println("  clone         Clone or update the repository")
	fmt.Println("  sync-code     Sync source code to remote robot")
	fmt.Println("  ssh           SSH tools (upload, download, cmd)")
	fmt.Println("  provision     Generate Tailscale Auth Key")
	fmt.Println("  logs          Tail remote logs")
	fmt.Println("  diagnostic    Run system diagnostics (local or remote)")
	fmt.Println("  branch <name>      Create or checkout a feature branch")
	fmt.Println("  swarm <topic>      Join a Hyperswarm topic")
	fmt.Println("  ide <subcmd>       IDE tools (setup-workflows)")
	fmt.Println("  github <subcmd>    Manage GitHub interactions (pr, check-deploy)")
	fmt.Println("  www <subcmd>       Manage public webpage (Vercel wrapper)")
	fmt.Println("  ui <subcmd>        Manage web UI (dev, build, install)")
	fmt.Println("  test <subcmd>      Run tests (legacy)")
	fmt.Println("  nix <subcmd>       Nix plugin tools (smoke)")
	fmt.Println("  dag <subcmd>       DAG plugin tools (install, dev, build, test, src)")
	fmt.Println("  wsl <subcmd>       WSL plugin tools (smoke)")
	fmt.Println("  template <subcmd>  Template plugin tools (install, dev, build, test, src)")

	fmt.Println("  ai <subcmd>        AI tools (opencode, developer, subagent)")
	fmt.Println("  go <subcmd>        Go toolchain tools (install, lint)")
	fmt.Println("  help               Show this help message")
}

// isPlugin checks if a directory exists in src/plugins
func isPlugin(name string) bool {
	path := filepath.Join("src", "plugins", name)
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// runBranch handles the branch command
func runBranch(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: ./dialtone.sh branch <name>")
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
