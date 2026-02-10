package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"dialtone/cli/src/core/browser"
	"dialtone/cli/src/dialtest"
	chrome_app "dialtone/cli/src/plugins/chrome/app"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

type smokeEntry struct {
	name       string
	conditions string
	status     string
	errorMsg   string
	screenshot string
	logs       []string
}

func RunSmoke(versionDir string, timeoutSec int) error {
	fmt.Printf(">> [DAG] Smoke: START for %s\n", versionDir)

	cwd, _ := os.Getwd()
	pluginDir := filepath.Join(cwd, "src", "plugins", "dag", versionDir)
	smokeFile := filepath.Join(pluginDir, "SMOKE.md")
	port := 8080

	os.WriteFile(smokeFile, []byte("# DAG Smoke Test Report\n\n**Started:** "+time.Now().Format(time.RFC1123)+"\n"), 0644)

	if err := runPreflight(smokeFile, cwd, versionDir); err != nil {
		return err
	}

	browser.CleanupPort(port)
	cmd := exec.Command("go", "run", "cmd/main.go")
	cmd.Dir = pluginDir
	logFile, _ := os.Create(filepath.Join(pluginDir, "smoke_server.log"))
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		fmt.Printf("   [ERROR] Failed to start dag plugin: %v\n", err)
		return err
	}
	defer cmd.Process.Kill()

	if err := waitForPort(port, 15*time.Second); err != nil {
		fmt.Printf("   [ERROR] Host node timeout: %v\n", err)
		return err
	}

	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	fmt.Printf(">> [DAG] Plugin started. Access UI at: %s\n", url)

	wsURL, isNew, err := resolveChrome(0, true)
	if err != nil {
		fmt.Printf("   [ERROR] Chrome resolution failed: %v\n", err)
		return err
	}
	fmt.Printf(">> [DAG] Chrome WebSocket: %s\n", wsURL)

	defer func() {
		if isNew {
			exec.Command("./dialtone.sh", "chrome", "kill", "all").Run()
		}
	}()

	allocCtx, cancel := chromedp.NewRemoteAllocator(context.Background(), wsURL)
	defer cancel()
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	ctx, cancelTimeout := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancelTimeout()

	var mu sync.Mutex
	var currentLogs []string

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			msg := formatConsoleArgs(ev.Args)
			mu.Lock()
			currentLogs = append(currentLogs, fmt.Sprintf("[%s] %s", ev.Type, msg))
			mu.Unlock()
			fmt.Printf("   [BROWSER] [%s] %s\n", ev.Type, msg)
		}
	})

	testResults := []smokeEntry{}

	runStep := func(name string, actions chromedp.Action) error {
		fmt.Printf(">> [DAG] Step: %s\n", name)

		err := chromedp.Run(ctx, actions)
		if err != nil {
			fmt.Printf("   [ERROR] Action failed: %v\n", err)
		}

		var buf []byte
		_ = chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
			b, capErr := page.CaptureScreenshot().Do(ctx)
			buf = b
			return capErr
		}))

		shotName := fmt.Sprintf("smoke_step_%d.png", len(testResults))
		if len(buf) > 0 {
			os.WriteFile(filepath.Join(pluginDir, shotName), buf, 0644)
		}

		mu.Lock()
		logs := append([]string{}, currentLogs...)
		currentLogs = nil
		mu.Unlock()

		status := "PASS"
		errMsg := ""
		if err != nil {
			status = "FAIL"
			errMsg = err.Error()
		}

		if writeErr := appendStep(smokeFile, name, status, shotName, logs, errMsg); writeErr != nil {
			return writeErr
		}

		testResults = append(testResults, smokeEntry{name: name, status: status, errorMsg: errMsg, screenshot: shotName, logs: logs})
		return err
	}

	if err := chromedp.Run(ctx,
		chromedp.EmulateViewport(1280, 800),
		chromedp.Navigate(fmt.Sprintf("http://127.0.0.1:%d", port)),
		dialtest.WaitForAriaLabel("DAG Hero Title"),
	); err != nil {
		fmt.Printf("   [ERROR] Initial navigation failed: %v\n", err)
		return err
	}

	if err := runStep("1. Hero Section Renders", chromedp.Tasks{
		dialtest.WaitForAriaLabel("DAG Hero Title"),
		dialtest.WaitForAriaLabel("DAG Hero Canvas"),
		assertElementVisible(".header-title"),
		assertElementVisible(".top-right-controls"),
		assertElementVisible(".main-nav"),
	}); err != nil {
		return err
	}

	if err := runStep("2. Docs Section Content", chromedp.Tasks{
		dialtest.NavigateToSection("dag-docs", "DAG Docs Title"),
		dialtest.WaitForAriaLabel("DAG Docs Commands"),
		dialtest.AssertElementHidden(".header-title"),
		dialtest.AssertElementHidden(".top-right-controls"),
		dialtest.AssertElementHidden(".main-nav"),
	}); err != nil {
		return err
	}

	if err := runStep("3. Layer Nest Visualization", chromedp.Tasks{
		dialtest.NavigateToSection("dag-layer-nest", "DAG Layer Nest"),
		dialtest.WaitForAriaLabel("DAG Layer Canvas"),
		dialtest.AssertElementHidden(".header-title"),
		dialtest.AssertElementHidden(".top-right-controls"),
		dialtest.AssertElementHidden(".main-nav"),
	}); err != nil {
		return err
	}

	if err := runStep("4. Return Hero", chromedp.Tasks{
		dialtest.NavigateToSection("dag-hero", "DAG Hero Title"),
		assertElementVisible(".header-title"),
		assertElementVisible(".top-right-controls"),
		assertElementVisible(".main-nav"),
	}); err != nil {
		return err
	}

	fmt.Printf(">> [DAG] Smoke: COMPLETE. Report at %s\n", smokeFile)
	return nil
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
	return fmt.Errorf("timeout")
}

