package sessionlifecycle

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	chrome "dialtone/dev/plugins/chrome/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
	"github.com/chromedp/chromedp"
)

var state struct {
	baseDir      string
	devUserData  string
	testUserData string
	devSession   *chrome.Session
	testSession  *chrome.Session
}

func Register(reg *testv1.Registry) {
	reg.Add(testv1.Step{
		Name:           "setup-and-launch-dev-headed-gpu",
		Timeout:        90 * time.Second,
		RunWithContext: runSetupAndLaunchDev,
	})
	reg.Add(testv1.Step{
		Name:           "reuse-dev-and-attach-new-tab",
		Timeout:        60 * time.Second,
		RunWithContext: runReuseAndAttach,
	})
	reg.Add(testv1.Step{
		Name:           "launch-test-headless-and-list-processes",
		Timeout:        90 * time.Second,
		RunWithContext: runLaunchTestAndList,
	})
	reg.Add(testv1.Step{
		Name:           "cleanup-test-preserve-dev",
		Timeout:        45 * time.Second,
		RunWithContext: runCleanupTestOnly,
	})
	reg.Add(testv1.Step{
		Name:           "cleanup-all",
		Timeout:        60 * time.Second,
		RunWithContext: runCleanupAll,
	})
}

func runSetupAndLaunchDev(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
	if chrome.FindChromePath() == "" {
		return testv1.StepRunResult{}, fmt.Errorf("chrome binary not found")
	}
	if err := chrome.KillDialtoneResources(); err != nil {
		ctx.Warnf("pre-cleanup warning: %v", err)
	}

	baseDir := filepath.Join(".chrome_data", fmt.Sprintf("chrome-test-%d", time.Now().UnixNano()))
	state.baseDir = baseDir
	if runtime.GOOS == "linux" && chrome.IsWSL() {
		winTemp, werr := windowsTempDir()
		if werr != nil {
			return testv1.StepRunResult{}, fmt.Errorf("resolve windows temp dir: %w", werr)
		}
		stamp := fmt.Sprintf("dialtone-chrome-test-%d", time.Now().UnixNano())
		state.devUserData = winTemp + "\\" + stamp + "-dev"
		state.testUserData = winTemp + "\\" + stamp + "-test"
	} else {
		state.devUserData = filepath.Join(baseDir, "dev")
		state.testUserData = filepath.Join(baseDir, "test")
	}

	dev, err := chrome.StartSession(chrome.SessionOptions{
		RequestedPort: 0,
		GPU:           true,
		Headless:      false,
		Role:          "dev",
		TargetURL:     "about:blank",
		ReuseExisting: false,
		UserDataDir:   state.devUserData,
	})
	if err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("start dev session: %w", err)
	}
	state.devSession = dev

	if err := chrome.WaitForDebugPort(dev.Port, 20*time.Second); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("dev debug port not ready: %w", err)
	}

	ctx.Infof("launched dev session pid=%d port=%d user_data_dir=%s", dev.PID, dev.Port, state.devUserData)
	return testv1.StepRunResult{Report: "launched headed dev session with gpu and debug port ready"}, nil
}

func runReuseAndAttach(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
	if state.devSession == nil {
		return testv1.StepRunResult{}, fmt.Errorf("missing dev session from previous step")
	}

	reused, err := chrome.StartSession(chrome.SessionOptions{
		RequestedPort: 0,
		GPU:           true,
		Headless:      false,
		Role:          "dev",
		ReuseExisting: true,
	})
	if err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("reuse existing dev session: %w", err)
	}
	if reused.IsNew || reused.PID != state.devSession.PID {
		return testv1.StepRunResult{}, fmt.Errorf("reuse mismatch got pid=%d isNew=%t expected pid=%d isNew=false", reused.PID, reused.IsNew, state.devSession.PID)
	}

	attachCtx, cancel, err := chrome.AttachToWebSocket(reused.WebSocketURL)
	if err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("attach websocket: %w", err)
	}
	defer cancel()

	if err := chromedp.Run(attachCtx, chromedp.Navigate("about:blank")); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("navigate on attached tab: %w", err)
	}

	childCtx, childCancel := chrome.NewTabContext(attachCtx)
	defer childCancel()
	var title string
	if err := chromedp.Run(childCtx,
		chromedp.Navigate("data:text/html,<title>dialtone-tab</title><h1>ok</h1>"),
		chromedp.Title(&title),
	); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("new tab navigate/title: %w", err)
	}
	if title != "dialtone-tab" {
		return testv1.StepRunResult{}, fmt.Errorf("unexpected new tab title: %q", title)
	}

	ctx.Infof("reused dev session and created new tab via chromedp")
	return testv1.StepRunResult{Report: "reused dev session and attached/new-tab checks passed"}, nil
}

