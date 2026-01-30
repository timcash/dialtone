package cli

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"dialtone/cli/src/core/logger"
	app "dialtone/cli/src/plugins/diagnostic/app"
)

// RunDiagnostic handles the 'diagnostic' command
func RunDiagnostic(args []string) {
	logger.LogInfo("Running System Diagnostics...")

	// 1. Check OS/Arch
	fmt.Printf("OS: %s\n", runtime.GOOS)
	fmt.Printf("Arch: %s\n", runtime.GOARCH)

	fs := flag.NewFlagSet("diagnostic", flag.ExitOnError)
	host := fs.String("host", os.Getenv("ROBOT_HOST"), "SSH host (user@host)")
	port := fs.String("port", "22", "SSH port")
	user := fs.String("user", os.Getenv("ROBOT_USER"), "SSH user")
	pass := fs.String("pass", os.Getenv("ROBOT_PASSWORD"), "SSH password")
	showHelp := fs.Bool("help", false, "Show help for diagnostic command")

	fs.Usage = func() {
		fmt.Println("Usage: dialtone-dev diagnostic [options]")
		fmt.Println("\nRun system diagnostics on local machine or remote robot.")
		fmt.Println("\nOptions:")
		fmt.Println("  --host        SSH host (user@host)")
		fmt.Println("  --port        SSH port (default: 22)")
		fmt.Println("  --user        SSH user")
		fmt.Println("  --pass        SSH password")
		fmt.Println("  help          Show this help message")
		fmt.Println("  --help        Show this help message")
	}

	if len(args) > 0 && args[0] == "help" {
		fs.Usage()
		return
	}

	fs.Parse(args)

	if *showHelp {
		fs.Usage()
		return
	}

	logger.LogInfo("Running System Diagnostics...")

	// 1. Check OS/Arch
	fmt.Printf("OS: %s\n", runtime.GOOS)
	fmt.Printf("Arch: %s\n", runtime.GOARCH)

	if *host == "" {
		// 2. Check dependencies (Go, Node, Tailscale)
		app.CheckLocalDependencies()

		logger.LogInfo("No host specified. Skipping remote diagnostics.")
		logger.LogInfo("Diagnostics Passed.")
		return
	}

	app.RunRemoteDiagnostics(*host, *port, *user, *pass)
}
