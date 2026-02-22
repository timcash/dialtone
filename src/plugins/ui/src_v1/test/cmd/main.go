package main

import (
	"os"

	"dialtone/dev/plugins/logs/src_v1/go"
	"dialtone/dev/plugins/ui/src_v1/test"
	buildserve "dialtone/dev/plugins/ui/src_v1/test/01_build_and_serve"
	navigation "dialtone/dev/plugins/ui/src_v1/test/02_sections_navigation"
	components "dialtone/dev/plugins/ui/src_v1/test/03_component_actions"
)

func main() {
	logs.SetOutput(os.Stdout)

	reg := test.NewRegistry()
	buildserve.Register(reg)
	navigation.Register(reg)
	components.Register(reg)

	logs.Info("Starting UI src_v1 suite with %d registered steps", len(reg.Steps))
	if err := test.RunSuiteV1(reg); err != nil {
		logs.Error("UI src_v1 suite failed: %v", err)
		os.Exit(1)
	}
	logs.Info("UI src_v1 suite passed")
}