func runPreflight(smokeFile, repoRoot, versionDir string) error {
	lintLog := &bytes.Buffer{}
	lintOk := true
	lintLog.WriteString("[command] ./dialtone.sh go exec vet ./src/plugins/dag/...\n")

	if out, err := runCommandCapture(repoRoot, "./dialtone.sh", "go", "exec", "vet", "./src/plugins/dag/..."); err != nil {
		lintOk = false
		lintLog.Write(out)
		lintLog.WriteString("\n[error] ")
		lintLog.WriteString(err.Error())
		lintLog.WriteString("\n")
	} else {
		lintLog.Write(out)
		lintLog.WriteString("\n[pass] go vet\n")
	}

	lintLog.WriteString("\n[command] ./dialtone.sh go exec fmt ./src/plugins/dag/...\n")
	if out, err := runCommandCapture(repoRoot, "./dialtone.sh", "go", "exec", "fmt", "./src/plugins/dag/..."); err != nil {
		lintOk = false
		lintLog.Write(out)
		lintLog.WriteString("\n[error] ")
		lintLog.WriteString(err.Error())
		lintLog.WriteString("\n")
	} else if len(out) > 0 {
		lintLog.Write(out)
		lintLog.WriteString("\n[warn] go fmt modified files\n")
	} else {
		lintLog.WriteString("\n[pass] go fmt\n")
	}

	uiDir := filepath.Join(repoRoot, "src", "plugins", "dag", versionDir, "ui")
	lintLog.WriteString("\n[command] bun install\n")
	if out, err := runCommandCapture(uiDir, "bun", "install"); err != nil {
		lintOk = false
		lintLog.Write(out)
		lintLog.WriteString("\n[error] ")
		lintLog.WriteString(err.Error())
		lintLog.WriteString("\n")
	} else {
		lintLog.Write(out)
		lintLog.WriteString("\n[pass] bun install\n")
	}

	lintLog.WriteString("\n[command] bun run lint\n")
	if out, err := runCommandCapture(uiDir, "bun", "run", "lint"); err != nil {
		lintOk = false
		lintLog.Write(out)
		lintLog.WriteString("\n[error] ")
		lintLog.WriteString(err.Error())
		lintLog.WriteString("\n")
	} else {
		lintLog.Write(out)
		lintLog.WriteString("\n[pass] bun run lint\n")
	}

	if err := appendPreflight(smokeFile, "Preflight: Lint", lintOk, lintLog.String()); err != nil {
		return err
	}
	if !lintOk {
		return fmt.Errorf("lint failed")
	}

	buildLog := &bytes.Buffer{}
	buildOk := true
	buildLog.WriteString("[command] bun install\n")
	if out, err := runCommandCapture(uiDir, "bun", "install"); err != nil {
		buildOk = false
		buildLog.Write(out)
		buildLog.WriteString("\n[error] ")
		buildLog.WriteString(err.Error())
		buildLog.WriteString("\n")
	} else {
		buildLog.Write(out)
		buildLog.WriteString("\n[pass] bun install\n")
	}

	buildLog.WriteString("\n[command] bun run build\n")
	if out, err := runCommandCapture(uiDir, "bun", "run", "build"); err != nil {
		buildOk = false
		buildLog.Write(out)
		buildLog.WriteString("\n[error] ")
		buildLog.WriteString(err.Error())
		buildLog.WriteString("\n")
	} else {
		buildLog.Write(out)
		buildLog.WriteString("\n[pass] bun run build\n")
	}

	if err := appendPreflight(smokeFile, "Preflight: Build", buildOk, buildLog.String()); err != nil {
		return err
	}
	if !buildOk {
		return fmt.Errorf("build failed")
	}

	return nil
}

