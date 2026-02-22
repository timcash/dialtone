package main

import (
	"os"

	selfcheck "dialtone/dev/plugins/bun/src_v1/test/01_self_check"
	logs "dialtone/dev/plugins/logs/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)

	reg := testv1.NewRegistry()
	selfcheck.Register(reg)

	logs.Info("Running bun src_v1 tests in single process (%d steps)", len(reg.Steps))
	err := reg.Run(testv1.SuiteOptions{
		Version:       "bun-src-v1",
		NATSURL:       "nats://127.0.0.1:4222",
		NATSSubject:   "logs.test.bun-src-v1",
		AutoStartNATS: true,
	})
	if err != nil {
		logs.Error("bun src_v1 tests failed: %v", err)
		os.Exit(1)
	}
	logs.Info("bun src_v1 tests passed")
}
