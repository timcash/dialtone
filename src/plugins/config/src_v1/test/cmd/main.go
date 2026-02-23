package main

import (
	"os"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
	s1 "dialtone/dev/plugins/config/src_v1/test/01_runtime"
	s2 "dialtone/dev/plugins/config/src_v1/test/02_apply"
	logs "dialtone/dev/plugins/logs/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)

	reg := testv1.NewRegistry()
	s1.Register(reg)
	s2.Register(reg)

	rt, err := configv1.ResolveRuntime("")
	if err != nil {
		logs.Error("config runtime error: %v", err)
		os.Exit(1)
	}

	if err := reg.Run(testv1.SuiteOptions{
		Version:       "src_v1",
		NATSURL:       "nats://127.0.0.1:4222",
		NATSSubject:   "logs.config.src_v1.test",
		AutoStartNATS: true,
		ReportPath:    configv1.PluginPath(rt, "config", "src_v1", "test", "TEST.md"),
	}); err != nil {
		logs.Error("config tests failed: %v", err)
		os.Exit(1)
	}

	logs.Info("config tests passed")
}
