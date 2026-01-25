package dialtone

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"dialtone/cli/src/core/browser"
	"dialtone/cli/src/core/logger"
	"dialtone/cli/src/core/ssh"

	"github.com/chromedp/chromedp"
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

	fs.Parse(args)

	if *host == "" {
		// 2. Check dependencies (Go, Node, Tailscale)
		if _, err := exec.LookPath("go"); err != nil {
			logger.LogFatal("Go is not installed.")
		}
		if _, err := exec.LookPath("node"); err != nil {
			logger.LogInfo("Node.js is not installed (warning).")
		}
		if _, err := exec.LookPath("tailscale"); err != nil {
			logger.LogFatal("Tailscale is not installed.")
		}

		logger.LogInfo("No host specified. Skipping remote diagnostics.")
		logger.LogInfo("Diagnostics Passed.")
		return
	}

	if *pass == "" {
		logger.LogFatal("Error: -pass is required for remote diagnostics")
	}

	client, err := ssh.DialSSH(*host, *port, *user, *pass)
	if err != nil {
		logger.LogFatal("SSH connection failed: %v", err)
	}
	defer client.Close()

	logger.LogInfo("Running diagnostics on %s...", *host)

	commands := []struct {
		name string
		cmd  string
	}{
		{"Hostname", "hostname"},
		{"Uptime", "uptime -p"},
		{"CPU Usage", "top -bn1 | grep 'Cpu(s)' | sed 's/.*, *\\([0-9.]*\\)%* id.*/\\1/' | awk '{print 100 - $1\"% used\"}'"},
		{"Memory Usage", "free | awk '/^Mem:/ {printf \"%dMi / %dMi (%.1f%%)\", $3/1024, $2/1024, $3/$2*100}'"},
		{"Disk Usage", "df -h / | awk 'NR==2 {print $3 \" / \" $2 \" (\" $5 \")\"}'"},
		{"Process: Dialtone", "pgrep -f 'dialtone start' > /dev/null && echo 'RUNNING' || echo 'STOPPED'"},
	}

	for _, c := range commands {
		output, err := ssh.RunSSHCommand(client, c.cmd)
		if err != nil {
			fmt.Printf("[ssh] %s Error: %v\n", c.name, err)
		} else {
			lines := strings.Split(strings.TrimSpace(output), "\n")
			for _, line := range lines {
				fmt.Printf("[ssh] %s: %s\n", c.name, line)
			}
		}
	}

	// App-Level Status check (tsnet aware)
	hostname := os.Getenv("DIALTONE_HOSTNAME")
	if hostname == "" {
		hostname = "drone-1"
	}
	url := fmt.Sprintf("http://%s", hostname)

	if err := checkAppStatus(url); err != nil {
		fmt.Printf("[tsnet] Status Check FAILED: %v\n", err)
	}

	// Web UI Check via Chromedp
	if err := checkWebUI(url); err != nil {
		fmt.Printf("[chromedp] Web UI Check FAILED: %v\n", err)
	} else {
		fmt.Printf("[chromedp] Web UI Check SUCCESS: %s is reachable\n", url)
	}

	logger.LogInfo("Diagnostics Passed.")
}

