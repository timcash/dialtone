package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	chromev3 "dialtone/dev/plugins/chrome/src_v3"
	logs "dialtone/dev/plugins/logs/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	fs := flag.NewFlagSet("chrome src_v3 test", flag.ContinueOnError)
	host := fs.String("host", "", "Mesh host (optional; default local)")
	role := fs.String("role", "dev", "Chrome role")
	lines := fs.Int("lines", 80, "Remote daemon log lines to include")
	filter := fs.String("filter", "", "Run only matching test steps")
	if err := fs.Parse(os.Args[1:]); err != nil {
		logs.Error("chrome src_v3 test parse failed: %v", err)
		os.Exit(1)
	}
	hostValue := strings.TrimSpace(*host)
	roleValue := defaultIfBlank(strings.TrimSpace(*role), "dev")
	reportNode := hostValue
	if reportNode == "" {
		reportNode = "local"
	}

	reg := testv1.NewRegistry()
	addChromeSuiteSteps(reg, hostValue, roleValue, *lines)
	if filteredSteps := filterSteps(reg.Steps, strings.TrimSpace(*filter)); len(filteredSteps) > 0 {
		reg.Steps = filteredSteps
	}

	logs.Info("chrome src_v3 test starting host=%s role=%s steps=%d", reportNode, roleValue, len(reg.Steps))
	if err := reg.Run(testv1.SuiteOptions{
		Version:          "chrome-src-v3",
		ReportPath:       "plugins/chrome/src_v3/TEST.md",
		RawReportPath:    "plugins/chrome/src_v3/TEST_RAW.md",
		ReportFormat:     "template",
		ReportTitle:      "Chrome src_v3 Test Report",
		ReportRunner:     "test/src_v1",
		ChromeReportNode: reportNode,
		NATSURL:          "nats://127.0.0.1:4222",
		NATSSubject:      "logs.test.chrome-src-v3",
		AutoStartNATS:    true,
	}); err != nil {
		logs.Error("chrome src_v3 test failed: %v", err)
		os.Exit(1)
	}
	logs.Info("chrome src_v3 test passed")
}

func addChromeSuiteSteps(reg *testv1.Registry, host, role string, lines int) {
	reg.Add(testv1.Step{
		Name:    "chrome-deploy-and-start",
		Timeout: 120 * time.Second,
		RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
			resp, err := chromev3.EnsureServiceByTarget(host, role, true)
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
		Name:    "chrome-browser-actions-and-screenshot",
		Timeout: 45 * time.Second,
		RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
			marker := fmt.Sprintf("%d", time.Now().UnixNano())
			steps := []chromev3.CommandRequest{
				{Command: "open", Role: role, URL: "about:blank"},
				{Command: "set-html", Role: role, Value: actionSmokeHTML(marker)},
				{Command: "wait-log", Role: role, Contains: "page-ready:" + marker, TimeoutMS: 8000},
				{Command: "type-aria", Role: role, AriaLabel: "Name Input", Value: "dialtone"},
				{Command: "wait-log", Role: role, Contains: "typed:dialtone:" + marker, TimeoutMS: 8000},
				{Command: "click-aria", Role: role, AriaLabel: "Do Thing"},
				{Command: "wait-log", Role: role, Contains: "clicked:" + marker, TimeoutMS: 8000},
				{Command: "screenshot", Role: role},
			}
			var screenshotResp *chromev3.CommandResponse
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
			}
			if screenshotResp == nil || strings.TrimSpace(screenshotResp.ScreenshotB64) == "" {
				appendTargetLogsToStep(sc, host, role, lines)
				return testv1.StepRunResult{}, fmt.Errorf("screenshot response missing image data")
			}
			shotPath := filepath.Join("plugins", "chrome", "src_v3", "screenshots", fmt.Sprintf("chrome_src_v3_actions_%s.png", sanitizeToken(defaultIfBlank(host, "local"))))
			if err := writeScreenshot(shotPath, screenshotResp.ScreenshotB64); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := sc.AddScreenshot(shotPath); err != nil {
				return testv1.StepRunResult{}, err
			}
			appendTargetLogsToStep(sc, host, role, lines)
			return testv1.StepRunResult{
				Report: fmt.Sprintf("chrome src_v3 action flow passed on %s with screenshot capture", defaultIfBlank(host, "local")),
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
			if !strings.Contains(stdout, "chrome src_v3 daemon ready") {
				return testv1.StepRunResult{}, fmt.Errorf("stdout log missing daemon ready line")
			}
			if !strings.Contains(stdout, "chrome src_v3 daemon handle") {
				return testv1.StepRunResult{}, fmt.Errorf("stdout log missing handled command lines")
			}
			logRemoteLogBlocks(sc, stdout, stderr)
			return testv1.StepRunResult{
				Report: fmt.Sprintf("chrome src_v3 logs captured and service remains healthy on %s (browser_pid=%d)", defaultIfBlank(host, "local"), resp.BrowserPID),
			}, nil
		},
	})
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

func writeScreenshot(path string, rawB64 string) error {
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(rawB64))
	if err != nil {
		return fmt.Errorf("decode screenshot: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func actionSmokeHTML(marker string) string {
	return fmt.Sprintf(`<!doctype html>
<html>
<head><meta charset="utf-8"><title>chrome-src-v3-actions</title></head>
<body>
  <input aria-label="Name Input" oninput="console.log('typed:' + this.value + ':%s')" />
  <button aria-label="Do Thing" onclick="document.querySelector('[aria-label=&quot;Status&quot;]').textContent='clicked'; console.log('clicked:%s')">Go</button>
  <div aria-label="Status">idle</div>
  <script>console.log('page-ready:%s')</script>
</body>
</html>`, marker, marker, marker)
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
