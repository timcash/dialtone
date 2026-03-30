package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	chromev3 "dialtone/dev/plugins/chrome/src_v3"
	configv1 "dialtone/dev/plugins/config/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	defaultActionOptions := resolveActionFlowOptions()
	fs := flag.NewFlagSet("chrome src_v3 test", flag.ContinueOnError)
	host := fs.String("host", "", "Mesh host (optional; default local)")
	role := fs.String("role", "dev", "Chrome role")
	lines := fs.Int("lines", 80, "Remote daemon log lines to include")
	filter := fs.String("filter", "", "Run only matching test steps")
	interactions := fs.Int("interactions", defaultActionOptions.InteractionCount, "Number of visible browser interaction loops in the action step")
	actionsPerSecond := fs.Float64("actions-per-second", defaultActionOptions.ActionsPerSecond, "Visible browser action rate for the action step")
	if err := fs.Parse(os.Args[1:]); err != nil {
		logs.Error("chrome src_v3 test parse failed: %v", err)
		os.Exit(1)
	}
	hostValue := strings.TrimSpace(*host)
	if hostValue == "" {
		hostValue = defaultChromeTestHost()
	}
	roleValue := defaultIfBlank(strings.TrimSpace(*role), "dev")
	reportNode := hostValue
	if reportNode == "" {
		reportNode = "local"
	}
	actionOptions := normalizeActionFlowOptions(actionFlowOptions{
		InteractionCount: *interactions,
		ActionsPerSecond: *actionsPerSecond,
	})

	reg := testv1.NewRegistry()
	addChromeSuiteSteps(reg, hostValue, roleValue, *lines, actionOptions)
	if filteredSteps := filterSteps(reg.Steps, strings.TrimSpace(*filter)); len(filteredSteps) > 0 {
		reg.Steps = filteredSteps
	}

	logs.Info("chrome src_v3 test starting host=%s role=%s steps=%d interactions=%d actions_per_second=%.3f", reportNode, roleValue, len(reg.Steps), actionOptions.InteractionCount, actionOptions.ActionsPerSecond)
	if err := reg.Run(testv1.SuiteOptions{
		Version:          "chrome-src-v3",
		ReportPath:       "plugins/chrome/src_v3/TEST.md",
		RawReportPath:    "plugins/chrome/src_v3/TEST_RAW.md",
		ReportFormat:     "template",
		ReportTitle:      "Chrome src_v3 Test Report",
		ReportRunner:     "test/src_v1",
		ChromeReportNode: reportNode,
		NATSURL:          resolveSuiteNATSURL(),
		NATSSubject:      "logs.test.chrome-src-v3",
		AutoStartNATS:    true,
	}); err != nil {
		logs.Error("chrome src_v3 test failed: %v", err)
		os.Exit(1)
	}
	logs.Info("chrome src_v3 test passed")
}

func defaultChromeTestHost() string {
	if v := strings.TrimSpace(configv1.LookupEnvString("DIALTONE_CHROME_TEST_HOST")); v != "" {
		return v
	}
	if strings.TrimSpace(os.Getenv("WSL_DISTRO_NAME")) == "" {
		return ""
	}
	if _, err := sshv1.ResolveMeshNode("legion"); err == nil {
		return "legion"
	}
	return ""
}

type actionFlowOptions struct {
	InteractionCount int
	ActionsPerSecond float64
}

