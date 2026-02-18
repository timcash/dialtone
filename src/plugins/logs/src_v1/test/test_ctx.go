package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	test_v2 "dialtone/cli/src/libs/test_v2"
)

type testCtx struct {
	sharedServer     *exec.Cmd
	sharedBrowser    *test_v2.BrowserSession
	attachMode       bool
	baseURL          string
	devBaseURL       string
	clickGap         time.Duration
	maxClicksPerStep int
	stepCtx          *test_v2.StepContext
}

func newTestCtx() *testCtx {
	attach := os.Getenv("LOGS_TEST_ATTACH") == "1"
	base := strings.TrimSpace(os.Getenv("LOGS_TEST_BASE_URL"))
	devBase := strings.TrimSpace(os.Getenv("LOGS_TEST_DEV_BASE_URL"))
	cpsRaw := strings.TrimSpace(os.Getenv("LOGS_TEST_CPS"))
	cps := 3
	if cpsRaw != "" {
		if parsed, err := strconv.Atoi(cpsRaw); err == nil && parsed >= 1 {
			cps = parsed
		}
	}
	if base == "" {
		if attach {
			base = "http://127.0.0.1:3000"
		} else {
			base = "http://127.0.0.1:8080"
		}
	}
	if devBase == "" {
		devBase = "http://127.0.0.1:3000"
	}
	base = strings.TrimRight(base, "/")
	devBase = strings.TrimRight(devBase, "/")
	return &testCtx{
		attachMode:       attach,
		baseURL:          base,
		devBaseURL:       devBase,
		clickGap:         time.Second / time.Duration(cps),
		maxClicksPerStep: 4,
	}
}

func (t *testCtx) beginStep(sc *test_v2.StepContext) {
	t.stepCtx = sc
}

func (t *testCtx) ensureSharedServer() error {
	if t.sharedServer != nil {
		return nil
	}
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}
	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "logs", "serve", "src_v1")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := test_v2.WaitForPort(8080, 12*time.Second); err != nil {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		return err
	}
	t.sharedServer = cmd
	return nil
}

func (t *testCtx) ensureSharedBrowser() (*test_v2.BrowserSession, error) {
	if err := t.ensureSharedServer(); err != nil {
		return nil, err
	}
	if t.sharedBrowser != nil {
		return t.sharedBrowser, nil
	}
	url := t.baseURL + "/#logs-log-xterm"
	s, err := test_v2.StartBrowser(test_v2.BrowserOptions{
		Headless:      true,
		Role:          "logs-test",
		ReuseExisting: false,
		URL:           url,
		LogWriter:     nil,
		LogPrefix:     "[BROWSER]",
	})
	if err != nil {
		return nil, err
	}
	t.sharedBrowser = s
	return t.sharedBrowser, nil
}
