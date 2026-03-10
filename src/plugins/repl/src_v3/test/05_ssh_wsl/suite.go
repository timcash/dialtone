package sshwsl

import (
	"os"
	"strings"
	"time"

	"dialtone/dev/plugins/repl/src_v3/test/support"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name:    "injected-ssh-wsl-command",
		Timeout: 180 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			rt, err := support.NewRuntime(ctx)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer rt.Stop()
			if err := rt.StartLeader(); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := rt.StartJoin("local-human"); err != nil {
				return testv1.StepRunResult{}, err
			}

			wslHost := strings.TrimSpace(os.Getenv("DIALTONE_REPL_V3_TEST_WSL_HOST"))
			if wslHost == "" {
				wslHost = "127.0.0.1"
			}
			wslUser := strings.TrimSpace(os.Getenv("DIALTONE_REPL_V3_TEST_WSL_USER"))
			if wslUser == "" {
				wslUser = "user"
			}

			if err := rt.Inject("llm-codex",
				"repl", "src_v3", "bootstrap", "--apply",
				"--wsl-host", wslHost,
				"--wsl-user", wslUser,
			); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := rt.WaitForPatterns(35*time.Second, []string{
				`/repl src_v3 bootstrap --apply`,
				`Subtone for repl src_v3 exited with code 0.`,
			}); err != nil {
				return testv1.StepRunResult{}, err
			}

			if err := rt.Inject("llm-codex",
				"ssh", "src_v1", "run", "--host", "wsl", "--cmd", "whoami",
			); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := rt.WaitForPatterns(90*time.Second, []string{
				`/ssh src_v1 run --host wsl --cmd whoami`,
				`Request received. Spawning subtone for ssh src_v1`,
				`Subtone for ssh src_v1 exited with code`,
			}); err != nil {
				return testv1.StepRunResult{}, err
			}

			ctx.TestPassf("ssh wsl command routed through repl subtone path")
			return testv1.StepRunResult{
				Report: "Injected ssh src_v1 run --host wsl --cmd whoami through REPL bus and verified subtone lifecycle output for ssh execution path.",
			}, nil
		},
	})
}