func addChromeSuiteSteps(reg *testv1.Registry, host, role string, lines int, actionOptions actionFlowOptions) {
	reg.Add(testv1.Step{
		Name:    "chrome-deploy-and-start",
		Timeout: 120 * time.Second,
		RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
			resp, err := chromev3.EnsureServiceByTarget(host, role, false)
			if err != nil {
				appendTargetLogsToStep(sc, host, role, lines)
				return testv1.StepRunResult{}, fmt.Errorf("ensure remote service: %w", err)
			}
			sc.Infof("service ready host=%s role=%s service_pid=%d browser_pid=%d chrome_port=%d nats_port=%d unhealthy=%t", host, role, resp.ServicePID, resp.BrowserPID, resp.ChromePort, resp.NATSPort, resp.Unhealthy)
			appendTargetLogsToStep(sc, host, role, lines)
			return testv1.StepRunResult{
				Report: fmt.Sprintf("chrome src_v3 deployed and service started on %s (service_pid=%d browser_pid=%d)", defaultIfBlank(host, "local"), resp.ServicePID, resp.BrowserPID),
			}, nil
		},
	})

	reg.Add(testv1.Step{
		Name:    "chrome-browser-pid-stable-across-commands",
		Timeout: 45 * time.Second,
		RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
			resp, err := chromev3.EnsureServiceByTarget(host, role, false)
			if err != nil {
				appendTargetLogsToStep(sc, host, role, lines)
				return testv1.StepRunResult{}, fmt.Errorf("ensure service before pid stability check: %w", err)
			}
			initialPID := resp.BrowserPID
			sc.Infof("initial browser pid=%d service_pid=%d", resp.BrowserPID, resp.ServicePID)

			marker := fmt.Sprintf("%d", time.Now().UnixNano())
			for _, step := range []chromev3.CommandRequest{
				{Command: "open", Role: role, URL: "about:blank"},
				{Command: "set-html", Role: role, Value: actionSmokeHTML(marker)},
				{Command: "wait-log", Role: role, Contains: "page-ready:" + marker, TimeoutMS: 8000},
				{Command: "status", Role: role},
			} {
				nextResp, err := chromev3.SendCommandByTarget(host, step)
				if err != nil {
					appendTargetLogsToStep(sc, host, role, lines)
					return testv1.StepRunResult{}, fmt.Errorf("%s during pid stability check failed: %w", step.Command, err)
				}
				sc.Infof("pid-check command=%s browser_pid=%d service_pid=%d tabs=%d", step.Command, nextResp.BrowserPID, nextResp.ServicePID, len(nextResp.Tabs))
				if initialPID == 0 {
					initialPID = nextResp.BrowserPID
				}
				if initialPID <= 0 || nextResp.BrowserPID <= 0 {
					appendTargetLogsToStep(sc, host, role, lines)
					return testv1.StepRunResult{}, fmt.Errorf("browser pid missing during stability check: initial=%d current=%d", initialPID, nextResp.BrowserPID)
				}
				if nextResp.BrowserPID != initialPID {
					appendTargetLogsToStep(sc, host, role, lines)
					return testv1.StepRunResult{}, fmt.Errorf("browser pid changed during normal commands: initial=%d current=%d", initialPID, nextResp.BrowserPID)
				}
			}

			appendTargetLogsToStep(sc, host, role, lines)
			return testv1.StepRunResult{
				Report: fmt.Sprintf("chrome src_v3 reused browser pid %d across normal commands on %s", initialPID, defaultIfBlank(host, "local")),
			}, nil
		},
	})

	reg.Add(testv1.Step{
		Name:    "chrome-reset-does-not-restart-browser",
		Timeout: 45 * time.Second,
		RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
			if _, err := chromev3.EnsureServiceByTarget(host, role, false); err != nil {
				appendTargetLogsToStep(sc, host, role, lines)
				return testv1.StepRunResult{}, fmt.Errorf("ensure service before reset stability check: %w", err)
			}

			before, err := chromev3.SendCommandByTarget(host, chromev3.CommandRequest{
				Command: "open",
				Role:    role,
				URL:     "about:blank",
			})
			if err != nil {
				appendTargetLogsToStep(sc, host, role, lines)
				return testv1.StepRunResult{}, fmt.Errorf("open before reset failed: %w", err)
			}
			if before.BrowserPID <= 0 {
				appendTargetLogsToStep(sc, host, role, lines)
				return testv1.StepRunResult{}, fmt.Errorf("missing browser pid before reset")
			}
			sc.Infof("before reset browser_pid=%d service_pid=%d", before.BrowserPID, before.ServicePID)

			if _, err := chromev3.SendCommandByTarget(host, chromev3.CommandRequest{
				Command: "reset",
				Role:    role,
			}); err != nil {
				appendTargetLogsToStep(sc, host, role, lines)
				return testv1.StepRunResult{}, fmt.Errorf("reset failed: %w", err)
			}

			after, err := chromev3.SendCommandByTarget(host, chromev3.CommandRequest{
				Command: "open",
				Role:    role,
				URL:     "about:blank",
			})
			if err != nil {
				appendTargetLogsToStep(sc, host, role, lines)
				return testv1.StepRunResult{}, fmt.Errorf("open after reset failed: %w", err)
			}
			sc.Infof("after reset browser_pid=%d service_pid=%d", after.BrowserPID, after.ServicePID)
			if after.BrowserPID <= 0 {
				appendTargetLogsToStep(sc, host, role, lines)
				return testv1.StepRunResult{}, fmt.Errorf("missing browser pid after reset")
			}
			if before.BrowserPID != after.BrowserPID {
				appendTargetLogsToStep(sc, host, role, lines)
				return testv1.StepRunResult{}, fmt.Errorf("reset restarted browser: before=%d after=%d", before.BrowserPID, after.BrowserPID)
			}

			appendTargetLogsToStep(sc, host, role, lines)
			return testv1.StepRunResult{
				Report: fmt.Sprintf("chrome src_v3 reset reused browser pid %d on %s", after.BrowserPID, defaultIfBlank(host, "local")),
			}, nil
		},
	})

	reg.Add(testv1.Step{
		Name:    "chrome-managed-tab-count-stays-bounded",
		Timeout: 45 * time.Second,
		RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
			if _, err := chromev3.EnsureServiceByTarget(host, role, false); err != nil {
				appendTargetLogsToStep(sc, host, role, lines)
				return testv1.StepRunResult{}, fmt.Errorf("ensure service before tab count check: %w", err)
			}

			urls := []string{
				"about:blank",
				"data:text/html,<html><body><h1>one</h1></body></html>",
				"data:text/html,<html><body><h1>two</h1></body></html>",
			}
			for i, targetURL := range urls {
				resp, err := chromev3.SendCommandByTarget(host, chromev3.CommandRequest{
					Command: "open",
					Role:    role,
					URL:     targetURL,
				})
				if err != nil {
					appendTargetLogsToStep(sc, host, role, lines)
					return testv1.StepRunResult{}, fmt.Errorf("open cycle %d failed: %w", i+1, err)
				}
				sc.Infof("open cycle=%d browser_pid=%d tabs=%d current_url=%s", i+1, resp.BrowserPID, len(resp.Tabs), strings.TrimSpace(resp.CurrentURL))
				if len(resp.Tabs) > 1 {
					appendTargetLogsToStep(sc, host, role, lines)
					return testv1.StepRunResult{}, fmt.Errorf("managed tab count grew beyond 1 after open cycle %d: tabs=%d", i+1, len(resp.Tabs))
				}
			}

			appendTargetLogsToStep(sc, host, role, lines)
			return testv1.StepRunResult{
				Report: fmt.Sprintf("chrome src_v3 kept a single managed tab across repeated opens on %s", defaultIfBlank(host, "local")),
			}, nil
		},
	})

	reg.Add(testv1.Step{
		Name:    "chrome-single-browser-process-per-role",
		Timeout: 45 * time.Second,
		RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
			if _, err := chromev3.EnsureServiceByTarget(host, role, false); err != nil {
				appendTargetLogsToStep(sc, host, role, lines)
				return testv1.StepRunResult{}, fmt.Errorf("ensure service before process count check: %w", err)
			}
			for i, step := range []chromev3.CommandRequest{
				{Command: "open", Role: role, URL: "about:blank"},
				{Command: "status", Role: role},
				{Command: "reset", Role: role},
				{Command: "open", Role: role, URL: "about:blank"},
			} {
				resp, err := chromev3.SendCommandByTarget(host, step)
				if err != nil {
					appendTargetLogsToStep(sc, host, role, lines)
					return testv1.StepRunResult{}, fmt.Errorf("process count command %d (%s) failed: %w", i+1, step.Command, err)
				}
				sc.Infof("process-count command=%s browser_pid=%d count=%d", step.Command, resp.BrowserPID, resp.ProcessCount)
				if resp.ProcessCount != 1 {
					appendTargetLogsToStep(sc, host, role, lines)
					return testv1.StepRunResult{}, fmt.Errorf("expected exactly 1 chrome process for role %s after %s, saw %d", role, step.Command, resp.ProcessCount)
				}
			}
			appendTargetLogsToStep(sc, host, role, lines)
			return testv1.StepRunResult{
				Report: fmt.Sprintf("chrome src_v3 kept exactly one chrome process for role %s on %s", role, defaultIfBlank(host, "local")),
			}, nil
		},
	})

	reg.Add(testv1.Step{
		Name:    "chrome-no-recovery-window-for-role",
		Timeout: 45 * time.Second,
		RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
			if _, err := chromev3.EnsureServiceByTarget(host, role, false); err != nil {
				appendTargetLogsToStep(sc, host, role, lines)
				return testv1.StepRunResult{}, fmt.Errorf("ensure service before recovery check: %w", err)
			}
			if _, err := chromev3.SendCommandByTarget(host, chromev3.CommandRequest{
				Command: "open",
				Role:    role,
				URL:     "about:blank",
			}); err != nil {
				appendTargetLogsToStep(sc, host, role, lines)
				return testv1.StepRunResult{}, fmt.Errorf("open before recovery check failed: %w", err)
			}
			resp, err := chromev3.SendCommandByTarget(host, chromev3.CommandRequest{
				Command: "eval",
				Role:    role,
				Script: `JSON.stringify({
				  href: String(location.href || ""),
				  title: String(document.title || ""),
				  text: String((document.body && document.body.innerText) || "").slice(0, 400)
				})`,
			})
			if err != nil {
				appendTargetLogsToStep(sc, host, role, lines)
				return testv1.StepRunResult{}, fmt.Errorf("eval recovery probe failed: %w", err)
			}
			var snapshot struct {
				Href  string `json:"href"`
				Title string `json:"title"`
				Text  string `json:"text"`
			}
			if err := json.Unmarshal([]byte(strings.TrimSpace(resp.Value)), &snapshot); err != nil {
				appendTargetLogsToStep(sc, host, role, lines)
				return testv1.StepRunResult{}, fmt.Errorf("decode recovery probe: %w", err)
			}
			sc.Infof("recovery-probe href=%q title=%q text=%q", strings.TrimSpace(snapshot.Href), strings.TrimSpace(snapshot.Title), strings.TrimSpace(snapshot.Text))
			if hasRecoveryIndicator(snapshot.Href, snapshot.Title, snapshot.Text) {
				appendTargetLogsToStep(sc, host, role, lines)
				return testv1.StepRunResult{}, fmt.Errorf("chrome recovery UI detected for role %s: href=%q title=%q text=%q", role, strings.TrimSpace(snapshot.Href), strings.TrimSpace(snapshot.Title), strings.TrimSpace(snapshot.Text))
			}
			appendTargetLogsToStep(sc, host, role, lines)
			return testv1.StepRunResult{
				Report: fmt.Sprintf("chrome src_v3 showed no recovery UI for role %s on %s", role, defaultIfBlank(host, "local")),
			}, nil
		},
	})

	reg.Add(testv1.Step{
		Name:    "chrome-roles-do-not-share-browser-pid",
		Timeout: 60 * time.Second,
		RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
			primaryRole := role
			secondaryRole := role + "-isolated"
			for _, currentRole := range []string{primaryRole, secondaryRole} {
				if _, err := chromev3.EnsureServiceByTarget(host, currentRole, false); err != nil {
					appendTargetLogsToStep(sc, host, currentRole, lines)
					return testv1.StepRunResult{}, fmt.Errorf("ensure service for role %s failed: %w", currentRole, err)
				}
			}

			primaryResp, err := chromev3.SendCommandByTarget(host, chromev3.CommandRequest{
				Command: "open",
				Role:    primaryRole,
				URL:     "data:text/html,<html><body><h1>primary</h1></body></html>",
			})
			if err != nil {
				appendTargetLogsToStep(sc, host, primaryRole, lines)
				return testv1.StepRunResult{}, fmt.Errorf("open for primary role failed: %w", err)
			}
			secondaryResp, err := chromev3.SendCommandByTarget(host, chromev3.CommandRequest{
				Command: "open",
				Role:    secondaryRole,
				URL:     "data:text/html,<html><body><h1>secondary</h1></body></html>",
			})
			if err != nil {
				appendTargetLogsToStep(sc, host, secondaryRole, lines)
				return testv1.StepRunResult{}, fmt.Errorf("open for secondary role failed: %w", err)
			}

			sc.Infof("primary role=%s browser_pid=%d service_pid=%d", primaryRole, primaryResp.BrowserPID, primaryResp.ServicePID)
			sc.Infof("secondary role=%s browser_pid=%d service_pid=%d", secondaryRole, secondaryResp.BrowserPID, secondaryResp.ServicePID)

			if primaryResp.BrowserPID <= 0 || secondaryResp.BrowserPID <= 0 {
				appendTargetLogsToStep(sc, host, primaryRole, lines)
				appendTargetLogsToStep(sc, host, secondaryRole, lines)
				return testv1.StepRunResult{}, fmt.Errorf("missing browser pid for role isolation check: primary=%d secondary=%d", primaryResp.BrowserPID, secondaryResp.BrowserPID)
			}
			if primaryResp.BrowserPID == secondaryResp.BrowserPID {
				appendTargetLogsToStep(sc, host, primaryRole, lines)
				appendTargetLogsToStep(sc, host, secondaryRole, lines)
				return testv1.StepRunResult{}, fmt.Errorf("roles shared browser pid %d (%s and %s)", primaryResp.BrowserPID, primaryRole, secondaryRole)
			}

			appendTargetLogsToStep(sc, host, primaryRole, lines)
			appendTargetLogsToStep(sc, host, secondaryRole, lines)
			return testv1.StepRunResult{
				Report: fmt.Sprintf("chrome src_v3 kept role browser isolation between %s and %s on %s", primaryRole, secondaryRole, defaultIfBlank(host, "local")),
			}, nil
		},
	})

	reg.Add(testv1.Step{
		Name:    "chrome-browser-actions-and-screenshot",
		Timeout: actionFlowTimeout(actionOptions),
		RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
			if _, err := chromev3.EnsureServiceByTarget(host, role, false); err != nil {
				appendTargetLogsToStep(sc, host, role, lines)
				return testv1.StepRunResult{}, fmt.Errorf("ensure service before actions: %w", err)
			}
			marker := fmt.Sprintf("%d", time.Now().UnixNano())
			steps := buildActionFlowRequests(role, marker, actionOptions)
			var screenshotResp *chromev3.CommandResponse
			lastTypedValue := ""
			for _, step := range steps {
				resp, err := chromev3.SendCommandByTarget(host, step)
				if err != nil {
					appendTargetLogsToStep(sc, host, role, lines)
					return testv1.StepRunResult{}, fmt.Errorf("%s failed: %w", step.Command, err)
				}
				sc.Infof("command=%s ok service_pid=%d browser_pid=%d current_url=%s tabs=%d", step.Command, resp.ServicePID, resp.BrowserPID, strings.TrimSpace(resp.CurrentURL), len(resp.Tabs))
				for _, line := range resp.ConsoleLines {
					line = strings.TrimSpace(line)
					if line != "" {
						sc.Infof("remote-console: %s", line)
					}
				}
				if step.Command == "screenshot" {
					screenshotResp = resp
				}
				if step.Command == "type-aria" {
					lastTypedValue = step.Value
				}
				if step.Command == "eval" {
					var snapshot struct {
						Status string `json:"status"`
						Count  string `json:"count"`
						Value  string `json:"value"`
					}
					if err := json.Unmarshal([]byte(strings.TrimSpace(resp.Value)), &snapshot); err != nil {
						appendTargetLogsToStep(sc, host, role, lines)
						return testv1.StepRunResult{}, fmt.Errorf("decode action summary: %w", err)
					}
					expectedCount := fmt.Sprintf("%d", actionOptions.InteractionCount)
					if strings.TrimSpace(snapshot.Count) != expectedCount {
						appendTargetLogsToStep(sc, host, role, lines)
						return testv1.StepRunResult{}, fmt.Errorf("unexpected action count: got=%q want=%q", strings.TrimSpace(snapshot.Count), expectedCount)
					}
					if strings.TrimSpace(snapshot.Value) != strings.TrimSpace(lastTypedValue) {
						appendTargetLogsToStep(sc, host, role, lines)
						return testv1.StepRunResult{}, fmt.Errorf("unexpected final input value: got=%q want=%q", strings.TrimSpace(snapshot.Value), strings.TrimSpace(lastTypedValue))
					}
					expectedStatus := fmt.Sprintf("clicked-%d", actionOptions.InteractionCount)
					if strings.TrimSpace(snapshot.Status) != expectedStatus {
						appendTargetLogsToStep(sc, host, role, lines)
						return testv1.StepRunResult{}, fmt.Errorf("unexpected final status: got=%q want=%q", strings.TrimSpace(snapshot.Status), expectedStatus)
					}
				}
			}
			if screenshotResp == nil || (strings.TrimSpace(screenshotResp.ScreenshotB64) == "" && strings.TrimSpace(screenshotResp.ScreenshotPath) == "") {
				appendTargetLogsToStep(sc, host, role, lines)
				return testv1.StepRunResult{}, fmt.Errorf("screenshot response missing image data")
			}
			shotPath := filepath.Join("plugins", "chrome", "src_v3", "screenshots", fmt.Sprintf("chrome_src_v3_actions_%s.png", sanitizeToken(defaultIfBlank(host, "local"))))
			if err := chromev3.WriteScreenshotByTarget(host, screenshotResp, shotPath); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := sc.AddScreenshot(shotPath); err != nil {
				return testv1.StepRunResult{}, err
			}
			appendTargetLogsToStep(sc, host, role, lines)
			return testv1.StepRunResult{
				Report: fmt.Sprintf("chrome src_v3 action flow passed on %s with %d interactions at %.3f actions/s and screenshot capture", defaultIfBlank(host, "local"), actionOptions.InteractionCount, actionOptions.ActionsPerSecond),
			}, nil
		},
	})

	reg.Add(testv1.Step{
		Name:    "chrome-logs-and-status",
		Timeout: 20 * time.Second,
		RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
			resp, err := chromev3.SendCommandByTarget(host, chromev3.CommandRequest{
				Command: "status",
				Role:    role,
			})
			if err != nil {
				appendTargetLogsToStep(sc, host, role, lines)
				return testv1.StepRunResult{}, fmt.Errorf("status failed: %w", err)
			}
			stdout, stderr, logErr := chromev3.ReadLogsByTarget(host, role, lines)
			if logErr != nil {
				return testv1.StepRunResult{}, fmt.Errorf("read logs: %w", logErr)
			}
			if !strings.Contains(stdout, "chrome src_v3 daemon handle") {
				return testv1.StepRunResult{}, fmt.Errorf("stdout log missing handled command lines")
			}
			if strings.Contains(stderr, "panic:") || strings.Contains(stderr, "fatal(") || strings.Contains(stderr, "fatal error:") {
				return testv1.StepRunResult{}, fmt.Errorf("stderr log contains fatal daemon output")
			}
			if !strings.Contains(stdout, "chrome src_v3 daemon ready") {
				sc.Infof("daemon ready line not present in current log tail; status rpc succeeded and command-handling lines are present")
			}
			logRemoteLogBlocks(sc, stdout, stderr)
			return testv1.StepRunResult{
				Report: fmt.Sprintf("chrome src_v3 logs captured and service remains healthy on %s (browser_pid=%d)", defaultIfBlank(host, "local"), resp.BrowserPID),
			}, nil
		},
	})
}

