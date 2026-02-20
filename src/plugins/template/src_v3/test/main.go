package main

import (
	"fmt"
	"os"

	test_v2 "dialtone/dev/plugins/dag/src_v3/suite"
)

func main() {
	steps := []test_v2.Step{
		{Name: "01 Preflight (Go/UI)", Run: Run01Preflight},
		{Name: "02 Go Run", Run: Run07GoRun},
		{Name: "03 UI Run", Run: Run08UIRun},
		{Name: "04 Expected Errors (Proof of Life)", Run: Run09ExpectedErrorsProofOfLife},
		{Name: "05 Dev Server Running (latest UI)", Run: Run10DevServerRunningLatestUI},
		{Name: "06 Hero Section Validation", Run: Run11HeroSectionValidation, SectionID: "template-hero-stage", Screenshot: "screenshots/test_step_1.png"},
		{Name: "07 Docs Section Validation", Run: Run12DocsSectionValidation, SectionID: "template-docs-docs", Screenshot: "screenshots/test_step_2.png"},
		{Name: "08 Table Section Validation", Run: Run13TableSectionValidation, SectionID: "template-meta-table", Screenshot: "screenshots/test_step_3.png"},
		{Name: "09 Three Section Validation", Run: Run14ThreeSectionValidation, SectionID: "template-three-stage", Screenshot: "screenshots/test_step_4.png"},
		{Name: "10 Log Section Validation", Run: Run15LogSectionValidation, SectionID: "template-log-xterm", Screenshot: "screenshots/test_step_5.png"},
		{Name: "11 Video Section Validation", Run: Run16VideoSectionValidation, SectionID: "template-demo-video", Screenshot: "screenshots/test_step_6.png"},
		{Name: "12 Lifecycle / Invariants", Run: Run17LifecycleInvariants},
		{Name: "13 Cleanup Verification", Run: Run18CleanupVerification},
	}

	if err := test_v2.RunSuite(test_v2.SuiteOptions{
		Version:    "src_v3",
		ReportPath: "src/plugins/template/src_v3/test/TEST.md",
		LogPath:    "src/plugins/template/src_v3/test/test.log",
	}, steps); err != nil {
		fmt.Printf("[TEST] SUITE ERROR: %v\n", err)
		os.Exit(1)
	}
}
