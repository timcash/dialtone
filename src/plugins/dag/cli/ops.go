package cli

import (
	test_v2 "dialtone/cli/src/libs/test_v2"
	chrome_app "dialtone/cli/src/plugins/chrome/app"
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
	fmt.Printf(">> [DAG] Fmt: %s\n", versionDir)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "fmt", "./src/plugins/dag/"+versionDir+"/...")
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunFormat(versionDir string) error {
	fmt.Printf(">> [DAG] Format: %s\n", versionDir)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	uiDir := filepath.Join(cwd, "src", "plugins", "dag", versionDir, "ui")
	cmd := runBun(cwd, uiDir, "run", "format")
	return cmd.Run()
}

func RunVet(versionDir string) error {
	fmt.Printf(">> [DAG] Vet: %s\n", versionDir)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "vet", "./src/plugins/dag/"+versionDir+"/...")
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunGoBuild(versionDir string) error {
	fmt.Printf(">> [DAG] Go Build: %s\n", versionDir)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "build", "./src/plugins/dag/"+versionDir+"/...")
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunServe(versionDir string) error {
	fmt.Printf(">> [DAG] Serve: %s\n", versionDir)
	cwd, _ := os.Getwd()
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "run", filepath.ToSlash(filepath.Join("src", "plugins", "dag", versionDir, "cmd", "main.go")))
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunUIRun(versionDir string, extraArgs []string) error {
	fmt.Printf(">> [DAG] UI Run: %s\n", versionDir)
	port := 3000
	if len(extraArgs) >= 2 && extraArgs[0] == "--port" {
		if p, err := strconv.Atoi(extraArgs[1]); err == nil {
			port = p
		}
	}

	cwd, _ := os.Getwd()
	uiDir := filepath.Join(cwd, "src", "plugins", "dag", versionDir, "ui")
	cmd := runBun(cwd, uiDir, "run", "dev", "--host", "127.0.0.1", "--port", strconv.Itoa(port))
	return cmd.Run()
}

func RunTest(versionDir string, attach bool) error {
	fmt.Printf(">> [DAG] Test: %s\n", versionDir)
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
		attachURL := fmt.Sprintf("http://127.0.0.1:%d/#three", devSession.port)
		if err := ensureAttachableDagDevBrowser(attachURL); err != nil {
			return err
		}
	}
	fmt.Printf(">> [DAG] Test: leaving dev preview running at http://127.0.0.1:%d after test completion\n", devSession.port)

	testPkg := "./" + filepath.ToSlash(filepath.Join("src", "plugins", "dag", versionDir, "test"))
	testMain := filepath.Join(cwd, "src", "plugins", "dag", versionDir, "test", "main.go")
	if _, err := os.Stat(testMain); os.IsNotExist(err) {
		return fmt.Errorf("test runner not found: %s/main.go", testPkg)
	}

	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "run", testPkg)
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "DAG_TEST_ATTACH=0")
	if attach {
		cmd.Env = append(cmd.Env, "DAG_TEST_ATTACH=1")
		fmt.Printf(">> [DAG] Test: attach mode enabled (reusing headed dev browser session)\n")
	}
	testErr := cmd.Run()

	// Preflight/build steps in the suite can interrupt the dev server.
	// Re-ensure preview availability after test completion so users can keep interacting.
	if _, err := ensureDevServerAndHeadedBrowser(cwd, versionDir, false); err != nil {
		if testErr == nil {
			return fmt.Errorf("tests finished, but failed to keep dev preview running: %w", err)
		}
		fmt.Printf(">> [DAG] Test: warning: failed to restore dev preview after test error: %v\n", err)
	}

	return testErr
}

type dagDevPreviewSession struct {
	port        int
	startedHere bool
}

func (s *dagDevPreviewSession) Close() {
	if s == nil {
		return
	}
}

