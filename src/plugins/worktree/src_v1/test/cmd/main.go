package main

import (
	"os"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
	selfcheck "dialtone/dev/plugins/worktree/src_v1/test/01_self_check"
)

func main() {
	logs.SetOutput(os.Stdout)

	reg := testv1.NewRegistry()
	selfcheck.Register(reg)

	logs.Info("Running worktree src_v1 tests in single process (%d steps)", len(reg.Steps))
	err := reg.Run(testv1.SuiteOptions{
		Version:       "worktree-src-v1",
		NATSURL:       "nats://127.0.0.1:4222",
		NATSSubject:   "logs.test.worktree-src-v1",
		AutoStartNATS: true,
	})
	if err != nil {
		logs.Error("worktree src_v1 tests failed: %v", err)
		os.Exit(1)
	}
	logs.Info("worktree src_v1 tests passed")
}
