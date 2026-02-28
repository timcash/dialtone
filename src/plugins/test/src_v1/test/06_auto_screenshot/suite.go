package autoscreenshot

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
	"github.com/chromedp/chromedp"
)

var (
	autoShotPath    string
	stepUsedBrowser bool
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name:           "auto-screenshot-uses-browser",
		Timeout:        25 * time.Second,
		RunWithContext: runUseBrowser,
	})
	r.Add(testv1.Step{
		Name:           "auto-screenshot-file-exists",
		Timeout:        10 * time.Second,
		RunWithContext: runVerifyShot,
	})
}

func runUseBrowser(sc *testv1.StepContext) (testv1.StepRunResult, error) {
	autoShotPath = expectedAutoScreenshotPath()
	stepUsedBrowser = false
	if !testv1.BrowserProviderAvailable() {
		sc.Warnf("browser provider not available; skipping auto screenshot test step")
		return testv1.StepRunResult{Report: "skipped auto screenshot setup (browser unavailable)"}, nil
	}
	_, err := sc.EnsureBrowser(testv1.BrowserOptions{
		Headless:      true,
		GPU:           false,
		Role:          "test",
		ReuseExisting: false,
		URL:           "data:text/html,<title>auto-shot</title><h1>ok</h1>",
	})
	if err != nil {
		return testv1.StepRunResult{}, err
	}
	stepUsedBrowser = true
	var title string
	if err := sc.RunBrowserWithTimeout(5*time.Second, chromedp.Title(&title)); err != nil {
		return testv1.StepRunResult{}, err
	}
	if title != "auto-shot" {
		return testv1.StepRunResult{}, fmt.Errorf("unexpected title %q", title)
	}
	return testv1.StepRunResult{Report: "browser used; auto screenshot should be captured after step"}, nil
}

func expectedAutoScreenshotPath() string {
	if rt, err := configv1.ResolveRuntime(""); err == nil && rt.RepoRoot != "" {
		return filepath.Join(rt.RepoRoot, "src", "plugins", "test", "src_v1", "screenshots", "auto_auto-screenshot-uses-browser.png")
	}
	return filepath.Join("src", "plugins", "test", "src_v1", "screenshots", "auto_auto-screenshot-uses-browser.png")
}

func runVerifyShot(sc *testv1.StepContext) (testv1.StepRunResult, error) {
	if !stepUsedBrowser {
		return testv1.StepRunResult{Report: "skipped auto screenshot verification (browser step skipped)"}, nil
	}
	if _, err := os.Stat(autoShotPath); err != nil {
		legacy := filepath.Join("test_report", "screenshots", "auto_auto-screenshot-uses-browser.png")
		if _, legacyErr := os.Stat(legacy); legacyErr == nil {
			return testv1.StepRunResult{Report: "auto screenshot file exists (legacy path)"}, nil
		}
		return testv1.StepRunResult{}, fmt.Errorf("expected auto screenshot missing: %s (%w)", autoShotPath, err)
	}
	return testv1.StepRunResult{Report: "auto screenshot file exists"}, nil
}
