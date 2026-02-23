package main

import (
	"os"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	infra "dialtone/dev/plugins/logs/src_v1/test/01_infra"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)

	paths, err := logs.ResolvePaths("", "src_v1")
	if err != nil {
		logs.Error("logs test init failed: %v", err)
		os.Exit(1)
	}
	reportPath := paths.TestReport
	logPath := paths.TestLog
	errorLogPath := paths.TestErrorLog

	reg := testv1.NewRegistry()
	infra.Register(reg)
	logs.Info("Running logs src_v1 tests in single process (%d steps)", len(reg.Steps))

	if err := reg.Run(testv1.SuiteOptions{
		Version:       "logs-src-v1",
		ReportPath:    reportPath,
		LogPath:       logPath,
		ErrorLogPath:  errorLogPath,
		NATSURL:       "nats://127.0.0.1:4222",
		NATSSubject:   "logs.test.logs-src-v1",
		AutoStartNATS: true,
	}); err != nil {
		logs.Error("logs src_v1 tests failed: %v", err)
		os.Exit(1)
	}
	logs.Info("logs src_v1 tests passed")
}
