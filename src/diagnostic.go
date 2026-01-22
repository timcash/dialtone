package dialtone

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"time"

	"dialtone/cli/src/core/ssh"
	"github.com/chromedp/chromedp"
)

// RunDiagnostic handles the 'diagnostic' command
func RunDiagnostic(args []string) {
	fs := flag.NewFlagSet("diagnostic", flag.ExitOnError)
	host := fs.String("host", os.Getenv("ROBOT_HOST"), "SSH host (user@host)")
	port := fs.String("port", "22", "SSH port")
	user := fs.String("user", os.Getenv("ROBOT_USER"), "SSH user")
	pass := fs.String("pass", os.Getenv("ROBOT_PASSWORD"), "SSH password")

	fs.Parse(args)

	if *host == "" {
		LogInfo("No host specified. Running local diagnostics...")
		runLocalDiagnostics()
		return
	}

	if *pass == "" {
		LogFatal("Error: -pass is required for remote diagnostics")
	}

	client, err := ssh.DialSSH(*host, *port, *user, *pass)
	if err != nil {
		LogFatal("SSH connection failed: %v", err)
	}
	defer client.Close()

	LogInfo("Running diagnostics on %s...", *host)

	commands := []struct {
		name string
		cmd  string
	}{
		{"CPU Usage", "top -bn1 | grep 'Cpu(s)'"},
		{"Memory Usage", "free -h"},
		{"Disk Usage", "df -h /"},
		{"Network Interfaces", "ip addr show"},
		{"Tailscale Status", "tailscale status"},
		{"NATS Status", "ps aux | grep nats-server | grep -v grep || echo 'NATS not running'"},
		{"Dialtone Status", "ps aux | grep dialtone | grep -v grep || echo 'Dialtone not running'"},
	}

	for _, c := range commands {
		fmt.Printf("\n--- %s ---\n", c.name)
		output, err := ssh.RunSSHCommand(client, c.cmd)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			fmt.Println(output)
		}
	}

	// Web UI Check via Chromedp
	fmt.Printf("\n--- Web UI Check (chromedp) ---\n")
	hostname := os.Getenv("DIALTONE_HOSTNAME")
	if hostname == "" {
		hostname = "drone_1"
	}
	url := fmt.Sprintf("http://%s", hostname)
	if err := checkWebUI(url); err != nil {
		fmt.Printf("Web UI Check FAILED: %v\n", err)
	} else {
		fmt.Printf("Web UI Check SUCCESS: %s is reachable and rendering\n", url)
	}
}

func runLocalDiagnostics() {
	fmt.Println("Local System Diagnostics:")
	fmt.Println("=========================")

	// Basic local checks
	fmt.Print("Checking Go version... ")
	out, err := execCommand("go", "version")
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Print(out)
	}

	fmt.Print("Checking Node version... ")
	out, err = execCommand("node", "--version")
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Print(out)
	}

	fmt.Print("Checking Zig version... ")
	out, err = execCommand("zig", "version")
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Print(out)
	}

	fmt.Println("\nLocal diagnostics complete.")
}

func execCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func checkWebUI(url string) error {
	// Create context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// Create a timeout
	ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var title string
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.Title(&title),
	)
	if err != nil {
		return err
	}

	if title == "" {
		return fmt.Errorf("page loaded but title is empty")
	}

	fmt.Printf("Dashboard Title: %s\n", title)
	return nil
}
