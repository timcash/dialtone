package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"dialtone/dev/browser"
	"dialtone/dev/config"
	"dialtone/dev/test_core"

	cdpRuntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

func init() {
	test.Register("swarm-dashboard", "swarm", []string{"plugin", "swarm", "browser"}, RunSwarmIntegration)
}

// RunAll is the standard entry point for `./dialtone.sh swarm test`.
// It runs the chromedp-based swarm integration test (dashboard in browser).
func RunAll() error {
	return test.RunPlugin("swarm")
}

// RunSwarmIntegration starts the swarm dashboard and runs chromedp tests:
// launch Chrome via CLI, attach chromedp, load dashboard, capture console/errors.
func RunSwarmIntegration() error {
	cwd, _ := os.Getwd()
	dialtoneSh := filepath.Join(cwd, "dialtone.sh")
	if _, err := os.Stat(dialtoneSh); os.IsNotExist(err) {
		return fmt.Errorf("could not find dialtone.sh in %s", cwd)
	}

	appDir := filepath.Join(cwd, "src", "plugins", "swarm", "app")
	if _, err := os.Stat(appDir); os.IsNotExist(err) {
		return fmt.Errorf("swarm app directory not found: %s", appDir)
	}

	envPath := config.GetDialtoneEnv()
	if envPath == "" {
		return fmt.Errorf("DIALTONE_ENV is not set. Please set it in env/.env or pass --env.")
	}
	pearBin, err := resolvePearBin(envPath)
	if err != nil {
		return err
	}
	toolEnv := envWithDialtoneTools(envPath)

	fmt.Println(">> [swarm] Running Pear sequential test suite...")
	if err := runPearUnitTests(appDir, pearBin, toolEnv); err != nil {
		return err
	}

	fmt.Println(">> [swarm] Cleaning up existing processes...")
	browser.CleanupPort(4000)
	_ = exec.Command("./dialtone.sh", "chrome", "kill", "all").Run()

	fmt.Println(">> [swarm] Starting dashboard on port 4000...")
	dashCmd := exec.Command(pearBin, "run", ".", "dashboard", cwd)
	dashCmd.Dir = appDir
	dashCmd.Env = append(toolEnv, "DIALTONE_REPO="+cwd)
	if err := dashCmd.Start(); err != nil {
		return fmt.Errorf("failed to start dashboard: %v", err)
	}
	defer func() {
		if dashCmd.Process != nil {
			fmt.Println(">> [swarm] Stopping dashboard...")
			dashCmd.Process.Kill()
		}
	}()

	fmt.Println(">> [swarm] Waiting for dashboard...")
	if err := waitForPort(4000, 30*time.Second); err != nil {
		return fmt.Errorf("dashboard port 4000 not ready: %v", err)
	}
	fmt.Println(">> [swarm] Dashboard ready.")

	fmt.Println(">> [swarm] Launching Chrome via CLI...")
	launchCmd := exec.Command("./dialtone.sh", "chrome", "new", "--gpu")
	launchCmd.Dir = cwd
	output, err := launchCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to launch chrome via CLI: %v\nOutput: %s", err, string(output))
	}

	re := regexp.MustCompile(`ws://127\.0\.0\.1:\d+/devtools/browser/[a-z0-9-]+`)
	wsURL := re.FindString(string(output))
	if wsURL == "" {
		return fmt.Errorf("failed to find WebSocket URL in CLI output: %s", string(output))
	}
	fmt.Printf(">> [swarm] Connected to Chrome via: %s\n", wsURL)

	allocCtx, cancel := chromedp.NewRemoteAllocator(context.Background(), wsURL)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	var consoleLogs []string
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *cdpRuntime.EventConsoleAPICalled:
			parts := make([]string, 0, len(ev.Args))
			for _, arg := range ev.Args {
				parts = append(parts, formatRemoteObject(arg))
			}
			line := fmt.Sprintf("[%s] %s", ev.Type, strings.Join(parts, " "))
			if ev.StackTrace != nil && len(ev.StackTrace.CallFrames) > 0 {
				f := ev.StackTrace.CallFrames[0]
				if f.URL != "" {
					line = fmt.Sprintf("%s (%s:%d:%d)", line, f.URL, f.LineNumber+1, f.ColumnNumber+1)
				}
			}
			consoleLogs = append(consoleLogs, line)
		case *cdpRuntime.EventExceptionThrown:
			d := ev.ExceptionDetails
			ex := d.Text
			if d.Exception != nil {
				exObj := formatRemoteObject(d.Exception)
				if strings.TrimSpace(exObj) != "" {
					ex = ex + " | " + exObj
				}
			}
			line := fmt.Sprintf("[EXCEPTION] %s", ex)
			if d.StackTrace != nil && len(d.StackTrace.CallFrames) > 0 {
				f := d.StackTrace.CallFrames[0]
				if f.URL != "" {
					line = fmt.Sprintf("%s (%s:%d:%d)", line, f.URL, f.LineNumber+1, f.ColumnNumber+1)
				}
			}
			consoleLogs = append(consoleLogs, line)
		}
	})

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	if err := verifyDashboard(ctx); err != nil {
		return err
	}

	if err := printConsoleLogs(consoleLogs); err != nil {
		return err
	}

	fmt.Println("\n[PASS] Swarm integration tests complete")

	fmt.Println(">> [swarm] Verifying no leaked Dialtone processes...")
	_ = exec.Command("./dialtone.sh", "chrome", "kill", "all").Run()
	listCmd := exec.Command("./dialtone.sh", "chrome", "list")
	listOutput, _ := listCmd.CombinedOutput()
	if strings.Contains(string(listOutput), "Dialtone") {
		fmt.Printf(">> [WARNING] Leaked Dialtone processes detected:\n%s\n", string(listOutput))
	} else {
		fmt.Println(">> [swarm] Cleanup verified.")
	}

	return nil
}

