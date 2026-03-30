package chrome

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	chromev3 "dialtone/dev/plugins/chrome/src_v3"
)

const defaultCompatRole = "chrome-v1-service"

var (
	roleFlagRE = regexp.MustCompile(`--dialtone-role=([^"'[:space:]]+)`)
	portFlagRE = regexp.MustCompile(`--remote-debugging-port=([0-9]+)`)
)

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
	PID          int    `json:"pid"`
	Port         int    `json:"port"`
	DebugPort    int    `json:"debug_port"`
	WebSocketURL string `json:"websocket_url,omitempty"`
	WebsocketURL string `json:"WebsocketURL,omitempty"`
	IsNew        bool   `json:"is_new,omitempty"`
	UserDataDir  string `json:"user_data_dir,omitempty"`
}

type SessionMetadata struct {
	PID          int    `json:"pid"`
	DebugPort    int    `json:"debug_port"`
	WebSocketURL string `json:"websocket_url,omitempty"`
}

type Resource struct {
	PID        int
	Role       string
	Origin     string
	DebugPort  int
	IsHeadless bool
	IsWindows  bool
}

type windowsProcess struct {
	ProcessID   int    `json:"ProcessId"`
	Name        string `json:"Name"`
	CommandLine string `json:"CommandLine"`
}

func StartSession(opts SessionOptions) (*Session, error) {
	if opts.ReuseExisting {
		if existing, err := findReusableSession(opts); err == nil && existing != nil {
			return existing, nil
		}
	}
	return launchSession(opts)
}

func LaunchChrome(requestedPort int, gpu bool, headless bool, targetURL string) (*Session, error) {
	return StartSession(SessionOptions{
		RequestedPort: requestedPort,
		GPU:           gpu,
		Headless:      headless,
		TargetURL:     targetURL,
	})
}

func CleanupSession(session *Session) error {
	if session == nil {
		return nil
	}
	if session.PID > 0 {
		return killPID(session.PID)
	}
	if session.DebugPort > 0 {
		return CleanupPort(session.DebugPort)
	}
	if session.Port > 0 {
		return CleanupPort(session.Port)
	}
	return nil
}

func BuildSessionMetadata(session *Session) *SessionMetadata {
	if session == nil {
		return nil
	}
	debugPort := session.DebugPort
	if debugPort == 0 {
		debugPort = session.Port
	}
	webSocketURL := strings.TrimSpace(session.WebSocketURL)
	if webSocketURL == "" {
		webSocketURL = strings.TrimSpace(session.WebsocketURL)
	}
	return &SessionMetadata{
		PID:          session.PID,
		DebugPort:    debugPort,
		WebSocketURL: webSocketURL,
	}
}

