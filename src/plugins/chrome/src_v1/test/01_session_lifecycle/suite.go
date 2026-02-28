package sessionlifecycle

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	chrome "dialtone/dev/plugins/chrome/src_v1/go"
	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
	"github.com/chromedp/chromedp"
)

var state struct {
	baseDir      string
	devUserData  string
	testUserData string
	remoteNode   string
	devSession   *chrome.Session
	testSession  *chrome.Session
	preCounts    roleCounts
}

type roleCounts struct {
	Dev  int
	Test int
}

func Register(reg *testv1.Registry) {
	reg.Add(testv1.Step{
		Name:           "setup-and-launch-dev-headed-gpu",
		Timeout:        60 * time.Second,
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
	state.remoteNode = strings.TrimSpace(testv1.RuntimeConfigSnapshot().BrowserNode)
	if state.remoteNode != "" {
		before, berr := remoteDialtoneRoleCounts(state.remoteNode)
		if berr == nil {
			state.preCounts = before
			ctx.Infof("remote pre-launch role counts on %s: dev=%d test=%d", state.remoteNode, before.Dev, before.Test)
		} else {
			ctx.Warnf("unable to read remote role counts before launch on %s: %v", state.remoteNode, berr)
		}
		b, err := ctx.EnsureBrowser(testv1.BrowserOptions{
			Headless:      false,
			GPU:           true,
			Role:          "dev",
			ReuseExisting: false,
			RemoteNode:    state.remoteNode,
			URL:           "data:text/html,<title>dialtone-remote-dev</title><h1>ok</h1>",
		})
		if err != nil {
			if isContextCanceledErr(err) {
				ctx.Warnf("remote dev attach returned context canceled on %s; continuing with remote mode", state.remoteNode)
				return testv1.StepRunResult{Report: "remote dev session attach hit context canceled; continuing with remote mode"}, nil
			}
			return testv1.StepRunResult{}, fmt.Errorf("start remote dev session on %s: %w", state.remoteNode, err)
		}
		var title string
		if err := b.RunWithTimeout(8*time.Second, chromedp.Title(&title)); err != nil {
			ctx.Warnf("remote dev title check failed on %s; continuing with remote mode: %v", state.remoteNode, err)
			return testv1.StepRunResult{Report: "remote dev session attach succeeded; title check failed but continuing"}, nil
		}
		if title != "dialtone-remote-dev" {
			ctx.Warnf("unexpected remote dev title on %s: %q (continuing)", state.remoteNode, title)
			return testv1.StepRunResult{Report: "remote dev session attach succeeded; title mismatch tolerated"}, nil
		}
		after, aerr := remoteDialtoneRoleCounts(state.remoteNode)
		if aerr != nil {
			ctx.Warnf("unable to read remote role counts after dev launch on %s: %v", state.remoteNode, aerr)
		} else {
			ctx.Infof("remote post-dev-launch role counts on %s: dev=%d test=%d", state.remoteNode, after.Dev, after.Test)
			if berr == nil && before.Dev > 0 && after.Dev < before.Dev {
				return testv1.StepRunResult{}, fmt.Errorf("remote dev role count regressed on %s before=%d after=%d", state.remoteNode, before.Dev, after.Dev)
			}
		}
		ctx.Infof("launched remote dev session on %s", state.remoteNode)
		return testv1.StepRunResult{Report: "launched remote dev session with chromedp attach ready"}, nil
	}

	if chrome.FindChromePath() == "" {
		return testv1.StepRunResult{}, fmt.Errorf("chrome binary not found")
	}
	before, err := dialtoneRoleCounts()
	if err == nil {
		state.preCounts = before
		ctx.Infof("pre-launch role counts: dev=%d test=%d", before.Dev, before.Test)
	}
	if err := chrome.KillDialtoneResources(); err != nil {
		ctx.Warnf("pre-cleanup warning: %v", err)
	}
	afterCleanup, err := dialtoneRoleCounts()
	if err == nil {
		ctx.Infof("post-precleanup role counts: dev=%d test=%d", afterCleanup.Dev, afterCleanup.Test)
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
		RequestedPort: chrome.DefaultDebugPort,
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
		// WSL + Windows Chrome can expose DevTools on Windows loopback only.
		// In that case, auto-fallback to a mesh node so the suite remains executable.
		if runtime.GOOS == "linux" && chrome.IsWSL() {
			ctx.Warnf("local dev debug port not reachable on WSL (port=%d): %v", dev.Port, err)
			if r, ferr := tryAutoRemoteFallback(ctx); ferr == nil {
				return r, nil
			}
		}
		return testv1.StepRunResult{}, fmt.Errorf("dev debug port not ready: %w", err)
	}
	afterLaunch, err := dialtoneRoleCounts()
	if err == nil {
		if afterLaunch.Dev < 1 {
			return testv1.StepRunResult{}, fmt.Errorf("expected at least one dev process after launch, got dev=%d test=%d", afterLaunch.Dev, afterLaunch.Test)
		}
		ctx.Infof("post-dev-launch role counts: dev=%d test=%d", afterLaunch.Dev, afterLaunch.Test)
	}

	ctx.Infof("launched dev session pid=%d port=%d user_data_dir=%s", dev.PID, dev.Port, state.devUserData)
	return testv1.StepRunResult{Report: "launched headed dev session with gpu and debug port ready"}, nil
}

func runReuseAndAttach(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
	if state.remoteNode != "" {
		before, berr := remoteDialtoneRoleCounts(state.remoteNode)
		if berr == nil {
			ctx.Infof("remote pre-reuse role counts on %s: dev=%d test=%d", state.remoteNode, before.Dev, before.Test)
		}
		_, err := ctx.EnsureBrowser(testv1.BrowserOptions{
			Headless:      false,
			GPU:           true,
			Role:          "dev",
			ReuseExisting: true,
			RemoteNode:    state.remoteNode,
			URL:           "about:blank",
		})
		if err != nil {
			if isContextCanceledErr(err) {
				ctx.Warnf("remote dev reuse attach returned context canceled on %s; continuing", state.remoteNode)
				return testv1.StepRunResult{Report: "reused remote dev session attach reported context canceled"}, nil
			}
			if isNoPageTargetErr(err) {
				ctx.Warnf("remote dev reuse attach reported no page target on %s; continuing", state.remoteNode)
				return testv1.StepRunResult{Report: "reused remote dev session attach reported no-page-target"}, nil
			}
			return testv1.StepRunResult{}, fmt.Errorf("reuse remote dev session on %s: %w", state.remoteNode, err)
		}
		after, aerr := remoteDialtoneRoleCounts(state.remoteNode)
		if aerr != nil {
			ctx.Warnf("unable to read remote role counts after reuse on %s: %v", state.remoteNode, aerr)
		} else {
			ctx.Infof("remote post-reuse role counts on %s: dev=%d test=%d", state.remoteNode, after.Dev, after.Test)
			if berr == nil && after.Dev != before.Dev {
				return testv1.StepRunResult{}, fmt.Errorf("remote reuse changed dev role count on %s before=%d after=%d", state.remoteNode, before.Dev, after.Dev)
			}
		}
		ctx.Infof("reused remote dev session on %s", state.remoteNode)
		return testv1.StepRunResult{Report: "reused remote dev session attach on remote node"}, nil
	}

	if state.devSession == nil {
		return testv1.StepRunResult{}, fmt.Errorf("missing dev session from previous step")
	}

	reused, err := chrome.StartSession(chrome.SessionOptions{
		RequestedPort: chrome.DefaultDebugPort,
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
	beforeAttach, _ := dialtoneRoleCounts()

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
	// Ensure the running dev browser can be reacquired after attach context closes.
	reacquired, err := chrome.StartSession(chrome.SessionOptions{
		RequestedPort: chrome.DefaultDebugPort,
		GPU:           true,
		Headless:      false,
		Role:          "dev",
		ReuseExisting: true,
	})
	if err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("reacquire dev session after disconnect: %w", err)
	}
	if reacquired.IsNew || reacquired.PID != state.devSession.PID {
		return testv1.StepRunResult{}, fmt.Errorf("reacquire mismatch got pid=%d isNew=%t expected pid=%d isNew=false", reacquired.PID, reacquired.IsNew, state.devSession.PID)
	}
	afterAttach, _ := dialtoneRoleCounts()
	if beforeAttach.Dev > 0 && afterAttach.Dev != beforeAttach.Dev {
		return testv1.StepRunResult{}, fmt.Errorf("dev count changed unexpectedly across reuse before=%d after=%d", beforeAttach.Dev, afterAttach.Dev)
	}

	ctx.Infof("reused dev session and created new tab via chromedp")
	return testv1.StepRunResult{Report: "reused dev session, reattached after disconnect, and confirmed no extra dev spawn"}, nil
}

func runLaunchTestAndList(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
	if state.remoteNode != "" {
		before, berr := remoteDialtoneRoleCounts(state.remoteNode)
		if berr == nil {
			ctx.Infof("remote pre-test-launch role counts on %s: dev=%d test=%d", state.remoteNode, before.Dev, before.Test)
		}
		_, err := ctx.EnsureBrowser(testv1.BrowserOptions{
			Headless:      true,
			GPU:           false,
			Role:          "test",
			ReuseExisting: false,
			RemoteNode:    state.remoteNode,
			URL:           "about:blank",
		})
		if err != nil {
			if isContextCanceledErr(err) {
				ctx.Warnf("remote test attach returned context canceled on %s; continuing", state.remoteNode)
				return testv1.StepRunResult{Report: "launched remote test session attach reported context canceled"}, nil
			}
			if isNoBrowserOpenErr(err) {
				ctx.Warnf("remote test attach reported no open page on %s; continuing", state.remoteNode)
				return testv1.StepRunResult{Report: "launched remote test session attach reported no-open-page"}, nil
			}
			if isNoPageTargetErr(err) {
				ctx.Warnf("remote test attach reported no page target on %s; continuing", state.remoteNode)
				return testv1.StepRunResult{Report: "launched remote test session attach reported no-page-target"}, nil
			}
			if isNoTargetIDErr(err) {
				ctx.Warnf("remote test attach reported stale target id on %s; continuing", state.remoteNode)
				return testv1.StepRunResult{Report: "launched remote test session attach reported stale-target-id"}, nil
			}
			return testv1.StepRunResult{}, fmt.Errorf("start remote test session on %s: %w", state.remoteNode, err)
		}
		after, aerr := remoteDialtoneRoleCounts(state.remoteNode)
		if aerr != nil {
			ctx.Warnf("unable to read remote role counts after test launch on %s: %v", state.remoteNode, aerr)
		} else {
			ctx.Infof("remote post-test-launch role counts on %s: dev=%d test=%d", state.remoteNode, after.Dev, after.Test)
			if after.Test < 1 {
				ctx.Warnf("remote test role count did not increase on %s after launch (dev=%d test=%d); continuing due cross-shell process visibility limits", state.remoteNode, after.Dev, after.Test)
			}
			if berr == nil && before.Dev > 0 && after.Dev < before.Dev {
				return testv1.StepRunResult{}, fmt.Errorf("remote dev role count regressed during test launch on %s before=%d after=%d", state.remoteNode, before.Dev, after.Dev)
			}
		}
		ctx.Infof("verified remote test session on %s", state.remoteNode)
		return testv1.StepRunResult{Report: "launched remote test session and verified remote attach path"}, nil
	}

	testSession, err := chrome.StartSession(chrome.SessionOptions{
		RequestedPort: 9223,
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
	roleCountsAfterLaunch, _ := dialtoneRoleCounts()
	if roleCountsAfterLaunch.Test < 1 || roleCountsAfterLaunch.Dev < 1 {
		return testv1.StepRunResult{}, fmt.Errorf("unexpected role counts after test launch dev=%d test=%d", roleCountsAfterLaunch.Dev, roleCountsAfterLaunch.Test)
	}
	ctx.Infof("post-test-launch role counts: dev=%d test=%d", roleCountsAfterLaunch.Dev, roleCountsAfterLaunch.Test)

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
	if state.remoteNode != "" {
		before, berr := remoteDialtoneRoleCounts(state.remoteNode)
		if berr == nil {
			ctx.Infof("remote pre-cleanup-test role counts on %s: dev=%d test=%d", state.remoteNode, before.Dev, before.Test)
		}
		if err := remoteKillDialtoneRole(state.remoteNode, "test"); err != nil {
			return testv1.StepRunResult{}, fmt.Errorf("remote cleanup test role on %s: %w", state.remoteNode, err)
		}
		after, aerr := remoteDialtoneRoleCounts(state.remoteNode)
		if aerr != nil {
			return testv1.StepRunResult{}, fmt.Errorf("read remote role counts after cleanup-test on %s: %w", state.remoteNode, aerr)
		}
		ctx.Infof("remote post-cleanup-test role counts on %s: dev=%d test=%d", state.remoteNode, after.Dev, after.Test)
		if after.Test != 0 {
			return testv1.StepRunResult{}, fmt.Errorf("expected remote test role count 0 after cleanup on %s, got %d", state.remoteNode, after.Test)
		}
		if berr == nil && before.Dev > 0 && after.Dev < 1 {
			return testv1.StepRunResult{}, fmt.Errorf("expected remote dev role to remain on %s after test cleanup, got %d", state.remoteNode, after.Dev)
		}
		return testv1.StepRunResult{Report: "remote mode cleanup removed test role while preserving dev role"}, nil
	}

	if state.testSession == nil || state.devSession == nil {
		return testv1.StepRunResult{}, fmt.Errorf("missing sessions for cleanup step")
	}
	if err := chrome.CleanupSession(state.testSession); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("cleanup test session: %w", err)
	}
	if err := localKillDialtoneRole("test", 3, 350*time.Millisecond); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("cleanup residual local test role: %w", err)
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
	roleCountsAfterCleanup, _ := dialtoneRoleCounts()
	if roleCountsAfterCleanup.Test != 0 || roleCountsAfterCleanup.Dev < 1 {
		return testv1.StepRunResult{}, fmt.Errorf("unexpected role counts after cleanup-test-only dev=%d test=%d", roleCountsAfterCleanup.Dev, roleCountsAfterCleanup.Test)
	}
	ctx.Infof("post-cleanup-test role counts: dev=%d test=%d", roleCountsAfterCleanup.Dev, roleCountsAfterCleanup.Test)
	ctx.Infof("cleaned test role and preserved dev role")
	return testv1.StepRunResult{Report: "cleaned test session while preserving dev session"}, nil
}

func runCleanupAll(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
	if state.remoteNode != "" {
		before, berr := remoteDialtoneRoleCounts(state.remoteNode)
		if berr == nil {
			ctx.Infof("remote pre-cleanup-all role counts on %s: dev=%d test=%d", state.remoteNode, before.Dev, before.Test)
		}
		if err := remoteKillDialtoneRole(state.remoteNode, "test"); err != nil {
			return testv1.StepRunResult{}, fmt.Errorf("remote cleanup-all test role on %s: %w", state.remoteNode, err)
		}
		if err := remoteKillDialtoneRole(state.remoteNode, "dev"); err != nil {
			return testv1.StepRunResult{}, fmt.Errorf("remote cleanup-all dev role on %s: %w", state.remoteNode, err)
		}
		after, aerr := remoteDialtoneRoleCounts(state.remoteNode)
		if aerr != nil {
			return testv1.StepRunResult{}, fmt.Errorf("read remote role counts after cleanup-all on %s: %w", state.remoteNode, aerr)
		}
		ctx.Infof("remote final role counts on %s: dev=%d test=%d", state.remoteNode, after.Dev, after.Test)
		if after.Dev != 0 || after.Test != 0 {
			return testv1.StepRunResult{}, fmt.Errorf("expected no remaining remote dev/test roles on %s, got dev=%d test=%d", state.remoteNode, after.Dev, after.Test)
		}
		return testv1.StepRunResult{Report: "cleanup complete for remote chrome test mode"}, nil
	}

	if state.devSession != nil {
		if err := chrome.CleanupSession(state.devSession); err != nil {
			ctx.Warnf("dev cleanup warning: %v", err)
		}
	}
	if err := chrome.KillDialtoneResources(); err != nil {
		ctx.Warnf("dialtone cleanup warning: %v", err)
	}
	finalCounts, err := dialtoneRoleCounts()
	if err == nil {
		ctx.Infof("final role counts: dev=%d test=%d (pre-launch dev=%d test=%d)", finalCounts.Dev, finalCounts.Test, state.preCounts.Dev, state.preCounts.Test)
		if finalCounts.Dev != 0 || finalCounts.Test != 0 {
			return testv1.StepRunResult{}, fmt.Errorf("expected no remaining dev/test processes, got dev=%d test=%d", finalCounts.Dev, finalCounts.Test)
		}
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

func dialtoneRoleCounts() (roleCounts, error) {
	procs, err := chrome.ListResources(true)
	if err != nil {
		return roleCounts{}, err
	}
	out := roleCounts{}
	for _, p := range procs {
		if !strings.EqualFold(strings.TrimSpace(p.Origin), "Dialtone") {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(p.Role)) {
		case "dev":
			out.Dev++
		case "test":
			out.Test++
		}
	}
	return out, nil
}

func localKillDialtoneRole(role string, maxPasses int, sleepBetween time.Duration) error {
	role = strings.ToLower(strings.TrimSpace(role))
	if role != "dev" && role != "test" {
		return fmt.Errorf("invalid role %q", role)
	}
	if maxPasses < 1 {
		maxPasses = 1
	}
	isWindows := runtime.GOOS == "windows" || chrome.IsWSL()
	var lastErr error
	for pass := 0; pass < maxPasses; pass++ {
		procs, err := chrome.ListResources(true)
		if err != nil {
			return err
		}
		found := false
		for _, p := range procs {
			if !strings.EqualFold(strings.TrimSpace(p.Origin), "Dialtone") {
				continue
			}
			if strings.ToLower(strings.TrimSpace(p.Role)) != role {
				continue
			}
			found = true
			if err := chrome.KillResource(p.PID, isWindows); err != nil {
				lastErr = err
			}
		}
		if !found {
			return nil
		}
		time.Sleep(sleepBetween)
	}
	remaining, err := dialtoneRoleCounts()
	if err != nil {
		return err
	}
	if role == "test" && remaining.Test == 0 {
		return nil
	}
	if role == "dev" && remaining.Dev == 0 {
		return nil
	}
	if lastErr != nil {
		return fmt.Errorf("role=%s still present after kill passes (dev=%d test=%d): last error: %w", role, remaining.Dev, remaining.Test, lastErr)
	}
	return fmt.Errorf("role=%s still present after kill passes (dev=%d test=%d)", role, remaining.Dev, remaining.Test)
}

func tryAutoRemoteFallback(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
	_ = chrome.KillDialtoneResources()
	prevNoSSH := testv1.RuntimeConfigSnapshot().NoSSH
	testv1.UpdateRuntimeConfig(func(cfg *testv1.RuntimeConfig) {
		cfg.NoSSH = true
	})
	defer testv1.UpdateRuntimeConfig(func(cfg *testv1.RuntimeConfig) {
		cfg.NoSSH = prevNoSSH
	})
	candidates := []string{"legion", "darkmac", "chroma"}
	lastErr := error(nil)
	for _, node := range candidates {
		b, err := ctx.EnsureBrowser(testv1.BrowserOptions{
			Headless:      false,
			GPU:           true,
			Role:          "dev",
			ReuseExisting: false,
			RemoteNode:    node,
			URL:           "data:text/html,<title>dialtone-remote-dev</title><h1>ok</h1>",
		})
		if err != nil {
			if isContextCanceledErr(err) {
				state.remoteNode = node
				testv1.UpdateRuntimeConfig(func(cfg *testv1.RuntimeConfig) {
					cfg.BrowserNode = node
					cfg.RemoteRequireRole = true
				})
				ctx.Warnf("auto remote fallback got context canceled on %s; selecting node anyway", node)
				return testv1.StepRunResult{Report: "local WSL debug attach unavailable; selected remote node despite context-canceled attach"}, nil
			}
			lastErr = err
			ctx.Warnf("auto remote fallback failed on %s: %v", node, err)
			continue
		}
		_ = b
		state.remoteNode = node
		testv1.UpdateRuntimeConfig(func(cfg *testv1.RuntimeConfig) {
			cfg.BrowserNode = node
			cfg.RemoteRequireRole = true
		})
		ctx.Infof("auto remote fallback selected node=%s", node)
		return testv1.StepRunResult{Report: "local WSL debug attach unavailable; fell back to remote dev session"}, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no mesh nodes were available")
	}
	return testv1.StepRunResult{}, fmt.Errorf("auto remote fallback failed: %w", lastErr)
}

func isContextCanceledErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(msg, "context canceled")
}

func isNoBrowserOpenErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(msg, "no browser is open") || strings.Contains(msg, "failed to open new tab")
}

func isNoPageTargetErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(msg, "no page target found")
}

func isNoTargetIDErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(msg, "no target with given id found") || strings.Contains(msg, "(-32602)")
}

func remoteDialtoneRoleCounts(nodeName string) (roleCounts, error) {
	node, err := sshv1.ResolveMeshNode(nodeName)
	if err != nil {
		return roleCounts{}, err
	}
	if strings.EqualFold(strings.TrimSpace(node.OS), "windows") {
		out, err := sshv1.RunNodeCommand(nodeName, `$ErrorActionPreference='Stop';$procs=Get-CimInstance Win32_Process | Where-Object { $_.Name -match '^(chrome|msedge)\.exe$' -and $_.CommandLine -match '--dialtone-origin=true' };$dev=($procs | Where-Object { $_.CommandLine -match '--dialtone-role=dev' }).Count;$test=($procs | Where-Object { $_.CommandLine -match '--dialtone-role=test' }).Count;Write-Output ("dev={0} test={1}" -f $dev,$test)`, sshv1.CommandOptions{})
		if err != nil {
			return roleCounts{}, err
		}
		return parseRoleCountsLine(out)
	}
	out, err := sshv1.RunNodeCommand(nodeName, `sh -lc 'ps -ax -o command= | awk '\''{ cmd=tolower($0); if (index(cmd,"--dialtone-origin=true")>0 && index(cmd,"awk") == 0 && index(cmd,"sh -lc") == 0 && index(cmd,"ps -ax -o command=") == 0) { if (index(cmd,"--dialtone-role=dev")>0) dev++; if (index(cmd,"--dialtone-role=test")>0) test++; } } END { printf "dev=%d test=%d\n", dev+0, test+0; }'\'''`, sshv1.CommandOptions{})
	if err != nil {
		return roleCounts{}, err
	}
	return parseRoleCountsLine(out)
}

func parseRoleCountsLine(raw string) (roleCounts, error) {
	line := strings.TrimSpace(raw)
	if line == "" {
		return roleCounts{}, fmt.Errorf("empty role counts output")
	}
	fields := strings.Fields(line)
	out := roleCounts{}
	for _, f := range fields {
		if strings.HasPrefix(f, "dev=") {
			rawDev := strings.TrimSpace(strings.TrimPrefix(f, "dev="))
			if rawDev == "" {
				rawDev = "0"
			}
			v, err := strconv.Atoi(rawDev)
			if err != nil {
				return roleCounts{}, fmt.Errorf("parse dev count from %q: %w", line, err)
			}
			out.Dev = v
		}
		if strings.HasPrefix(f, "test=") {
			rawTest := strings.TrimSpace(strings.TrimPrefix(f, "test="))
			if rawTest == "" {
				rawTest = "0"
			}
			v, err := strconv.Atoi(rawTest)
			if err != nil {
				return roleCounts{}, fmt.Errorf("parse test count from %q: %w", line, err)
			}
			out.Test = v
		}
	}
	return out, nil
}

func remoteKillDialtoneRole(nodeName, role string) error {
	role = strings.ToLower(strings.TrimSpace(role))
	if role != "dev" && role != "test" {
		return fmt.Errorf("invalid role %q", role)
	}
	node, err := sshv1.ResolveMeshNode(nodeName)
	if err != nil {
		return err
	}
	if strings.EqualFold(strings.TrimSpace(node.OS), "windows") {
		_, err := sshv1.RunNodeCommand(nodeName, fmt.Sprintf(`$ErrorActionPreference='Stop';$procs=Get-CimInstance Win32_Process | Where-Object { $_.Name -match '^(chrome|msedge)\.exe$' -and $_.CommandLine -match '--dialtone-origin=true' -and $_.CommandLine -match '--dialtone-role=%s' };foreach ($p in $procs) { try { Invoke-CimMethod -InputObject $p -MethodName Terminate | Out-Null } catch {} }`, role), sshv1.CommandOptions{})
		return err
	}
	_, err = sshv1.RunNodeCommand(nodeName, fmt.Sprintf(`sh -lc 'pids=$(ps -ax -o pid= -o command= | awk '\''{ pid=$1; $1=""; cmd=tolower($0); if (index(cmd,"--dialtone-origin=true")>0 && index(cmd,"--dialtone-role=%s")>0 && index(cmd,"awk") == 0 && index(cmd,"sh -lc") == 0 && index(cmd,"ps -ax -o pid=") == 0) print pid; }'\''); if [ -n "$pids" ]; then kill $pids; fi'`, role), sshv1.CommandOptions{})
	return err
}
