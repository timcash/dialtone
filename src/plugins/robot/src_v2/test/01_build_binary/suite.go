package buildbinary

import (
	"os/exec"
	"strings"
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(reg *testv1.Registry) {
	reg.Add(testv1.Step{
		Name:    "01-build-robot-v2-binary",
		Timeout: 30 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			if err := ctx.WaitForStepMessageAfterAction("build complete", 20*time.Second, func() error {
				ctx.Infof("[ACTION] build robot src_v2 server binary")
				cmd := exec.Command("./dialtone.sh", "go", "src_v1", "exec", "build", "-o", "../bin/dialtone_robot_v2", "./plugins/robot/src_v2/cmd/server/main.go")
				cmd.Dir = ctx.RepoRoot()
				out, err := cmd.CombinedOutput()
				if err != nil {
					ctx.Errorf("build failed: %s", strings.TrimSpace(string(out)))
					return err
				}
				ctx.Infof("build complete")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "binary build verified"}, nil
		},
	})
}
