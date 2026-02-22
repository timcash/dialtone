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
		Version:       "ui-src-v1",
		ReportPath:    "plugins/ui/src_v1/test/TEST.md",
		LogPath:       "plugins/ui/src_v1/test/test.log",
		ErrorLogPath:  "plugins/ui/src_v1/test/error.log",
		NATSURL:       "nats://127.0.0.1:4222",
		NATSSubject:   "logs.test.ui.src-v1",
		AutoStartNATS: true,
	})
}
