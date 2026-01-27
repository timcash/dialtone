package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"dialtone/cli/src/core/config"
	"dialtone/cli/src/core/logger"
)

func findCloudflared() string {
	depsDir := config.GetDialtoneEnv()

	cfPath := filepath.Join(depsDir, "cloudflare", "cloudflared")
	if _, err := os.Stat(cfPath); err == nil {
		return cfPath
	}

	// Fallback to system PATH
	if p, err := exec.LookPath("cloudflared"); err == nil {
		return p
	}

	return "cloudflared"
}

// RunCloudflare handles 'cloudflare <subcommand>'
func RunCloudflare(args []string) {
	if len(args) == 0 {
		printCloudflareUsage()
		return
	}

	subcommand := args[0]
	restArgs := args[1:]

	switch subcommand {
	case "login":
		runLogin(restArgs)
	case "tunnel":
		runTunnel(restArgs)
	case "serve":
		runServe(restArgs)
	case "help", "-h", "--help":
		printCloudflareUsage()
	default:
		fmt.Printf("Unknown cloudflare command: %s\n", subcommand)
		printCloudflareUsage()
		os.Exit(1)
	}
}

func printCloudflareUsage() {
	fmt.Println("Usage: dialtone cloudflare <command> [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  login       Authenticate with Cloudflare")
	fmt.Println("  tunnel      Manage Cloudflare tunnels (create, list, etc.)")
	fmt.Println("  serve       Forward a local HTTP server to the web")
	fmt.Println("  help        Show this help message")
}

func runLogin(args []string) {
	cf := findCloudflared()
	logger.LogInfo("Logging into Cloudflare...")
	
	cmd := exec.Command(cf, "tunnel", "login")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		logger.LogFatal("Cloudflare login failed: %v", err)
	}
	logger.LogInfo("Cloudflare login complete.")
}

func runTunnel(args []string) {
	cf := findCloudflared()
	if len(args) == 0 {
		fmt.Println("Usage: dialtone cloudflare tunnel <subcommand>")
		fmt.Println("\nSubcommands:")
		fmt.Println("  create <name>   Create a new tunnel")
		fmt.Println("  list            List existing tunnels")
		return
	}
	
	sub := args[0]
	subArgs := args[1:]
	
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "tunnel")
	
	switch sub {
	case "create":
		cmdArgs = append(cmdArgs, "create")
		cmdArgs = append(cmdArgs, subArgs...)
	case "list":
		cmdArgs = append(cmdArgs, "list")
		cmdArgs = append(cmdArgs, subArgs...)
	default:
		fmt.Printf("Unknown tunnel subcommand: %s\n", sub)
		return
	}
	
	cmd := exec.Command(cf, cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logger.LogFatal("Cloudflare tunnel %s failed: %v", sub, err)
	}
}

func runServe(args []string) {
	cf := findCloudflared()
	if len(args) < 1 {
		fmt.Println("Usage: dialtone cloudflare serve <port-or-url>")
		return
	}
	
	target := args[0]
	logger.LogInfo("Starting Cloudflare tunnel to serve %s...", target)
	
	// cloudflared tunnel --url http://localhost:PORT
	// Or just cloudflared tunnel --url target
	
	cmdArgs := []string{"tunnel", "--url", target}
	cmd := exec.Command(cf, cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Run(); err != nil {
		logger.LogFatal("Cloudflare serve failed: %v", err)
	}
}
