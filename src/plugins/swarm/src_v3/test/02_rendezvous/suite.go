package rendezvoustest

import (
	"fmt"
	"strings"
	"time"

	swarmv3 "dialtone/dev/plugins/swarm/src_v3/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(r *testv1.Registry, rendezvousURL string) {
	url := strings.TrimSpace(rendezvousURL)
	if url == "" {
		url = "https://relay.dialtone.earth"
	}
	r.Add(testv1.Step{
		Name: "swarm-v3-rendezvous-self-test",
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			paths, err := swarmv3.ResolvePaths("")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			bin, err := swarmv3.EnsureHostBinary(paths)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := swarmv3.RunRendezvousSelfTest(bin, url); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForStepMessageAfterAction("swarm-v3-rendezvous-self-test-ok", 3*time.Second, func() error {
				ctx.Infof("swarm-v3-rendezvous-self-test-ok")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: fmt.Sprintf("rendezvous self-test passed via %s", url)}, nil
		},
	})
}
