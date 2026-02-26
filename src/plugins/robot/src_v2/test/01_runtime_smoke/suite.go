package runtimesmoke

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
	"github.com/chromedp/chromedp"
	"github.com/coder/websocket"
	"github.com/nats-io/nats.go"
)

func Register(reg *testv1.Registry) {
	reg.Add(testv1.Step{
		Name:    "01-build-robot-v2-binary",
		Timeout: 30 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			if err := ctx.WaitForStepMessageAfterAction("build complete", 20*time.Second, func() error {
				ctx.Infof("[ACTION] build robot src_v2 server binary")
				cmd := exec.Command("./dialtone.sh", "go", "src_v1", "exec", "build", "-o", "../bin/dialtone_robot_v2", "./plugins/robot/src_v2/cmd/server/main.go")
				cmd.Dir = repoRoot()
				out, err := cmd.CombinedOutput()
				if err != nil {
					ctx.Errorf("build failed: %s", strings.TrimSpace(string(out)))
					return err
				}
				ctx.Infof("build complete")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "binary build verified"}, nil
		},
	})

	reg.Add(testv1.Step{
		Name:    "02-server-health-and-root-behavior",
		Timeout: 25 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			repo := repoRoot()
			binPath := filepath.Join(repo, "bin", "dialtone_robot_v2")
			port := "18082"
			baseURL := "http://127.0.0.1:" + port

			cmd := exec.Command(
				binPath,
				"--listen", ":"+port,
				"--nats-port", "18222",
				"--nats-ws-port", "18223",
			)
			cmd.Dir = repo
			if err := cmd.Start(); err != nil {
				return testv1.StepRunResult{}, err
			}
			defer func() {
				_ = cmd.Process.Kill()
				_, _ = cmd.Process.Wait()
			}()

			if err := ctx.WaitForStepMessageAfterAction("health ok", 8*time.Second, func() error {
				ctx.Infof("[ACTION] probe /health on %s", baseURL)
				deadline := time.Now().Add(6 * time.Second)
				for time.Now().Before(deadline) {
					resp, err := http.Get(baseURL + "/health")
					if err == nil {
						_ = resp.Body.Close()
						if resp.StatusCode == http.StatusOK {
							ctx.Infof("health ok")
							return nil
						}
					}
					time.Sleep(150 * time.Millisecond)
				}
				ctx.Errorf("health endpoint did not become ready")
				return fmt.Errorf("health endpoint not ready")
			}); err != nil {
				return testv1.StepRunResult{}, err
			}

			if err := ctx.WaitForStepMessageAfterAction("root behavior verified", 5*time.Second, func() error {
				ctx.Infof("[ACTION] probe / expecting 200 (ui dist present) or 503 (scaffold)")
				resp, err := http.Get(baseURL + "/")
				if err != nil {
					ctx.Errorf("root probe failed: %v", err)
					return err
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusServiceUnavailable {
					ctx.Errorf("unexpected root status: %d", resp.StatusCode)
					return fmt.Errorf("expected 200 or 503, got %d", resp.StatusCode)
				}
				ctx.Infof("root behavior verified")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}

			if err := ctx.WaitForStepMessageAfterAction("api init returned wsPath", 5*time.Second, func() error {
				ctx.Infof("[ACTION] probe /api/init scaffold payload")
				resp, err := http.Get(baseURL + "/api/init")
				if err != nil {
					ctx.Errorf("api/init probe failed: %v", err)
					return err
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					ctx.Errorf("unexpected /api/init status: %d", resp.StatusCode)
					return fmt.Errorf("expected 200 from /api/init, got %d", resp.StatusCode)
				}
				bodyBytes, _ := io.ReadAll(resp.Body)
				body := string(bodyBytes)
				if !strings.Contains(body, "\"wsPath\":\"/natsws\"") {
					ctx.Errorf("missing wsPath in /api/init payload: %s", body)
					return fmt.Errorf("missing wsPath in /api/init payload")
				}
				if !strings.Contains(body, "\"ws_path\":\"/natsws\"") {
					ctx.Errorf("missing ws_path in /api/init payload: %s", body)
					return fmt.Errorf("missing ws_path in /api/init payload")
				}
				ctx.Infof("api init returned wsPath")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}

			if err := ctx.WaitForStepMessageAfterAction("natsws websocket connected", 5*time.Second, func() error {
				ctx.Infof("[ACTION] websocket dial /natsws")
				wsURL := strings.Replace(baseURL, "http://", "ws://", 1) + "/natsws"
				conn, _, err := websocket.Dial(context.Background(), wsURL, nil)
				if err != nil {
					ctx.Errorf("natsws websocket dial failed: %v", err)
					return err
				}
				_ = conn.Close(websocket.StatusNormalClosure, "test done")
				ctx.Infof("natsws websocket connected")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}

			if err := ctx.WaitForStepMessageAfterAction("stream returned 503", 5*time.Second, func() error {
				ctx.Infof("[ACTION] probe /stream scaffold behavior")
				resp, err := http.Get(baseURL + "/stream")
				if err != nil {
					ctx.Errorf("stream probe failed: %v", err)
					return err
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusServiceUnavailable {
					ctx.Errorf("unexpected /stream status: %d", resp.StatusCode)
					return fmt.Errorf("expected 503 from /stream, got %d", resp.StatusCode)
				}
				ctx.Infof("stream returned 503")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}

			if err := ctx.WaitForStepMessageAfterAction("integration health reported degraded", 5*time.Second, func() error {
				ctx.Infof("[ACTION] probe /api/integration-health scaffold payload")
				resp, err := http.Get(baseURL + "/api/integration-health")
				if err != nil {
					ctx.Errorf("integration-health probe failed: %v", err)
					return err
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					ctx.Errorf("unexpected /api/integration-health status: %d", resp.StatusCode)
					return fmt.Errorf("expected 200 from /api/integration-health, got %d", resp.StatusCode)
				}
				bodyBytes, _ := io.ReadAll(resp.Body)
				body := string(bodyBytes)
				if !strings.Contains(body, "\"status\":\"degraded\"") {
					ctx.Errorf("integration health missing degraded status: %s", body)
					return fmt.Errorf("integration health missing degraded status")
				}
				if !strings.Contains(body, "\"camera\":{\"status\":\"not-configured\"}") {
					ctx.Errorf("integration health missing camera scaffold status: %s", body)
					return fmt.Errorf("integration health missing camera scaffold status")
				}
				if !strings.Contains(body, "\"mavlink\":{\"status\":\"not-configured\"}") {
					ctx.Errorf("integration health missing mavlink scaffold status: %s", body)
					return fmt.Errorf("integration health missing mavlink scaffold status")
				}
				ctx.Infof("integration health reported degraded")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}

			return testv1.StepRunResult{Report: "server runtime smoke verified"}, nil
		},
	})

	reg.Add(testv1.Step{
		Name:    "03-manifest-has-required-sync-artifacts",
		Timeout: 10 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			if err := ctx.WaitForStepMessageAfterAction("manifest contains required artifact keys", 5*time.Second, func() error {
				repo := repoRoot()
				manifestPath := filepath.Join(repo, "src", "plugins", "robot", "src_v2", "config", "composition.manifest.json")
				data, err := os.ReadFile(manifestPath)
				if err != nil {
					ctx.Errorf("manifest read failed: %v", err)
					return err
				}
				body := string(data)
				required := []string{
					"\"autoswap\"",
					"\"robot\"",
					"\"repl\"",
					"\"camera\"",
					"\"mavlink\"",
					"\"wlan\"",
					"\"ui_dist\"",
					"dialtone_robot_v2",
				}
				for _, token := range required {
					if !strings.Contains(body, token) {
						ctx.Errorf("manifest missing token %s", token)
						return fmt.Errorf("manifest missing token %s", token)
					}
				}
				ctx.Infof("manifest contains required artifact keys")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "manifest sync artifact contract verified"}, nil
		},
	})

	reg.Add(testv1.Step{
		Name: "04-local-ui-mock-e2e-smoke",
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			repo := repoRoot()
			uiDist := filepath.Join(repo, "src", "plugins", "robot", "src_v2", "ui", "dist")

			if err := ctx.WaitForStepMessageAfterAction("ui build complete", 60*time.Second, func() error {
				cmd := exec.Command("./dialtone.sh", "robot", "src_v2", "build")
				cmd.Dir = repo
				out, err := cmd.CombinedOutput()
				if err != nil {
					ctx.Errorf("ui build failed: %s", strings.TrimSpace(string(out)))
					return err
				}
				ctx.Infof("ui build complete")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}

			binPath := filepath.Join(repo, "bin", "dialtone_robot_v2")
			port := "18083"
			baseURL := "http://127.0.0.1:" + port
			cmd := exec.Command(
				binPath,
				"--listen", ":"+port,
				"--nats-port", "18224",
				"--nats-ws-port", "18225",
				"--ui-dist", uiDist,
			)
			cmd.Dir = repo
			if err := cmd.Start(); err != nil {
				return testv1.StepRunResult{}, err
			}
			defer func() {
				_ = cmd.Process.Kill()
				_, _ = cmd.Process.Wait()
			}()

			if err := ctx.WaitForStepMessageAfterAction("ui root returned 200", 10*time.Second, func() error {
				deadline := time.Now().Add(8 * time.Second)
				for time.Now().Before(deadline) {
					resp, err := http.Get(baseURL + "/")
					if err == nil {
						_ = resp.Body.Close()
						if resp.StatusCode == http.StatusOK {
							ctx.Infof("ui root returned 200")
							return nil
						}
					}
					time.Sleep(200 * time.Millisecond)
				}
				return fmt.Errorf("ui root did not return 200")
			}); err != nil {
				return testv1.StepRunResult{}, err
			}

			if strings.TrimSpace(os.Getenv("ROBOT_SRC_V2_E2E_BROWSER")) == "1" {
				if err := ctx.WaitForStepMessageAfterAction("browser title loaded", 15*time.Second, func() error {
					_, err := ctx.EnsureBrowser(testv1.BrowserOptions{
						Headless: true,
						GPU:      false,
						Role:     "robot-src-v2-e2e",
						URL:      baseURL,
					})
					if err != nil {
						return err
					}
					var title string
					if err := ctx.RunBrowser(chromedp.Title(&title)); err != nil {
						return err
					}
					if strings.TrimSpace(title) == "" {
						return fmt.Errorf("empty page title")
					}
					ctx.Infof("browser title loaded")
					return nil
				}); err != nil {
					return testv1.StepRunResult{}, err
				}
			} else {
				if err := ctx.WaitForStepMessageAfterAction("browser step skipped", 2*time.Second, func() error {
					ctx.Infof("browser step skipped")
					return nil
				}); err != nil {
					return testv1.StepRunResult{}, err
				}
			}

			if err := ctx.WaitForStepMessageAfterAction("mock nats publish ok", 5*time.Second, func() error {
				nc, err := nats.Connect("nats://127.0.0.1:18224", nats.Timeout(2*time.Second))
				if err != nil {
					return err
				}
				defer nc.Close()
				msg := `{"type":"HEARTBEAT","timestamp":12345}`
				if err := nc.Publish("mavlink.heartbeat", []byte(msg)); err != nil {
					return err
				}
				if err := nc.Flush(); err != nil {
					return err
				}
				ctx.Infof("mock nats publish ok")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}

			return testv1.StepRunResult{Report: "local UI mock E2E smoke verified"}, nil
		},
		Timeout: 90 * time.Second,
	})
}

func repoRoot() string {
	if v := strings.TrimSpace(os.Getenv("DIALTONE_REPO_ROOT")); v != "" {
		return v
	}
	cwd, _ := os.Getwd()
	return cwd
}
