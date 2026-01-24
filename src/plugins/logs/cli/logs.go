package cli

import (
	"flag"
	"fmt"
	"os"
	
	"dialtone/cli/src/core/logger"
	"dialtone/cli/src/core/ssh"
)

// RunLogs handles the logs command
func RunLogs(args []string) {
	fs := flag.NewFlagSet("logs", flag.ExitOnError)
	remote := fs.Bool("remote", false, "Stream logs from remote robot")
	host := fs.String("host", os.Getenv("ROBOT_HOST"), "SSH host")
	port := fs.String("port", "22", "SSH port")
	user := fs.String("user", os.Getenv("ROBOT_USER"), "SSH user")
	pass := fs.String("pass", os.Getenv("ROBOT_PASSWORD"), "SSH password")
	lines := fs.Int("lines", 0, "Number of lines to show (default: stream logs)")
	showHelp := fs.Bool("help", false, "Show help for logs command")

	fs.Usage = func() {
		fmt.Println("Usage: dialtone logs [options]")
		fmt.Println()
		fmt.Println("Stream logs from the Dialtone service.")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --remote      Stream logs from remote robot")
		fmt.Println("  --lines       Number of lines to show (if set, does not stream)")
		fmt.Println("  --host        SSH host (user@host) [env: ROBOT_HOST]")
		fmt.Println("  --port        SSH port (default: 22)")
		fmt.Println("  --user        SSH username [env: ROBOT_USER]")
		fmt.Println("  --pass        SSH password [env: ROBOT_PASSWORD]")
		fmt.Println("  --help        Show this help message")
		fmt.Println()
	}

	fs.Parse(args)

	if *showHelp {
		fs.Usage()
		return
	}

	if *remote {
		if *host == "" || *pass == "" {
			logger.LogFatal("Error: --host and --pass are required for remote logs")
		}
		
		runRemoteLogs(*host, *port, *user, *pass, *lines)
	} else {
		// Local logs (placeholder for now, or maybe just tell user to check locally)
		logger.LogInfo("Looking for local logs...")
		// Assuming local logs might be in a standard place or stdout if running locally
		fmt.Println("Local log streaming is not yet implemented. Use --remote to view robot logs.")
	}
}

func runRemoteLogs(host, port, user, pass string, lines int) {
	logger.LogInfo("Connecting to %s to stream logs...", host)
	
	client, err := ssh.DialSSH(host, port, user, pass)
	if err != nil {
		logger.LogFatal("SSH connection failed: %v", err)
	}
	defer client.Close()
	
	// We want to tail the log file. 
	// Based on the ticket description, the start command redirects output to ~/nats.log
	var cmd string
	if lines > 0 {
		cmd = fmt.Sprintf("tail -n %d ~/nats.log", lines)
		logger.LogInfo("Getting last %d lines from ~/nats.log...", lines)
	} else {
		cmd = "tail -f ~/nats.log"
		logger.LogInfo("Streaming logs from ~/nats.log...")
	}
	
	// Use RunSSHCommand but we actually want to stream it. 
	// RunSSHCommand waits for completion, but tail -f runs forever. 
	// So we need a way to stream stdout.
	
	session, err := client.NewSession()
	if err != nil {
		logger.LogFatal("Failed to create SSH session: %v", err)
	}
	defer session.Close()

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	if err := session.Run(cmd); err != nil {
		// Ignore error if it's just a signal kill (which happens when user Ctrl+C)
		// But for -n, it should exit cleanly.
		if lines > 0 {
			logger.LogFatal("Command failed: %v", err)
		}
	}
}