func resolveSuiteNATSURL() string {
	return configv1.ResolveREPLNATSURL()
}

func filterSteps(steps []testv1.Step, filterExpr string) []testv1.Step {
	filterExpr = strings.TrimSpace(strings.ToLower(filterExpr))
	if filterExpr == "" {
		return nil
	}
	parts := strings.Split(filterExpr, ",")
	tokens := make([]string, 0, len(parts))
	for _, part := range parts {
		token := strings.TrimSpace(strings.ToLower(part))
		if token != "" {
			tokens = append(tokens, token)
		}
	}
	if len(tokens) == 0 {
		return nil
	}
	out := make([]testv1.Step, 0, len(steps))
	for _, step := range steps {
		name := strings.ToLower(strings.TrimSpace(step.Name))
		for _, token := range tokens {
			if strings.Contains(name, token) {
				out = append(out, step)
				break
			}
		}
	}
	if len(out) == 0 {
		logs.Warn("chrome src_v3 test --filter=%q matched no steps; running all steps", filterExpr)
		return nil
	}
	return out
}

func appendTargetLogsToStep(sc *testv1.StepContext, host, role string, lines int) {
	stdout, stderr, err := chromev3.ReadLogsByTarget(host, role, lines)
	if err != nil {
		sc.Warnf("read remote logs failed: %v", err)
		return
	}
	logRemoteLogBlocks(sc, stdout, stderr)
}

