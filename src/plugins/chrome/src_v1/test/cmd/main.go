package main

import (
	"flag"
	"os"
	"strings"

	sessions "dialtone/dev/plugins/chrome/src_v1/test/01_session_lifecycle"
	example "dialtone/dev/plugins/chrome/src_v1/test/02_example_library"
	logs "dialtone/dev/plugins/logs/src_v1/go"
	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	fs := flag.NewFlagSet("chrome test", flag.ContinueOnError)
	commonFlags := testv1.BindCommonTestFlags(fs, testv1.CommonTestCLIOptions{})
	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(1)
	}
	common, err := commonFlags.Resolve()
	if err != nil {
		logs.Error("chrome test parse failed: %v", err)
		os.Exit(1)
	}
	common.ApplyRuntimeConfig()
	defer func() {
		if r := recover(); r != nil {
			logs.Error("[PROCESS][PANIC] chrome src_v1 test runner panic: %v", r)
			os.Exit(1)
		}
	}()

	reg := testv1.NewRegistry()
	example.Register(reg)
	sessions.Register(reg)
	if filtered := filterSteps(reg.Steps, strings.TrimSpace(common.FilterExpr)); len(filtered) > 0 {
		reg.Steps = filtered
	}
	if strings.TrimSpace(common.AttachNode) != "" {
		attachNode := strings.TrimSpace(common.AttachNode)
		if !common.NoSSH {
			if node, nerr := sshv1.ResolveMeshNode(attachNode); nerr == nil && strings.EqualFold(strings.TrimSpace(node.OS), "windows") && node.PreferWSLPowerShell {
				testv1.UpdateRuntimeConfig(func(cfg *testv1.RuntimeConfig) {
					// For Windows+powershell transport, keep attach mode remote-first
					// and disable SSH fallback by default.
					cfg.NoSSH = true
					if cfg.RemoteDebugPort <= 0 {
						cfg.RemoteDebugPort = 9333
					}
					if len(cfg.RemoteDebugPorts) == 0 {
						cfg.RemoteDebugPorts = []int{9333, 9334, 9335}
					}
				})
				logs.Info("chrome test attach mode auto-enabled --no-ssh for windows powershell node=%s", attachNode)
			}
		}
		logs.Info("chrome test attach mode node=%s url=%s", strings.TrimSpace(common.AttachNode), strings.TrimSpace(common.TargetURL))
	}

	logs.Info("Running chrome src_v1 tests in single process (%d steps)", len(reg.Steps))
	err = reg.Run(common.ApplySuiteOptions(testv1.SuiteOptions{
		Version:       "chrome-src-v1",
		ReportPath:    "plugins/chrome/src_v1/TEST.md",
		RawReportPath: "plugins/chrome/src_v1/TEST_RAW.md",
		ReportFormat:  "template",
		ReportTitle:   "Chrome Plugin src_v1 Test Report",
		ReportRunner:  "test/src_v1",
		NATSURL:       "nats://127.0.0.1:4222",
		NATSSubject:   "logs.test.chrome-src-v1",
		AutoStartNATS: true,
	}))
	if err != nil {
		logs.Error("[PROCESS][ERROR] chrome src_v1 tests failed: %v", err)
		logs.Error("chrome src_v1 tests failed: %v", err)
		os.Exit(1)
	}
	logs.Info("chrome src_v1 tests passed")
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
		logs.Warn("chrome test --filter=%q matched no steps; running all steps", filterExpr)
		return nil
	}
	names := make([]string, 0, len(out))
	for _, step := range out {
		names = append(names, step.Name)
	}
	logs.Info("chrome test --filter=%q selected steps: %s", filterExpr, strings.Join(names, ", "))
	return out
}
