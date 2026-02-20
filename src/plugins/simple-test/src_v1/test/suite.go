package test

import (
	"fmt"
	"os"

	test_v2 "dialtone/dev/plugins/test/src_v1/go"
)

func RunSuiteV1() {
	ctx := newTestCtx()
	defer ctx.teardown()

	steps := []test_v2.Step{
		{Name: "00 Reset Workspace", RunWithContext: wrapRun(ctx, Run00Reset)},
		{
			Name:           "01 UI Section Load Test",
			RunWithContext: wrapRun(ctx, Run01UITest),
			Screenshots:    []string{"screenshots/01_ui_loaded.png"},
		},
		{
			Name:           "02 Interaction Test",
			RunWithContext: wrapRun(ctx, Run02InteractionTest),
			Screenshots:    []string{"screenshots/02_interacted.png"},
		},
	}

	opts := test_v2.SuiteOptions{
		Version:      "src_v1",
		ReportPath:   "plugins/simple-test/src_v1/test/TEST.md",
		LogPath:      "plugins/simple-test/src_v1/test/test.log",
		ErrorLogPath: "plugins/simple-test/src_v1/test/error.log",
	}

	if err := test_v2.RunSuite(opts, steps); err != nil {
		fmt.Printf("[TEST] SUITE ERROR: %v\n", err)
		os.Exit(1)
	}
}

func wrapRun(ctx *testCtx, fn func(*testCtx) (string, error)) func(*test_v2.StepContext) (test_v2.StepRunResult, error) {
	return func(sc *test_v2.StepContext) (test_v2.StepRunResult, error) {
		report, err := fn(ctx)
		return test_v2.StepRunResult{Report: report}, err
	}
}
