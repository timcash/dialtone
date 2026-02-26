package selfcheck

import (
	"fmt"
	"strings"
	"time"

	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name:    "mesh-nodes-known",
		Timeout: 5 * time.Second,
		RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
			nodes := sshv1.ListMeshNodes()
			if len(nodes) < 5 {
				return testv1.StepRunResult{}, fmt.Errorf("expected at least 5 mesh nodes, got %d", len(nodes))
			}
			if err := sc.WaitForStepMessageAfterAction("mesh-nodes-known-ok", 2*time.Second, func() error {
				sc.Infof("mesh-nodes-known-ok")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "mesh node list populated"}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "resolve-node-aliases",
		Timeout: 5 * time.Second,
		RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
			legion, err := sshv1.ResolveMeshNode("legion")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if legion.Port != "2223" || legion.User == "" {
				return testv1.StepRunResult{}, fmt.Errorf("unexpected legion mapping: user=%s port=%s", legion.User, legion.Port)
			}
			rover, err := sshv1.ResolveMeshNode("rover-1.shad-artichoke.ts.net")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if !strings.Contains(rover.Host, "rover-1.") {
				return testv1.StepRunResult{}, fmt.Errorf("unexpected rover mapping host=%s", rover.Host)
			}
			return testv1.StepRunResult{Report: "mesh alias resolution verified"}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "transport-resolution",
		Timeout: 5 * time.Second,
		RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
			t, err := sshv1.ResolveCommandTransport("rover")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if t != "ssh" {
				return testv1.StepRunResult{}, fmt.Errorf("expected ssh transport for rover, got %s", t)
			}
			if err := sc.WaitForStepMessageAfterAction("transport-resolution-ok", 2*time.Second, func() error {
				sc.Infof("transport-resolution-ok")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "default transport resolution verified"}, nil
		},
	})
}
