package main

import (
	"fmt"
	"os"
	"time"

	test_v2 "dialtone/cli/src/libs/test_v2"
)

func main() {
	steps := []test_v2.Step{
		{Name: "01 Preflight (Go/UI)", Run: Run01Preflight, Timeout: 60 * time.Second},
		{Name: "02 Go Run (Mock Server Check)", Run: Run07GoRun, Timeout: 10 * time.Second},
		{Name: "03 UI Run", Run: Run08UIRun, Timeout: 10 * time.Second},
		{Name: "04 Expected Errors (Proof of Life)", Run: Run09ExpectedErrorsProofOfLife, Timeout: 10 * time.Second},
		{Name: "05 Dev Server Running (latest UI)", Run: Run10DevServerRunningLatestUI, Timeout: 10 * time.Second},
		{Name: "06 Hero Section Validation", Run: Run11HeroSectionValidation, SectionID: "hero", Screenshots: []string{"screenshots/test_step_1.png"}, Timeout: 10 * time.Second},
		{Name: "07 Docs Section Validation", Run: Run12DocsSectionValidation, SectionID: "docs", Screenshots: []string{"screenshots/test_step_2.png"}, Timeout: 10 * time.Second},
		{Name: "08 Table Section Validation", Run: Run13TableSectionValidation, SectionID: "table", Screenshots: []string{"screenshots/test_step_3.png"}, Timeout: 10 * time.Second},
		{Name: "09 Three Section Validation", Run: Run14ThreeSectionValidation, SectionID: "three", Screenshots: []string{"screenshots/test_step_4.png"}, Timeout: 10 * time.Second},
		{Name: "10 Xterm Section Validation", Run: Run15XtermSectionValidation, SectionID: "xterm", Screenshots: []string{"screenshots/test_step_5.png"}, Timeout: 10 * time.Second},
		{Name: "11 Video Section Validation", Run: Run16VideoSectionValidation, SectionID: "video", Screenshots: []string{"screenshots/test_step_6.png"}, Timeout: 10 * time.Second},
		{Name: "12 Lifecycle / Invariants", Run: Run17LifecycleInvariants, Timeout: 10 * time.Second},
		{
			Name:           "13 Menu Navigation Validation",
			Run:            Run19MenuNavigationValidation,
			Screenshots:    []string{"screenshots/menu_1_hero.png", "screenshots/menu_2_open.png", "screenshots/menu_3_telemetry.png"},
			ScreenshotGrid: "screenshots/menu_nav_grid.png",
			Timeout:        15 * time.Second,
		},
		{Name: "14 Cleanup Verification", Run: Run18CleanupVerification, Timeout: 10 * time.Second},
	}

	if err := test_v2.RunSuite(test_v2.SuiteOptions{
		Version:        "src_v1",
		ReportPath:     "src/plugins/robot/src_v1/test/TEST.md",
		LogPath:        "src/plugins/robot/src_v1/test/test.log",
		ErrorLogPath:   "src/plugins/robot/src_v1/test/error.log",
		BrowserLogMode: "errors_only",
	}, steps); err != nil {
		fmt.Printf("[TEST] SUITE ERROR: %v\n", err)
		os.Exit(1)
	}
}
