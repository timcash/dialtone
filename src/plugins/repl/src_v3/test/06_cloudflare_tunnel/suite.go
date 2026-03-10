package cloudflaretunnel

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"dialtone/dev/plugins/repl/src_v3/test/support"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name:    "injected-cloudflare-tunnel-start",
		Timeout: 120 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			rt, err := support.NewRuntime(ctx)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			realMode := strings.TrimSpace(os.Getenv("DIALTONE_REPL_V3_TEST_REAL")) == "1"
			tunnelName := strings.TrimSpace(os.Getenv("DIALTONE_REPL_V3_TEST_TUNNEL_NAME"))
			if tunnelName == "" {
				tunnelName = "repl-src-v3-test"
			}
			tunnelURL := strings.TrimSpace(os.Getenv("DIALTONE_REPL_V3_TEST_TUNNEL_URL"))
			if tunnelURL == "" {
				tunnelURL = "http://127.0.0.1:8080"
			}

			if !realMode {
				tmpBin, restorePath, err := installMockCloudflared(rt.RepoRoot)
				if err != nil {
					return testv1.StepRunResult{}, err
				}
				defer restorePath()
				oldCF := os.Getenv("DIALTONE_CLOUDFLARED_BIN")
				_ = os.Setenv("DIALTONE_CLOUDFLARED_BIN", tmpBin)
				defer func() {
					_ = os.Setenv("DIALTONE_CLOUDFLARED_BIN", oldCF)
				}()

				cmd := exec.Command(rt.GoBin,
					"run", "./plugins/cloudflare/scaffold/main.go",
					"src_v1", "tunnel", "start", tunnelName,
					"--url", tunnelURL,
					"--token", "token-test",
				)
				cmd.Dir = rt.SrcRoot
				cmd.Env = append(os.Environ(), "DIALTONE_CLOUDFLARED_BIN="+tmpBin)
				outBytes, err := cmd.CombinedOutput()
				out := string(outBytes)
				if err != nil {
					return testv1.StepRunResult{}, fmt.Errorf("direct cloudflare tunnel start failed: %w\n%s", err, out)
				}
				if !strings.Contains(out, "MOCK-CLOUDFLARED args: tunnel run --token token-test --url "+tunnelURL) {
					return testv1.StepRunResult{}, fmt.Errorf("direct tunnel start did not execute cloudflared as expected:\n%s", out)
				}
			}

			defer rt.Stop()
			if err := rt.StartLeader(); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := rt.StartJoin("local-human"); err != nil {
				return testv1.StepRunResult{}, err
			}

			injectedArgs := []string{
				"cloudflare", "src_v1", "tunnel", "start", tunnelName,
				"--url", tunnelURL,
			}
			if !realMode {
				injectedArgs = append(injectedArgs, "--token", "token-test")
			}
			if err := rt.Inject("llm-codex", injectedArgs...); err != nil {
				return testv1.StepRunResult{}, err
			}

			if err := rt.WaitForPatterns(40*time.Second, []string{
				`Request received. Spawning subtone for cloudflare src_v1`,
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			if realMode {
				if err := rt.Inject("llm-codex",
					"cloudflare", "src_v1", "tunnel", "stop",
				); err != nil {
					return testv1.StepRunResult{}, err
				}
				if err := rt.WaitForPatterns(40*time.Second, []string{
					`/cloudflare src_v1 tunnel stop`,
					`Subtone for cloudflare src_v1 exited with code 0.`,
				}); err != nil {
					return testv1.StepRunResult{}, err
				}
			}

			ctx.TestPassf("cloudflare tunnel start executed through repl src_v3 injection")
			return testv1.StepRunResult{
				Report: "Injected cloudflare tunnel start through REPL/NATS and verified REPL command ingress (mock mode by default, real start/stop when DIALTONE_REPL_V3_TEST_REAL=1).",
			}, nil
		},
	})
}

func installMockCloudflared(repoRoot string) (string, func(), error) {
	dir, err := os.MkdirTemp("", "dialtone-cloudflared-mock-*")
	if err != nil {
		return "", nil, err
	}
	var binPath string
	var content string
	if runtime.GOOS == "windows" {
		binPath = filepath.Join(dir, "cloudflared.bat")
		content = "@echo off\r\necho MOCK-CLOUDFLARED args: %*\r\nexit /b 0\r\n"
	} else {
		binPath = filepath.Join(dir, "cloudflared")
		content = "#!/bin/sh\n" +
			"echo \"MOCK-CLOUDFLARED args: $*\"\n" +
			"exit 0\n"
	}
	if err := os.WriteFile(binPath, []byte(content), 0o755); err != nil {
		return "", nil, err
	}
	oldPath := os.Getenv("PATH")
	sep := string(os.PathListSeparator)
	if strings.TrimSpace(oldPath) == "" {
		_ = os.Setenv("PATH", dir)
	} else {
		_ = os.Setenv("PATH", fmt.Sprintf("%s%s%s", dir, sep, oldPath))
	}
	restore := func() {
		_ = os.Setenv("PATH", oldPath)
		_ = os.RemoveAll(dir)
	}
	return binPath, restore, nil
}
