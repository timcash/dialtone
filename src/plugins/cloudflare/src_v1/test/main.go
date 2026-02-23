package main

import (
	"fmt"
	"os"

	cloudflarev1 "dialtone/dev/plugins/cloudflare/src_v1/go"
	test_v2 "dialtone/dev/plugins/test/src_v1/go"
)

func wrapStep(run func() error) func(*test_v2.StepContext) (test_v2.StepRunResult, error) {
	return func(*test_v2.StepContext) (test_v2.StepRunResult, error) {
		return test_v2.StepRunResult{}, run()
	}
}

func main() {
	steps := []test_v2.Step{
		{Name: "01 Preflight (Go/UI)", RunWithContext: wrapStep(Run01Preflight)},
		{Name: "02 Go Run", RunWithContext: wrapStep(Run07GoRun)},
		{Name: "03 UI Run", RunWithContext: wrapStep(Run08UIRun)},
		{Name: "04 Expected Errors (Proof of Life)", RunWithContext: wrapStep(Run09ExpectedErrorsProofOfLife)},
		{Name: "05 Dev Server Running (latest UI)", RunWithContext: wrapStep(Run10DevServerRunningLatestUI)},
		{Name: "06 Hero Section Validation", RunWithContext: wrapStep(Run11HeroSectionValidation), SectionID: "hero", Screenshots: []string{"screenshots/test_step_1.png"}},
		{Name: "07 Docs Section Validation", RunWithContext: wrapStep(Run12DocsSectionValidation), SectionID: "docs", Screenshots: []string{"screenshots/test_step_2.png"}},
		{Name: "08 Status Section Validation", RunWithContext: wrapStep(Run13TableSectionValidation), SectionID: "status", Screenshots: []string{"screenshots/test_step_3.png"}},
		{Name: "09 Three Section Validation", RunWithContext: wrapStep(Run14ThreeSectionValidation), SectionID: "three", Screenshots: []string{"screenshots/test_step_4.png"}},
		{Name: "10 Xterm Section Validation", RunWithContext: wrapStep(Run15XtermSectionValidation), SectionID: "xterm", Screenshots: []string{"screenshots/test_step_5.png"}},
		{Name: "11 Lifecycle / Invariants", RunWithContext: wrapStep(Run17LifecycleInvariants)},
		{Name: "12 Cleanup Verification", RunWithContext: wrapStep(Run18CleanupVerification)},
	}

	paths, err := cloudflarev1.ResolvePaths("", "src_v1")
	if err != nil {
		fmt.Printf("[TEST] PATH RESOLVE ERROR: %v\n", err)
		os.Exit(1)
	}

	if err := test_v2.RunSuite(test_v2.SuiteOptions{
		Version:      "src_v1",
		ReportPath:   paths.TestReport,
		LogPath:      paths.TestLog,
		ErrorLogPath: paths.TestErrorLog,
	}, steps); err != nil {
		fmt.Printf("[TEST] SUITE ERROR: %v\n", err)
		os.Exit(1)
	}
}
