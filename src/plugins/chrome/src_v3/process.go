package src_v3

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
)

const windowsProcessQueryTimeout = 2 * time.Second

func findChromePath() (string, error) {
	if runtime.GOOS == "windows" {
		candidates := []string{
			filepath.Join(os.Getenv("ProgramFiles"), "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(os.Getenv("ProgramFiles(x86)"), "Google", "Chrome", "Application", "chrome.exe"),
		}
		for _, candidate := range candidates {
			if strings.TrimSpace(candidate) != "" {
				if _, err := os.Stat(candidate); err == nil {
					return candidate, nil
				}
			}
		}
		return "", fmt.Errorf("chrome.exe not found")
	}
	if runtime.GOOS == "darwin" {
		candidates := []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		}
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}
	}
	if p, err := exec.LookPath("google-chrome"); err == nil {
		return p, nil
	}
	if p, err := exec.LookPath("chromium"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("chrome not found in PATH")
}

func waitForWebSocket(port int, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 1200 * time.Millisecond}
	for time.Now().Before(deadline) {
		resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", port))
		if err == nil {
			var payload struct {
				WS string `json:"webSocketDebuggerUrl"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&payload); err == nil {
				_ = resp.Body.Close()
				if strings.TrimSpace(payload.WS) != "" {
					return strings.TrimSpace(payload.WS), nil
				}
			}
			_ = resp.Body.Close()
		}
		time.Sleep(250 * time.Millisecond)
	}
	return "", fmt.Errorf("timed out waiting for chrome debug websocket on port %d", port)
}

func detectBrowserPID(port int, role, profileDir string) (int, error) {
	switch runtime.GOOS {
	case "windows":
		for i := 0; i < 5; i++ {
			script := fmt.Sprintf(`$port=%d; `+
				`$listener=$null; `+
				`try { $listener=Get-NetTCPConnection -LocalAddress '127.0.0.1' -LocalPort $port -State Listen -ErrorAction Stop | Select-Object -First 1 -ExpandProperty OwningProcess } catch {}; `+
				`if(-not $listener){ try { $listener=Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction Stop | Select-Object -First 1 -ExpandProperty OwningProcess } catch {} }; `+
				`if($listener){ Write-Output $listener; exit 0 }; `+
				`$role=%s; $profile=%s; `+
				`$procs=Get-CimInstance Win32_Process | Where-Object { $_.Name -eq 'chrome.exe' -and $_.CommandLine -like ('*--remote-debugging-port=' + $port + '*') -and ($_.CommandLine -like ('*--dialtone-role=' + $role + '*') -or $_.CommandLine -like ('*' + $profile + '*')) } | Select-Object -First 1 -ExpandProperty ProcessId; `+
				`if($procs){ Write-Output $procs }`, port, psQuote(role), psQuote(windowsPath(profileDir)))
			out, err := runWindowsPowerShell(script, windowsProcessQueryTimeout)
			if err == nil {
				if n, convErr := strconv.Atoi(strings.TrimSpace(string(out))); convErr == nil && n > 0 {
					return n, nil
				}
			}
			time.Sleep(500 * time.Millisecond)
		}
		return 0, fmt.Errorf("chrome pid not found for port %d", port)
	default:
		out, err := exec.Command("bash", "-lc", fmt.Sprintf("ps -eo pid,args | grep '[c]hrome' | grep -- '--remote-debugging-port=%d' | grep -- '--dialtone-role=%s' | head -n1 | awk '{print $1}'", port, shellEscapeGrep(role))).Output()
		if err != nil {
			return 0, err
		}
		n, err := strconv.Atoi(strings.TrimSpace(string(out)))
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("chrome pid not found")
		}
		return n, nil
	}
}

func countLocalChromeProcesses(role string) (int, error) {
	role = normalizeRole(role)
	profileDir := defaultProfileDir(role)
	pids, err := chromeBrowserPIDsForRole(role, profileDir, roleChromePort(role))
	if err != nil {
		return 0, err
	}
	return len(pids), nil
}

func countLocalChromeProcessesQuick(role string, timeout time.Duration) (int, error) {
	if timeout <= 0 {
		timeout = 400 * time.Millisecond
	}
	type result struct {
		count int
		err   error
	}
	done := make(chan result, 1)
	go func() {
		count, err := countLocalChromeProcesses(role)
		done <- result{count: count, err: err}
	}()
	select {
	case res := <-done:
		return res.count, res.err
	case <-time.After(timeout):
		return 0, fmt.Errorf("local chrome process count timed out after %v", timeout)
	}
}

func countRemoteChromeProcesses(node sshv1.MeshNode, role string) (int, error) {
	role = normalizeRole(role)
	profileDir := defaultProfileDir(role)
	if strings.EqualFold(node.OS, "windows") {
		script := fmt.Sprintf(`$role=%s; $profile=%s; $port=%d; `+
			`$items=Get-CimInstance Win32_Process | Where-Object { $_.Name -eq 'chrome.exe' -and $_.CommandLine -like ('*--remote-debugging-port=' + $port + '*') -and $_.CommandLine -notlike '*--type=*' -and (($_.CommandLine -like ('*--dialtone-role=' + $role + '*')) -or ($_.CommandLine -like ('*' + $profile + '*'))) } | Select-Object -ExpandProperty ProcessId; `+
			`if($items){ ($items | Measure-Object).Count } else { 0 }`,
			psQuote(role), psQuote(windowsPath(profileDir)), roleChromePort(role))
		out, err := sshv1.RunNodeCommand(node.Name, script, sshv1.CommandOptions{})
		if err != nil {
			return 0, err
		}
		n, convErr := strconv.Atoi(strings.TrimSpace(out))
		if convErr != nil {
			return 0, convErr
		}
		return n, nil
	}
	cmd := fmt.Sprintf("ps -eo pid,args | grep '[c]hrome' | grep -- '--remote-debugging-port=%d' | grep -- '--dialtone-role=%s' | grep -v -- '--type=' | wc -l", roleChromePort(role), shellEscapeGrep(role))
	out, err := sshv1.RunNodeCommand(node.Name, cmd, sshv1.CommandOptions{})
	if err != nil {
		return 0, err
	}
	n, convErr := strconv.Atoi(strings.TrimSpace(out))
	if convErr != nil {
		return 0, convErr
	}
	return n, nil
}

func chromeBrowserPIDsForRole(role, profileDir string, port int) ([]int, error) {
	role = normalizeRole(role)
	switch runtime.GOOS {
	case "windows":
		script := fmt.Sprintf(`$role=%s; $profile=%s; $port=%d; `+
			`$items=Get-CimInstance Win32_Process | Where-Object { $_.Name -eq 'chrome.exe' -and $_.CommandLine -like ('*--remote-debugging-port=' + $port + '*') -and $_.CommandLine -notlike '*--type=*' -and (($_.CommandLine -like ('*--dialtone-role=' + $role + '*')) -or ($_.CommandLine -like ('*' + $profile + '*'))) } | Select-Object -ExpandProperty ProcessId; `+
			`if($items){ $items }`,
			psQuote(role), psQuote(windowsPath(profileDir)), port)
		out, err := runWindowsPowerShell(script, windowsProcessQueryTimeout)
		if err != nil {
			return nil, fmt.Errorf("list chrome pids failed: %w (%s)", err, strings.TrimSpace(string(out)))
		}
		return parsePIDList(string(out)), nil
	default:
		cmd := fmt.Sprintf("ps -eo pid,args | grep '[c]hrome' | grep -- '--remote-debugging-port=%d' | grep -- '--dialtone-role=%s' | grep -v -- '--type=' | awk '{print $1}'", port, shellEscapeGrep(role))
		out, err := exec.Command("bash", "-lc", cmd).Output()
		if err != nil {
			return nil, err
		}
		return parsePIDList(string(out)), nil
	}
}

func runWindowsPowerShell(script string, timeout time.Duration) ([]byte, error) {
	if timeout <= 0 {
		timeout = windowsProcessQueryTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return out, fmt.Errorf("powershell timed out after %v", timeout)
	}
	return out, err
}

func parsePIDList(raw string) []int {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	out := make([]int, 0, len(lines))
	seen := map[int]struct{}{}
	for _, line := range lines {
		n, err := strconv.Atoi(strings.TrimSpace(line))
		if err != nil || n <= 0 {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	return out
}

func ensureSingleChromeProcessForRole(role, profileDir string, port int, keepPID int) error {
	pids, err := chromeBrowserPIDsForRole(role, profileDir, port)
	if err != nil {
		return err
	}
	for _, pid := range pids {
		if keepPID > 0 && pid == keepPID {
			continue
		}
		if err := killPID(pid); err != nil {
			return err
		}
	}
	return nil
}

func cleanupChromeProfileLocks(profileDir string) error {
	lockNames := []string{"SingletonLock", "SingletonCookie", "SingletonSocket"}
	var errs []string
	for _, name := range lockNames {
		path := filepath.Join(profileDir, name)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func startDetachedWindowsProcess(exePath string, args []string, hidden bool) (int, error) {
	quotedArgs := make([]string, 0, len(args))
	for _, arg := range args {
		quotedArgs = append(quotedArgs, psQuote(arg))
	}
	windowStyle := "Normal"
	if hidden {
		windowStyle = "Hidden"
	}
	script := fmt.Sprintf("$p = Start-Process -FilePath %s -ArgumentList @(%s) -WindowStyle %s -PassThru; $p.Id",
		psQuote(windowsPath(exePath)),
		strings.Join(quotedArgs, ","),
		windowStyle,
	)
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("start detached chrome failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	pid, convErr := strconv.Atoi(strings.TrimSpace(string(out)))
	if convErr != nil {
		return 0, fmt.Errorf("unable to parse detached chrome pid from %q: %w", strings.TrimSpace(string(out)), convErr)
	}
	return pid, nil
}

func killPID(pid int) error {
	if pid <= 0 {
		return nil
	}
	if runtime.GOOS == "windows" {
		return exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid)).Run()
	}
	return exec.Command("kill", "-9", strconv.Itoa(pid)).Run()
}

func waitForPIDExit(pid int, timeout time.Duration) error {
	if pid <= 0 {
		return nil
	}
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	deadline := time.Now().Add(timeout)
	for {
		if !processAlive(pid) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("process %d still running after %v", pid, timeout)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func normalizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "://") ||
		strings.HasPrefix(raw, "about:") ||
		strings.HasPrefix(raw, "data:") ||
		strings.HasPrefix(raw, "file:") {
		return raw
	}
	return "https://" + raw
}

func ariaSelector(label string) string {
	return fmt.Sprintf(`[aria-label=%q]`, strings.TrimSpace(label))
}

func psQuote(raw string) string {
	return "'" + strings.ReplaceAll(raw, "'", "''") + "'"
}

func shellQuote(raw string) string {
	return "'" + strings.ReplaceAll(raw, "'", "'\"'\"'") + "'"
}

func windowsPath(raw string) string {
	raw = strings.ReplaceAll(raw, "/", `\`)
	return strings.ReplaceAll(raw, `\\`, `\`)
}

func shellEscapeGrep(raw string) string {
	return strings.ReplaceAll(raw, `'`, `'\''`)
}

func binaryName(goos, goarch string) string {
	name := fmt.Sprintf("dialtone_chrome_v3-%s-%s", goos, goarch)
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

func remoteBinaryPath(node sshv1.MeshNode) (string, error) {
	homeCmd := "$HOME"
	if strings.EqualFold(node.OS, "windows") {
		homeCmd = "$env:USERPROFILE"
	}
	home, err := sshv1.RunNodeCommand(node.Name, homeCmd, sshv1.CommandOptions{})
	if err != nil {
		return "", err
	}
	base := strings.TrimSpace(home)
	if strings.EqualFold(node.OS, "windows") {
		return windowsPath(filepath.Join(base, ".dialtone", "bin", "dialtone_chrome_v3.exe")), nil
	}
	return filepath.Join(base, ".dialtone", "bin", "dialtone_chrome_v3"), nil
}

func mapNodeGOOS(nodeOS string) string {
	switch strings.ToLower(strings.TrimSpace(nodeOS)) {
	case "windows":
		return "windows"
	case "macos", "darwin":
		return "darwin"
	default:
		return "linux"
	}
}

func detectRemoteGOARCH(node sshv1.MeshNode) string {
	if strings.EqualFold(node.OS, "windows") {
		out, err := sshv1.RunNodeCommand(node.Name, "$env:PROCESSOR_ARCHITECTURE", sshv1.CommandOptions{})
		if err == nil && strings.Contains(strings.ToLower(strings.TrimSpace(out)), "arm64") {
			return "arm64"
		}
		return "amd64"
	}
	out, err := sshv1.RunNodeCommand(node.Name, "uname -m", sshv1.CommandOptions{})
	if err == nil {
		v := strings.ToLower(strings.TrimSpace(out))
		if strings.Contains(v, "arm64") || strings.Contains(v, "aarch64") {
			return "arm64"
		}
	}
	return "amd64"
}

func resolveRepoRoot() string {
	if rt, err := configv1.ResolveRuntime(""); err == nil && strings.TrimSpace(rt.RepoRoot) != "" {
		return strings.TrimSpace(rt.RepoRoot)
	}
	cwd, _ := os.Getwd()
	cur := strings.TrimSpace(cwd)
	for cur != "" {
		if _, err := os.Stat(filepath.Join(cur, "src", "go.mod")); err == nil {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return cwd
}

func resolveSrcRoot() string {
	if rt, err := configv1.ResolveRuntime(""); err == nil && strings.TrimSpace(rt.SrcRoot) != "" {
		return strings.TrimSpace(rt.SrcRoot)
	}
	repoRoot := resolveRepoRoot()
	if _, err := os.Stat(filepath.Join(repoRoot, "go.mod")); err == nil {
		return repoRoot
	}
	return filepath.Join(repoRoot, "src")
}

func preferredHost(node sshv1.MeshNode) string {
	if node.PreferWSLPowerShell && runtime.GOOS == "linux" {
		return "127.0.0.1"
	}
	return strings.TrimSpace(sshv1.PreferredHost(node, node.Port))
}

func defaultProfileDir(role string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".dialtone", "chrome-v3", role)
}
