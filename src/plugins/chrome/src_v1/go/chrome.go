package chrome

import (
	"context"
	"errors"
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

	"dialtone/dev/plugins/logs/src_v1/go"
	"github.com/chromedp/chromedp"
)

// VerifyChrome attempts to find and connect to a Chrome/Chromium 
func VerifyChrome(port int, debug bool) error {
	path := FindChromePath()
	if path == "" {
		return fmt.Errorf("no Chrome or Chromium browser found in PATH or standard locations for %s", runtime.GOOS)
	}

	if debug {
		logs.Info("DEBUG: System OS: %s", runtime.GOOS)
		logs.Info("DEBUG: Selected Browser Path: %s", path)
	}

	// Automated Cleanup: Kill any process on the target port to avoid connection refusal
	if err := CleanupPort(port); err != nil {
		logs.Info("Warning: Failed to cleanup port %d: %v", port, err)
	}

	logs.Info("Browser check: Found %s", path)

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Headless,
		chromedp.ExecPath(path),
		chromedp.Flag("remote-debugging-port", fmt.Sprintf("%d", port)),
		chromedp.Flag("remote-debugging-address", "127.0.0.1"), // Force IPv4 to avoid [::1] connection issues on WSL
		chromedp.Flag("dialtone-origin", true),
		chromedp.Flag("disable-gpu", true),
	)

	if debug {
		logs.Info("DEBUG: Initializing allocator on port %d...", port)
		// Set output to see browser logs in debug mode
		opts = append(opts, chromedp.CombinedOutput(os.Stderr))
	}

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer func() {
		if debug {
			logs.Info("DEBUG: Shutting down allocator...")
		}
		cancel()
	}()

	logs.Info("Chrome: Starting browser instance...")
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer func() {
		logs.Info("Chrome: Stopping ..")
		cancel()
	}()

	ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	logs.Info("Chrome: Navigating to about:blank...")
	var title string
	err := chromedp.Run(ctx,
		chromedp.Navigate("about:blank"),
		chromedp.Title(&title),
	)

	if err != nil {
		return fmt.Errorf("failed to run chromedp: %w", err)
	}

	logs.Info("Chrome: Page loaded successfully. Title: '%s'", title)
	return nil
}

// ListResources returns a list of all detected Chrome processes.
func ListResources(showAll bool) ([]ChromeProcess, error) {
	return ListChromeProcesses(showAll)
}

// KillResource kills a Chrome process by PID.
func KillResource(pid int, isWindows bool) error {
	return KillProcessByPID(pid, isWindows)
}

// KillAllResources kills all Chrome processes.
func KillAllResources() error {
	return KillAllChromeProcesses()
}

func KillDialtoneResources() error {
	return KillDialtoneChromeProcesses()
}

// LaunchResult contains the details of a launched 
type LaunchResult struct {
	PID          int
	Port         int
	WebsocketURL string
}

type SessionOptions struct {
	RequestedPort int
	GPU           bool
	Headless      bool
	TargetURL     string
	Role          string
	ReuseExisting bool
}

type Session struct {
	PID          int
	Port         int
	WebSocketURL string
	IsNew        bool
	IsWindows    bool
}

