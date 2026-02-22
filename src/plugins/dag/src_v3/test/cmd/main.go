package main

import (
	"os"

	"dialtone/dev/plugins/dag/src_v3/test"
	"dialtone/dev/plugins/logs/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	if err := test.RunSuiteV3(); err != nil {
		logs.Error("DAG src_v3 suite failed: %v", err)
		os.Exit(1)
	}
	logs.Info("DAG src_v3 suite passed")
}
