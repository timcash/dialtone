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
					ExpectRoom: support.CombinePatterns([]string{
						fmt.Sprintf(`"message":"%s"`, cmd),
						fmt.Sprintf(`Verified mesh host %s persisted to %s`, hostName, filepath.Join(rt.RepoRoot, "env", "dialtone.json")),
					}, support.StandardSubtoneRoomPatterns("repl src_v3", "")),
					ExpectOutput: support.CombinePatterns([]string{
						`/repl src_v3 add-host`,
						fmt.Sprintf("Verified mesh host %s persisted to %s", hostName, filepath.Join(rt.RepoRoot, "env", "dialtone.json")),
					}, support.StandardSubtoneOutputPatterns("repl src_v3", "")),
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
				Report: "Joined REPL as llm-codex, typed /repl src_v3 add-host through the live prompt path, and verified the production add-host flow structurally persisted the mesh host to env/dialtone.json.",
			}, nil
		},
	})
}