func ensureDevServerAndHeadedBrowser(repoRoot, versionDir string, allowOpenBrowser bool) (*dagDevPreviewSession, error) {
	uiDir := filepath.Join(repoRoot, "src", "plugins", "dag", versionDir, "ui")
	if _, err := os.Stat(filepath.Join(uiDir, "index.html")); err != nil {
		return nil, fmt.Errorf("dag ui entry not found for %s: %w", versionDir, err)
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
			fmt.Printf(">> [DAG] Test: dev server already running for %s at http://127.0.0.1:%d\n", versionDir, port)
		} else {
			freePort, pickErr := test_v2.PickFreePort()
			if pickErr != nil {
				return nil, fmt.Errorf("dev server on %d is not %s and no free port could be picked: %w", port, versionDir, pickErr)
			}
			fmt.Printf(">> [DAG] Test: existing dev server on :%d is not %s; starting %s on :%d\n", port, versionDir, versionDir, freePort)
			port = freePort
		}
	}

	session := &dagDevPreviewSession{port: port}
	if !reuse {
		if err := startDetachedDagDevServer(repoRoot, versionDir, port); err != nil {
			return nil, err
		}
		if err := test_v2.WaitForPort(port, 30*time.Second); err != nil {
			return nil, fmt.Errorf("dag dev server for %s did not become ready on :%d: %w", versionDir, port, err)
		}
		session.startedHere = true
		fmt.Printf(">> [DAG] Test: started dev server for %s at http://127.0.0.1:%d\n", versionDir, port)
		if allowOpenBrowser {
			url := fmt.Sprintf("http://127.0.0.1:%d/#three", port)
			if err := openPersistentDevChrome(url); err != nil {
				session.Close()
				return nil, err
			}
			fmt.Printf(">> [DAG] Test: opened headed Chrome preview at %s\n", url)
		}
	} else if allowOpenBrowser {
		fmt.Printf(">> [DAG] Test: keeping existing dev preview tab at http://127.0.0.1:%d/#three\n", port)
	}
	return session, nil
}

func openPersistentDevChrome(url string) error {
	_, err := chrome_app.StartSession(chrome_app.SessionOptions{
		GPU:           true,
		Headless:      false,
		TargetURL:     url,
		Role:          "dag-dev",
		ReuseExisting: true,
	})
	if err != nil {
		return fmt.Errorf("failed to open persistent dev chrome preview at %s: %w", url, err)
	}
	return nil
}

func hasAttachableDagDevBrowser() bool {
	procs, err := chrome_app.ListResources(true)
	if err != nil {
		return false
	}
	for _, p := range procs {
		if p.Origin != "Dialtone" || p.Role != "dag-dev" || p.IsHeadless {
			continue
		}
		if p.DebugPort > 0 && hasReachableDevtoolsWebSocket(p.DebugPort) {
			return true
		}
	}
	return false
}

