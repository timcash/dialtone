package browserctx

import (
	"fmt"
	"net"
	"net/http"
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
		Timeout:        8 * time.Second,
		RunWithContext: runBrowserCtxSmoke,
	})
}

func runBrowserCtxSmoke(sc *testv1.StepContext) (testv1.StepRunResult, error) {
	if !testv1.BrowserProviderAvailable() {
		sc.Warnf("browser provider not available; use --attach <node> for remote mode")
		return testv1.StepRunResult{Report: "skipped browser ctx smoke (chrome not installed)"}, nil
	}
	pageDir := filepath.Dir(mustCallerFile())
	remoteNode := strings.TrimSpace(testv1.RuntimeConfigSnapshot().BrowserNode)
	remoteManaged := remoteNode != ""
	pageURL, fixtureLen, closeFixture, err := startFixtureURL(pageDir, remoteNode)
	if err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("start browser ctx fixture server: %w", err)
	}
	defer closeFixture()

	sc.Infof("browser ctx smoke ensure start remote_managed=%t fixture_len=%d url=%q", remoteManaged, fixtureLen, pageURL)
	_, err = sc.EnsureBrowser(testv1.BrowserOptions{
		Headless:      true,
		GPU:           false,
		Role:          "test",
		ReuseExisting: false,
		URL:           pageURL,
	})
	if err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("ensure browser: %w", err)
	}
	sc.Infof("browser ctx smoke ensure done remote_managed=%t", remoteManaged)

	sc.Infof("browser ctx smoke ready wait start")
	if err := sc.WaitForAriaLabel("Smoke Button", 2500*time.Millisecond); err != nil {
		return testv1.StepRunResult{}, err
	}
	sc.Infof("browser ctx smoke ready wait done")
	if !remoteManaged {
		if err := sc.WaitForAriaLabel("Definitely Missing Label", 500*time.Millisecond); err == nil {
			return testv1.StepRunResult{}, fmt.Errorf("expected wait timeout for missing aria-label")
		}
	}
	sc.Infof("browser ctx smoke type flow start")
	if err := sc.TypeAriaLabel("Search Input", "dialtone"); err != nil {
		return testv1.StepRunResult{}, err
	}
	sc.Infof("browser ctx smoke type done")
	if err := sc.PressEnterAriaLabel("Search Input"); err != nil {
		return testv1.StepRunResult{}, err
	}
	sc.Infof("browser ctx smoke press-enter done")

	return testv1.StepRunResult{Report: "StepContext browser API verified through chrome src_v3 service: real URL navigation, aria wait, type, and press-enter actions"}, nil
}

func mustCallerFile() string {
	_, thisFile, _, ok := runtime.Caller(1)
	if !ok {
		return "."
	}
	return thisFile
}

func startFixtureURL(pageDir, remoteNode string) (string, int, func(), error) {
	indexPath := filepath.Join(pageDir, "index.html")
	info, err := os.Stat(indexPath)
	if err != nil {
		return "", 0, nil, err
	}
	ln, err := net.Listen("tcp4", "0.0.0.0:0")
	if err != nil {
		return "", 0, nil, err
	}
	srv := &http.Server{Handler: http.FileServer(http.Dir(pageDir))}
	go func() {
		_ = srv.Serve(ln)
	}()
	closeFixture := func() {
		_ = srv.Close()
		_ = ln.Close()
	}
	port := ln.Addr().(*net.TCPAddr).Port
	rawURL := fmt.Sprintf("http://127.0.0.1:%d/index.html", port)
	if strings.TrimSpace(remoteNode) == "" {
		return rawURL, int(info.Size()), closeFixture, nil
	}
	rewritten, err := testv1.RewriteBrowserURLForRemoteNode(rawURL, remoteNode)
	if err != nil {
		closeFixture()
		return "", 0, nil, err
	}
	return rewritten, int(info.Size()), closeFixture, nil
}