func WriteSessionMetadata(path string, session *Session) error {
	meta := BuildSessionMetadata(session)
	if meta == nil {
		return fmt.Errorf("session metadata unavailable")
	}
	raw, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func HasReachableDevtoolsWebSocket(port int) bool {
	if port <= 0 {
		return false
	}
	_, err := webSocketURLForPort(port)
	return err == nil
}

func ListResources(includeChrome bool) ([]Resource, error) {
	if !includeChrome {
		return nil, nil
	}
	switch runtime.GOOS {
	case "windows":
		return listWindowsResources()
	default:
		return listPOSIXResources()
	}
}

func KillDialtoneResources() error {
	firstErr := chromev3.KillDialtoneResources()
	resources, err := ListResources(true)
	if err != nil {
		if firstErr != nil {
			return firstErr
		}
		return err
	}
	for _, resource := range resources {
		if killErr := killPID(resource.PID); killErr != nil && firstErr == nil {
			firstErr = killErr
		}
	}
	return firstErr
}

func CleanupPort(port int) error {
	if port <= 0 {
		return nil
	}
	pids, err := pidsListeningOnPort(port)
	if err != nil {
		return err
	}
	var firstErr error
	for _, pid := range pids {
		if killErr := killPID(pid); killErr != nil && firstErr == nil {
			firstErr = killErr
		}
	}
	return firstErr
}

func findReusableSession(opts SessionOptions) (*Session, error) {
	resources, err := ListResources(true)
	if err != nil {
		return nil, err
	}
	role := normalizeRole(opts.Role)
	for _, resource := range resources {
		if resource.Origin != "Dialtone" {
			continue
		}
		if normalizeRole(resource.Role) != role {
			continue
		}
		if opts.RequestedPort > 0 && resource.DebugPort != opts.RequestedPort {
			continue
		}
		if resource.IsHeadless != opts.Headless {
			continue
		}
		if resource.DebugPort <= 0 {
			continue
		}
		wsURL, err := webSocketURLForPort(resource.DebugPort)
		if err != nil {
			continue
		}
		return &Session{
			PID:          resource.PID,
			Port:         resource.DebugPort,
			DebugPort:    resource.DebugPort,
			WebSocketURL: wsURL,
			WebsocketURL: wsURL,
			IsNew:        false,
		}, nil
	}
	return nil, nil
}

func launchSession(opts SessionOptions) (*Session, error) {
	port := opts.RequestedPort
	if port == 0 {
		var err error
		port, err = findFreePort()
		if err != nil {
			return nil, fmt.Errorf("allocate chrome debug port: %w", err)
		}
	}
	chromePath, err := findChromePath()
	if err != nil {
		return nil, err
	}
	role := normalizeRole(opts.Role)
	debugAddress := strings.TrimSpace(opts.DebugAddress)
	if debugAddress == "" {
		debugAddress = "127.0.0.1"
	}
	targetURL := strings.TrimSpace(opts.TargetURL)
	if targetURL == "" {
		targetURL = "about:blank"
	}
	userDataDir := strings.TrimSpace(opts.UserDataDir)
	if userDataDir == "" {
		userDataDir = defaultUserDataDir(role, port)
	}
	if err := os.MkdirAll(userDataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create chrome profile dir: %w", err)
	}

	args := []string{
		"--remote-debugging-port=" + strconv.Itoa(port),
		"--remote-debugging-address=" + debugAddress,
		"--remote-allow-origins=*",
		"--user-data-dir=" + userDataDir,
		"--no-first-run",
		"--no-default-browser-check",
		"--new-window",
		"--dialtone-origin=true",
		"--dialtone-role=" + role,
	}
	if opts.Headless {
		args = append(args, "--headless=new")
	}
	if !opts.GPU {
		args = append(args, "--disable-gpu")
	}
	if opts.Kiosk {
		args = append(args, "--kiosk")
	}
	args = append(args, targetURL)

	cmd := exec.Command(chromePath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start chrome: %w", err)
	}
	go func() {
		_ = cmd.Wait()
	}()

	wsURL, err := waitForWebSocketURL(port, 20*time.Second)
	if err != nil {
		_ = killPID(cmd.Process.Pid)
		return nil, fmt.Errorf("wait for devtools: %w", err)
	}

	return &Session{
		PID:          cmd.Process.Pid,
		Port:         port,
		DebugPort:    port,
		WebSocketURL: wsURL,
		WebsocketURL: wsURL,
		IsNew:        true,
		UserDataDir:  userDataDir,
	}, nil
}

func waitForWebSocketURL(port int, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		wsURL, err := webSocketURLForPort(port)
		if err == nil && strings.TrimSpace(wsURL) != "" {
			return wsURL, nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return "", fmt.Errorf("timed out waiting for chrome devtools on port %d", port)
}

func webSocketURLForPort(port int) (string, error) {
	client := &http.Client{Timeout: 800 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", port)) //nolint:gosec
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected devtools status %d", resp.StatusCode)
	}
	var payload struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.WebSocketDebuggerURL) == "" {
		return "", fmt.Errorf("chrome devtools websocket unavailable on port %d", port)
	}
	return strings.TrimSpace(payload.WebSocketDebuggerURL), nil
}

func findChromePath() (string, error) {
	candidates := []string{}
	switch runtime.GOOS {
	case "darwin":
		candidates = append(candidates,
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		)
	case "windows":
		candidates = append(candidates,
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
		)
	default:
		candidates = append(candidates, "google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "microsoft-edge")
	}
	for _, candidate := range candidates {
		if filepath.IsAbs(candidate) {
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
			continue
		}
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("chrome executable not found")
}

func findFreePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

func defaultUserDataDir(role string, port int) string {
	repoRoot := findRepoRoot()
	if repoRoot == "" {
		return filepath.Join(os.TempDir(), "dialtone", "chrome", sanitizeRoleForDir(role), strconv.Itoa(port))
	}
	chromeRoot := filepath.Join(repoRoot, ".chrome_data")
	dirName := fmt.Sprintf("dialtone-chrome-%s-port-%d", sanitizeRoleForDir(role), port)
	if normalizeRole(role) == defaultCompatRole {
		dirName = fmt.Sprintf("dialtone-chrome-port-%d", port)
	}
	return filepath.Join(chromeRoot, dirName)
}

func findRepoRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(cwd, "dialtone.sh")); err == nil {
			return cwd
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			return ""
		}
		cwd = parent
	}
}

