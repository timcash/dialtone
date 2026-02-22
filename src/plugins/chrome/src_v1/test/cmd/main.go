package main

import (
	"os"

	sessions "dialtone/dev/plugins/chrome/src_v1/test/01_session_lifecycle"
	example "dialtone/dev/plugins/chrome/src_v1/test/02_example_library"
	logs "dialtone/dev/plugins/logs/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	defer func() {
		if r := recover(); r != nil {
			logs.Error("[PROCESS][PANIC] chrome src_v1 test runner panic: %v", r)
			os.Exit(1)
		}
	}()

	reg := testv1.NewRegistry()
	example.Register(reg)
	sessions.Register(reg)

	logs.Info("Running chrome src_v1 tests in single process (%d steps)", len(reg.Steps))
	err := reg.Run(testv1.SuiteOptions{
		Version:       "chrome-src-v1",
		NATSURL:       "nats://127.0.0.1:4222",
		NATSSubject:   "logs.test.chrome-src-v1",
		AutoStartNATS: true,
	})
	if err != nil {
		logs.Error("[PROCESS][ERROR] chrome src_v1 tests failed: %v", err)
		logs.Error("chrome src_v1 tests failed: %v", err)
		os.Exit(1)
	}
	logs.Info("chrome src_v1 tests passed")
}
