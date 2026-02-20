package main

import (
	"fmt"
	"os"

	test_v2 "dialtone/dev/plugins/test"
)

func main() {
	steps := []test_v2.Step{
		{Name: "01 Preflight (Go/UI)", Run: Run01Preflight},
		{Name: "02 Go Run", Run: Run07GoRun},
		{Name: "03 UI Run", Run: Run08UIRun},
		{Name: "04 Expected Errors (Proof of Life)", Run: Run09ExpectedErrorsProofOfLife},
		{Name: "05 Dev Server Running (latest UI)", Run: Run10DevServerRunningLatestUI},
		{Name: "06 Hero Section Validation", Run: Run11HeroSectionValidation, SectionID: "hero", Screenshot: "screenshots/test_step_1.png"},
		{Name: "07 Docs Section Validation", Run: Run12DocsSectionValidation, SectionID: "docs", Screenshot: "screenshots/test_step_2.png"},
		{Name: "08 Table Section Validation", Run: Run13TableSectionValidation, SectionID: "table", Screenshot: "screenshots/test_step_3.png"},
		{Name: "09 Three Section Validation", Run: Run14ThreeSectionValidation, SectionID: "three", Screenshot: "screenshots/test_step_4.png"},
		{Name: "10 Xterm Section Validation", Run: Run15XtermSectionValidation, SectionID: "xterm", Screenshot: "screenshots/test_step_5.png"},
		{Name: "11 Lifecycle / Invariants", Run: Run17LifecycleInvariants},
		{Name: "12 Cleanup Verification", Run: Run18CleanupVerification},
	}

	if err := test_v2.RunSuite(test_v2.SuiteOptions{
		Version:    "src_v1",
		ReportPath: "src/plugins/vpn/src_v1/test/TEST.md",
		LogPath:    "src/plugins/vpn/src_v1/test/test.log",
	}, steps); err != nil {
		fmt.Printf("[TEST] SUITE ERROR: %v\n", err)
		os.Exit(1)
	}
}