func ensureAttachableDagDevBrowser(url string) error {
	if hasAttachableDagDevBrowser() {
		return nil
	}
	fmt.Printf(">> [DAG] Test: no attachable dag-dev browser found; relaunching profile browser in debug mode...\n")
	if err := relaunchProfileChromeDebug(url); err != nil {
		return fmt.Errorf("failed to relaunch profile browser in debug mode: %w", err)
	}
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if hasAttachableDagDevBrowser() {
			fmt.Printf(">> [DAG] Test: attachable dag-dev browser session is ready\n")
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	fmt.Printf(">> [DAG] Test: profile debug relaunch was not attachable; launching managed dag-dev debug browser fallback...\n")
	if err := openPersistentDevChrome(url); err != nil {
		return fmt.Errorf("timed out waiting for attachable dag-dev browser session after profile relaunch, and managed fallback failed: %w", err)
	}
	deadline = time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if hasAttachableDagDevBrowser() {
			fmt.Printf(">> [DAG] Test: attachable dag-dev browser session is ready (managed fallback)\n")
			return nil
		}
		time.Sleep(400 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for attachable dag-dev browser session after profile relaunch and managed fallback")
}

func relaunchProfileChromeDebug(url string) error {
	home, _ := os.UserHomeDir()
	userDataDir := filepath.Join(home, ".chrome_dag_debug_profile")
	if envDir := os.Getenv("CHROME_USER_DATA_DIR"); strings.TrimSpace(envDir) != "" {
		userDataDir = strings.TrimSpace(envDir)
	}
	switch runtime.GOOS {
	case "darwin":
		_ = exec.Command("osascript", "-e", `tell application "Google Chrome" to quit`).Run()
		time.Sleep(1200 * time.Millisecond)
		chromeBin := "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
		if _, err := os.Stat(chromeBin); err != nil {
			chromeBin = "Google Chrome"
		}
		logPath := filepath.Join(os.TempDir(), "dag_profile_debug_chrome.log")
		cmdLine := fmt.Sprintf(
			"nohup %q --remote-debugging-port=9222 --remote-debugging-address=127.0.0.1 --user-data-dir=%q --profile-directory=Default --dialtone-origin=true --dialtone-role=dag-dev --new-window %q >> %q 2>&1 < /dev/null & disown",
			chromeBin, userDataDir, url, logPath,
		)
		return exec.Command("bash", "-lc", cmdLine).Run()
	case "linux":
		_ = exec.Command("pkill", "-f", "google-chrome").Run()
		time.Sleep(1200 * time.Millisecond)
		return exec.Command(
			"google-chrome",
			"--remote-debugging-port=9222",
			"--remote-debugging-address=127.0.0.1",
			"--user-data-dir="+userDataDir,
			"--dialtone-origin=true",
			"--dialtone-role=dag-dev",
			"--new-window",
			url,
		).Run()
	case "windows":
		_ = exec.Command("cmd", "/c", "taskkill", "/IM", "chrome.exe", "/F").Run()
		time.Sleep(1200 * time.Millisecond)
		return exec.Command(
			"cmd", "/c", "start", "", "chrome",
			"--remote-debugging-port=9222",
			"--remote-debugging-address=127.0.0.1",
			"--user-data-dir="+userDataDir,
			"--dialtone-origin=true",
			"--dialtone-role=dag-dev",
			"--new-window",
			url,
		).Run()
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
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

func startDetachedDagDevServer(repoRoot, versionDir string, port int) error {
	logPath := filepath.Join(repoRoot, "src", "plugins", "dag", versionDir, "test", "dev_preview.log")
	uiDir := filepath.Join(repoRoot, "src", "plugins", "dag", versionDir, "ui")
	if _, err := os.Stat(filepath.Join(uiDir, "package.json")); err != nil {
		return fmt.Errorf("dag ui package.json not found for %s: %w", versionDir, err)
	}
	backgroundCmd := fmt.Sprintf(
		"cd %s && nohup bun run dev --host 127.0.0.1 --port %d >> %s 2>&1 < /dev/null & disown",
		strconv.Quote(uiDir),
		port,
		strconv.Quote(logPath),
	)
	cmd := exec.Command("bash", "-lc", backgroundCmd)
	cmd.Dir = repoRoot
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start detached dag dev server for %s on :%d: %w", versionDir, port, err)
	}
	return nil
}

func readHTMLTitle(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read html title from %s: %w", path, err)
	}
	re := regexp.MustCompile(`(?is)<title>\s*(.*?)\s*</title>`)
	m := re.FindStringSubmatch(string(raw))
	if len(m) < 2 {
		return "", fmt.Errorf("missing <title> in %s", path)
	}
	return strings.TrimSpace(m[1]), nil
}

func devServerMatchesVersion(port int, expectedTitle string) (bool, error) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/", port))
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return false, err
	}

	re := regexp.MustCompile(`(?is)<title>\s*(.*?)\s*</title>`)
	m := re.FindStringSubmatch(string(body))
	if len(m) < 2 {
		return false, nil
	}
	servedTitle := strings.TrimSpace(m[1])
	return servedTitle == expectedTitle, nil
}
