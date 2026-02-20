package cli

import (
	test_v2 "dialtone/dev/plugins/test/src_v1/go"
	chrome_app "dialtone/dev/plugins/chrome/app"
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
)

func RunFmt(versionDir string) error {
	fmt.Printf(">> [LOGS] Fmt: %s\n", versionDir)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "fmt", "./src/plugins/logs/"+versionDir+"/...")
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunFormat(versionDir string) error {
	fmt.Printf(">> [LOGS] Format: %s\n", versionDir)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	uiDir := filepath.Join(cwd, "src", "plugins", "logs", versionDir, "ui")
	cmd := runBun(cwd, uiDir, "run", "format")
	return cmd.Run()
}

func RunVet(versionDir string) error {
	fmt.Printf(">> [LOGS] Vet: %s\n", versionDir)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "vet", "./src/plugins/logs/"+versionDir+"/...")
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunGoBuild(versionDir string) error {
	fmt.Printf(">> [LOGS] Go Build: %s\n", versionDir)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "build", "./src/plugins/logs/"+versionDir+"/...")
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunServe(versionDir string) error {
	fmt.Printf(">> [LOGS] Serve: %s\n", versionDir)
	cwd, _ := os.Getwd()
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "run", filepath.ToSlash(filepath.Join("src", "plugins", "logs", versionDir, "cmd", "main.go")))
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunUIRun(versionDir string, extraArgs []string) error {
	fmt.Printf(">> [LOGS] UI Run: %s\n", versionDir)
	port := 3000
	if len(extraArgs) >= 2 && extraArgs[0] == "--port" {
		if p, err := strconv.Atoi(extraArgs[1]); err == nil {
			port = p
		}
	}

	cwd, _ := os.Getwd()
	uiDir := filepath.Join(cwd, "src", "plugins", "logs", versionDir, "ui")
	cmd := runBun(cwd, uiDir, "run", "dev", "--host", "127.0.0.1", "--port", strconv.Itoa(port))
	return cmd.Run()
}

func RunTest(versionDir string, attach bool, cps int) error {
	fmt.Printf(">> [LOGS] Test: %s\n", versionDir)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	allowOpenBrowser := !attach
	devSession, err := ensureDevServerAndHeadedBrowser(cwd, versionDir, allowOpenBrowser)
	if err != nil {
		return err
	}
	if attach {
		attachURL := fmt.Sprintf("http://127.0.0.1:%d/#logs-log-xterm", devSession.port)
		if err := ensureAttachableLogsDevBrowser(attachURL); err != nil {
			return err
		}
	}
	fmt.Printf(">> [LOGS] Test: leaving dev preview running at http://127.0.0.1:%d after test completion\n", devSession.port)

	testPkg := "./" + filepath.ToSlash(filepath.Join("src", "plugins", "logs", versionDir, "test"))
	testMain := filepath.Join(cwd, "src", "plugins", "logs", versionDir, "test", "main.go")
	if _, err := os.Stat(testMain); os.IsNotExist(err) {
		return fmt.Errorf("test runner not found: %s/main.go", testPkg)
	}

	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "run", testPkg)
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	baseURL := "http://127.0.0.1:8080"
	devBaseURL := fmt.Sprintf("http://127.0.0.1:%d", devSession.port)
	if attach {
		baseURL = devBaseURL
	}
	cmd.Env = append(
		os.Environ(),
		"LOGS_TEST_ATTACH=0",
		"LOGS_TEST_BASE_URL="+baseURL,
		"LOGS_TEST_DEV_BASE_URL="+devBaseURL,
		"LOGS_TEST_CPS="+strconv.Itoa(cps),
	)
	if attach {
		cmd.Env = append(cmd.Env, "LOGS_TEST_ATTACH=1")
		fmt.Printf(">> [LOGS] Test: attach mode enabled (reusing headed dev browser session)\n")
	}
	testErr := cmd.Run()

	if _, err := ensureDevServerAndHeadedBrowser(cwd, versionDir, false); err != nil {
		if testErr == nil {
			return fmt.Errorf("tests finished, but failed to keep dev preview running: %w", err)
		}
		fmt.Printf(">> [LOGS] Test: warning: failed to restore dev preview after test error: %v\n", err)
	}

	return testErr
}

type logsDevPreviewSession struct {
	port        int
	startedHere bool
}

func (s *logsDevPreviewSession) Close() {}

func ensureDevServerAndHeadedBrowser(repoRoot, versionDir string, allowOpenBrowser bool) (*logsDevPreviewSession, error) {
	uiDir := filepath.Join(repoRoot, "src", "plugins", "logs", versionDir, "ui")
	if _, err := os.Stat(filepath.Join(uiDir, "index.html")); err != nil {
		return nil, fmt.Errorf("logs ui entry not found for %s: %w", versionDir, err)
	}

	targetTitle, err := readHTMLTitle(filepath.Join(uiDir, "index.html"))
	if err != nil {
		return nil, err
	}

	port := 3000
	reuse := false
	if err := test_v2.WaitForPort(port, 600*time.Millisecond); err == nil {
		matched, probeErr := devServerMatchesVersion(port, targetTitle)
		if probeErr == nil && matched {
			reuse = true
			fmt.Printf(">> [LOGS] Test: dev server already running for %s at http://127.0.0.1:%d\n", versionDir, port)
		} else {
			freePort, pickErr := test_v2.PickFreePort()
			if pickErr != nil {
				return nil, fmt.Errorf("dev server on %d is not %s and no free port could be picked: %w", port, versionDir, pickErr)
			}
			fmt.Printf(">> [LOGS] Test: existing dev server on :%d is not %s; starting %s on :%d\n", port, versionDir, versionDir, freePort)
			port = freePort
		}
	}

	session := &logsDevPreviewSession{port: port}
	if !reuse {
		if err := startDetachedLogsDevServer(repoRoot, versionDir, port); err != nil {
			return nil, err
		}
		if err := test_v2.WaitForPort(port, 30*time.Second); err != nil {
			return nil, fmt.Errorf("logs dev server for %s did not become ready on :%d: %w", versionDir, port, err)
		}
		session.startedHere = true
		fmt.Printf(">> [LOGS] Test: started dev server for %s at http://127.0.0.1:%d\n", versionDir, port)
		if allowOpenBrowser {
			url := fmt.Sprintf("http://127.0.0.1:%d/#logs-log-xterm", port)
			if err := openPersistentLogsDevChrome(url); err != nil {
				session.Close()
				return nil, err
			}
			fmt.Printf(">> [LOGS] Test: opened headed Chrome preview at %s\n", url)
		}
	} else if allowOpenBrowser {
		fmt.Printf(">> [LOGS] Test: keeping existing dev preview tab at http://127.0.0.1:%d/#logs-log-xterm\n", port)
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
	fmt.Printf(">> [LOGS] Test: no attachable logs-dev browser found; opening managed browser...\n")
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
	logPath := filepath.Join(repoRoot, "src", "plugins", "logs", versionDir, "test", "dev_preview.log")
	uiDir := filepath.Join(repoRoot, "src", "plugins", "logs", versionDir, "ui")
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