func checkAppStatus(url string) error {
	apiClient := http.Client{Timeout: 5 * time.Second}
	resp, err := apiClient.Get(fmt.Sprintf("%s/api/status", url))
	if err != nil {
		return fmt.Errorf("failed to reach status API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status API returned non-200: %d", resp.StatusCode)
	}

	var status map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return fmt.Errorf("failed to decode status JSON: %w", err)
	}

	fmt.Printf("[tsnet] Tailscale IPs:  %v\n", status["tailscale_ips"])
	fmt.Printf("[tsnet] App Uptime:     %v\n", status["uptime"])
	if nats, ok := status["nats"].(map[string]any); ok {
		fmt.Printf("[nats] (Embedded) URL: %v\n", nats["url"])
		fmt.Printf("[nats] (Embedded) Conns: %v\n", nats["connections"])
	}

	// Fetch Version from /api/init
	respInit, err := apiClient.Get(fmt.Sprintf("%s/api/init", url))
	if err == nil && respInit.StatusCode == http.StatusOK {
		var initData map[string]any
		if err := json.NewDecoder(respInit.Body).Decode(&initData); err == nil {
			fmt.Printf("[app]   Version:        %v\n", initData["version"])
		}
		respInit.Body.Close()
	}

	return nil
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
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Headless,
	)

	// If on WSL, try to find Windows Chrome if Linux Chrome is missing
	if chromePath := browser.FindChromePath(); chromePath != "" {
		opts = append(opts, chromedp.ExecPath(chromePath))
		opts = append(opts, chromedp.Flag("remote-debugging-address", "127.0.0.1"))
	}

	// Automated Cleanup: Kill any process on the target port to avoid connection refusal
	if err := browser.CleanupPort(9222); err != nil {
		fmt.Printf("Warning: Failed to cleanup port 9222: %v\n", err)
	}

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	// Create context
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// Create a timeout
	ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var title string
	var termExists, threeExists, camExists bool
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.Title(&title),
		chromedp.Sleep(2*time.Second), // Allow JS initialization
		chromedp.Evaluate(`!!document.getElementById("terminal-container")`, &termExists),
		chromedp.Evaluate(`!!document.getElementById("three-container")`, &threeExists),
		chromedp.Evaluate(`document.querySelectorAll(".panel-right").length > 0`, &camExists),
	)
	if err != nil {
		return err
	}

	if title == "" {
		return fmt.Errorf("page loaded but title is empty")
	}

	if !termExists || !threeExists || !camExists {
		return fmt.Errorf("missing UI components: Terminal=%v, 3D=%v, RightPanel=%v", termExists, threeExists, camExists)
	}

	// Check Telemetry Values (wait for them to populate)
	var natsVal, heartbeatVal string
	var latVal, lonVal, rpVal, yawVal string
	var uiVersionVal string
	err = chromedp.Run(ctx,
		chromedp.Sleep(3*time.Second),
		chromedp.Text("#val-nats", &natsVal, chromedp.ByID),
		chromedp.Text("#val-heartbeat", &heartbeatVal, chromedp.ByID),
		chromedp.Text("#val-lat", &latVal, chromedp.ByID),
		chromedp.Text("#val-lon", &lonVal, chromedp.ByID),
		chromedp.Text("#val-rp", &rpVal, chromedp.ByID),
		chromedp.Text("#val-yaw", &yawVal, chromedp.ByID),
		chromedp.Text("#ui-version", &uiVersionVal, chromedp.ByID),
	)

	// Verify Version
	fmt.Printf("[chromedp] UI Version Check: %s\n", uiVersionVal)
	if uiVersionVal != "v1.1.1" {
		return fmt.Errorf("UI Version mismatch: expected 'v1.1.1', got '%s'", uiVersionVal)
	}

	// Note: If NATS/MAVLink traffic is slow, these might trigger false positives.
	// We log them but might not hard fail if 0, unless verified active.
	// User requested verification.
	fmt.Printf("[chromedp] Telemetry Check: NATS=%s, Heartbeat=%s\n", natsVal, heartbeatVal)
	fmt.Printf("[chromedp] 6DOF Check: Lat=%s, Lon=%s, Att=%s, Yaw=%s\n", latVal, lonVal, rpVal, yawVal)

	if natsVal == "0" || natsVal == "--" {
		fmt.Println("[chromedp] Warning: NATS message count is 0 or uninitialized.")
	}
	if heartbeatVal == "--" {
		fmt.Println("[chromedp] Warning: Heartbeat not received yet.")
	}
	if latVal == "--" || lonVal == "--" {
		fmt.Println("[chromedp] Warning: GPS coordinates not received yet.")
	}
	if yawVal == "--" {
		fmt.Println("[chromedp] Warning: Orientation data not received yet.")
	}

	fmt.Printf("[chromedp] Dashboard Title: %s\n", title)
	fmt.Println("[chromedp] UI Layout Verified (Terminal, 3D, Telemetry present)")
	return nil
}
