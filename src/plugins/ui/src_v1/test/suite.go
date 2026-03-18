package test

import (
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

type Registry = testv1.Registry
type Step = testv1.Step
type StepContext = testv1.StepContext
type StepRunResult = testv1.StepRunResult

func NewRegistry() *Registry {
	return testv1.NewRegistry()
}

func RunSuiteV1(reg *Registry) error {
	return reg.Run(testv1.SuiteOptions{
		Version:               "ui-src-v1",
		ReportPath:            "plugins/ui/src_v1/TEST.md",
		RawReportPath:         "plugins/ui/src_v1/TEST_RAW.md",
		ReportFormat:          "template",
		ReportTitle:           "UI Plugin src_v1 Test Report",
		ReportRunner:          "test/src_v1",
		NATSURL:               ResolveSuiteNATSURL(),
		NATSSubject:           "logs.test.ui.src-v1",
		AutoStartNATS:         true,
		BrowserCleanupRole:    "ui-test",
		PreserveSharedBrowser: false,
		SkipBrowserCleanup:    false,
	})
}
