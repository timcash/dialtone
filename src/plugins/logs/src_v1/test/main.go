package main

import (
	"fmt"
	"os"

	test_v2 "dialtone/dev/plugins/dag/src_v3/suite"
)

func main() {
	ctx := newTestCtx()
	steps := []test_v2.Step{
		{Name: "01 Preflight (Go/UI)", RunWithContext: wrapRun(ctx, Run01Preflight)},
		{Name: "02 Log section load", RunWithContext: wrapRun(ctx, Run02LogSectionLoad), SectionID: "logs-log-xterm"},
		{Name: "03 Finalize", RunWithContext: wrapRun(ctx, Run03Finalize)},
	}

	if err := test_v2.RunSuite(test_v2.SuiteOptions{
		Version:        "src_v1",
		ReportPath:     "src/plugins/logs/src_v1/test/TEST.md",
		LogPath:        "src/plugins/logs/src_v1/test/test.log",
		ErrorLogPath:   "src/plugins/logs/src_v1/test/error.log",
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