func logRemoteLogBlocks(sc *testv1.StepContext, stdout, stderr string) {
	if strings.TrimSpace(stdout) != "" {
		sc.Infof("REMOTE_STDOUT_BEGIN")
		for _, line := range strings.Split(stdout, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				sc.Infof("REMOTE_STDOUT %s", line)
			}
		}
		sc.Infof("REMOTE_STDOUT_END")
	}
	if strings.TrimSpace(stderr) != "" {
		sc.Infof("REMOTE_STDERR_BEGIN")
		for _, line := range strings.Split(stderr, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				sc.Infof("REMOTE_STDERR %s", line)
			}
		}
		sc.Infof("REMOTE_STDERR_END")
	}
}

func resolveActionFlowOptions() actionFlowOptions {
	return normalizeActionFlowOptions(actionFlowOptions{
		InteractionCount: envIntDefault("DIALTONE_CHROME_SRC_V3_INTERACTION_COUNT", 6),
		ActionsPerSecond: envFloatDefault("DIALTONE_CHROME_SRC_V3_ACTIONS_PER_SECOND", 1.0),
	})
}

func normalizeActionFlowOptions(opts actionFlowOptions) actionFlowOptions {
	if opts.InteractionCount <= 0 {
		opts.InteractionCount = 1
	}
	return opts
}

