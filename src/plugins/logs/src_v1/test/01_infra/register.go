package infra

import (
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name:           "01 Embedded NATS + topic publish",
		SectionID:      "infra",
		RunWithContext: Run01EmbeddedNATSAndPublish,
	})
	r.Add(testv1.Step{
		Name:           "02 Listener filtering (error.topic)",
		SectionID:      "infra",
		RunWithContext: Run02ErrorTopicFiltering,
	})
	r.Add(testv1.Step{
		Name:           "04 Two-process pingpong via dialtone logs",
		SectionID:      "infra",
		Timeout:        35 * time.Second,
		RunWithContext: Run04TwoProcessPingPong,
	})
	r.Add(testv1.Step{
		Name:           "05 Example plugin binary imports logs library",
		SectionID:      "infra",
		RunWithContext: Run05ExamplePluginImport,
	})
	r.Add(testv1.Step{
		Name:           "03 Finalize artifacts",
		SectionID:      "infra",
		RunWithContext: Run03Finalize,
	})
}
