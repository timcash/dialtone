package browserctx

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name:           "browser-stepcontext-aria-and-console",
		Timeout:        45 * time.Second,
		RunWithContext: runBrowserCtxSmoke,
	})
}

func runBrowserCtxSmoke(sc *testv1.StepContext) (testv1.StepRunResult, error) {
	if !testv1.BrowserProviderAvailable() {
		sc.Warnf("browser provider not available; use --attach <node> for remote mode")
		return testv1.StepRunResult{Report: "skipped browser ctx smoke (chrome not installed)"}, nil
	}
	pageDir := filepath.Dir(mustCallerFile())
	pageURL := ""
	if strings.TrimSpace(testv1.RuntimeConfigSnapshot().BrowserNode) != "" {
		raw, err := os.ReadFile(filepath.Join(pageDir, "index.html"))
		if err != nil {
			return testv1.StepRunResult{}, fmt.Errorf("read browser ctx fixture: %w", err)
		}
		pageURL = "data:text/html;base64," + base64.StdEncoding.EncodeToString(raw)
	} else {
		srv := httptest.NewServer(http.FileServer(http.Dir(pageDir)))
		defer srv.Close()
		pageURL = srv.URL + "/index.html"
	}

	_, err := sc.EnsureBrowser(testv1.BrowserOptions{
		Headless:      true,
		GPU:           false,
		Role:          "test",
		ReuseExisting: false,
		URL:           pageURL,
	})
	if err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("ensure browser: %w", err)
	}

	if err := sc.WaitForAriaLabel("Smoke Button", 10*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForAriaLabel("Search Input", 10*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForAriaLabel("Search Status", 10*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForAriaLabel("Status", 10*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForAriaLabel("Tap Area", 10*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForAriaLabelAttrEquals("Status", "data-state", "idle", 5*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	// Prove wait timeout behavior by asserting a missing aria label errors within timeout.
	if err := sc.WaitForAriaLabel("Definitely Missing Label", 700*time.Millisecond); err == nil {
		return testv1.StepRunResult{}, fmt.Errorf("expected wait timeout for missing aria-label")
	}
	if err := sc.WaitForBrowserMessageAfterAction("clicked-smoke", 5*time.Second, func() error {
		return sc.ClickAriaLabel("Smoke Button")
	}); err != nil {
		if err := sc.WaitForConsoleContains("clicked-smoke", 5*time.Second); err != nil {
			return testv1.StepRunResult{}, err
		}
	}
	if err := sc.WaitForAriaLabelAttrEquals("Status", "data-state", "done", 5*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.ClickAriaLabel("Tap Area"); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForConsoleContains("coord-hit-1", 5*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.TypeAriaLabel("Search Input", "dialtone"); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.PressEnterAriaLabel("Search Input"); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForConsoleContains("search-enter:dialtone", 5*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForAriaLabelAttrEquals("Search Status", "data-last", "dialtone", 5*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}

	entries := sc.Session.Entries()
	found := false
	for _, e := range entries {
		if strings.Contains(e.Text, "clicked-smoke") {
			found = true
			break
		}
	}
	if !found {
		return testv1.StepRunResult{}, fmt.Errorf("expected clicked-smoke in browser console entries")
	}

	return testv1.StepRunResult{Report: "StepContext browser API verified through chrome src_v3 service: aria wait timeout, goto, aria click, type+enter, screenshots, browser console waits"}, nil
}

func mustCallerFile() string {
	_, thisFile, _, ok := runtime.Caller(1)
	if !ok {
		return "."
	}
	return thisFile
}