func StartSession(opts SessionOptions) (*Session, error) {
	if opts.Role == "" {
		opts.Role = "default"
	}

	if opts.ReuseExisting {
		procs, err := ListResources(true)
		if err == nil {
			for _, p := range procs {
				if p.Origin != "Dialtone" || p.Role != opts.Role {
					continue
				}
				if p.IsHeadless != opts.Headless {
					continue
				}
				wsURL := ""
				port := p.DebugPort
				if p.DebugPort > 0 {
					if resolvedWS, err := getWebsocketURL(p.DebugPort); err == nil {
						wsURL = resolvedWS
					}
				}
				if wsURL == "" {
					resolvedWS, resolvedPort, err := getWebsocketFromProcessUserDataDir(p.Command)
					if err == nil {
						wsURL = resolvedWS
						port = resolvedPort
					}
				}
				if wsURL == "" {
					continue
				}
				return &Session{
					PID:          p.PID,
					Port:         port,
					WebSocketURL: wsURL,
					IsNew:        false,
					IsWindows:    p.IsWindows,
				}, nil
			}
		}
	}

	res, err := LaunchChromeWithRole(opts.RequestedPort, opts.GPU, opts.Headless, opts.TargetURL, opts.Role)
	if err != nil {
		return nil, err
	}

	// Important for WSL: res.PID is the Linux-side shim PID.
	// We need the Windows PID to correctly kill the process tree later.
	finalPID := res.PID
	isWindows := false
	
	// Wait a moment for Windows process listing to reflect the new instance
	if runtime.GOOS == "linux" && IsWSL() {
		time.Sleep(3 * time.Second)
	}

	procs, err := ListResources(true)
	if err == nil {
		for _, p := range procs {
			// In WSL, we match the Windows process by its debug port or role
			// Match by role if port detection failed or is slow
			if p.IsWindows && p.Role == opts.Role && (res.Port == 0 || p.DebugPort == res.Port || p.DebugPort == 0) {
				finalPID = p.PID
				isWindows = true
				break
			}
			// Fallback: match by shim PID check (sometimes they are adjacent or related)
			if p.PID == res.PID {
				isWindows = p.IsWindows
				break
			}
		}
	}

	return &Session{
		PID:          finalPID,
		Port:         res.Port,
		WebSocketURL: res.WebsocketURL,
		IsNew:        true,
		IsWindows:    isWindows,
	}, nil
}

func CleanupSession(session *Session) error {
	if session == nil || !session.IsNew || session.PID <= 0 {
		return nil
	}
	return KillResource(session.PID, session.IsWindows)
}