func sanitizeRoleForDir(role string) string {
	role = normalizeRole(role)
	role = strings.ToLower(strings.TrimSpace(role))
	var b strings.Builder
	for _, r := range role {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	if out == "" {
		return "default"
	}
	return out
}

func normalizeRole(role string) string {
	role = strings.TrimSpace(role)
	if role == "" {
		return defaultCompatRole
	}
	return role
}

func listPOSIXResources() ([]Resource, error) {
	out, err := exec.Command("ps", "-eo", "pid=,args=").Output()
	if err != nil {
		return nil, err
	}
	resources := make([]Resource, 0)
	for _, line := range strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil || pid <= 0 {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(line, fields[0]))
		if !looksLikeDialtoneBrowserCommand(rest) {
			continue
		}
		resources = append(resources, buildResource(pid, rest, false))
	}
	sort.Slice(resources, func(i, j int) bool {
		if resources[i].Role != resources[j].Role {
			return resources[i].Role < resources[j].Role
		}
		return resources[i].PID < resources[j].PID
	})
	return resources, nil
}

func listWindowsResources() ([]Resource, error) {
	script := strings.Join([]string{
		"$items = Get-CimInstance Win32_Process | Where-Object { $_.CommandLine -like '*--dialtone-role=*' -and $_.CommandLine -notlike '*--type=*' -and ($_.Name -eq 'chrome.exe' -or $_.Name -eq 'msedge.exe' -or $_.Name -like 'chromium*.exe') } | Select-Object ProcessId, Name, CommandLine",
		"if ($null -eq $items) { '[]' } else { $items | ConvertTo-Json -Compress }",
	}, "; ")
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script).Output()
	if err != nil {
		return nil, err
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil, nil
	}
	var many []windowsProcess
	if err := json.Unmarshal(out, &many); err == nil {
		return buildWindowsResources(many), nil
	}
	var one windowsProcess
	if err := json.Unmarshal(out, &one); err == nil && one.ProcessID > 0 {
		return buildWindowsResources([]windowsProcess{one}), nil
	}
	return nil, fmt.Errorf("parse windows chrome resources")
}

func buildWindowsResources(items []windowsProcess) []Resource {
	resources := make([]Resource, 0, len(items))
	for _, item := range items {
		if item.ProcessID <= 0 {
			continue
		}
		if !looksLikeDialtoneBrowserCommand(item.CommandLine) {
			continue
		}
		resources = append(resources, buildResource(item.ProcessID, item.CommandLine, true))
	}
	sort.Slice(resources, func(i, j int) bool {
		if resources[i].Role != resources[j].Role {
			return resources[i].Role < resources[j].Role
		}
		return resources[i].PID < resources[j].PID
	})
	return resources
}

func buildResource(pid int, commandLine string, isWindows bool) Resource {
	return Resource{
		PID:        pid,
		Role:       normalizeRole(matchFirst(roleFlagRE, commandLine)),
		Origin:     "Dialtone",
		DebugPort:  atoiDefault(matchFirst(portFlagRE, commandLine)),
		IsHeadless: strings.Contains(strings.ToLower(commandLine), "--headless"),
		IsWindows:  isWindows,
	}
}

