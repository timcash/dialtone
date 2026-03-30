package logsv1

import (
	chrome_app "dialtone/dev/plugins/chrome/src_v1/go"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func RunFmt(versionDir string) error {
	logs.Info("logs %s fmt", versionDir)
	paths, err := resolveLogsPaths(versionDir)
	if err != nil {
		return err
	}
	goBin := strings.TrimSpace(paths.Runtime.GoBin)
	if goBin == "" {
		goBin = "go"
	}
	cmd := exec.Command(goBin, "fmt", "./plugins/logs/"+versionDir+"/...")
	cmd.Dir = paths.Runtime.SrcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	return cmd.Run()
}

func RunFormat(versionDir string) error {
	logs.Info("logs %s format", versionDir)
	paths, err := resolveLogsPaths(versionDir)
	if err != nil {
		return err
	}
	uiDir := paths.Preset.UI
	cmd := runBun(paths.Runtime.RepoRoot, uiDir, "run", "format")
	return cmd.Run()
}

func RunVet(versionDir string) error {
	logs.Info("logs %s vet", versionDir)
	paths, err := resolveLogsPaths(versionDir)
	if err != nil {
		return err
	}
	goBin := strings.TrimSpace(paths.Runtime.GoBin)
	if goBin == "" {
		goBin = "go"
	}
	cmd := exec.Command(goBin, "vet", "./plugins/logs/"+versionDir+"/...")
	cmd.Dir = paths.Runtime.SrcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	return cmd.Run()
}

func RunGoBuild(versionDir string) error {
	logs.Info("logs %s go-build", versionDir)
	paths, err := resolveLogsPaths(versionDir)
	if err != nil {
		return err
	}
	goBin := strings.TrimSpace(paths.Runtime.GoBin)
	if goBin == "" {
		goBin = "go"
	}
	cmd := exec.Command(goBin, "build", "./plugins/logs/"+versionDir+"/...")
	cmd.Dir = paths.Runtime.SrcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	return cmd.Run()
}

func RunServe(versionDir string) error {
	logs.Info("logs %s serve", versionDir)
	paths, err := resolveLogsPaths(versionDir)
	if err != nil {
		return err
	}
	goBin := strings.TrimSpace(paths.Runtime.GoBin)
	if goBin == "" {
		goBin = "go"
	}
	cmd := exec.Command(goBin, "run", filepath.ToSlash(filepath.Join("plugins", "logs", versionDir, "cmd", "main.go")))
	cmd.Dir = paths.Runtime.SrcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	return cmd.Run()
}

func RunUIRun(versionDir string, extraArgs []string) error {
	logs.Info("logs %s ui-run", versionDir)
	port := 3000
	if len(extraArgs) >= 2 && extraArgs[0] == "--port" {
		if p, err := strconv.Atoi(extraArgs[1]); err == nil {
			port = p
		}
	}

	paths, err := resolveLogsPaths(versionDir)
	if err != nil {
		return err
	}
	uiDir := paths.Preset.UI
	cmd := runBun(paths.Runtime.RepoRoot, uiDir, "run", "dev", "--host", "127.0.0.1", "--port", strconv.Itoa(port))
	return cmd.Run()
}

func RunTest(versionDir string, extraArgs []string) error {
	logs.Info("logs %s test", versionDir)
	paths, err := resolveLogsPaths(versionDir)
	if err != nil {
		return err
	}
	testPkg := "./" + filepath.ToSlash(filepath.Join("plugins", "logs", versionDir, "test", "cmd"))
	testMain := filepath.Join(paths.Preset.TestCmd, "main.go")
	if _, err := os.Stat(testMain); os.IsNotExist(err) {
		return fmt.Errorf("test runner not found: %s/main.go", testPkg)
	}
	goBin := strings.TrimSpace(paths.Runtime.GoBin)
	if goBin == "" {
		goBin = "go"
	}

	args := append([]string{"run", testPkg}, extraArgs...)
	cmd := exec.Command(goBin, args...)
	cmd.Dir = paths.Runtime.SrcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	return cmd.Run()
}

type logsDevPreviewSession struct {
	port        int
	startedHere bool
}

func (s *logsDevPreviewSession) Close() {}

func ensureDevServerAndHeadedBrowser(repoRoot, versionDir string, allowOpenBrowser bool) (*logsDevPreviewSession, error) {
	paths, err := resolveLogsPaths(versionDir)
	if err != nil {
		return nil, err
	}
	uiDir := paths.Preset.UI
	if _, err := os.Stat(filepath.Join(uiDir, "index.html")); err != nil {
		return nil, fmt.Errorf("logs ui entry not found for %s: %w", versionDir, err)
	}

	targetTitle, err := readHTMLTitle(filepath.Join(uiDir, "index.html"))
	if err != nil {
		return nil, err
	}

	port := 3000
	reuse := false
	if err := waitForPort(port, 600*time.Millisecond); err == nil {
		matched, probeErr := devServerMatchesVersion(port, targetTitle)
		if probeErr == nil && matched {
			reuse = true
			logs.Info("logs %s test: dev server already running at http://127.0.0.1:%d", versionDir, port)
		} else {
			freePort, pickErr := pickFreePort()
			if pickErr != nil {
				return nil, fmt.Errorf("dev server on %d is not %s and no free port could be picked: %w", port, versionDir, pickErr)
			}
			logs.Info("logs %s test: existing dev server on :%d is different; starting on :%d", versionDir, port, freePort)
			port = freePort
		}
	}

	session := &logsDevPreviewSession{port: port}
	if !reuse {
		if err := startDetachedLogsDevServer(repoRoot, versionDir, port); err != nil {
			return nil, err
		}
		if err := waitForPort(port, 30*time.Second); err != nil {
			return nil, fmt.Errorf("logs dev server for %s did not become ready on :%d: %w", versionDir, port, err)
		}
		session.startedHere = true
		logs.Info("logs %s test: started dev server at http://127.0.0.1:%d", versionDir, port)
		if allowOpenBrowser {
			url := fmt.Sprintf("http://127.0.0.1:%d/#logs-log-xterm", port)
			if err := openPersistentLogsDevChrome(url); err != nil {
				session.Close()
				return nil, err
			}
			logs.Info("logs %s test: opened headed Chrome preview at %s", versionDir, url)
		}
	} else if allowOpenBrowser {
		logs.Info("logs %s test: keeping existing dev preview at http://127.0.0.1:%d/#logs-log-xterm", versionDir, port)
	}
	return session, nil
}

func openPersistentLogsDevChrome(url string) error {
	_, err := chrome_app.StartSession(chrome_app.SessionOptions{
		GPU:           true,
		Headless:      false,
		TargetURL:     url,
		Role:          "logs-dev",
		ReuseExisting: true,
	})
	if err != nil {
		return fmt.Errorf("failed to open persistent logs dev chrome preview at %s: %w", url, err)
	}
	return nil
}

func hasAttachableLogsDevBrowser() bool {
	procs, err := chrome_app.ListResources(true)
	if err != nil {
		return false
	}
	for _, p := range procs {
		if p.Origin != "Dialtone" || p.Role != "logs-dev" || p.IsHeadless {
			continue
		}
		if p.DebugPort > 0 && hasReachableDevtoolsWebSocket(p.DebugPort) {
			return true
		}
	}
	return false
}

func ensureAttachableLogsDevBrowser(url string) error {
	if hasAttachableLogsDevBrowser() {
		return nil
	}
	logs.Info("logs test: no attachable logs-dev browser found; opening managed browser")
	return openPersistentLogsDevChrome(url)
}

func hasReachableDevtoolsWebSocket(port int) bool {
	client := &http.Client{Timeout: 700 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return false
	}
	var meta struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.Unmarshal(body, &meta); err != nil {
		return false
	}
	return strings.HasPrefix(meta.WebSocketDebuggerURL, "ws://")
}

func startDetachedLogsDevServer(repoRoot, versionDir string, port int) error {
	paths, err := resolveLogsPaths(versionDir)
	if err != nil {
		return err
	}
	logPath := paths.DevPreviewLog
	uiDir := paths.Preset.UI
	if _, err := os.Stat(filepath.Join(uiDir, "package.json")); err != nil {
		return fmt.Errorf("logs ui package.json not found for %s: %w", versionDir, err)
	}
	backgroundCmd := fmt.Sprintf(
		"cd %s && nohup bun run dev --host 127.0.0.1 --port %d >> %s 2>&1 < /dev/null & disown",
		strconv.Quote(uiDir),
		port,
		strconv.Quote(logPath),
	)
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "/b", "cmd", "/c", "cd /d "+uiDir+" && bun run dev --host 127.0.0.1 --port "+strconv.Itoa(port))
	default:
		cmd = exec.Command("bash", "-c", backgroundCmd)
	}
	cmd.Dir = repoRoot
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start detached logs dev server for %s on :%d: %w", versionDir, port, err)
	}
	_ = cmd.Process.Release()
	return nil
}

func readHTMLTitle(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed reading %s: %w", path, err)
	}
	re := regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	m := re.FindSubmatch(raw)
	if len(m) < 2 {
		return "", fmt.Errorf("missing <title> in %s", path)
	}
	title := strings.TrimSpace(string(m[1]))
	if title == "" {
		return "", fmt.Errorf("empty <title> in %s", path)
	}
	return title, nil
}

func devServerMatchesVersion(port int, targetTitle string) (bool, error) {
	client := &http.Client{Timeout: 1200 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d", port))
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return false, err
	}
	html := string(body)
	return strings.Contains(html, "<title>"+targetTitle+"</title>") || strings.Contains(html, targetTitle), nil
}
