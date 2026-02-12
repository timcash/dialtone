package main

import (
	"fmt"
	"os"

	test_v2 "dialtone/cli/src/libs/test_v2"
)

func main() {
	steps := []test_v2.Step{
		{Name: "01 Go Format", Run: Run01GoFormat},
		{Name: "02 Go Vet", Run: Run02GoVet},
		{Name: "03 Go Build", Run: Run03GoBuild},
		{Name: "04 UI Lint", Run: Run04UILint},
		{Name: "05 UI Format", Run: Run05UIFormat},
		{Name: "06 UI Build", Run: Run06UIBuild},
		{Name: "07 Go Run", Run: Run07GoRun},
		{Name: "08 UI Run", Run: Run08UIRun},
		{Name: "09 Expected Errors (Proof of Life)", Run: Run09ExpectedErrorsProofOfLife},
		{Name: "10 Dev Server Running (latest UI)", Run: Run10DevServerRunningLatestUI},
		{Name: "11 Hero Section Validation", Run: Run11HeroSectionValidation, SectionID: "hero", Screenshot: "screenshots/test_step_1.png"},
		{Name: "12 Docs Section Validation", Run: Run12DocsSectionValidation, SectionID: "docs", Screenshot: "screenshots/test_step_2.png"},
		{Name: "13 Table Section Validation", Run: Run13TableSectionValidation, SectionID: "table", Screenshot: "screenshots/test_step_3.png"},
		{Name: "14 Three Section Validation", Run: Run14ThreeSectionValidation, SectionID: "three", Screenshot: "screenshots/test_step_4.png"},
		{Name: "15 Xterm Section Validation", Run: Run15XtermSectionValidation, SectionID: "xterm", Screenshot: "screenshots/test_step_5.png"},
		{Name: "16 Video Section Validation", Run: Run16VideoSectionValidation, SectionID: "video", Screenshot: "screenshots/test_step_6.png"},
		{Name: "17 Lifecycle / Invariants", Run: Run17LifecycleInvariants},
		{Name: "18 Cleanup Verification", Run: Run18CleanupVerification},
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
