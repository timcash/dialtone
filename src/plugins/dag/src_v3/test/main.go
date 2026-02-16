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
		{Name: "03 DAG Table Section Validation", Run: Run02DagTableSectionValidation, SectionID: "dag-table", Screenshot: "screenshots/test_step_1.png"},
		{Name: "04 User Story: Empty DAG Start + First Node", Run: Run03ThreeUserStoryStartEmpty, SectionID: "three", Screenshot: "screenshots/test_step_2.png"},
		{Name: "05 User Story: Build Root IO", Run: Run04ThreeUserStoryBuildIO, SectionID: "three", Screenshot: "screenshots/test_step_3.png"},
		{Name: "06 User Story: Nest + Dive + Nested Build", Run: Run05ThreeUserStoryNestAndDive, SectionID: "three", Screenshot: "screenshots/test_step_4.png"},
		{Name: "07 User Story: Rename + Undive + Camera History", Run: Run06ThreeUserStoryRenameAndUndive, SectionID: "three", Screenshot: "screenshots/test_step_5.png"},
		{Name: "08 User Story: Deep Nested Build", Run: Run08ThreeUserStoryDeepNestedBuild, SectionID: "three", Screenshot: "screenshots/test_step_6.png"},
		{Name: "09 User Story: Deep Undive + Camera History", Run: Run09ThreeUserStoryDeepUndiveHistory, SectionID: "three", Screenshot: "screenshots/test_step_7.png"},
		{Name: "10 User Story: Delete + Relabel + Camera Readability", Run: Run10ThreeUserStoryDeleteAndRelabel, SectionID: "three", Screenshot: "screenshots/test_step_8.png"},
		{Name: "11 Cleanup Verification", Run: Run11CleanupVerification},
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
