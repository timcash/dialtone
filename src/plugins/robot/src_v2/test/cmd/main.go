package main

import (
	"os"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	runtimesmoke "dialtone/dev/plugins/robot/src_v2/test/01_runtime_smoke"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	reg := testv1.NewRegistry()
	runtimesmoke.Register(reg)

	err := reg.Run(testv1.SuiteOptions{
		Version:       "robot-src-v2",
		NATSURL:       "nats://127.0.0.1:4222",
		NATSSubject:   "logs.test.robot-src-v2",
		AutoStartNATS: true,
		ReportPath:    "plugins/robot/src_v2/test/TEST.md",
		LogPath:       "plugins/robot/src_v2/test/test.log",
		ErrorLogPath:  "plugins/robot/src_v2/test/error.log",
	})
	if err != nil {
		logs.Error("robot src_v2 suite failed: %v", err)
		os.Exit(1)
	}
}
