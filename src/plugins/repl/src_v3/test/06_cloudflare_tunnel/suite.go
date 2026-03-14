package cloudflaretunnel

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"dialtone/dev/plugins/repl/src_v3/test/support"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name:    "interactive-cloudflare-tunnel-start",
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

				out, err := rt.RunDialtone(
					"cloudflare", "src_v1", "tunnel", "start", tunnelName,
					"--url", tunnelURL,
					"--token", "token-test",
				)
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
			if err := rt.StartJoin("llm-codex"); err != nil {
				return testv1.StepRunResult{}, err
			}

			replCmd := fmt.Sprintf("/cloudflare src_v1 tunnel start %s --url %s", tunnelName, tunnelURL)
			if !realMode {
				replCmd += " --token token-test"
			}
			if err := rt.RunTranscript([]support.TranscriptStep{
				{
					Send: replCmd,
					ExpectRoom: support.CombinePatterns([]string{
						fmt.Sprintf(`"message":"%s"`, replCmd),
					}, support.StandardSubtoneRoomPatterns("cloudflare src_v1", ``)),
					ExpectOutput: support.CombinePatterns([]string{
						`/cloudflare src_v1 tunnel start`,
					}, support.StandardSubtoneOutputPatterns("cloudflare src_v1", ``)),
					Timeout: 40 * time.Second,
				},
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			if realMode {
				stopCmd := "/cloudflare src_v1 tunnel stop"
				if err := rt.RunTranscript([]support.TranscriptStep{
					{
						Send: stopCmd,
						ExpectRoom: support.CombinePatterns([]string{
							fmt.Sprintf(`"message":"%s"`, stopCmd),
						}, support.StandardSubtoneRoomPatterns("cloudflare src_v1", ``)),
						ExpectOutput: support.CombinePatterns([]string{
							`/cloudflare src_v1 tunnel stop`,
						}, support.StandardSubtoneOutputPatterns("cloudflare src_v1", ``)),
						Timeout: 40 * time.Second,
					},
				}); err != nil {
					return testv1.StepRunResult{}, err
				}
			}

			ctx.TestPassf("cloudflare tunnel start executed through llm-codex REPL prompt path")
			return testv1.StepRunResult{
				Report: "Joined REPL as llm-codex, typed /cloudflare src_v1 tunnel start through the live prompt path, and verified REPL command ingress (mock mode by default, real start/stop when DIALTONE_REPL_V3_TEST_REAL=1).",
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
