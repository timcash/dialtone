package chrome

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"encoding/json"
	"io"
	"net"
	"net/http"

	"dialtone/cli/src/core/browser"
	"dialtone/cli/src/core/logger"
	"github.com/chromedp/chromedp"
)

// VerifyChrome attempts to find and connect to a Chrome/Chromium browser.
func VerifyChrome(port int, debug bool) error {
	path := browser.FindChromePath()
	if path == "" {
		return fmt.Errorf("no Chrome or Chromium browser found in PATH or standard locations for %s", runtime.GOOS)
	}

	if debug {
		logger.LogInfo("DEBUG: System OS: %s", runtime.GOOS)
		logger.LogInfo("DEBUG: Selected Browser Path: %s", path)
	}

	// Automated Cleanup: Kill any process on the target port to avoid connection refusal
	if err := browser.CleanupPort(port); err != nil {
		logger.LogInfo("Warning: Failed to cleanup port %d: %v", port, err)
	}

	logger.LogInfo("Browser check: Found %s", path)

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Headless,
		chromedp.ExecPath(path),
		chromedp.Flag("remote-debugging-port", fmt.Sprintf("%d", port)),
		chromedp.Flag("remote-debugging-address", "127.0.0.1"), // Force IPv4 to avoid [::1] connection issues on WSL
		chromedp.Flag("disable-gpu", true),
	)

	if debug {
		logger.LogInfo("DEBUG: Initializing allocator on port %d...", port)
		// Set output to see browser logs in debug mode
		opts = append(opts, chromedp.CombinedOutput(os.Stderr))
	}

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer func() {
		if debug {
			logger.LogInfo("DEBUG: Shutting down allocator...")
		}
		cancel()
	}()

	logger.LogInfo("Chrome: Starting browser instance...")
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer func() {
		logger.LogInfo("Chrome: Stopping browser...")
		cancel()
	}()

	ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	logger.LogInfo("Chrome: Navigating to about:blank...")
	var title string
	err := chromedp.Run(ctx,
		chromedp.Navigate("about:blank"),
		chromedp.Title(&title),
	)

	if err != nil {
		return fmt.Errorf("failed to run chromedp: %w", err)
	}

	logger.LogInfo("Chrome: Page loaded successfully. Title: '%s'", title)
	return nil
}

// ListResources returns a list of all detected Chrome processes.
func ListResources(showAll bool) ([]browser.ChromeProcess, error) {
	return browser.ListChromeProcesses(showAll)
}

// KillResource kills a Chrome process by PID.
func KillResource(pid int, isWindows bool) error {
	return browser.KillProcessByPID(pid, isWindows)
}

// KillAllResources kills all Chrome processes.
func KillAllResources() error {
	return browser.KillAllChromeProcesses()
}

func KillDialtoneResources() error {
	return browser.KillDialtoneChromeProcesses()
}

// LaunchResult contains the details of a launched browser.
type LaunchResult struct {
	PID          int
	Port         int
	WebsocketURL string
}

// LaunchChrome starts a new headed Chrome instance and returns its debug info.
func LaunchChrome(port int) (*LaunchResult, error) {
	path := browser.FindChromePath()
	if path == "" {
		return nil, fmt.Errorf("chrome not found")
	}

	if port == 0 {
		// Find an available port
		addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, err
		}
		l, err := net.ListenTCP("tcp", addr)
		if err != nil {
			return nil, err
		}
		port = l.Addr().(*net.TCPAddr).Port
		l.Close()
	}

	// Use a local user data dir in the workspace, segregated by port to allow multiple instances
	var userDataDir string
	if runtime.GOOS == "linux" && browser.IsWSL() {
		// Use Windows %TEMP% to avoid "Network Drive" warnings on /mnt/c
		out, err := exec.Command("cmd.exe", "/c", "echo %TEMP%").Output()
		if err == nil {
			winTemp := strings.TrimSpace(string(out))
			// Ensure we use backslashes for the Windows process
			userDataDir = winTemp + "\\" + fmt.Sprintf("dialtone-chrome-port-%d", port)
		}
	}

	if userDataDir == "" {
		cwd, _ := os.Getwd()
		userDataDir = filepath.Join(cwd, ".chrome_data", fmt.Sprintf("instance_%d", port))
		_ = os.MkdirAll(userDataDir, 0755)
	}

	args := []string{
		"--remote-debugging-port=0",
		"--remote-debugging-address=127.0.0.1",
		"--remote-allow-origins=*",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-gpu",
		"--user-data-dir=" + userDataDir,
		"--new-window",
		"--dialtone-origin=true",
	}

	logger.LogInfo("DEBUG: Launching Chrome: %s %v", path, args)
	cmd := exec.Command(path, args...)
	
	// Capture output to a log file for debugging
	logFile, err := os.Create("chrome_launch.log")
	if err == nil {
		defer logFile.Close() // Ensure the log file is closed
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start chrome: %v", err)
	}

	// Wait for the browser to create the DevToolsActivePort file
	// We need the linux path to read it
	linuxUserDataDir := userDataDir
	if runtime.GOOS == "linux" && browser.IsWSL() && strings.Contains(userDataDir, "\\") {
		out, err := exec.Command("wslpath", "-u", userDataDir).Output()
		if err == nil {
			linuxUserDataDir = strings.TrimSpace(string(out))
		}
	}
	activePortFile := filepath.Join(linuxUserDataDir, "DevToolsActivePort")
	
	var wsURL string
	var assignedPort int

	for i := 0; i < 60; i++ {
		time.Sleep(1 * time.Second)
		
		// If on WSL, the file is in winTemp folder which is usually /mnt/c/Users/.../AppData/Local/Temp/...
		// We need to make sure we can read it.
		// If we used a custom winUserDataDir that we know the Linux path for, that's better.
		
		data, err := os.ReadFile(activePortFile)
		if err == nil {
			lines := strings.Split(string(data), "\n")
			if len(lines) >= 2 {
				fmt.Sscanf(lines[0], "%d", &assignedPort)
				// Second line is the browser websocket path part (e.g. /devtools/browser/...)
				wsURL = fmt.Sprintf("ws://127.0.0.1:%d%s", assignedPort, strings.TrimSpace(lines[1]))
				break
			}
		}

		if i%10 == 0 {
			logger.LogInfo("DEBUG: Waiting for Chrome to initialize... (attempt %d/60)", i)
		}

		// Check if process finished already (crashed)
		if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
			return nil, fmt.Errorf("chrome exited prematurely, check chrome_launch.log")
		}
	}

	if wsURL == "" {
		return nil, fmt.Errorf("timed out waiting for DevToolsActivePort file")
	}

	return &LaunchResult{
		PID:          cmd.Process.Pid,
		Port:         assignedPort,
		WebsocketURL: wsURL,
	}, nil
}

func getWebsocketURL(port int) (string, error) {
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", port))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var data struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", err
	}

	return data.WebSocketDebuggerURL, nil
}

