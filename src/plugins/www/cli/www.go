package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func logInfo(format string, args ...interface{}) {
	fmt.Printf("[www] "+format+"\n", args...)
}

func logFatal(format string, args ...interface{}) {
	fmt.Printf("[www] FATAL: "+format+"\n", args...)
	os.Exit(1)
}

// RunWww handles 'www <subcommand>'
func RunWww(args []string) {
	// Check if vercel CLI is available
	homeDir, _ := os.UserHomeDir()
	vercelPath := filepath.Join(homeDir, ".dialtone_env", "node", "bin", "vercel")
	if _, err := os.Stat(vercelPath); os.IsNotExist(err) {
		// Fallback to searching in PATH
		if p, err := exec.LookPath("vercel"); err == nil {
			vercelPath = p
		} else {
			logFatal("Vercel CLI not found. Run 'dialtone install' to install dependencies.")
		}
	}

	// Handle help explicitly
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Println("Usage: dialtone-dev www <subcommand> [options]")
		fmt.Println("\nSubcommands:")
		fmt.Println("  publish            Deploy the webpage to Vercel")
		fmt.Println("  logs               View deployment logs")
		fmt.Println("  domain             Manage the dialtone.earth domain")
		fmt.Println("  login              Login to Vercel")
		return
	}

	subcommand := args[0]
	// Determine the directory where the webpage code is located
	// Used to be "dialtone-earth", now it is "src/plugins/www/app"
	// We need to resolve it relative to the project root (where dialtone-dev runs)
	webDir := filepath.Join("src", "plugins", "www", "app")

	switch subcommand {
	case "publish":
		logInfo("Deploying webpage to Vercel...")
		vArgs := append([]string{"deploy", "--prod"}, args[1:]...)
		cmd := exec.Command(vercelPath, vArgs...)
		cmd.Dir = webDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			logFatal("Failed to deploy: %v", err)
		}
		logInfo("Deployment successful!")

	case "logs":
		vArgs := append([]string{"logs"}, args[1:]...)
		cmd := exec.Command(vercelPath, vArgs...)
		cmd.Dir = webDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			logFatal("Failed to show logs: %v", err)
		}

	case "domain":
		// Usage: dialtone-dev www domain [deployment-url]
		// If no deployment-url is given, it will attempt to alias the most recent deployment.
		vArgs := []string{"alias", "set"}
		vArgs = append(vArgs, args[1:]...)
		vArgs = append(vArgs, "dialtone.earth")
		cmd := exec.Command(vercelPath, vArgs...)
		cmd.Dir = webDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			logFatal("Failed to set domain alias: %v", err)
		}

	case "login":
		cmd := exec.Command(vercelPath, "login")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			logFatal("Failed to login: %v", err)
		}

	default:
		// Generic pass-through to vercel CLI
		logInfo("Running: vercel %s %s", subcommand, strings.Join(args[1:], " "))
		vArgs := append([]string{subcommand}, args[1:]...)
		cmd := exec.Command(vercelPath, vArgs...)
		cmd.Dir = webDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			logFatal("Vercel command failed: %v", err)
		}
	}
}
