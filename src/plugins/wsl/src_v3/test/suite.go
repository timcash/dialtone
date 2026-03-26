package test

import (
	"os"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
	test_v2 "dialtone/dev/plugins/test/src_v1/go"
)

func RunSuiteV3() {
	if node := defaultWSLTestBrowserNode(); node != "" {
		test_v2.SetRuntimeConfig(test_v2.RuntimeConfig{
			BrowserNode:       node,
			RemoteBrowserRole: "wsl-test",
		})
	}
	ctx := newTestCtx()
	defer ctx.teardown()
	steps := []test_v2.Step{
		{Name: "00 Reset Workspace", RunWithContext: wrapRun(ctx, Run00Reset)},
		{Name: "01 Preflight (Go/UI)", RunWithContext: wrapRun(ctx, Run01Preflight), Timeout: 60 * time.Second},
		{Name: "02 Server Check", RunWithContext: wrapRun(ctx, Run02ServerCheck), Timeout: 10 * time.Second},
		{Name: "03 Home Section Validation", RunWithContext: wrapRun(ctx, Run03HomeSectionValidation), SectionID: "home", Screenshots: []string{"screenshots/home.png"}, Timeout: 30 * time.Second},
		{Name: "04 Docs Section Validation", RunWithContext: wrapRun(ctx, Run04DocsSectionValidation), SectionID: "docs", Screenshots: []string{"screenshots/docs.png"}, Timeout: 30 * time.Second},
		{Name: "05 Table Section Validation", RunWithContext: wrapRun(ctx, Run05TableSectionValidation), SectionID: "table", Screenshots: []string{"screenshots/table.png"}, Timeout: 30 * time.Second},
		{Name: "06 Cleanup Verification", RunWithContext: wrapRun(ctx, Run06CleanupVerification), Timeout: 10 * time.Second},
	}

	if err := test_v2.RunSuite(test_v2.SuiteOptions{
		Version:        "src_v3",
		ReportPath:     "plugins/wsl/src_v3/test/TEST.md",
		LogPath:        "plugins/wsl/src_v3/test/test.log",
		ErrorLogPath:   "plugins/wsl/src_v3/test/error.log",
		BrowserLogMode: "errors_only",
	}, steps); err != nil {
		logs.ErrorFromTest("wsl-test", "SUITE ERROR: %v", err)
		os.Exit(1)
	}
}

func defaultWSLTestBrowserNode() string {
	if envNode := strings.TrimSpace(os.Getenv("DIALTONE_TEST_BROWSER_NODE")); envNode != "" {
		return envNode
	}
	if strings.TrimSpace(os.Getenv("WSL_DISTRO_NAME")) == "" {
		return ""
	}
	for _, node := range sshv1.ListMeshNodes() {
		if strings.EqualFold(strings.TrimSpace(node.OS), "windows") && node.PreferWSLPowerShell {
			return strings.TrimSpace(node.Name)
		}
	}
	return "legion"
}

func wrapRun(ctx *testCtx, fn func(*testCtx) (string, error)) func(*test_v2.StepContext) (test_v2.StepRunResult, error) {
	return func(sc *test_v2.StepContext) (test_v2.StepRunResult, error) {
		ctx.bindStep(sc)
		report, err := fn(ctx)
		return test_v2.StepRunResult{Report: report}, err
	}
}
