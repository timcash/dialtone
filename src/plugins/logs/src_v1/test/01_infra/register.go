package infra

import testv1 "dialtone/dev/plugins/test/src_v1/go"

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name:           "01 Embedded NATS + topic publish",
		RunWithContext: Run01EmbeddedNATSAndPublish,
	})
	r.Add(testv1.Step{
		Name:           "02 Listener filtering (error.topic)",
		RunWithContext: Run02ErrorTopicFiltering,
	})
	r.Add(testv1.Step{
		Name:           "04 Two-process pingpong via dialtone logs",
		RunWithContext: Run04TwoProcessPingPong,
	})
	r.Add(testv1.Step{
		Name:           "05 Example plugin binary imports logs library",
		RunWithContext: Run05ExamplePluginImport,
	})
	r.Add(testv1.Step{
		Name:           "03 Finalize artifacts",
		RunWithContext: Run03Finalize,
	})
}
