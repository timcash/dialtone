package main

import (
	"fmt"
	"os"
)

type step struct {
	Name       string
	Conditions string
	Run        func(*testCtx) (string, error)
}

func main() {
	ctx, err := newTestCtx()
	if err != nil {
		fmt.Printf("[TEST] init failed: %v\n", err)
		os.Exit(1)
	}
	defer ctx.cleanup()

	steps := []step{
		{
			Name:       "01 Embedded NATS + topic publish",
			Conditions: "Embedded broker starts and wildcard listener captures topic logs.",
			Run:        Run01EmbeddedNATSAndPublish,
		},
		{
			Name:       "02 Listener filtering (error.topic)",
			Conditions: "Listener on logs.error.topic only receives error topic messages.",
			Run:        Run02ErrorTopicFiltering,
		},
		{
			Name:       "04 Two-process pingpong via dialtone logs",
			Conditions: "Two ./dialtone.sh logs pingpong processes exchange at least 3 ping/pong rounds on one topic.",
			Run:        Run04TwoProcessPingPong,
		},
		{
			Name:       "05 Example plugin binary imports logs library",
			Conditions: "A built binary under logs/src_v1/test imports logs library, auto-starts embedded NATS when missing, and publishes topic messages.",
			Run:        Run05ExamplePluginImport,
		},
		{
			Name:       "03 Finalize artifacts",
			Conditions: "Artifacts exist and include captured topic lines.",
			Run:        Run03Finalize,
		},
	}

	if err := ctx.run(steps); err != nil {
		fmt.Printf("[TEST] SUITE ERROR: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[TEST] Report written to %s\n", ctx.reportPath)
}
