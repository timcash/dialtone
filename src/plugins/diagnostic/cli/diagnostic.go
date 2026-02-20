package cli

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"dialtone/dev/plugins/logs/src_v1/go"
	app "dialtone/dev/plugins/diagnostic/app"
)

// RunDiagnostic handles the 'diagnostic' command
func RunDiagnostic(args []string) {
	logs.Info("Running System Diagnostics...")

	// 1. Check OS/Arch
	fmt.Printf("OS: %s\n", runtime.GOOS)
	fmt.Printf("Arch: %s\n", runtime.GOARCH)

	fs := flag.NewFlagSet("diagnostic", flag.ExitOnError)
	host := fs.String("host", os.Getenv("ROBOT_HOST"), "SSH host (user@host)")
	port := fs.String("port", "22", "SSH port")
	user := fs.String("user", os.Getenv("ROBOT_USER"), "SSH user")
	pass := fs.String("pass", os.Getenv("ROBOT_PASSWORD"), "SSH password")

	fs.Parse(args)

	if *host == "" {
		// 2. Check dependencies (Go, Node, Tailscale)
		app.CheckLocalDependencies()

		logs.Info("No host specified. Skipping remote diagnostics.")
		logs.Info("Diagnostics Passed.")
		return
	}

	app.RunRemoteDiagnostics(*host, *port, *user, *pass)
}
