package browserlifecycleoptions

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"time"

	chrome "dialtone/dev/plugins/chrome/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
	"github.com/chromedp/chromedp"
)

var pageURL string

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{Name: "browser-lifecycle-setup-options", Timeout: 40 * time.Second, RunWithContext: runSetup})
	r.Add(testv1.Step{Name: "browser-lifecycle-reuse-shared-session", Timeout: 20 * time.Second, RunWithContext: runReuse})
}

func runSetup(sc *testv1.StepContext) (testv1.StepRunResult, error) {
	if chrome.FindChromePath() == "" {
		sc.Warnf("chrome not found; skipping browser lifecycle options")
		return testv1.StepRunResult{Report: "skipped browser lifecycle options (chrome not installed)"}, nil
	}
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return testv1.StepRunResult{}, fmt.Errorf("unable to resolve caller path")
	}
	srv := httptest.NewServer(http.FileServer(http.Dir(filepath.Dir(thisFile))))
	pageURL = srv.URL + "/index.html"
	defer srv.Close()

	_, err := sc.EnsureBrowser(testv1.BrowserOptions{
		Headless:      true,
		GPU:           false,
		Role:          "test",
		ReuseExisting: false,
		UserDataDir:   ".chrome_data/testv1-shared-session",
		URL:           pageURL,
	})
	if err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForAriaLabel("Option Button", 8*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForBrowserMessageAfterAction("option-clicked", 5*time.Second, func() error {
		return sc.ClickAriaLabelAfterWait("Option Button", 5*time.Second)
	}); err != nil {
		return testv1.StepRunResult{}, err
	}
	if err := sc.WaitForAriaLabelAttrEquals("Lifecycle Status", "data-state", "clicked", 5*time.Second); err != nil {
		return testv1.StepRunResult{}, err
	}
	return testv1.StepRunResult{Report: "browser options + aria-click helper verified"}, nil
}

func runReuse(sc *testv1.StepContext) (testv1.StepRunResult, error) {
	if chrome.FindChromePath() == "" {
		return testv1.StepRunResult{Report: "skipped browser lifecycle reuse (chrome not installed)"}, nil
	}
	b, err := sc.EnsureBrowser(testv1.BrowserOptions{})
	if err != nil {
		return testv1.StepRunResult{}, err
	}
	var marker string
	if err := b.Run(chromedp.Evaluate(`window.__suiteMarker || ''`, &marker)); err != nil {
		return testv1.StepRunResult{}, err
	}
	if marker != "alive" {
		return testv1.StepRunResult{}, fmt.Errorf("expected shared browser marker 'alive', got %q", marker)
	}
	if err := sc.WaitForBrowserMessageAfterAction("shared-session-ok", 5*time.Second, func() error {
		return sc.RunBrowserWithTimeout(5*time.Second, chromedp.Evaluate(`console.log('shared-session-ok')`, nil))
	}); err != nil {
		return testv1.StepRunResult{}, err
	}
	return testv1.StepRunResult{Report: "shared suite browser session reuse verified across steps"}, nil
}
