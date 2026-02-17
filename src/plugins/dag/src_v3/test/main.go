package main

import (
	"fmt"
	"os"

	test_v2 "dialtone/cli/src/libs/test_v2"
)

func main() {
	steps := []test_v2.Step{
		{Name: "01 DuckDB Graph Query Validation", Run: Run01DuckDBGraphQueries},
		{Name: "02 Preflight (Go/UI)", Run: Run01Preflight},
		{Name: "03 DAG Table Section Validation", Run: Run02DagTableSectionValidation, SectionID: "dag-table", Screenshots: []string{"screenshots/test_step_1_pre.png", "screenshots/test_step_1.png"}, ScreenshotGrid: "screenshots/test_step_1_grid.png"},
		{Name: "04 Menu/Nav Section Switch Validation", Run: Run03MenuNavSectionSwitch, SectionID: "three", Screenshots: []string{"screenshots/test_step_menu_nav_pre.png", "screenshots/test_step_menu_nav.png"}, ScreenshotGrid: "screenshots/test_step_menu_nav_grid.png"},
		{Name: "05 User Story: Empty DAG Start + First Node", Run: Run03ThreeUserStoryStartEmpty, SectionID: "three", Screenshots: []string{"screenshots/test_step_2_pre.png", "screenshots/test_step_2.png"}, ScreenshotGrid: "screenshots/test_step_2_grid.png"},
		{Name: "06 User Story: Build Root IO", Run: Run04ThreeUserStoryBuildIO, SectionID: "three", Screenshots: []string{"screenshots/test_step_3_pre.png", "screenshots/test_step_3.png"}, ScreenshotGrid: "screenshots/test_step_3_grid.png"},
		{Name: "07 User Story: Nest + Open Layer + Nested Build", Run: Run05ThreeUserStoryNestAndOpenLayer, SectionID: "three", Screenshots: []string{"screenshots/test_step_4_pre.png", "screenshots/test_step_4.png"}, ScreenshotGrid: "screenshots/test_step_4_grid.png"},
		{Name: "08 User Story: Rename + Close Layer + Camera History", Run: Run06ThreeUserStoryRenameAndCloseLayer, SectionID: "three", Screenshots: []string{"screenshots/test_step_5_pre.png", "screenshots/test_step_5.png"}, ScreenshotGrid: "screenshots/test_step_5_grid.png"},
		{Name: "09 User Story: Deep Nested Build", Run: Run08ThreeUserStoryDeepNestedBuild, SectionID: "three", Screenshots: []string{"screenshots/test_step_6_pre.png", "screenshots/test_step_6.png"}, ScreenshotGrid: "screenshots/test_step_6_grid.png"},
		{Name: "10 User Story: Deep Close Layer + Camera History", Run: Run09ThreeUserStoryDeepCloseLayerHistory, SectionID: "three", Screenshots: []string{"screenshots/test_step_7_pre.png", "screenshots/test_step_7.png"}, ScreenshotGrid: "screenshots/test_step_7_grid.png"},
		{Name: "11 User Story: Unlink + Relabel + Camera Readability", Run: Run10ThreeUserStoryUnlinkAndRelabel, SectionID: "three", Screenshots: []string{"screenshots/test_step_8_pre.png", "screenshots/test_step_8.png"}, ScreenshotGrid: "screenshots/test_step_8_grid.png"},
		{Name: "12 Cleanup Verification", Run: Run11CleanupVerification},
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
