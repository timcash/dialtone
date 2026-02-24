package main

import (
	"os"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	replcore "dialtone/dev/plugins/repl/src_v1/test/01_repl_core"
	procplugin "dialtone/dev/plugins/repl/src_v1/test/02_proc_plugin"
	logsplugin "dialtone/dev/plugins/repl/src_v1/test/03_logs_plugin"
	testplugin "dialtone/dev/plugins/repl/src_v1/test/04_test_plugin"
	chromeplugin "dialtone/dev/plugins/repl/src_v1/test/05_chrome_plugin"
	gobunplugins "dialtone/dev/plugins/repl/src_v1/test/06_go_bun_plugins"
	multiplayer "dialtone/dev/plugins/repl/src_v1/test/99_multiplayer"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	mode := "all"
	if len(os.Args) > 1 {
		mode = strings.TrimSpace(os.Args[1])
	}

	reg := testv1.NewRegistry()
	switch mode {
	case "multiplayer":
		multiplayer.Register(reg)
	default:
		replcore.Register(reg)
		procplugin.Register(reg)
		logsplugin.Register(reg)
		testplugin.Register(reg)
		chromeplugin.Register(reg)
		gobunplugins.Register(reg)
		multiplayer.Register(reg)
	}

	logs.Info("Running repl src_v1 tests in single process (%d steps, mode=%s)", len(reg.Steps), mode)
	suiteNATSURL := "nats://127.0.0.1:4222"
	suiteNATSListenURL := ""
	if mode == "multiplayer" {
		suiteNATSURL = "nats://127.0.0.1:44222"
		suiteNATSListenURL = "nats://0.0.0.0:44222"
	}
	err := reg.Run(testv1.SuiteOptions{
		Version:       "repl-src-v1",
		NATSURL:       suiteNATSURL,
		NATSListenURL: suiteNATSListenURL,
		NATSSubject:   "logs.test.repl-src-v1",
		AutoStartNATS: true,
	})
	if err != nil {
		logs.Error("repl src_v1 tests failed: %v", err)
		os.Exit(1)
	}
	logs.Info("repl src_v1 tests passed")
}
