package main

import (
	"os"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
	selfcheck "dialtone/dev/plugins/test/src_v1/test/01_self_check"
	example "dialtone/dev/plugins/test/src_v1/test/02_example_plugin_template"
)

func main() {
	logs.SetOutput(os.Stdout)

	reg := testv1.NewRegistry()
	selfcheck.Register(reg)
	example.Register(reg)

	logs.Info("Starting test plugin suite in single process with %d registered steps", len(reg.Steps))
	err := reg.Run(testv1.SuiteOptions{
		Version:       "src-v1-self-check",
		NATSURL:       "nats://127.0.0.1:4222",
		NATSSubject:   "logs.test.src-v1-self-check",
		AutoStartNATS: true,
	})
	if err != nil {
		logs.Error("Test plugin suite failed: %v", err)
		os.Exit(1)
	}
	logs.Info("Test plugin suite passed")
}
