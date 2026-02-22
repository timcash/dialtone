package browserctx

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	chrome "dialtone/dev/plugins/chrome/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
	"github.com/chromedp/chromedp"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name:           "browser-stepcontext-aria-and-console",
		Timeout:        45 * time.Second,
		RunWithContext: runBrowserCtxSmoke,
	})
}

func runBrowserCtxSmoke(sc *testv1.StepContext) (testv1.StepRunResult, error) {
	if chrome.FindChromePath() == "" {
		sc.Warnf("chrome not found; skipping browser ctx smoke")
		return testv1.StepRunResult{Report: "skipped browser ctx smoke (chrome not installed)"}, nil
	}

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return testv1.StepRunResult{}, fmt.Errorf("unable to resolve caller path")
	}
	pageDir := filepath.Dir(thisFile)
	srv := httptest.NewServer(http.FileServer(http.Dir(pageDir)))
	defer srv.Close()
	pageURL := srv.URL + "/index.html"

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
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForAriaLabelAttrEquals("Status", "data-state", "done", 5*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	var pt []float64
	if err := sc.RunBrowser(chromedp.Evaluate(`(() => {
		const el = document.querySelector("[aria-label='Tap Area']");
		if (!el) return [];
		const r = el.getBoundingClientRect();
		return [Math.floor(r.left + r.width / 2), Math.floor(r.top + r.height / 2)];
	})()`, &pt)); err != nil {
		return testv1.StepRunResult{}, err
	}
	if len(pt) != 2 {
		return testv1.StepRunResult{}, fmt.Errorf("unable to resolve tap area coordinates")
	}
	if err := sc.WaitForBrowserMessageAfterAction("coord-hit-1", 5*time.Second, func() error {
		return sc.ClickAt(pt[0], pt[1])
	}); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForBrowserMessageAfterAction("coord-hit-2", 5*time.Second, func() error {
		return sc.TapAt(pt[0], pt[1])
	}); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForAriaLabelAttrEquals("Status", "data-coord-hits", "2", 5*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.TypeAriaLabel("Search Input", "dialtone"); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForBrowserMessageAfterAction("search-enter:dialtone", 5*time.Second, func() error {
		return sc.PressEnterAriaLabel("Search Input")
	}); err != nil {
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

	return testv1.StepRunResult{Report: "StepContext browser API verified: aria wait timeout, aria click, type+enter, coordinate click/tap, browser console logs via NATS waits"}, nil
}
