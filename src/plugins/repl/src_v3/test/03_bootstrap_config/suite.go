package bootstrapconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dialtone/dev/plugins/repl/src_v3/test/support"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name:    "interactive-add-host-updates-dialtone-json",
		Timeout: 150 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			rt, err := support.NewRuntime(ctx)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer rt.Stop()
			if err := rt.StartLeader(); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := rt.StartJoin("llm-codex"); err != nil {
				return testv1.StepRunResult{}, err
			}

			hostName := "wsl"
			hostAddr := strings.TrimSpace(os.Getenv("DIALTONE_REPL_V3_TEST_WSL_HOST"))
			if hostAddr == "" {
				hostAddr = "127.0.0.1"
			}
			hostUser := strings.TrimSpace(os.Getenv("DIALTONE_REPL_V3_TEST_WSL_USER"))
			if hostUser == "" {
				hostUser = "user"
			}

			cmd := fmt.Sprintf("/repl src_v3 add-host --name %s --host %s --user %s", hostName, hostAddr, hostUser)
			if err := rt.RunTranscript([]support.TranscriptStep{
				{
					Send: cmd,
					ExpectRoom: []string{
						fmt.Sprintf(`"message":"%s"`, cmd),
						`"scope":"index"`,
						`Subtone started as pid`,
						`Subtone room: subtone-`,
						`Subtone log file: `,
						fmt.Sprintf(`Verified mesh host %s persisted to %s`, hostName, filepath.Join(rt.RepoRoot, "env", "dialtone.json")),
						`"scope":"subtone"`,
						`Subtone for repl src_v3 exited with code 0.`,
					},
					ExpectOutput: []string{
						`DIALTONE> Request received. Spawning subtone for repl src_v3`,
						`DIALTONE> Subtone started as pid `,
						`DIALTONE> Subtone room: subtone-`,
						`DIALTONE> Subtone log file: `,
						`DIALTONE> Subtone for repl src_v3 exited with code 0.`,
					},
					Timeout: 40 * time.Second,
				},
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			cfgPath := filepath.Join(rt.RepoRoot, "env", "dialtone.json")
			if _, err := os.Stat(cfgPath); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("expected config file %s after add-host: %w", cfgPath, err)
			}

			ctx.TestPassf("interactive add-host wrote %s mesh node to env/dialtone.json", hostName)
			return testv1.StepRunResult{
				Report: "Joined REPL as llm-codex, ran the add-host prompt flow through the live REPL path, and verified the production add-host flow structurally persisted the mesh host to env/dialtone.json.",
			}, nil
		},
	})
}
