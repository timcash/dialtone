package examplelibrary

import (
	"fmt"
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
	tsnetv1 "dialtone/dev/plugins/tsnet/src_v1/go"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name: "example-library-core-smoke",
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			cfg, err := tsnetv1.ResolveConfig("Robot_1 Dev", ".dialtone/tsnet-example")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if cfg.Hostname != "robot-1-dev" {
				return testv1.StepRunResult{}, fmt.Errorf("hostname normalization failed")
			}

			srv := tsnetv1.BuildServer(cfg)
			if srv.Hostname != "robot-1-dev" {
				return testv1.StepRunResult{}, fmt.Errorf("server hostname mismatch")
			}

			usages := tsnetv1.InferKeyUsage(
				[]tsnetv1.AuthKey{{ID: "k1", Description: "robot-1-dev", Tags: []string{"tag:robot"}}},
				[]tsnetv1.Device{{Name: "robot-1-dev", Hostname: "robot-1-dev", Tags: []string{"tag:robot"}}},
			)
			if len(usages) != 1 || len(usages[0].Matches) == 0 {
				return testv1.StepRunResult{}, fmt.Errorf("key usage inference mismatch")
			}

			if err := ctx.WaitForStepMessageAfterAction("tsnet-example-library-ok", 3*time.Second, func() error {
				ctx.Infof("tsnet-example-library-ok")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "tsnet example library smoke verified"}, nil
		},
	})
}
