package main

import (
	"errors"
	"flag"
	"os"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
	selfcheck "dialtone/dev/plugins/test/src_v1/test/01_self_check"
	example "dialtone/dev/plugins/test/src_v1/test/02_example_plugin_template"
	browserctx "dialtone/dev/plugins/test/src_v1/test/03_browser_ctx"
	natswait "dialtone/dev/plugins/test/src_v1/test/04_nats_wait_patterns"
	browseropts "dialtone/dev/plugins/test/src_v1/test/05_browser_lifecycle_options"
	autoscreenshot "dialtone/dev/plugins/test/src_v1/test/06_auto_screenshot"
)

func main() {
	logs.SetOutput(os.Stdout)
	fs := flag.NewFlagSet("test test", flag.ContinueOnError)
	commonFlags := testv1.BindCommonTestFlags(fs, testv1.CommonTestCLIOptions{})
	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		logs.Error("test src_v1 parse failed: %v", err)
		os.Exit(1)
	}
	common, err := commonFlags.Resolve()
	if err != nil {
		logs.Error("test src_v1 parse failed: %v", err)
		os.Exit(1)
	}
	// Keep attach-node opt-in. In WSL, local browser start should work and is
	// more reliable than depending on remote node reachability.
	common.ApplyRuntimeConfig()
	if attachNode := strings.TrimSpace(common.AttachNode); attachNode != "" && !common.NoSSH {
		if node, nerr := sshv1.ResolveMeshNode(attachNode); nerr == nil && strings.EqualFold(strings.TrimSpace(node.OS), "windows") && node.PreferWSLPowerShell {
			testv1.UpdateRuntimeConfig(func(cfg *testv1.RuntimeConfig) {
				if cfg.RemoteDebugPort <= 0 {
					cfg.RemoteDebugPort = 9333
				}
				if len(cfg.RemoteDebugPorts) == 0 {
					cfg.RemoteDebugPorts = []int{9333, 9334, 9335}
				}
			})
			logs.Info("test src_v1 enabled windows debug port defaults for node=%s (SSH fallback allowed)", attachNode)
		}
	}

	reg := testv1.NewRegistry()
	selfcheck.Register(reg)
	example.Register(reg)
	browserctx.Register(reg)
	natswait.Register(reg)
	browseropts.Register(reg)
	autoscreenshot.Register(reg)
	if filtered := filterSteps(reg.Steps, strings.TrimSpace(common.FilterExpr)); len(filtered) > 0 {
		reg.Steps = filtered
	}

	logs.Info("Starting test plugin suite in single process with %d registered steps", len(reg.Steps))
	err = reg.Run(common.ApplySuiteOptions(testv1.SuiteOptions{
		Version:       "src-v1-self-check",
		ReportPath:    "plugins/test/src_v1/TEST.md",
		RawReportPath: "plugins/test/src_v1/TEST_RAW.md",
		ReportFormat:  "template",
		ReportTitle:   "Test Plugin src_v1 Self-Check Report",
		ReportRunner:  "test/src_v1",
		NATSURL:       "nats://127.0.0.1:4222",
		NATSSubject:   "logs.test.src-v1-self-check",
		AutoStartNATS: true,
	}))
	if err != nil {
		logs.Error("Test plugin suite failed: %v", err)
		os.Exit(1)
	}
	logs.Info("Test plugin suite passed")
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
		if token == "" {
			continue
		}
		tokens = append(tokens, token)
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
		logs.Warn("test src_v1 --filter=%q matched no steps; running all steps", filterExpr)
		return nil
	}
	names := make([]string, 0, len(out))
	for _, step := range out {
		names = append(names, step.Name)
	}
	logs.Info("test src_v1 --filter=%q selected steps: %s", filterExpr, strings.Join(names, ", "))
	return out
}