func actionFlowTimeout(opts actionFlowOptions) time.Duration {
	opts = normalizeActionFlowOptions(opts)
	if opts.ActionsPerSecond <= 0 {
		return 60 * time.Second
	}
	pacedCommands := 2 + (opts.InteractionCount * 2)
	timeout := 30*time.Second + time.Duration(float64(pacedCommands)*float64(time.Second)/opts.ActionsPerSecond)
	if timeout < 60*time.Second {
		return 60 * time.Second
	}
	return timeout
}

func buildActionFlowRequests(role, marker string, opts actionFlowOptions) []chromev3.CommandRequest {
	opts = normalizeActionFlowOptions(opts)
	steps := []chromev3.CommandRequest{
		{Command: "open", Role: role, URL: "about:blank", ActionsPerSecond: opts.ActionsPerSecond},
		{Command: "set-html", Role: role, Value: actionSmokeHTML(marker), ActionsPerSecond: opts.ActionsPerSecond},
		{Command: "wait-log", Role: role, Contains: "page-ready:" + marker, TimeoutMS: 8000},
	}
	for i := 1; i <= opts.InteractionCount; i++ {
		value := fmt.Sprintf("dialtone-%02d", i)
		steps = append(steps,
			chromev3.CommandRequest{
				Command:          "type-aria",
				Role:             role,
				AriaLabel:        "Name Input",
				Value:            value,
				ActionsPerSecond: opts.ActionsPerSecond,
			},
			chromev3.CommandRequest{
				Command:   "wait-log",
				Role:      role,
				Contains:  fmt.Sprintf("typed:%s:%s", value, marker),
				TimeoutMS: 8000,
			},
			chromev3.CommandRequest{
				Command:          "click-aria",
				Role:             role,
				AriaLabel:        "Do Thing",
				ActionsPerSecond: opts.ActionsPerSecond,
			},
			chromev3.CommandRequest{
				Command:   "wait-log",
				Role:      role,
				Contains:  fmt.Sprintf("clicked:%d:%s", i, marker),
				TimeoutMS: 8000,
			},
		)
	}
	steps = append(steps,
		chromev3.CommandRequest{
			Command: "eval",
			Role:    role,
			Script: `JSON.stringify({
			  status: String((document.querySelector('[aria-label="Status"]') || {}).textContent || ""),
			  count: String(window.__dialtoneClickCount || 0),
			  value: String((document.querySelector('[aria-label="Name Input"]') || {}).value || "")
			})`,
		},
		chromev3.CommandRequest{
			Command:   "screenshot",
			Role:      role,
			TimeoutMS: 60000,
		},
	)
	return steps
}

