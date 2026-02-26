package runtimesmoke

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(reg *testv1.Registry) {
	reg.Add(testv1.Step{
		Name: "01-build-robot-v2-binary",
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
		Name: "02-server-health-and-root-behavior",
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			repo := repoRoot()
			binPath := filepath.Join(repo, "bin", "dialtone_robot_v2")
			port := "18082"
			baseURL := "http://127.0.0.1:" + port

			cmd := exec.Command(binPath, "--listen", ":"+port)
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

			if err := ctx.WaitForStepMessageAfterAction("root returned 503", 5*time.Second, func() error {
				ctx.Infof("[ACTION] probe / expecting 503 before ui/dist wiring")
				resp, err := http.Get(baseURL + "/")
				if err != nil {
					ctx.Errorf("root probe failed: %v", err)
					return err
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusServiceUnavailable {
					ctx.Errorf("unexpected root status: %d", resp.StatusCode)
					return fmt.Errorf("expected 503, got %d", resp.StatusCode)
				}
				ctx.Infof("root returned 503")
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
				buf := make([]byte, 4096)
				n, _ := resp.Body.Read(buf)
				body := string(buf[:n])
				if !strings.Contains(body, "\"wsPath\":\"/natsws\"") {
					ctx.Errorf("missing wsPath in /api/init payload: %s", body)
					return fmt.Errorf("missing wsPath in /api/init payload")
				}
				ctx.Infof("api init returned wsPath")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}

			if err := ctx.WaitForStepMessageAfterAction("natsws returned 503", 5*time.Second, func() error {
				ctx.Infof("[ACTION] probe /natsws scaffold behavior")
				resp, err := http.Get(baseURL + "/natsws")
				if err != nil {
					ctx.Errorf("natsws probe failed: %v", err)
					return err
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusServiceUnavailable {
					ctx.Errorf("unexpected /natsws status: %d", resp.StatusCode)
					return fmt.Errorf("expected 503 from /natsws, got %d", resp.StatusCode)
				}
				ctx.Infof("natsws returned 503")
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

			return testv1.StepRunResult{Report: "server runtime smoke verified"}, nil
		},
	})
}

func repoRoot() string {
	if v := strings.TrimSpace(os.Getenv("DIALTONE_REPO_ROOT")); v != "" {
		return v
	}
	cwd, _ := os.Getwd()
	return cwd
}
