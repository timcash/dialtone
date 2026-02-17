package main

import (
	"fmt"
	"os"

	test_v2 "dialtone/cli/src/libs/test_v2"
)

func main() {
	ctx := newTestCtx()
	steps := []test_v2.Step{
		{Name: "01 DuckDB Graph Query Validation", RunWithContext: wrapRun(ctx, Run01DuckDBGraphQueries)},
		{Name: "02 Preflight (Go/UI)", RunWithContext: wrapRun(ctx, Run01Preflight)},
		{Name: "03 Startup: Menu -> Stage Fresh Load", RunWithContext: wrapRun(ctx, Run02StartupMenuToStageFresh), SectionID: "three", Screenshots: []string{"screenshots/test_step_startup_menu_stage_pre.png", "screenshots/test_step_startup_menu_stage.png"}, ScreenshotGrid: "screenshots/test_step_startup_menu_stage_grid.png"},
		{Name: "04 DAG Table Section Validation", RunWithContext: wrapRun(ctx, Run02DagTableSectionValidation), SectionID: "dag-table", Screenshots: []string{"screenshots/test_step_1_pre.png", "screenshots/test_step_1.png"}, ScreenshotGrid: "screenshots/test_step_1_grid.png"},
		{Name: "05 Menu/Nav Section Switch Validation", RunWithContext: wrapRun(ctx, Run03MenuNavSectionSwitch), SectionID: "three", Screenshots: []string{"screenshots/test_step_menu_nav_pre.png", "screenshots/test_step_menu_nav.png"}, ScreenshotGrid: "screenshots/test_step_menu_nav_grid.png"},
		{Name: "06 User Story: Empty DAG Start + First Node", RunWithContext: wrapRun(ctx, Run03ThreeUserStoryStartEmpty), SectionID: "three", Screenshots: []string{"screenshots/test_step_2_pre.png", "screenshots/test_step_2.png"}, ScreenshotGrid: "screenshots/test_step_2_grid.png"},
		{Name: "07 User Story: Build Root IO", RunWithContext: wrapRun(ctx, Run04ThreeUserStoryBuildIO), SectionID: "three", Screenshots: []string{"screenshots/test_step_3_pre.png", "screenshots/test_step_3.png"}, ScreenshotGrid: "screenshots/test_step_3_grid.png"},
		{Name: "08 User Story: Nest + Open Layer + Nested Build", RunWithContext: wrapRun(ctx, Run05ThreeUserStoryNestAndOpenLayer), SectionID: "three", Screenshots: []string{"screenshots/test_step_4_pre.png", "screenshots/test_step_4.png"}, ScreenshotGrid: "screenshots/test_step_4_grid.png"},
		{Name: "09 User Story: Rename + Close Layer + Camera History", RunWithContext: wrapRun(ctx, Run06ThreeUserStoryRenameAndCloseLayer), SectionID: "three", Screenshots: []string{"screenshots/test_step_5_pre.png", "screenshots/test_step_5.png"}, ScreenshotGrid: "screenshots/test_step_5_grid.png"},
		{Name: "10 User Story: Deep Nested Build", RunWithContext: wrapRun(ctx, Run08ThreeUserStoryDeepNestedBuild), SectionID: "three", Screenshots: []string{"screenshots/test_step_6_pre.png", "screenshots/test_step_6.png"}, ScreenshotGrid: "screenshots/test_step_6_grid.png"},
		{Name: "11 User Story: Deep Close Layer + Camera History", RunWithContext: wrapRun(ctx, Run09ThreeUserStoryDeepCloseLayerHistory), SectionID: "three", Screenshots: []string{"screenshots/test_step_7_pre.png", "screenshots/test_step_7.png"}, ScreenshotGrid: "screenshots/test_step_7_grid.png"},
		{Name: "12 User Story: Unlink + Relabel + Camera Readability", RunWithContext: wrapRun(ctx, Run10ThreeUserStoryUnlinkAndRelabel), SectionID: "three", Screenshots: []string{"screenshots/test_step_8_pre.png", "screenshots/test_step_8.png"}, ScreenshotGrid: "screenshots/test_step_8_grid.png"},
		{Name: "13 Cleanup Verification", RunWithContext: wrapRun(ctx, Run11CleanupVerification)},
	}

	if err := test_v2.RunSuite(test_v2.SuiteOptions{
		Version:        "src_v3",
		ReportPath:     "src/plugins/dag/src_v3/test/TEST.md",
		LogPath:        "src/plugins/dag/src_v3/test/test.log",
		ErrorLogPath:   "src/plugins/dag/src_v3/test/error.log",
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