func getWebsocketFromProcessUserDataDir(cmdline string) (string, int, error) {
	userDataDir, err := extractUserDataDir(cmdline)
	if err != nil {
		return "", 0, err
	}
	activePath := filepath.Join(userDataDir, "DevToolsActivePort")
	data, err := os.ReadFile(activePath)
	if err != nil && runtime.GOOS == "linux" && IsWSL() && strings.Contains(userDataDir, "\\") {
		out, convErr := exec.Command("wslpath", "-u", userDataDir).Output()
		if convErr == nil {
			activePath = filepath.Join(strings.TrimSpace(string(out)), "DevToolsActivePort")
			data, err = os.ReadFile(activePath)
		}
	}
	if err != nil {
		return "", 0, err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 2 {
		return "", 0, fmt.Errorf("invalid DevToolsActivePort contents")
	}
	var port int
	if _, err := fmt.Sscanf(lines[0], "%d", &port); err != nil {
		return "", 0, err
	}
	wsURL := fmt.Sprintf("ws://127.0.0.1:%d%s", port, strings.TrimSpace(lines[1]))
	return wsURL, port, nil
}

func extractUserDataDir(cmdline string) (string, error) {
	const prefix = "--user-data-dir="
	i := strings.Index(cmdline, prefix)
	if i < 0 {
		return "", errors.New("user-data-dir flag not found")
	}
	value := cmdline[i+len(prefix):]
	if value == "" {
		return "", errors.New("empty user-data-dir value")
	}
	// Handle quoted value: --user-data-dir=\"...\" and plain value up to next space.
	if value[0] == '"' {
		value = value[1:]
		j := strings.Index(value, "\"")
		if j < 0 {
			return "", errors.New("unterminated quoted user-data-dir value")
		}
		return value[:j], nil
	}
	parts := strings.Fields(value)
	if len(parts) == 0 {
		return "", errors.New("invalid user-data-dir value")
	}
	return parts[0], nil
}

// LaunchChrome starts a new Chrome instance and returns its debug info.
func LaunchChrome(port int, gpu bool, headless bool, targetURL string) (*LaunchResult, error) {
	return LaunchChromeWithRole(port, gpu, headless, targetURL, "")
}

// LaunchChromeWithRole starts a new Chrome instance and tags it with a dialtone role (e.g. "dev", "smoke").
func LaunchChromeWithRole(port int, gpu bool, headless bool, targetURL, role string) (*LaunchResult, error) {
	path := FindChromePath()
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

	rolePart := role
	if rolePart == "" {
		rolePart = "default"
	}
	profileDirName := fmt.Sprintf("dialtone-chrome-%s-port-%d", rolePart, port)
	if rolePart == "dev" {
		// Keep a stable profile for persistent logins/cookies in dev mode.
		profileDirName = "dialtone-chrome-dev-profile"
	}

	// Use a local user data dir in the workspace, segregated by port to allow multiple instances
	var userDataDir string
	if runtime.GOOS == "linux" && IsWSL() {
		// Detect if we are in an SSH session where cmd.exe might have issues with UNC paths
		// We MUST use a local Windows drive (e.g. C:\) for Chrome profiles.
		cmdPath := "cmd.exe"
		if _, err := exec.LookPath(cmdPath); err != nil {
			// Fallback for SSH environments where C:\Windows\System32 might not be in PATH
			if _, err := os.Stat("/mnt/c/Windows/System32/cmd.exe"); err == nil {
				cmdPath = "/mnt/c/Windows/System32/cmd.exe"
			}
		}

		out, err := exec.Command(cmdPath, "/c", "echo %TEMP%").Output()
		if err == nil {
			// Handle potential UNC path warnings and SSH noise in output
			// We look for a line that specifically starts with a drive letter (e.g. C:\)
			lines := strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n")
			winTemp := ""
			for _, l := range lines {
				l = strings.TrimSpace(l)
				if len(l) >= 3 && l[1] == ':' && l[2] == '\\' {
					winTemp = l
					break
				}
			}

			if winTemp != "" {
				userDataDir = winTemp + "\\" + profileDirName
			}
		}

		if userDataDir == "" {
			cwd, _ := os.Getwd()
			out, err := exec.Command("wslpath", "-w", cwd).Output()
			if err == nil {
				winCwd := strings.TrimSpace(string(out))
				// Only use it if it's NOT a UNC path (unlikely to work for profiles but better than nothing)
				if !strings.HasPrefix(winCwd, "\\\\") {
					userDataDir = winCwd + "\\" + ".chrome_data" + "\\" + profileDirName
				}
			}
		}
	}

	if userDataDir == "" {
		cwd, _ := os.Getwd()
		userDataDir = filepath.Join(cwd, ".chrome_data", profileDirName)
		_ = os.MkdirAll(userDataDir, 0755)
	}

	args := []string{
		"--remote-debugging-port=0",
		"--remote-debugging-address=127.0.0.1",
		"--remote-allow-origins=*",
		"--no-first-run",
		"--no-default-browser-check",
		"--user-data-dir=" + userDataDir,
		"--new-window",
		"--dialtone-origin=true",
	}
	if role != "" {
		args = append(args, "--dialtone-role="+role)
	}
	if role == "dev" && !headless {
		if os.Getenv("DIALTONE_DEVTOOLS_AUTO_OPEN") == "1" {
			args = append(args, "--auto-open-devtools-for-tabs")
		}
		if targetURL != "" {
			args = append(args, "--app="+targetURL)
			targetURL = ""
		}
	}

	if !gpu {
		args = append(args, "--disable-gpu")
	}

	if headless {
		args = append(args, "--headless=new")
	}

	if targetURL != "" {
		args = append(args, targetURL)
	}

	logs.Info("DEBUG: Launching Chrome: %s %v", path, args)
	cmd := exec.Command(path, args...)

	// Capture output to a log file for debugging
	logFile, err := os.Create("chrome_launch.log")
	if err == nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start chrome: %v", err)
	}

	// Wait for the browser to create the DevToolsActivePort file
	// We need the linux path to read it
	linuxUserDataDir := userDataDir
	if runtime.GOOS == "linux" && IsWSL() && strings.Contains(userDataDir, "\\") {
		out, err := exec.Command("wslpath", "-u", userDataDir).Output()
		if err == nil {
			linuxUserDataDir = strings.TrimSpace(string(out))
		}
	}
	activePortFile := filepath.Join(linuxUserDataDir, "DevToolsActivePort")

	var wsURL string
	var assignedPort int

	for i := 0; i < 60; i++ {
		time.Sleep(300 * time.Millisecond)

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

		if i%20 == 0 {
			logs.Info("DEBUG: Waiting for Chrome to initialize... (attempt %d/60)", i)
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
