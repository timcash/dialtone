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
