package browsersmoke

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	cadv1 "dialtone/dev/plugins/cad/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name:    "cad-ui-browser-smoke-src-v1",
		Timeout: 70 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			if !testv1.BrowserProviderAvailable() {
				ctx.Warnf("browser provider not available; use --attach <node> for remote mode")
				return testv1.StepRunResult{Report: "skipped CAD UI browser smoke (browser provider unavailable)"}, nil
			}

			baseURL, cleanup, err := ensureCADServer(ctx)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer cleanup()

			_, err = ctx.EnsureBrowser(testv1.BrowserOptions{
				Headless:      false,
				GPU:           true,
				Role:          browserRole(),
				ReuseExisting: false,
				URL:           baseURL,
			})
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("ensure browser: %w", err)
			}

			if err := ctx.WaitForAriaLabel("CAD Stage", 15*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabel("CAD Mode Form", 15*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabel("CAD Model Status", 15*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabelAttrEquals("CAD Stage", "data-model-state", "ready", 25*time.Second); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("cad model never reached ready state: %w", err)
			}
			if err := ctx.WaitForConsoleContains("cad-model-ready:", 15*time.Second); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("cad model ready console marker missing: %w", err)
			}
			if err := clickAndWaitReady(ctx, "CAD Thumb 1", "cad-model-ready:2"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := clickAndWaitReady(ctx, "CAD Thumb 3", "cad-model-ready:3"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := clickAndWaitReady(ctx, "CAD Thumb 5", "cad-model-ready:4"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := clickAndWaitReady(ctx, "CAD Thumb 7", "cad-model-ready:5"); err != nil {
				return testv1.StepRunResult{}, err
			}

			if err := ctx.CaptureScreenshot(filepath.Join("plugins", "cad", "src_v1", "screenshots", "cad_ui_browser_smoke.png")); err != nil {
				ctx.Warnf("screenshot capture failed: %v", err)
			}

			for _, entry := range ctx.Session.Entries() {
				if strings.Contains(strings.ToLower(entry.Text), "regenerate failed") {
					return testv1.StepRunResult{}, fmt.Errorf("cad regenerate failure detected in browser console: %s", entry.Text)
				}
				if entry.Type == "exception" {
					return testv1.StepRunResult{}, fmt.Errorf("browser exception detected: %s", entry.Text)
				}
				if entry.Type == "error" {
					return testv1.StepRunResult{}, fmt.Errorf("browser console error detected: %s", entry.Text)
				}
				if strings.Contains(strings.ToLower(entry.Text), "failed to load resource") {
					return testv1.StepRunResult{}, fmt.Errorf("resource load error detected: %s", entry.Text)
				}
			}

			return testv1.StepRunResult{Report: "CAD UI loaded in chrome src_v3 with no browser exceptions and STL model reached ready state"}, nil
		},
	})
}

func browserRole() string {
	role := strings.TrimSpace(testv1.RuntimeConfigSnapshot().RemoteBrowserRole)
	if role == "" {
		return "dev"
	}
	return role
}

func clickAndWaitReady(ctx *testv1.StepContext, label string, expectedMarker string) error {
	if err := ctx.ClickAriaLabel(label); err != nil {
		return fmt.Errorf("button %s click failed: %w", label, err)
	}
	if err := ctx.WaitForConsoleContains(expectedMarker, 15*time.Second); err != nil {
		return fmt.Errorf("button %s did not emit %s: %w", label, expectedMarker, err)
	}
	time.Sleep(250 * time.Millisecond)
	return nil
}

func ensureCADServer(ctx *testv1.StepContext) (string, func(), error) {
	if err := ensureCADUIBuilt(ctx); err != nil {
		return "", nil, err
	}
	paths, err := cadv1.ResolvePaths("", "src_v1")
	if err != nil {
		return "", nil, err
	}
	handler := cadv1.NewHandler(paths)
	ln, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		return "", nil, err
	}
	srv := &http.Server{Handler: handler}
	go func() {
		_ = srv.Serve(ln)
	}()
	addr := ln.Addr().(*net.TCPAddr)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", addr.Port)
	if err := waitHTTP(baseURL+"/health", 10*time.Second); err != nil {
		_ = srv.Close()
		return "", nil, err
	}
	ctx.Infof("cad browser smoke server ready at %s", baseURL)
	cleanup := func() {
		_ = srv.Close()
	}
	return baseURL, cleanup, nil
}

func ensureCADUIBuilt(ctx *testv1.StepContext) error {
	paths, err := cadv1.ResolvePaths("", "src_v1")
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(paths.UIDist, "index.html")); err == nil {
		return nil
	}
	bunBin := filepath.Join(paths.Runtime.DialtoneEnv, "bun", "bin", "bun")
	if _, err := os.Stat(bunBin); err != nil {
		bunBin = "bun"
	}
	for _, args := range [][]string{{"install"}, {"run", "build"}} {
		cmd := exec.Command(bunBin, args...)
		cmd.Dir = paths.UIDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		ctx.Infof("cad browser smoke running %s %s", bunBin, strings.Join(args, " "))
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("cad ui command failed (%s %s): %w", bunBin, strings.Join(args, " "), err)
		}
	}
	return nil
}

func waitHTTP(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode < 500 {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %s", url)
}
