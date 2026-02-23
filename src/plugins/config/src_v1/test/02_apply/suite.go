package apply

import (
	"fmt"
	"os"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(reg *testv1.Registry) {
	reg.Add(testv1.Step{
		Name: "apply-sets-runtime-vars",
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			rt, err := configv1.ResolveRuntime("")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := configv1.ApplyRuntimeEnv(rt); err != nil {
				return testv1.StepRunResult{}, err
			}
			if got := os.Getenv("DIALTONE_REPO_ROOT"); got == "" || got != rt.RepoRoot {
				return testv1.StepRunResult{}, fmt.Errorf("DIALTONE_REPO_ROOT mismatch: got=%q want=%q", got, rt.RepoRoot)
			}
			if got := os.Getenv("DIALTONE_ENV_FILE"); got == "" || got != rt.EnvFile {
				return testv1.StepRunResult{}, fmt.Errorf("DIALTONE_ENV_FILE mismatch: got=%q want=%q", got, rt.EnvFile)
			}
			return testv1.StepRunResult{Report: "runtime env applied"}, nil
		},
	})
}
