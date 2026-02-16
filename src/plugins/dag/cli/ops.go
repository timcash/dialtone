package cli

import (
	test_v2 "dialtone/cli/src/libs/test_v2"
	chrome_app "dialtone/cli/src/plugins/chrome/app"
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

func RunTest(versionDir string) error {
	fmt.Printf(">> [DAG] Test: %s\n", versionDir)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	devSession, err := ensureDevServerAndHeadedBrowser(cwd, versionDir)
	if err != nil {
		return err
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
	testErr := cmd.Run()

	// Preflight/build steps in the suite can interrupt the dev server.
	// Re-ensure preview availability after test completion so users can keep interacting.
	if _, err := ensureDevServerAndHeadedBrowser(cwd, versionDir); err != nil {
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

func ensureDevServerAndHeadedBrowser(repoRoot, versionDir string) (*dagDevPreviewSession, error) {
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
			fmt.Printf(">> [DAG] Test: reusing dev server for %s at http://127.0.0.1:%d\n", versionDir, port)
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
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/#three", port)
	if err := openPersistentDevChrome(url); err != nil {
		session.Close()
		return nil, err
	}
	fmt.Printf(">> [DAG] Test: opened headed Chrome preview at %s\n", url)
	return session, nil
}

func openPersistentDevChrome(url string) error {
	_, err := chrome_app.StartSession(chrome_app.SessionOptions{
		GPU:           true,
		Headless:      false,
		TargetURL:     url,
		Role:          "dev",
		ReuseExisting: true,
	})
	if err != nil {
		return fmt.Errorf("failed to open persistent dev chrome preview at %s: %w", url, err)
	}
	_ = openPreviewURL(url)
	return nil
}

func openPreviewURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", "-a", "Google Chrome", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", url)
	default:
		return nil
	}
	return cmd.Start()
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
