package main

import (
	"errors"
	"flag"
	"os"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	processmanager "dialtone/dev/plugins/repl/src_v3/test/00_process_manager"
	tmpworkspace "dialtone/dev/plugins/repl/src_v3/test/01_tmp_workspace"
	clihelp "dialtone/dev/plugins/repl/src_v3/test/02_cli_help"
	bootstrapcfg "dialtone/dev/plugins/repl/src_v3/test/03_bootstrap_config"
	replhelpps "dialtone/dev/plugins/repl/src_v3/test/04_repl_help_ps"
	tsnetephemeral "dialtone/dev/plugins/repl/src_v3/test/07_tsnet_ephemeral"
	replloggingcontract "dialtone/dev/plugins/repl/src_v3/test/10_repl_logging_contract"
	testdaemonfixture "dialtone/dev/plugins/repl/src_v3/test/11_testdaemon_fixture"
	taskkvstate "dialtone/dev/plugins/repl/src_v3/test/12_task_kv_state"
	"dialtone/dev/plugins/repl/src_v3/test/support"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func main() {
	os.Exit(run())
}

func run() int {
	logs.SetOutput(os.Stdout)
	const reportPath = "plugins/repl/src_v3/TEST.md"
	const rawReportPath = "plugins/repl/src_v3/TEST_RAW.md"
	fs := flag.NewFlagSet("repl-src-v3-test", flag.ContinueOnError)
	filter := fs.String("filter", "", "Run only matching test steps")
	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		logs.Error("repl src_v3 test parse failed: %v", err)
		return 1
	}

	reg := testv1.NewRegistry()
	processmanager.Register(reg)
	tmpworkspace.Register(reg)
	clihelp.Register(reg)
	tsnetephemeral.Register(reg)
	bootstrapcfg.Register(reg)
	replhelpps.Register(reg)
	replloggingcontract.Register(reg)
	testdaemonfixture.Register(reg)
	taskkvstate.Register(reg)
	if filtered := filterSteps(reg.Steps, strings.TrimSpace(*filter)); len(filtered) > 0 {
		reg.Steps = filtered
	}
	logs.Info("Running repl src_v3 tests in single process (%d steps)", len(reg.Steps))
	support.EnableSharedRuntime()
	defer support.CloseSharedRuntime()

	err := reg.Run(testv1.SuiteOptions{
		Version:       "repl-src-v3",
		NATSURL:       "nats://127.0.0.1:46222",
		NATSListenURL: "nats://0.0.0.0:46222",
		NATSSubject:   "logs.test.repl-src-v3",
		AutoStartNATS: false,
		QuietConsole:  true,
		ReportPath:    reportPath,
		RawReportPath: rawReportPath,
		ReportFormat:  "template",
		ReportTitle:   "REPL Plugin src_v3 Test Report",
		ReportRunner:  "test/src_v1",
	})
	if err != nil {
		logs.Error("repl src_v3 tests failed: %v", err)
		return 1
	}
	logs.Info("repl src_v3 tests passed (report=%s raw=%s)", reportPath, rawReportPath)
	return 0
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
		logs.Warn("repl src_v3 test --filter=%q matched no steps; running all steps", filterExpr)
		return nil
	}
	names := make([]string, 0, len(out))
	for _, step := range out {
		names = append(names, step.Name)
	}
	logs.Info("repl src_v3 test --filter=%q selected steps: %s", filterExpr, strings.Join(names, ", "))
	return out
}
