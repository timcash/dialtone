package localuimocke2e

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
	"github.com/chromedp/chromedp"
	"github.com/nats-io/nats.go"
)

func Register(reg *testv1.Registry) {
	reg.Add(testv1.Step{
		Name:    "04-local-ui-mock-e2e-smoke",
		Timeout: 90 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			repo := ctx.RepoRoot()
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
			browserBaseURL := baseURL
			remoteNode := strings.TrimSpace(testv1.RuntimeConfigSnapshot().BrowserNode)
			if remoteNode == "" {
				remoteNode = strings.TrimSpace(os.Getenv("DIALTONE_TEST_BROWSER_NODE"))
			}
			if remoteNode == "" {
				remoteNode = strings.TrimSpace(os.Getenv("ROBOT_TEST_BROWSER_NODE"))
			}
			if remoteNode != "" {
				if v := strings.TrimSpace(os.Getenv("DIALTONE_TEST_BROWSER_BASE_URL")); v != "" {
					browserBaseURL = strings.TrimRight(v, "/")
				} else if dnsName := tailscaleSelfDNSName(); dnsName != "" {
					browserBaseURL = "http://" + dnsName + ":" + port
				} else if host, err := os.Hostname(); err == nil && strings.TrimSpace(host) != "" {
					// For remote browser nodes, avoid loopback URL targets.
					browserBaseURL = "http://" + strings.TrimSpace(host) + ":" + port
				}
			} else if isWSL() {
				// In WSL local mode, tests still launch Windows Chrome; avoid localhost.
				if hostIP := wslHostIPForWindowsBrowser(); hostIP != "" {
					browserBaseURL = "http://" + hostIP + ":" + port
				}
			}
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

			if err := ctx.WaitForStepMessageAfterAction("browser ui checks passed", 20*time.Second, func() error {
				userDataDir := filepath.Join(os.TempDir(), fmt.Sprintf("dialtone-robot-e2e-%d", time.Now().UnixNano()))
				_, err := ctx.EnsureBrowser(testv1.BrowserOptions{
					Headless:    false,
					GPU:         true,
					Role:        "robot-src-v2-e2e",
					RemoteNode:  remoteNode,
					UserDataDir: userDataDir,
					URL:         browserBaseURL + "/#hero",
				})
				if err != nil {
					return err
				}
				if err := waitForHeroReadyByID(ctx, 8*time.Second); err != nil {
					return err
				}

				sections := []struct {
					Menu  string
					Aria  string
					Extra string
				}{
					{Menu: "Navigate Docs", Aria: "Docs Section"},
					{Menu: "Navigate Telemetry", Aria: "Telemetry Section", Extra: "Robot Table"},
					{Menu: "Navigate Three", Aria: "Three Section"},
					{Menu: "Navigate Terminal", Aria: "Xterm Section"},
					{Menu: "Navigate Camera", Aria: "Video Section"},
					{Menu: "Navigate Settings", Aria: "Settings Section"},
				}
				for _, s := range sections {
					if err := ctx.WaitForAriaLabel("Toggle Global Menu", 8*time.Second); err != nil {
						return err
					}
					if err := ctx.ClickAriaLabel("Toggle Global Menu"); err != nil {
						return err
					}
					if err := ctx.WaitForAriaLabel(s.Menu, 8*time.Second); err != nil {
						return err
					}
					if err := ctx.ClickAriaLabel(s.Menu); err != nil {
						return err
					}
					if err := ctx.WaitForAriaLabel(s.Aria, 8*time.Second); err != nil {
						return err
					}
					if s.Extra != "" {
						if err := ctx.WaitForAriaLabel(s.Extra, 8*time.Second); err != nil {
							return err
						}
					}
					if err := ctx.WaitForAriaLabelAttrEquals(s.Aria, "data-active", "true", 8*time.Second); err != nil {
						return err
					}
				}

				ctx.Infof("browser ui checks passed")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
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
	})
}

func tailscaleSelfDNSName() string {
	cmd := exec.Command("tailscale", "status", "--json")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	var status struct {
		Self struct {
			DNSName string `json:"DNSName"`
		} `json:"Self"`
	}
	if err := json.Unmarshal(out, &status); err != nil {
		return ""
	}
	return strings.TrimSuffix(strings.TrimSpace(status.Self.DNSName), ".")
}

func isWSL() bool {
	raw, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(raw)), "microsoft")
}

func wslHostIPForWindowsBrowser() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if (iface.Flags&net.FlagUp) == 0 || (iface.Flags&net.FlagLoopback) != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok || ipnet == nil {
				continue
			}
			ip := ipnet.IP.To4()
			if ip == nil || ip.IsLoopback() {
				continue
			}
			return ip.String()
		}
	}
	return ""
}

func waitForHeroReadyByID(ctx *testv1.StepContext, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var ready bool
		err := ctx.RunBrowserWithTimeout(1500*time.Millisecond, chromedp.Evaluate(`(() => {
			const el = document.getElementById('robot-hero-stage');
			if (!el) return false;
			return el.getAttribute('data-ready') === 'true' && el.getAttribute('data-active') === 'true';
		})()`, &ready))
		if err == nil && ready {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for #robot-hero-stage ready/active after %s", timeout)
}
