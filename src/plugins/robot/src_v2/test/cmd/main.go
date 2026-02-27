package main

import (
	"os"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
	buildbinary "dialtone/dev/plugins/robot/src_v2/test/01_build_binary"
	serverruntime "dialtone/dev/plugins/robot/src_v2/test/02_server_runtime"
	manifestcontract "dialtone/dev/plugins/robot/src_v2/test/03_manifest_contract"
	localuimocke2e "dialtone/dev/plugins/robot/src_v2/test/04_local_ui_mock_e2e"
	autoswapcomposerun "dialtone/dev/plugins/robot/src_v2/test/05_autoswap_compose_run"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	rt, err := configv1.ResolveRuntime("")
	if err != nil {
		logs.Error("robot src_v2 resolve runtime failed: %v", err)
		os.Exit(1)
	}
	reg := testv1.NewRegistry()
	buildbinary.Register(reg)
	serverruntime.Register(reg)
	manifestcontract.Register(reg)
	localuimocke2e.Register(reg)
	autoswapcomposerun.Register(reg)

	err = reg.Run(testv1.SuiteOptions{
		Version:       "robot-src-v2",
		RepoRoot:      rt.RepoRoot,
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
