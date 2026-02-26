package main

import (
	"os"

	composesmoke "dialtone/dev/plugins/autoswap/src_v1/test/01_compose_smoke"
	logs "dialtone/dev/plugins/logs/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	reg := testv1.NewRegistry()
	composesmoke.Register(reg)

	err := reg.Run(testv1.SuiteOptions{
		Version:       "autoswap-src-v1",
		NATSURL:       "nats://127.0.0.1:4222",
		NATSSubject:   "logs.test.autoswap-src-v1",
		AutoStartNATS: true,
		ReportPath:    "plugins/autoswap/src_v1/test/TEST.md",
	})
	if err != nil {
		logs.Error("autoswap src_v1 suite failed: %v", err)
		os.Exit(1)
	}
}
