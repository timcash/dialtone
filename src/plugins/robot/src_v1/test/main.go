package main

import (
	"fmt"
	"os"
	"time"

	test_v2 "dialtone/cli/src/libs/test_v2"
)

func main() {
	ctx := newTestCtx()
	steps := []test_v2.Step{
		{Name: "01 Preflight (Go/UI)", RunWithContext: wrapRun(ctx, Run01Preflight), Timeout: 60 * time.Second},
		{Name: "02 Go Run (Mock Server Check)", RunWithContext: wrapRun(ctx, Run07GoRun), Timeout: 10 * time.Second},
		{Name: "03 UI Run", RunWithContext: wrapRun(ctx, Run08UIRun), Timeout: 10 * time.Second},
		{Name: "04 Expected Errors (Proof of Life)", RunWithContext: wrapRun(ctx, Run09ExpectedErrorsProofOfLife), Timeout: 10 * time.Second},
		{Name: "05 Dev Server Running (latest UI)", RunWithContext: wrapRun(ctx, Run10DevServerRunningLatestUI), Timeout: 10 * time.Second},
		{Name: "06 Hero Section Validation", RunWithContext: wrapRun(ctx, Run11HeroSectionValidation), SectionID: "hero", Screenshots: []string{"screenshots/test_step_1.png"}, Timeout: 2 * time.Second},
		{Name: "07 Docs Section Validation", RunWithContext: wrapRun(ctx, Run12DocsSectionValidation), SectionID: "docs", Screenshots: []string{"screenshots/test_step_2.png"}, Timeout: 2 * time.Second},
		{Name: "08 Table Section Validation", RunWithContext: wrapRun(ctx, Run13TableSectionValidation), SectionID: "table", Screenshots: []string{"screenshots/test_step_3.png"}, Timeout: 2 * time.Second},
		{Name: "09 Three Section Validation", RunWithContext: wrapRun(ctx, Run14ThreeSectionValidation), SectionID: "three", Screenshots: []string{"screenshots/test_step_4.png"}, Timeout: 2 * time.Second},
		{Name: "10 Xterm Section Validation", RunWithContext: wrapRun(ctx, Run15XtermSectionValidation), SectionID: "xterm", Screenshots: []string{"screenshots/test_step_5.png"}, Timeout: 2 * time.Second},
		{Name: "11 Video Section Validation", RunWithContext: wrapRun(ctx, Run16VideoSectionValidation), SectionID: "video", Screenshots: []string{"screenshots/test_step_6.png"}, Timeout: 2 * time.Second},
		{Name: "12 Lifecycle / Invariants", RunWithContext: wrapRun(ctx, Run17LifecycleInvariants), Timeout: 20 * time.Second},
		{
			Name:           "13 Menu Navigation Validation",
			RunWithContext: wrapRun(ctx, Run19MenuNavigationValidation),
			Screenshots:    []string{"screenshots/menu_1_hero.png", "screenshots/menu_2_open.png", "screenshots/menu_3_telemetry.png"},
			ScreenshotGrid: "screenshots/menu_nav_grid.png",
			Timeout:        15 * time.Second,
		},
		{Name: "14 Cleanup Verification", RunWithContext: wrapRun(ctx, Run18CleanupVerification), Timeout: 10 * time.Second},
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

func wrapRun(ctx *testCtx, fn func(*testCtx) (string, error)) func(*test_v2.StepContext) (test_v2.StepRunResult, error) {
	return func(_ *test_v2.StepContext) (test_v2.StepRunResult, error) {
		report, err := fn(ctx)
		return test_v2.StepRunResult{Report: report}, err
	}
}