func runLaunchTestAndList(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
	testSession, err := chrome.StartSession(chrome.SessionOptions{
		RequestedPort: 0,
		GPU:           false,
		Headless:      true,
		Role:          "test",
		ReuseExisting: false,
		UserDataDir:   state.testUserData,
	})
	if err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("start test session: %w", err)
	}
	state.testSession = testSession

	if err := chrome.WaitForDebugPort(testSession.Port, 20*time.Second); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("test debug port not ready: %w", err)
	}

	procs, err := chrome.ListResources(true)
	if err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("list resources: %w", err)
	}

	devProc, devFound := findProc(procs, state.devSession.PID)
	testProc, testFound := findProc(procs, state.testSession.PID)
	if !devFound || !testFound {
		return testv1.StepRunResult{}, fmt.Errorf("missing expected pids in list dev_found=%t test_found=%t", devFound, testFound)
	}
	if devProc.Role != "dev" || devProc.IsHeadless {
		return testv1.StepRunResult{}, fmt.Errorf("dev process metadata mismatch role=%s headless=%t", devProc.Role, devProc.IsHeadless)
	}
	if testProc.Role != "test" || !testProc.IsHeadless {
		return testv1.StepRunResult{}, fmt.Errorf("test process metadata mismatch role=%s headless=%t", testProc.Role, testProc.IsHeadless)
	}
	if testProc.GPUEnabled {
		return testv1.StepRunResult{}, fmt.Errorf("expected headless test process GPU disabled")
	}
	if state.testUserData != "" && !strings.Contains(testProc.Command, state.testUserData) {
		return testv1.StepRunResult{}, fmt.Errorf("expected command to contain user-data-dir %q", state.testUserData)
	}

	ctx.Infof("verified list shows dev/test roles, headed/headless, gpu and user-data-dir")
	return testv1.StepRunResult{Report: "launched headless test session and validated process listing metadata"}, nil
}

func runCleanupTestOnly(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
	if state.testSession == nil || state.devSession == nil {
		return testv1.StepRunResult{}, fmt.Errorf("missing sessions for cleanup step")
	}
	if err := chrome.CleanupSession(state.testSession); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("cleanup test session: %w", err)
	}

	procs, err := chrome.ListResources(true)
	if err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("list resources post-cleanup: %w", err)
	}
	_, testStillPresent := findProc(procs, state.testSession.PID)
	_, devStillPresent := findProc(procs, state.devSession.PID)
	if testStillPresent {
		return testv1.StepRunResult{}, fmt.Errorf("test session still present after cleanup pid=%d", state.testSession.PID)
	}
	if !devStillPresent {
		return testv1.StepRunResult{}, fmt.Errorf("dev session missing after test cleanup pid=%d", state.devSession.PID)
	}
	ctx.Infof("cleaned test role and preserved dev role")
	return testv1.StepRunResult{Report: "cleaned test session while preserving dev session"}, nil
}

func runCleanupAll(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
	if state.devSession != nil {
		if err := chrome.CleanupSession(state.devSession); err != nil {
			ctx.Warnf("dev cleanup warning: %v", err)
		}
	}
	if err := chrome.KillDialtoneResources(); err != nil {
		ctx.Warnf("dialtone cleanup warning: %v", err)
	}
	ctx.Infof("cleanup complete")
	return testv1.StepRunResult{Report: "cleanup complete for chrome role sessions"}, nil
}

func findProc(procs []chrome.ChromeProcess, pid int) (chrome.ChromeProcess, bool) {
	for _, p := range procs {
		if p.PID == pid {
			return p, true
		}
	}
	return chrome.ChromeProcess{}, false
}

func windowsTempDir() (string, error) {
	cmdPath := "cmd.exe"
	if _, err := exec.LookPath(cmdPath); err != nil {
		cmdPath = "/mnt/c/Windows/System32/cmd.exe"
	}
	out, err := exec.Command(cmdPath, "/c", "echo %TEMP%").Output()
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) >= 3 && line[1] == ':' && line[2] == '\\' {
			return line, nil
		}
	}
	return "", fmt.Errorf("unable to parse TEMP from output: %q", string(out))
}
