package main

import (
	"os"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	bootinject "dialtone/dev/plugins/repl/src_v3/test/01_bootstrap_inject"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)

	reg := testv1.NewRegistry()
	bootinject.Register(reg)

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
