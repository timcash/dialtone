package main

import (
	"errors"
	"flag"
	"os"
	"os/exec"
	"strings"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
	buildbinary "dialtone/dev/plugins/robot/src_v2/test/01_build_binary"
	serverruntime "dialtone/dev/plugins/robot/src_v2/test/02_server_runtime"
	manifestcontract "dialtone/dev/plugins/robot/src_v2/test/03_manifest_contract"
	localuimocke2e "dialtone/dev/plugins/robot/src_v2/test/04_local_ui_mock_e2e"
	autoswapcomposerun "dialtone/dev/plugins/robot/src_v2/test/05_autoswap_compose_run"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	rt, err := configv1.ResolveRuntime("")
	if err != nil {
		logs.Error("robot src_v2 resolve runtime failed: %v", err)
		os.Exit(1)
	}

	fs := flag.NewFlagSet("robot-src-v2-test", flag.ExitOnError)
	defaultAttachNode := strings.TrimSpace(os.Getenv("DIALTONE_TEST_BROWSER_NODE"))
	if defaultAttachNode == "" && robotIsWSL() {
		defaultAttachNode = "legion"
	}
	commonBindings := testv1.BindCommonTestFlags(fs, testv1.CommonTestCLIOptions{
		AttachNode:       defaultAttachNode,
		AttachRole:       "robot-dev",
		ActionsPerMinute: 300,
	})
	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		logs.Error("robot src_v2 parse failed: %v", err)
		os.Exit(1)
	}
	commonOpts, err := commonBindings.Resolve()
	if err != nil {
		logs.Error("robot src_v2 parse failed: %v", err)
		os.Exit(1)
	}
	commonOpts.ApplyRuntimeConfig()
	cleanupRobotTestServers()

	reg := testv1.NewRegistry()
	buildbinary.Register(reg)
	serverruntime.Register(reg)
	manifestcontract.Register(reg)
	localuimocke2e.Register(reg)
	autoswapcomposerun.Register(reg)
	if filtered := filterSteps(reg.Steps, strings.TrimSpace(commonOpts.FilterExpr)); len(filtered) > 0 {
		reg.Steps = filtered
	}

	suiteOpts := commonOpts.ApplySuiteOptions(testv1.SuiteOptions{
		Version:              "robot-src-v2",
		RepoRoot:             rt.RepoRoot,
		NATSURL:              "nats://127.0.0.1:4222",
		NATSSubject:          "logs.test.robot-src-v2",
		AutoStartNATS:        true,
		ReportPath:           "plugins/robot/src_v2/test/TEST.md",
		LogPath:              "plugins/robot/src_v2/test/test.log",
		ErrorLogPath:         "plugins/robot/src_v2/test/error.log",
		PreserveSharedBrowser: true,
		SkipBrowserCleanup:    true,
	})
	err = reg.Run(suiteOpts)
	if err != nil {
		logs.Error("robot src_v2 suite failed: %v", err)
		os.Exit(1)
	}
}

func cleanupRobotTestServers() {
	patterns := []string{
		`dialtone_robot_v2.*--listen :18082`,
		`dialtone_robot_v2.*--listen :18083`,
	}
	for _, pattern := range patterns {
		cmd := exec.Command("pkill", "-f", pattern)
		_ = cmd.Run()
	}
}

func robotIsWSL() bool {
	if strings.TrimSpace(os.Getenv("WSL_DISTRO_NAME")) != "" {
		return true
	}
	raw, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(raw)), "microsoft")
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
		logs.Warn("robot src_v2 --filter=%q matched no steps; running all steps", filterExpr)
		return nil
	}
	names := make([]string, 0, len(out))
	for _, step := range out {
		names = append(names, step.Name)
	}
	logs.Info("robot src_v2 --filter=%q selected steps: %s", filterExpr, strings.Join(names, ", "))
	return out
}
