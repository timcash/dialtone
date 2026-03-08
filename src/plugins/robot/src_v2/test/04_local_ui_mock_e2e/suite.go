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
	"syscall"
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
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

			port := "18083"
			baseURL, browserBaseURL, remoteNode, err := startLocalMockServer(repo, uiDist, port)
			if err != nil {
				return testv1.StepRunResult{}, err
			}

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
					URL:         browserBaseURL + "/#robot-hero-stage",
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

			if err := ctx.WaitForStepMessageAfterAction("arm failure surfaced in terminal", 20*time.Second, func() error {
				nc, err := nats.Connect("nats://127.0.0.1:18224", nats.Timeout(2*time.Second))
				if err != nil {
					return err
				}
				defer nc.Close()

				if err := ctx.ClickAriaLabelAfterWait("Toggle Global Menu", 8*time.Second); err != nil {
					return err
				}
				if err := ctx.ClickAriaLabelAfterWait("Navigate Three", 8*time.Second); err != nil {
					return err
				}
				if err := ctx.WaitForAriaLabel("Three Section", 8*time.Second); err != nil {
					return err
				}
				if err := ctx.WaitForAriaLabelAttrEquals("Three Section", "data-active", "true", 8*time.Second); err != nil {
					return err
				}
				if err := ctx.ClickAriaLabel("Three Mode"); err != nil {
					return err
				}
				if err := ctx.ClickAriaLabel("Three Thumb 1"); err != nil {
					return err
				}
				if err := ctx.ClickAriaLabelAfterWait("Toggle Global Menu", 8*time.Second); err != nil {
					return err
				}
				if err := ctx.ClickAriaLabelAfterWait("Navigate Terminal", 8*time.Second); err != nil {
					return err
				}
				if err := ctx.WaitForAriaLabel("Xterm Terminal", 8*time.Second); err != nil {
					return err
				}
				if err := ctx.WaitForAriaLabelAttrEquals("Xterm Terminal", "data-ready", "true", 8*time.Second); err != nil {
					return err
				}
				for i := 0; i < 8; i++ {
					ack := fmt.Sprintf(`{"type":"COMMAND_ACK","command":"MAV_CMD_COMPONENT_ARM_DISARM","result":"MAV_RESULT_FAILED","timestamp":%d,"t_raw":%d}`, 12346+i, 12346+i)
					text := fmt.Sprintf(`{"type":"STATUSTEXT","severity":"MAV_SEVERITY_CRITICAL","text":"Arm: Radio failsafe on","timestamp":%d,"t_raw":%d}`, 12446+i, 12446+i)
					if err := nc.Publish("mavlink.command_ack", []byte(ack)); err != nil {
						return err
					}
					if err := nc.Publish("mavlink.statustext", []byte(text)); err != nil {
						return err
					}
				}
				if err := nc.Flush(); err != nil {
					return err
				}
				if err := ctx.WaitForAriaLabelAttrEquals("Xterm Terminal", "data-last-command-ack-result", "MAV_RESULT_FAILED", 8*time.Second); err != nil {
					return err
				}
				if err := ctx.WaitForAriaLabelAttrEquals("Xterm Terminal", "data-last-status-text", "Arm: Radio failsafe on", 8*time.Second); err != nil {
					return err
				}
				shotPath := filepath.Join(repo, "src", "plugins", "robot", "src_v2", "test", "screenshots", "arm_failure_xterm.png")
				if err := os.MkdirAll(filepath.Dir(shotPath), 0755); err != nil {
					return err
				}
				if err := ctx.CaptureScreenshot(shotPath); err != nil {
					return err
				}
				ctx.Infof("arm failure surfaced in terminal")
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

			return testv1.StepRunResult{Report: "local UI mock E2E smoke verified, including arm failure terminal path"}, nil
		},
	})
	reg.Add(testv1.Step{
		Name:    "04-three-system-arm-cli",
		Timeout: 45 * time.Second,
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

			_, browserBaseURL, remoteNode, err := startLocalMockServer(repo, uiDist, "18083")
			if err != nil {
				return testv1.StepRunResult{}, err
			}

			if err := ctx.WaitForStepMessageAfterAction("three arm command published", 20*time.Second, func() error {
				userDataDir := filepath.Join(os.TempDir(), fmt.Sprintf("dialtone-robot-arm-%d", time.Now().UnixNano()))
				_, err := ctx.EnsureBrowser(testv1.BrowserOptions{
					Headless:    false,
					GPU:         true,
					Role:        "robot-src-v2-arm",
					RemoteNode:  remoteNode,
					UserDataDir: userDataDir,
					URL:         browserBaseURL + fmt.Sprintf("/?arm=%d#robot-three-stage", time.Now().UnixNano()),
				})
				if err != nil {
					return err
				}
				if err := ctx.WaitForAriaLabel("Three Section", 8*time.Second); err != nil {
					return err
				}
				if err := ctx.WaitForAriaLabelAttrEquals("Three Section", "data-active", "true", 8*time.Second); err != nil {
					return err
				}
				if err := ctx.ClickAriaLabel("Three Mode"); err != nil {
					return err
				}
				if err := ctx.ClickAriaLabel("Three Thumb 1"); err != nil {
					return err
				}
				if err := ctx.WaitForConsoleContains("Publishing rover.command cmd=arm", 5*time.Second); err != nil {
					return err
				}
				shotPath := filepath.Join(repo, "src", "plugins", "robot", "src_v2", "test", "screenshots", "three_system_arm.png")
				if err := os.MkdirAll(filepath.Dir(shotPath), 0755); err != nil {
					return err
				}
				if err := ctx.CaptureScreenshot(shotPath); err != nil {
					return err
				}
				ctx.Infof("three arm command published")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "three system arm CLI flow verified"}, nil
		},
	})
}