func looksLikeDialtoneBrowserCommand(commandLine string) bool {
	commandLine = strings.TrimSpace(commandLine)
	if commandLine == "" {
		return false
	}
	lower := strings.ToLower(commandLine)
	if !strings.Contains(lower, "--dialtone-role=") {
		return false
	}
	if strings.Contains(lower, "--type=") {
		return false
	}
	for _, needle := range []string{"chrome", "chromium", "msedge"} {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

func matchFirst(re *regexp.Regexp, text string) string {
	matches := re.FindStringSubmatch(text)
	if len(matches) < 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
}

func atoiDefault(raw string) int {
	v, _ := strconv.Atoi(strings.TrimSpace(raw))
	return v
}

func pidsListeningOnPort(port int) ([]int, error) {
	switch runtime.GOOS {
	case "windows":
		script := fmt.Sprintf("Get-NetTCPConnection -State Listen -LocalPort %d -ErrorAction SilentlyContinue | Select-Object -ExpandProperty OwningProcess -Unique", port)
		out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script).Output()
		if err != nil {
			return nil, err
		}
		return parsePIDList(string(out)), nil
	default:
		script := fmt.Sprintf(`
if command -v lsof >/dev/null 2>&1; then
  lsof -tiTCP:%d -sTCP:LISTEN 2>/dev/null || true
elif command -v fuser >/dev/null 2>&1; then
  fuser %d/tcp 2>/dev/null || true
elif command -v ss >/dev/null 2>&1; then
  ss -ltnp 2>/dev/null | grep -E ':%d\b' | sed -nE 's/.*pid=([0-9]+).*/\1/p' || true
fi
`, port, port, port)
		out, err := exec.Command("bash", "-lc", script).Output()
		if err != nil {
			return nil, err
		}
		return parsePIDList(string(out)), nil
	}
}

func parsePIDList(raw string) []int {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r < '0' || r > '9'
	})
	seen := map[int]struct{}{}
	out := make([]int, 0, len(fields))
	for _, field := range fields {
		pid, err := strconv.Atoi(strings.TrimSpace(field))
		if err != nil || pid <= 0 {
			continue
		}
		if _, ok := seen[pid]; ok {
			continue
		}
		seen[pid] = struct{}{}
		out = append(out, pid)
	}
	sort.Ints(out)
	return out
}

func killPID(pid int) error {
	if pid <= 0 {
		return nil
	}
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid))
		out, err := cmd.CombinedOutput()
		if err != nil {
			msg := strings.ToLower(strings.TrimSpace(string(out)))
			if strings.Contains(msg, "not found") || strings.Contains(msg, "no running instance") {
				return nil
			}
			return fmt.Errorf("taskkill pid %d: %w (%s)", pid, err, strings.TrimSpace(string(out)))
		}
		return nil
	default:
		termCmd := exec.Command("kill", "-TERM", strconv.Itoa(pid))
		if out, err := termCmd.CombinedOutput(); err != nil {
			msg := strings.ToLower(strings.TrimSpace(string(out)))
			if strings.Contains(msg, "no such process") {
				return nil
			}
			return fmt.Errorf("kill pid %d: %w (%s)", pid, err, strings.TrimSpace(string(out)))
		}
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if !processExists(pid) {
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}
		killCmd := exec.Command("kill", "-KILL", strconv.Itoa(pid))
		out, err := killCmd.CombinedOutput()
		if err != nil {
			msg := strings.ToLower(strings.TrimSpace(string(out)))
			if strings.Contains(msg, "no such process") {
				return nil
			}
			return fmt.Errorf("kill -9 pid %d: %w (%s)", pid, err, strings.TrimSpace(string(out)))
		}
		return nil
	}
}

func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	switch runtime.GOOS {
	case "windows":
		out, err := exec.Command("tasklist", "/FI", "PID eq "+strconv.Itoa(pid)).CombinedOutput()
		if err != nil {
			return false
		}
		return strings.Contains(string(out), strconv.Itoa(pid))
	default:
		err := exec.Command("kill", "-0", strconv.Itoa(pid)).Run()
		return err == nil
	}
}
