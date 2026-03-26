package buildbinary

import (
	configv1 "dialtone/dev/plugins/config/src_v1/go"
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
				out := configv1.PluginBinaryPath(configv1.Runtime{RepoRoot: ctx.RepoRoot()}, "robot", "src_v2", "dialtone_robot_v2")
				cmd := exec.Command("./dialtone.sh", "go", "src_v1", "exec", "build", "-o", out, "./plugins/robot/src_v2/cmd/server/main.go")
				cmd.Dir = ctx.RepoRoot()
				cmdOut, err := cmd.CombinedOutput()
				if err != nil {
					ctx.Errorf("build failed: %s", strings.TrimSpace(string(cmdOut)))
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
