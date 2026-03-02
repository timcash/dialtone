package chrome

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"

	"dialtone/dev/plugins/logs/src_v1/go"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

var devtoolsHTTPClient = &http.Client{Timeout: 900 * time.Millisecond}

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
	Kiosk         bool
	TargetURL     string
	Role          string
	ReuseExisting bool
	UserDataDir   string
	DebugAddress  string
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
	if opts.Headless {
		opts.Kiosk = false
	}
	// Dialtone browser sessions should always run with GPU enabled.
	opts.GPU = true

	if opts.ReuseExisting {
		procs, err := listResourcesWithTimeout(true, 2*time.Second)
		if err == nil {
			for _, p := range procs {
				if p.Origin != "Dialtone" {
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
				enforceSingleDialtoneBrowser(p.PID)
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

	res, err := LaunchChromeWithRoleAndUserDataDir(opts.RequestedPort, opts.GPU, opts.Headless, opts.Kiosk, opts.TargetURL, opts.Role, opts.UserDataDir, opts.DebugAddress)
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

	procs, err := listResourcesWithTimeout(true, 2*time.Second)
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
	if finalPID > 0 {
		enforceSingleDialtoneBrowser(finalPID)
	}

	return &Session{
		PID:          finalPID,
		Port:         res.Port,
		WebSocketURL: res.WebsocketURL,
		IsNew:        true,
		IsWindows:    isWindows,
	}, nil
}

func enforceSingleDialtoneBrowser(keepPID int) {
	if keepPID <= 0 {
		return
	}
	procs, err := listResourcesWithTimeout(true, 2*time.Second)
	if err != nil {
		return
	}
	for _, p := range procs {
		if p.PID <= 0 || p.PID == keepPID {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(p.Origin), "dialtone") {
			continue
		}
		_ = KillResource(p.PID, p.IsWindows)
	}
}

func listResourcesWithTimeout(includeSystem bool, timeout time.Duration) ([]ChromeProcess, error) {
	type result struct {
		procs []ChromeProcess
		err   error
	}
	ch := make(chan result, 1)
	go func() {
		procs, err := ListResources(includeSystem)
		ch <- result{procs: procs, err: err}
	}()
	select {
	case res := <-ch:
		return res.procs, res.err
	case <-time.After(timeout):
		return nil, fmt.Errorf("list resources timed out after %s", timeout)
	}
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
	return LaunchChromeWithRoleAndUserDataDir(port, gpu, headless, false, targetURL, role, "", "")
}

func LaunchChromeWithRoleAndUserDataDir(port int, gpu bool, headless bool, kiosk bool, targetURL, role, requestedUserDataDir, debugAddress string) (*LaunchResult, error) {
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
	if strings.TrimSpace(requestedUserDataDir) != "" {
		userDataDir = strings.TrimSpace(requestedUserDataDir)
	}
	if runtime.GOOS == "linux" && IsWSL() && userDataDir == "" {
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
	}
	// Ensure local path exists when it's a filesystem path (skip obvious windows cmd paths).
	if !strings.Contains(userDataDir, "\\") || strings.HasPrefix(userDataDir, "/") {
		_ = os.MkdirAll(userDataDir, 0755)
	}

	debugAddress = strings.TrimSpace(debugAddress)
	if debugAddress == "" {
		// WSL NAT mode cannot reliably access Windows loopback; bind debug socket on all interfaces.
		if runtime.GOOS == "linux" && IsWSL() {
			debugAddress = "0.0.0.0"
		} else {
			debugAddress = "127.0.0.1"
		}
	}

	args := []string{
		fmt.Sprintf("--remote-debugging-port=%d", port),
		"--remote-debugging-address=" + debugAddress,
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
	}

	if !gpu {
		args = append(args, "--disable-gpu")
	}

	if headless {
		args = append(args, "--headless=new")
	}
	if kiosk && !headless {
		args = append(args, "--kiosk", "--start-fullscreen")
	}

	if targetURL != "" {
		args = append(args, targetURL)
	}

	logs.Info("DEBUG: Launching Chrome: %s %v", path, args)
	cmd := exec.Command(path, args...)

	// Capture output to a log file for debugging
	logPath := "chrome_launch.log"
	logFile, err := os.Create(logPath)
	if err == nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
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
	// Avoid stale port/path data from previous runs on reused profiles.
	_ = os.Remove(activePortFile)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start chrome: %v", err)
	}
	launchStart := time.Now()

	var wsURL string
	var assignedPort int

	for i := 0; i < 120; i++ {
		time.Sleep(250 * time.Millisecond)

		// If on WSL, the file is in winTemp folder which is usually /mnt/c/Users/.../AppData/Local/Temp/...
		// We need to make sure we can read it.
		// If we used a custom winUserDataDir that we know the Linux path for, that's better.

		data, err := os.ReadFile(activePortFile)
		if err == nil {
			if fi, serr := os.Stat(activePortFile); serr == nil && fi.ModTime().Before(launchStart) {
				continue
			}
			lines := strings.Split(string(data), "\n")
			if len(lines) >= 2 {
				fmt.Sscanf(lines[0], "%d", &assignedPort)
				wsPath := strings.TrimSpace(lines[1])
				// Second line is the browser websocket path part (e.g. /devtools/browser/...)
				if assignedPort > 0 {
					ensureWindowsDebugFirewallRule(assignedPort)
					if waitErr := WaitForDebugPort(assignedPort, 2*time.Second); waitErr == nil {
						if resolvedWS, rerr := getWebsocketURL(assignedPort); rerr == nil && strings.TrimSpace(resolvedWS) != "" {
							wsURL = resolvedWS
							if p := portFromWebSocketURL(resolvedWS); p > 0 {
								assignedPort = p
							}
							break
						}
					}
					if runtime.GOOS == "linux" && IsWSL() && wsPath != "" {
						// Keep waiting for a reachable endpoint; unverified WS URLs can cause
						// immediate attach failures in WSL NAT mode.
						continue
					}
				}
			}
		}
		if wsFromLog, portFromLog := readDevToolsURLFromLog(logPath); wsFromLog != "" && portFromLog > 0 {
			assignedPort = portFromLog
			ensureWindowsDebugFirewallRule(assignedPort)
			if waitErr := WaitForDebugPort(assignedPort, 1200*time.Millisecond); waitErr == nil {
				if resolvedWS, rerr := getWebsocketURL(assignedPort); rerr == nil && strings.TrimSpace(resolvedWS) != "" {
					wsURL = resolvedWS
					if p := portFromWebSocketURL(resolvedWS); p > 0 {
						assignedPort = p
					}
					break
				}
			}
			if runtime.GOOS == "linux" && IsWSL() {
				// Keep waiting for a reachable endpoint; unverified WS URLs can cause
				// immediate attach failures in WSL NAT mode.
				continue
			}
		}

		if i%20 == 0 {
			logs.Info("DEBUG: Waiting for Chrome to initialize... (attempt %d/120)", i)
		}

		// Check if process finished already (crashed)
		if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
			return nil, fmt.Errorf("chrome exited prematurely, check chrome_launch.log")
		}
	}

	if wsURL == "" {
		if assignedPort > 0 {
			return nil, fmt.Errorf("timed out waiting for reachable DevTools endpoint on port %d", assignedPort)
		}
		return nil, fmt.Errorf("timed out waiting for DevToolsActivePort file")
	}

	return &LaunchResult{
		PID:          cmd.Process.Pid,
		Port:         assignedPort,
		WebsocketURL: wsURL,
	}, nil
}

func WaitForDebugPort(port int, timeout time.Duration) error {
	if port <= 0 {
		return fmt.Errorf("invalid debug port: %d", port)
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := getWebsocketURL(port); err == nil {
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for debug port %d", port)
}

func AttachToWebSocket(websocketURL string) (context.Context, context.CancelFunc, error) {
	ws := strings.TrimSpace(websocketURL)
	if ws == "" {
		return nil, nil, fmt.Errorf("websocket url is required")
	}
	allocCtx, cancelAlloc := chromedp.NewRemoteAllocator(context.Background(), ws)
	tabCtx, cancelTab := chromedp.NewContext(allocCtx, attachContextOptionsFromWS(ws)...)
	cancel := func() {
		// Keep existing browser tabs alive across CLI calls.
		// Canceling only the allocator detaches the client without closing targets.
		cancelAlloc()
		_ = cancelTab
	}
	return tabCtx, cancel, nil
}

func attachContextOptionsFromWS(ws string) []chromedp.ContextOption {
	id := websocketTargetID(ws)
	if strings.TrimSpace(id) == "" {
		return nil
	}
	return []chromedp.ContextOption{chromedp.WithTargetID(target.ID(id))}
}

func websocketTargetID(ws string) string {
	ws = strings.TrimSpace(ws)
	if ws == "" {
		return ""
	}
	u, err := url.Parse(ws)
	if err != nil {
		return ""
	}
	path := strings.TrimSpace(u.Path)
	if !strings.Contains(path, "/devtools/page/") {
		return ""
	}
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[len(parts)-1])
}

func NewTabContext(parent context.Context) (context.Context, context.CancelFunc) {
	return chromedp.NewContext(parent)
}

func getWebsocketURL(port int) (string, error) {
	var lastErr error
	for _, probePort := range debugProbePorts(port) {
		for _, host := range debugProbeHosts() {
			wsURL, err := getWebsocketURLForHost(host, probePort)
			if err != nil {
				lastErr = err
				continue
			}
			if normalized := normalizeWebSocketURLHost(wsURL, host, probePort); normalized != "" {
				return normalized, nil
			}
			return wsURL, nil
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("unable to resolve websocket url")
	}
	return "", lastErr
}

func debugProbePorts(port int) []int {
	out := make([]int, 0, 2)
	seen := map[int]struct{}{}
	add := func(p int) {
		if p <= 0 {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	add(port)
	if runtime.GOOS == "linux" && IsWSL() {
		// Optional Windows relay port convention (listen=target+10000).
		add(port + 10000)
	}
	return out
}

func portFromWebSocketURL(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	u, err := url.Parse(raw)
	if err != nil {
		return 0
	}
	p, err := strconv.Atoi(strings.TrimSpace(u.Port()))
	if err != nil || p <= 0 {
		return 0
	}
	return p
}

func getWebsocketURLForHost(host string, port int) (string, error) {
	resp, err := devtoolsHTTPClient.Get(fmt.Sprintf("http://%s:%d/json/version", host, port))
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
	return strings.TrimSpace(data.WebSocketDebuggerURL), nil
}

func debugProbeHosts() []string {
	seen := map[string]struct{}{}
	add := func(dst *[]string, v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		if _, ok := seen[v]; ok {
			return
		}
		seen[v] = struct{}{}
		*dst = append(*dst, v)
	}
	hosts := make([]string, 0, 4)
	add(&hosts, os.Getenv("DIALTONE_CHROME_DEBUG_HOST"))
	if runtime.GOOS == "linux" && IsWSL() {
		// In WSL NAT mode, Chrome usually runs on Windows and is reachable via host gateway.
		add(&hosts, detectWSLHostGatewayIP())
		// Some launches expose DevTools only on local loopback; keep localhost as fallback.
		add(&hosts, "127.0.0.1")
	} else {
		add(&hosts, "127.0.0.1")
	}
	return hosts
}

func detectWSLHostGatewayIP() string {
	if out, err := exec.Command("sh", "-lc", "ip route | awk '/^default / {print $3; exit}'").Output(); err == nil {
		if ip := strings.TrimSpace(string(out)); ip != "" && ip != "100.100.100.100" {
			return ip
		}
	}
	raw, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "nameserver ") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		ip := strings.TrimSpace(parts[1])
		if ip == "100.100.100.100" {
			continue
		}
		return ip
	}
	return ""
}

func normalizeWebSocketURLHost(raw, host string, port int) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	u.Scheme = "ws"
	if strings.TrimSpace(host) != "" && port > 0 {
		u.Host = fmt.Sprintf("%s:%d", host, port)
	}
	return u.String()
}

func ensureWindowsDebugFirewallRule(port int) {
	if port <= 0 || runtime.GOOS != "linux" || !IsWSL() {
		return
	}
	ps := fmt.Sprintf(`$ErrorActionPreference='SilentlyContinue'
$rule=("Dialtone Chrome DevTools "+%d)
if(-not (Get-NetFirewallRule -DisplayName $rule -ErrorAction SilentlyContinue)){
  New-NetFirewallRule -DisplayName $rule -Direction Inbound -Action Allow -Protocol TCP -LocalPort %d -Profile Any | Out-Null
}
`, port, port)
	powershell := "powershell.exe"
	if _, err := exec.LookPath(powershell); err != nil {
		powershell = "/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe"
	}
	_ = exec.Command(powershell, "-NoProfile", "-NonInteractive", "-Command", ps).Run()
}

func readDevToolsURLFromLog(logPath string) (string, int) {
	if strings.TrimSpace(logPath) == "" {
		return "", 0
	}
	raw, err := os.ReadFile(logPath)
	if err != nil {
		return "", 0
	}
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		const marker = "DevTools listening on "
		idx := strings.Index(line, marker)
		if idx < 0 {
			continue
		}
		ws := strings.TrimSpace(line[idx+len(marker):])
		if !strings.HasPrefix(strings.ToLower(ws), "ws://") {
			continue
		}
		u, err := url.Parse(ws)
		if err != nil {
			continue
		}
		p, err := strconv.Atoi(strings.TrimSpace(u.Port()))
		if err != nil || p <= 0 {
			continue
		}
		return ws, p
	}
	return "", 0
}
