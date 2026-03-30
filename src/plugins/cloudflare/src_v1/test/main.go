package main

import (
	"errors"
	"flag"
	"os"
	"strings"

	cloudflarev1 "dialtone/dev/plugins/cloudflare/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
	test_v2 "dialtone/dev/plugins/test/src_v1/go"
)

func wrapStep(run func() error) func(*test_v2.StepContext) (test_v2.StepRunResult, error) {
	return func(*test_v2.StepContext) (test_v2.StepRunResult, error) {
		return test_v2.StepRunResult{}, run()
	}
}

func main() {
	logs.SetOutput(os.Stdout)
	fs := flag.NewFlagSet("cloudflare src_v1 test", flag.ContinueOnError)
	filter := fs.String("filter", "", "Run only matching test steps")
	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		logs.Error("cloudflare src_v1 test parse failed: %v", err)
		os.Exit(1)
	}

	steps := buildSteps()
	if filtered := filterSteps(steps, strings.TrimSpace(*filter)); len(filtered) > 0 {
		steps = filtered
	}

	logs.Info("Running cloudflare src_v1 tests in single process (%d steps)", len(steps))
	paths, err := cloudflarev1.ResolvePaths("", "src_v1")
	if err != nil {
		logs.Error("cloudflare src_v1 test init failed: %v", err)
		os.Exit(1)
	}

	if err := test_v2.RunSuite(test_v2.SuiteOptions{
		Version:      "src_v1",
		ReportPath:   paths.TestReport,
		LogPath:      paths.TestLog,
		ErrorLogPath: paths.TestErrorLog,
	}, steps); err != nil {
		logs.Error("cloudflare src_v1 tests failed: %v", err)
		os.Exit(1)
	}
	logs.Info("cloudflare src_v1 tests passed")
}

func buildSteps() []test_v2.Step {
	return []test_v2.Step{
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
}

func filterSteps(steps []test_v2.Step, filterExpr string) []test_v2.Step {
	filterExpr = strings.TrimSpace(strings.ToLower(filterExpr))
	if filterExpr == "" {
		return nil
	}
	parts := strings.Split(filterExpr, ",")
	tokens := make([]string, 0, len(parts))
	for _, part := range parts {
		token := strings.TrimSpace(strings.ToLower(part))
		if token == "" {
			continue
		}
		tokens = append(tokens, token)
	}
	if len(tokens) == 0 {
		return nil
	}

	out := make([]test_v2.Step, 0, len(steps))
	for _, step := range steps {
		name := strings.ToLower(strings.TrimSpace(step.Name))
		sectionID := strings.ToLower(strings.TrimSpace(step.SectionID))
		for _, token := range tokens {
			if strings.Contains(name, token) || strings.Contains(sectionID, token) {
				out = append(out, step)
				break
			}
		}
	}

	if len(out) == 0 {
		logs.Warn("cloudflare src_v1 --filter=%q matched no steps; running all steps", filterExpr)
		return nil
	}
	names := make([]string, 0, len(out))
	for _, step := range out {
		names = append(names, step.Name)
	}
	logs.Info("cloudflare src_v1 --filter=%q selected steps: %s", filterExpr, strings.Join(names, ", "))
	return out
}