func appendPreflight(smokeFile, title string, ok bool, log string) error {
	status := "PASS"
	if !ok {
		status = "FAIL"
	}
	f, err := os.OpenFile(smokeFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintf(f, "\n## %s\n\n**Status:** %s\n\n```log\n%s\n```\n\n---", title, status, strings.TrimSpace(log))
	return nil
}

func appendStep(smokeFile, title, status, screenshot string, logs []string, errMsg string) error {
	f, err := os.OpenFile(smokeFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintf(f, "\n### %s\n\n**Status:** %s\n\n", title, status)
	if screenshot != "" {
		fmt.Fprintf(f, "![%s](%s)\n\n", title, screenshot)
	}
	if errMsg != "" {
		fmt.Fprintf(f, "**Error:** `%s`\n\n", errMsg)
	}
	if len(logs) > 0 {
		fmt.Fprintf(f, "**Browser Logs:**\n")
		for _, line := range logs {
			fmt.Fprintf(f, "- %s\n", line)
		}
		fmt.Fprint(f, "\n")
	}
	fmt.Fprint(f, "---")
	return nil
}

func runCommandCapture(dir string, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.Bytes(), err
}

func resolveChrome(requestedPort int, headless bool) (string, bool, error) {
	procs, err := chrome_app.ListResources(true)
	if err == nil {
		for _, p := range procs {
			if p.DebugPort > 0 {
				resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", p.DebugPort))
				if err == nil {
					var data struct {
						WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
					}
					json.NewDecoder(resp.Body).Decode(&data)
					resp.Body.Close()
					if data.WebSocketDebuggerURL != "" {
						return data.WebSocketDebuggerURL, false, nil
					}
				}
			}
		}
	}
	res, err := chrome_app.LaunchChrome(requestedPort, true, headless, "")
	if err != nil {
		return "", false, err
	}
	return res.WebsocketURL, true, nil
}

func formatConsoleArgs(args []*runtime.RemoteObject) string {
	var parts []string
	for _, arg := range args {
		if arg == nil {
			continue
		}
		if len(arg.Value) > 0 {
			var v interface{}
			if err := json.Unmarshal(arg.Value, &v); err == nil {
				b, _ := json.Marshal(v)
				parts = append(parts, string(b))
			} else {
				parts = append(parts, string(arg.Value))
			}
		} else if arg.Description != "" {
			parts = append(parts, arg.Description)
		}
	}
	return strings.Join(parts, " ")
}

func assertElementVisible(selector string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		start := time.Now()
		var display string
		for time.Since(start) < 3*time.Second {
			err := chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`getComputedStyle(document.querySelector('%s')).display`, selector), &display))
			if err == nil && display != "none" {
				return nil
			}
			time.Sleep(200 * time.Millisecond)
		}
		return fmt.Errorf("element %s should be visible (last display: %s)", selector, display)
	})
}
