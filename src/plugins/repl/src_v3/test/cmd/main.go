package main

import (
	"os"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	tmpworkspace "dialtone/dev/plugins/repl/src_v3/test/01_tmp_workspace"
	clihelp "dialtone/dev/plugins/repl/src_v3/test/02_cli_help"
	bootstrapcfg "dialtone/dev/plugins/repl/src_v3/test/03_bootstrap_config"
	replhelpps "dialtone/dev/plugins/repl/src_v3/test/04_repl_help_ps"
	sshwsl "dialtone/dev/plugins/repl/src_v3/test/05_ssh_wsl"
	cloudflaretunnel "dialtone/dev/plugins/repl/src_v3/test/06_cloudflare_tunnel"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)

	reg := testv1.NewRegistry()
	tmpworkspace.Register(reg)
	clihelp.Register(reg)
	bootstrapcfg.Register(reg)
	replhelpps.Register(reg)
	sshwsl.Register(reg)
	cloudflaretunnel.Register(reg)

	err := reg.Run(testv1.SuiteOptions{
		Version:       "repl-src-v3",
		NATSURL:       "nats://127.0.0.1:46222",
		NATSListenURL: "nats://0.0.0.0:46222",
		NATSSubject:   "logs.test.repl-src-v3",
		AutoStartNATS: false,
		ReportPath:    "plugins/repl/src_v3/TEST.md",
		RawReportPath: "plugins/repl/src_v3/TEST_RAW.md",
		ReportFormat:  "template",
		ReportTitle:   "REPL Plugin src_v3 Test Report",
		ReportRunner:  "test/src_v1",
	})
	if err != nil {
		logs.Error("repl src_v3 tests failed: %v", err)
		os.Exit(1)
	}
	logs.Info("repl src_v3 tests passed")
}
