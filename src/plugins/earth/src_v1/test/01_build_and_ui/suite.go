package buildandui

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

type Options struct {
	AttachNode string
	TargetURL  string
}

func Register(reg *testv1.Registry, opts Options) {
	reg.Add(testv1.Step{
		Name:    "ui-quality-fmt-lint-build",
		Timeout: 120 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			logs := []string{"install", "format", "build"}
			for _, cmd := range logs {
				if err := ctx.WaitForStepMessageAfterAction("earth-cmd-ok", 60*time.Second, func() error {
					ctx.Infof("[ACTION] earth src_v1 %s", cmd)
					if err := runDialtone(ctx.RepoRoot(), "earth", "src_v1", cmd); err != nil {
						return err
					}
					ctx.Infof("earth-cmd-ok")
					return nil
				}); err != nil {
					return testv1.StepRunResult{}, err
				}
			}
			return testv1.StepRunResult{Report: "earth quality checks (install/format/build) passed"}, nil
		},
	})

	reg.Add(testv1.Step{
		Name:    "ui-browser-smoke-hero",
		Timeout: 90 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			attachNode := strings.TrimSpace(opts.AttachNode)
			targetURL := strings.TrimSpace(opts.TargetURL)
			
			localURL := "http://127.0.0.1:8891"
			if targetURL == "" {
				targetURL = localURL
			}

			if err := testv1.WaitForPort(8891, 1*time.Second); err != nil {
				ctx.Infof("[ACTION] starting earth server")
				go func() {
					_ = runDialtone(ctx.RepoRoot(), "earth", "src_v1", "serve", "--addr", ":8891")
				}()
				if err := testv1.WaitForPort(8891, 15*time.Second); err != nil {
					return testv1.StepRunResult{}, fmt.Errorf("earth server failed to start: %w", err)
				}
			}

			browserOpts := testv1.BrowserOptions{
				Role:          "earth-test",
				URL:           targetURL,
				RemoteNode:    attachNode,
				Headless:      attachNode == "",
				GPU:           true,
				ReuseExisting: true,
			}

			if _, err := ctx.EnsureBrowser(browserOpts); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("ensure browser: %w", err)
			}

			if err := ctx.WaitForConsoleContains("[SectionManager] RESUME #earth-hero-stage", 30*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}

			if err := ctx.WaitForAriaLabel("Hero Section", 10*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}

			if err := ctx.CaptureScreenshot("earth_hero_smoke.png"); err != nil {
				ctx.Warnf("failed to capture screenshot: %v", err)
			}

			return testv1.StepRunResult{Report: "earth hero section verified via chrome src_v3"}, nil
		},
	})
}

func runDialtone(repoRoot string, args ...string) error {
	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), args...)
	cmd.Dir = repoRoot
	return cmd.Run()
}
