package test

import (
	"fmt"
	"os"
	"time"

	test_v2 "dialtone/dev/plugins/test/src_v1/go"
)

func RunSuiteV3() {
	ctx := newTestCtx()
	defer ctx.teardown()
	steps := []test_v2.Step{
		{Name: "01 DuckDB Graph Query Validation", RunWithContext: wrapRun(ctx, Run01DuckDBGraphQueries)},
		{Name: "02 Preflight (Go/UI)", RunWithContext: wrapRun(ctx, Run01Preflight)},
		{Name: "03 Startup: No Backend Menu -> Stage", RunWithContext: wrapRun(ctx, Run02NoBackendMenuToStage), SectionID: "dag-3d-stage", Screenshots: []string{"screenshots/test_step_no_backend_menu_stage_pre.png", "screenshots/test_step_no_backend_menu_stage.png"}, ScreenshotGrid: "screenshots/test_step_no_backend_menu_stage_grid.png", Timeout: 40 * time.Second},
		{Name: "04 Startup: Menu -> Stage Fresh Load", RunWithContext: wrapRun(ctx, Run02StartupMenuToStageFresh), SectionID: "dag-3d-stage", Screenshots: []string{"screenshots/test_step_startup_menu_stage_pre.png", "screenshots/test_step_startup_menu_stage.png"}, ScreenshotGrid: "screenshots/test_step_startup_menu_stage_grid.png", Timeout: 40 * time.Second},
		{Name: "05 DAG Table Section Validation", RunWithContext: wrapRun(ctx, Run02DagTableSectionValidation), SectionID: "dag-meta-table", Screenshots: []string{"screenshots/test_step_1_pre.png", "screenshots/test_step_1.png"}, ScreenshotGrid: "screenshots/test_step_1_grid.png", Timeout: 30 * time.Second},
		{Name: "06 Menu/Nav Section Switch Validation", RunWithContext: wrapRun(ctx, Run03MenuNavSectionSwitch), SectionID: "dag-3d-stage", Screenshots: []string{"screenshots/test_step_menu_nav_pre.png", "screenshots/test_step_menu_nav.png"}, ScreenshotGrid: "screenshots/test_step_menu_nav_grid.png", Timeout: 30 * time.Second},
		{Name: "07 Log Section Echo Command", RunWithContext: wrapRun(ctx, Run03LogSectionEcho), SectionID: "dag-log-xterm"},
		{Name: "07 Test-DAG: Program A Node", RunWithContext: wrapRun(ctx, Run03ThreeUserStoryStartEmpty), SectionID: "dag-3d-stage", Screenshots: []string{"screenshots/test_step_2_pre.png", "screenshots/test_step_2.png"}, ScreenshotGrid: "screenshots/test_step_2_grid.png"},
		{Name: "08 Test-DAG: Program A -> Agent A", RunWithContext: wrapRun(ctx, Run04ThreeUserStoryBuildIO), SectionID: "dag-3d-stage", Screenshots: []string{"screenshots/test_step_3_pre.png", "screenshots/test_step_3.png"}, ScreenshotGrid: "screenshots/test_step_3_grid.png"},
		{Name: "09 Test-DAG: Agent A -> Link", RunWithContext: wrapRun(ctx, Run05ThreeUserStoryNestAndOpenLayer), SectionID: "dag-3d-stage", Screenshots: []string{"screenshots/test_step_4_pre.png", "screenshots/test_step_4.png"}, ScreenshotGrid: "screenshots/test_step_4_grid.png"},
		{Name: "10 Test-DAG: Link -> Agent B", RunWithContext: wrapRun(ctx, Run06ThreeUserStoryRenameAndCloseLayer), SectionID: "dag-3d-stage", Screenshots: []string{"screenshots/test_step_5_pre.png", "screenshots/test_step_5.png"}, ScreenshotGrid: "screenshots/test_step_5_grid.png"},
		{Name: "11 Test-DAG: Agent B -> Program B", RunWithContext: wrapRun(ctx, Run08ThreeUserStoryDeepNestedBuild), SectionID: "dag-3d-stage", Screenshots: []string{"screenshots/test_step_6_pre.png", "screenshots/test_step_6.png"}, ScreenshotGrid: "screenshots/test_step_6_grid.png"},
		{Name: "12 Test-DAG: Open Link Protocol Layer", RunWithContext: wrapRun(ctx, Run09ThreeUserStoryDeepCloseLayerHistory), SectionID: "dag-3d-stage", Screenshots: []string{"screenshots/test_step_7_pre.png", "screenshots/test_step_7.png"}, ScreenshotGrid: "screenshots/test_step_7_grid.png"},
		{Name: "13 Test-DAG: Build Protocol Tx/Rx", RunWithContext: wrapRun(ctx, Run10ThreeUserStoryUnlinkAndRelabel), SectionID: "dag-3d-stage", Screenshots: []string{"screenshots/test_step_8_pre.png", "screenshots/test_step_8.png"}, ScreenshotGrid: "screenshots/test_step_8_grid.png"},
		{Name: "14 Test-DAG: Close Protocol Layer", RunWithContext: wrapRun(ctx, Run11CleanupVerification), SectionID: "dag-3d-stage"},
		{Name: "15 Forms: Switch To Build Mode", RunWithContext: wrapRun(ctx, Run11SwitchToBuildMode), SectionID: "dag-3d-stage"},
		{Name: "16 Forms: Build Mode Buttons A", RunWithContext: wrapRun(ctx, Run11BuildModeCoverageA), SectionID: "dag-3d-stage"},
		{Name: "17 Forms: Build Mode Buttons B", RunWithContext: wrapRun(ctx, Run11BuildModeCoverageB), SectionID: "dag-3d-stage"},
		{Name: "18 Forms: Switch To Layer Mode", RunWithContext: wrapRun(ctx, Run12SwitchToLayerMode), SectionID: "dag-3d-stage"},
		{Name: "19 Forms: Layer Mode Buttons A", RunWithContext: wrapRun(ctx, Run13LayerModeCoverageA), SectionID: "dag-3d-stage"},
		{Name: "20 Forms: Layer Mode Buttons B", RunWithContext: wrapRun(ctx, Run14LayerModeCoverageB), SectionID: "dag-3d-stage"},
		{Name: "21 Forms: Switch To View Mode", RunWithContext: wrapRun(ctx, Run15SwitchToCameraMode), SectionID: "dag-3d-stage"},
		{Name: "22 Forms: View Mode Buttons A", RunWithContext: wrapRun(ctx, Run16CameraModeCoverageA), SectionID: "dag-3d-stage"},
		{Name: "23 Forms: View Mode Buttons B", RunWithContext: wrapRun(ctx, Run17CameraModeCoverageB), SectionID: "dag-3d-stage"},
		{Name: "24 Finalize + Teardown", RunWithContext: wrapRun(ctx, Run18FinalizeAndTeardown)},
	}

	if err := test_v2.RunSuite(test_v2.SuiteOptions{
		Version:        "src_v3",
		ReportPath:     "plugins/dag/src_v3/test/TEST.md",
		LogPath:        "plugins/dag/src_v3/test/test.log",
		ErrorLogPath:   "plugins/dag/src_v3/test/error.log",
		BrowserLogMode: "errors_only",
	}, steps); err != nil {
		fmt.Printf("[TEST] SUITE ERROR: %v\n", err)
		os.Exit(1)
	}
}

func wrapRun(ctx *testCtx, fn func(*testCtx) (string, error)) func(*test_v2.StepContext) (test_v2.StepRunResult, error) {
	return func(sc *test_v2.StepContext) (test_v2.StepRunResult, error) {
		ctx.beginStep(sc)
		report, err := fn(ctx)
		return test_v2.StepRunResult{Report: report}, err
	}
}
