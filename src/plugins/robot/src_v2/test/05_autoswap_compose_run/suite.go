package autoswapcomposerun

import (
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(reg *testv1.Registry) {
	reg.Add(testv1.Step{
		Name:    "05-autoswap-compose-run-smoke",
		Timeout: 50 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			if err := ctx.WaitForStepMessageAfterAction("autoswap compose run passed", 50*time.Second, func() error {
				repo := ctx.RepoRoot()
				manifest := filepath.Join(repo, "src", "plugins", "robot", "src_v2", "config", "composition.manifest.json")
				cmd := exec.Command(
					"./dialtone.sh", "autoswap", "src_v1", "run",
					"--manifest", manifest,
					"--repo-root", repo,
					"--listen", ":18086",
					"--nats-port", "18236",
					"--nats-ws-port", "18237",
					"--timeout", "35s",
				)
				cmd.Dir = repo
				out, err := cmd.CombinedOutput()
				if err != nil {
					ctx.Errorf("autoswap compose run failed: %s", strings.TrimSpace(string(out)))
					return err
				}
				ctx.Infof("autoswap compose run passed")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "autoswap manifest composition run verified"}, nil
		},
	})
}
