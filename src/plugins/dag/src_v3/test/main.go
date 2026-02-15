package main

import (
	"fmt"
	"os"

	test_v2 "dialtone/cli/src/libs/test_v2"
)

func main() {
	steps := []test_v2.Step{
		{Name: "01 Preflight (Go/UI)", Run: Run01Preflight},
		{Name: "02 Hit-Test Section Validation", Run: Run02HitTestSectionValidation, SectionID: "hit-test", Screenshot: "screenshots/test_step_1.png"},
		{Name: "03 Cleanup Verification", Run: Run03CleanupVerification},
	}

	if err := test_v2.RunSuite(test_v2.SuiteOptions{
		Version:      "src_v3",
		ReportPath:   "src/plugins/dag/src_v3/test/TEST.md",
		LogPath:      "src/plugins/dag/src_v3/test/test.log",
		ErrorLogPath: "src/plugins/dag/src_v3/test/error.log",
	}, steps); err != nil {
		fmt.Printf("[TEST] SUITE ERROR: %v\n", err)
		os.Exit(1)
	}
}