func startLocalMockServer(repo, uiDist, port string) (baseURL, browserBaseURL, remoteNode string, err error) {
	binPath := filepath.Join(repo, "bin", "dialtone_robot_v2")
	baseURL = "http://127.0.0.1:" + port
	browserBaseURL = baseURL
	remoteNode = strings.TrimSpace(testv1.RuntimeConfigSnapshot().BrowserNode)
	if remoteNode == "" {
		remoteNode = strings.TrimSpace(os.Getenv("DIALTONE_TEST_BROWSER_NODE"))
	}
	if remoteNode == "" {
		remoteNode = strings.TrimSpace(os.Getenv("ROBOT_TEST_BROWSER_NODE"))
	}
	if remoteNode != "" {
		if v := strings.TrimSpace(os.Getenv("DIALTONE_TEST_BROWSER_BASE_URL")); v != "" {
			browserBaseURL = strings.TrimRight(v, "/")
		} else if isWSL() && strings.EqualFold(remoteNode, "legion") {
			browserBaseURL = baseURL
		} else if dnsName := tailscaleSelfDNSName(); dnsName != "" {
			browserBaseURL = "http://" + dnsName + ":" + port
		} else if host, herr := os.Hostname(); herr == nil && strings.TrimSpace(host) != "" {
			browserBaseURL = "http://" + strings.TrimSpace(host) + ":" + port
		}
	} else if isWSL() {
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
	logPath := filepath.Join(repo, "src", "plugins", "robot", "src_v2", "test", "local_ui_mock_server.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return "", "", "", err
	}
	defer logFile.Close()
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return "", "", "", err
	}
	_ = cmd.Process.Release()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		resp, reqErr := http.Get(baseURL + "/")
		if reqErr == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return baseURL, browserBaseURL, remoteNode, nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return "", "", "", fmt.Errorf("ui root did not return 200")
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
	if err := ctx.WaitForAriaLabel("Hero Section", timeout); err != nil {
		return err
	}
	if err := ctx.WaitForAriaLabel("Hero Canvas", timeout); err != nil {
		return err
	}
	if err := ctx.WaitForAriaLabelAttrEquals("Hero Section", "data-ready", "true", timeout); err != nil {
		return err
	}
	return ctx.WaitForAriaLabelAttrEquals("Hero Section", "data-active", "true", timeout)
}
