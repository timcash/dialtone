package buildandui

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

type Options struct {
	AttachNode string
	TargetURL  string
}

func Register(reg *testv1.Registry, opts Options) {
	reg.Add(testv1.Step{
		Name:    "01-build-earth-ui-and-binary",
		Timeout: 90 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			if err := ctx.WaitForStepMessageAfterAction("earth-build-ok", 70*time.Second, func() error {
				ctx.Infof("[ACTION] run earth src_v1 build")
				cmd := exec.Command("bash", "-lc", "if [ -x ./dialtone.sh ]; then ./dialtone.sh earth src_v1 build; elif [ -x ../dialtone.sh ]; then ../dialtone.sh earth src_v1 build; else echo 'dialtone.sh not found'; exit 1; fi")
				cmd.Dir = ctx.RepoRoot()
				out, err := cmd.CombinedOutput()
				if err != nil {
					ctx.Errorf("earth build failed: %s", strings.TrimSpace(string(out)))
					return err
				}
				ctx.Infof("[ACTION] run earth src_v1 go-build")
				cmd = exec.Command("bash", "-lc", "if [ -x ./dialtone.sh ]; then ./dialtone.sh earth src_v1 go-build; elif [ -x ../dialtone.sh ]; then ../dialtone.sh earth src_v1 go-build; else echo 'dialtone.sh not found'; exit 1; fi")
				cmd.Dir = ctx.RepoRoot()
				out, err = cmd.CombinedOutput()
				if err != nil {
					ctx.Errorf("earth go-build failed: %s", strings.TrimSpace(string(out)))
					return err
				}
				ctx.Infof("earth-build-ok")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForStepMessageAfterAction("earth-dist-contract-ok", 8*time.Second, func() error {
				ctx.Infof("[ACTION] verify earth UI index contains ARIA labels")
				cmd := exec.Command("bash", "-lc", "f=''; for p in src/plugins/earth/src_v1/ui/dist/index.html plugins/earth/src_v1/ui/dist/index.html; do if [ -f \"$p\" ]; then f=\"$p\"; break; fi; done; test -n \"$f\" && grep -q 'aria-label=\"Hero Section\"' \"$f\"")
				cmd.Dir = ctx.RepoRoot()
				out, err := cmd.CombinedOutput()
				if err != nil {
					return fmt.Errorf("dist contract check failed: %s", strings.TrimSpace(string(out)))
				}
				ctx.Infof("earth-dist-contract-ok")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "earth UI and server binary build succeeded"}, nil
		},
	})

	reg.Add(testv1.Step{
		Name:    "02-earth-browser-smoke",
		Timeout: 75 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			attachNode := strings.TrimSpace(opts.AttachNode)
			targetURL := strings.TrimSpace(opts.TargetURL)
			localURL := "http://127.0.0.1:8891"
			if targetURL == "" {
				targetURL = localURL
			}

			var srv *exec.Cmd
			if err := waitHTTPReady(localURL, 1200*time.Millisecond); err != nil {
				srv = exec.Command("bash", "-lc", "if [ -x ./dialtone.sh ]; then ./dialtone.sh earth src_v1 serve --addr :8891; elif [ -x ../dialtone.sh ]; then ../dialtone.sh earth src_v1 serve --addr :8891; else echo 'dialtone.sh not found'; exit 1; fi")
				srv.Dir = ctx.RepoRoot()
				if err := srv.Start(); err != nil {
					return testv1.StepRunResult{}, fmt.Errorf("start earth serve failed: %w", err)
				}
				defer func() {
					_ = srv.Process.Kill()
					_, _ = srv.Process.Wait()
				}()
				if err := waitHTTPReady(localURL, 8*time.Second); err != nil {
					return testv1.StepRunResult{}, fmt.Errorf("earth serve not ready: %w", err)
				}
			}

			if attachNode != "" {
				if strings.Contains(targetURL, "127.0.0.1:8891") || strings.Contains(targetURL, "localhost:8891") {
					if inferred, err := inferWSLURL(8891); err == nil {
						targetURL = inferred
					}
				}
			}

			role := "earth-test"
			headless := true
			reuse := false
			remoteNode := ""
			if attachNode != "" {
				role = "test"
				headless = false
				reuse = true
				remoteNode = attachNode
			} else {
				remoteNode = ""
				if remoteNode != "" {
					// Robot hero requires a real WebGL context; use headed mode on remote hosts.
					headless = false
				}
				if remoteNode != "" && (targetURL == "" || strings.Contains(targetURL, "127.0.0.1:8891") || strings.Contains(targetURL, "localhost:8891")) {
					if inferred, err := inferWSLURL(8891); err == nil {
						targetURL = inferred
					}
				}
			}
			if _, err := ctx.EnsureBrowser(testv1.BrowserOptions{
				Headless:      headless,
				GPU:           true,
				Role:          role,
				ReuseExisting: reuse,
				URL:           targetURL,
				RemoteNode:    remoteNode,
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("ensure browser failed: %w", err)
			}
			if err := ctx.WaitForConsoleContains("NAVIGATE TO #hero", 20*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: fmt.Sprintf("browser smoke passed (role=%s attach=%t)", role, attachNode != "")}, nil
		},
	})
}

func isWSL() bool {
	raw, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	v := strings.ToLower(string(raw))
	return strings.Contains(v, "microsoft")
}

func inferWSLURL(port int) (string, error) {
	wsl, err := sshv1.ResolveMeshNode("wsl")
	if err != nil {
		return "", err
	}
	host := strings.TrimSpace(wsl.Host)
	if host == "" {
		return "", fmt.Errorf("wsl mesh host empty")
	}
	return fmt.Sprintf("http://%s:%d", host, port), nil
}

func waitHTTPReady(rawURL string, timeout time.Duration) error {
	u := strings.TrimSpace(rawURL)
	if u == "" {
		return fmt.Errorf("url is empty")
	}
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 800 * time.Millisecond}
	for time.Now().Before(deadline) {
		resp, err := client.Get(u)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 500 {
				return nil
			}
		} else if isConnRefused(err) {
			// keep retrying
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %s", u)
}

func isConnRefused(err error) bool {
	if err == nil {
		return false
	}
	e := strings.ToLower(err.Error())
	return strings.Contains(e, "connection refused") || strings.Contains(e, "connect:")
}