func actionSmokeHTML(marker string) string {
	return fmt.Sprintf(`<!doctype html>
<html>
<head><meta charset="utf-8"><title>chrome-src-v3-actions</title></head>
<body>
  <input aria-label="Name Input" oninput="console.log('typed:' + this.value + ':%s')" />
  <button aria-label="Do Thing" onclick="window.__dialtoneClickCount=(window.__dialtoneClickCount||0)+1; document.querySelector('[aria-label=&quot;Status&quot;]').textContent='clicked-' + window.__dialtoneClickCount; console.log('clicked:' + window.__dialtoneClickCount + ':%s')">Go</button>
  <div aria-label="Status">idle</div>
  <script>window.__dialtoneClickCount=0;</script>
  <script>console.log('page-ready:%s')</script>
</body>
</html>`, marker, marker, marker)
}

func envIntDefault(name string, fallback int) int {
	raw := strings.TrimSpace(configv1.LookupEnvString(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func envFloatDefault(name string, fallback float64) float64 {
	raw := strings.TrimSpace(configv1.LookupEnvString(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return value
}

func defaultIfBlank(v, fallback string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return fallback
	}
	return v
}

func sanitizeToken(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "" {
		return "default"
	}
	v = strings.ReplaceAll(v, " ", "-")
	v = strings.ReplaceAll(v, "/", "-")
	v = strings.ReplaceAll(v, "\\", "-")
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-' || r == '_':
			return r
		default:
			return '-'
		}
	}, v)
}

func hasRecoveryIndicator(parts ...string) bool {
	needles := []string{
		"chrome didn't shut down correctly",
		"restore pages",
		"restore",
		"session crashed",
		"restore your pages",
		"chrome-error://",
		"chrome://crash",
	}
	for _, part := range parts {
		value := strings.ToLower(strings.TrimSpace(part))
		if value == "" {
			continue
		}
		for _, needle := range needles {
			if strings.Contains(value, needle) {
				return true
			}
		}
	}
	return false
}
