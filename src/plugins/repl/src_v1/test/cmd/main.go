package main

import (
	"os"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	replcore "dialtone/dev/plugins/repl/src_v1/test/01_repl_core"
	procplugin "dialtone/dev/plugins/repl/src_v1/test/02_proc_plugin"
	logsplugin "dialtone/dev/plugins/repl/src_v1/test/03_logs_plugin"
	testplugin "dialtone/dev/plugins/repl/src_v1/test/04_test_plugin"
	chromeplugin "dialtone/dev/plugins/repl/src_v1/test/05_chrome_plugin"
	gobunplugins "dialtone/dev/plugins/repl/src_v1/test/06_go_bun_plugins"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)

	reg := testv1.NewRegistry()
	replcore.Register(reg)
	procplugin.Register(reg)
	logsplugin.Register(reg)
	testplugin.Register(reg)
	chromeplugin.Register(reg)
	gobunplugins.Register(reg)

	logs.Info("Running repl src_v1 tests in single process (%d steps)", len(reg.Steps))
	err := reg.Run(testv1.SuiteOptions{
		Version:       "repl-src-v1",
		NATSURL:       "nats://127.0.0.1:4222",
		NATSSubject:   "logs.test.repl-src-v1",
		AutoStartNATS: true,
	})
	if err != nil {
		logs.Error("repl src_v1 tests failed: %v", err)
		os.Exit(1)
	}
	logs.Info("repl src_v1 tests passed")
}
