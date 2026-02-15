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
		{Name: "04 Three Section Validation", Run: Run03ThreeSectionValidation, SectionID: "three", Screenshot: "screenshots/test_step_2.png"},
		{Name: "05 Cleanup Verification", Run: Run04CleanupVerification},
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