func formatRemoteObject(o *cdpRuntime.RemoteObject) string {
	if o == nil {
		return ""
	}
	if len(o.Value) > 0 {
		var v interface{}
		if err := json.Unmarshal(o.Value, &v); err == nil {
			b, err := json.Marshal(v)
			if err == nil {
				return string(b)
			}
		}
		return string(o.Value)
	}
	if o.UnserializableValue != "" {
		return string(o.UnserializableValue)
	}
	if o.Description != "" {
		return o.Description
	}
	if o.Type != "" {
		return string(o.Type)
	}
	return ""
}

func printConsoleLogs(logs []string) error {
	fmt.Println("\n>> [swarm] Browser console logs:")
	fmt.Println("   ----------------------------------------")
	if len(logs) == 0 {
		fmt.Println("   (no console output)")
		return nil
	}
	for _, log := range logs {
		fmt.Printf("   %s\n", log)
	}
	hasErrors := false
	for _, log := range logs {
		if strings.Contains(log, "[error]") || strings.Contains(log, "[EXCEPTION]") {
			hasErrors = true
			break
		}
	}
	fmt.Println("   ----------------------------------------")
	if hasErrors {
		fmt.Println("   [FAIL] Console errors or exceptions detected!")
		for _, log := range logs {
			if strings.Contains(log, "[error]") || strings.Contains(log, "[EXCEPTION]") {
				fmt.Printf("   >> %s\n", log)
			}
		}
		return fmt.Errorf("critical console errors detected during browser execution")
	}
	fmt.Printf("   [PASS] %d console messages, no errors\n", len(logs))
	return nil
}

func verifyDashboard(ctx context.Context) error {
	fmt.Println(">> [swarm] Loading dashboard at http://127.0.0.1:4000...")
	var title string
	err := chromedp.Run(ctx,
		chromedp.Navigate("http://127.0.0.1:4000"),
		chromedp.WaitReady("body"),
		chromedp.Title(&title),
	)
	if err != nil {
		return fmt.Errorf("dashboard load failed: %v", err)
	}
	fmt.Printf(">> [swarm] Page title: %q\n", title)
	return nil
}

func runPearUnitTests(appDir, pearBin string, env []string) error {
	tests := []struct {
		file string
		args []string
	}{
		{"test_1_warm_node.js", nil},
		{"test_2_corestore_lock.js", nil},
		{"test_3_corestore.js", nil},
		{"test_4_autobase_static.js", nil},
		{"test_5_handshake.js", nil},
		{"test_6_full_stack.js", nil},
		{"test_7_convergence.js", []string{"lifecycle"}},
		{"test_8_warm_connect.js", nil},
	}

	for _, t := range tests {
		fmt.Printf(">> [swarm] Running %s...\n", t.file)
		if err := runPearTest(appDir, pearBin, env, t.file, t.args...); err != nil {
			return fmt.Errorf("%s failed: %v", t.file, err)
		}
		// Allow network to settle
		time.Sleep(5 * time.Second)
	}

	fmt.Println(">> [swarm] All sequential tests passed.")
	return nil
}

func runPearTest(appDir, pearBin string, env []string, fileName string, args ...string) error {
	fullArgs := append([]string{"run", fileName}, args...)
	cmd := exec.Command(pearBin, fullArgs...)
	cmd.Dir = appDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func envWithDialtoneTools(envPath string) []string {
	env := os.Environ()
	if envPath == "" {
		return env
	}
	pearRuntimeBin := ""
	if homeDir, err := os.UserHomeDir(); err == nil {
		candidate := filepath.Join(homeDir, "Library", "Application Support", "pear", "bin")
		if _, err := os.Stat(candidate); err == nil {
			pearRuntimeBin = candidate
		}
	}
	paths := []string{
		pearRuntimeBin,
		filepath.Join(envPath, "node", "bin"),
		filepath.Join(envPath, "bin"),
		os.Getenv("PATH"),
	}
	filtered := make([]string, 0, len(paths))
	for _, p := range paths {
		if p == "" {
			continue
		}
		filtered = append(filtered, p)
	}
	newPath := strings.Join(filtered, string(os.PathListSeparator))
	return replaceEnv(env, "PATH", newPath)
}

func replaceEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

func resolvePearBin(envPath string) (string, error) {
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".cmd"
	}
	candidates := append(pearRuntimeCandidates(), []string{
		filepath.Join(envPath, "node", "bin", "pear"+ext),
		filepath.Join(envPath, "bin", "pear"+ext),
	}...)
	for _, p := range candidates {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("pear not found in DIALTONE_ENV (%s). Run ./dialtone.sh swarm install", envPath)
}

func pearRuntimeCandidates() []string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	pearDir := ""
	switch runtime.GOOS {
	case "darwin":
		pearDir = filepath.Join(homeDir, "Library", "Application Support", "pear")
	case "linux":
		pearDir = filepath.Join(homeDir, ".config", "pear")
	case "windows":
		pearDir = filepath.Join(homeDir, "AppData", "Roaming", "pear")
	default:
		return nil
	}
	runtimeExt := ""
	if runtime.GOOS == "windows" {
		runtimeExt = ".exe"
	}
	host := runtime.GOOS + "-" + runtime.GOARCH
	return []string{
		filepath.Join(pearDir, "bin", "pear"+runtimeExt),
		filepath.Join(pearDir, "current", "by-arch", host, "bin", "pear-runtime"+runtimeExt),
	}
}

func waitForPort(port int, timeout time.Duration) error {
	start := time.Now()
	for time.Since(start) < timeout {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 1*time.Second)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for port %d", port)
}
