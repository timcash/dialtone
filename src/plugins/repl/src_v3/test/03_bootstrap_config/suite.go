package bootstrapconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dialtone/dev/plugins/repl/src_v3/test/support"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

type config struct {
	MeshNodes []struct {
		Name string `json:"name"`
		Host string `json:"host"`
		User string `json:"user"`
	} `json:"mesh_nodes"`
}

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name:    "bootstrap-apply-updates-dialtone-json",
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
			if err := rt.WaitForPatterns(40*time.Second, []string{
				`"type":"command","from":"llm-codex"`,
				fmt.Sprintf(`/repl src_v3 bootstrap --apply --wsl-host %s --wsl-user %s`, wslHost, wslUser),
				`Request received. Spawning subtone for repl src_v3`,
				`Subtone for repl src_v3 exited with code 0.`,
			}); err != nil {
				return testv1.StepRunResult{}, err
			}

			cfgPath := filepath.Join(rt.RepoRoot, "env", "dialtone.json")
			raw, err := os.ReadFile(cfgPath)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			var cfg config
			if err := json.Unmarshal(raw, &cfg); err != nil {
				return testv1.StepRunResult{}, err
			}
			found := false
			for _, n := range cfg.MeshNodes {
				if strings.EqualFold(strings.TrimSpace(n.Name), "wsl") {
					found = true
					if strings.TrimSpace(n.Host) != wslHost {
						return testv1.StepRunResult{}, fmt.Errorf("wsl host mismatch: got %q", n.Host)
					}
					if strings.TrimSpace(n.User) != wslUser {
						return testv1.StepRunResult{}, fmt.Errorf("wsl user mismatch: got %q", n.User)
					}
					break
				}
			}
			if !found {
				return testv1.StepRunResult{}, fmt.Errorf("wsl mesh node was not written to %s", cfgPath)
			}

			ctx.TestPassf("bootstrap apply wrote wsl mesh node to env/dialtone.json")
			return testv1.StepRunResult{
				Report: "Injected repl src_v3 bootstrap --apply command over NATS and verified env/dialtone.json mesh_nodes includes a wsl host entry.",
			}, nil
		},
	})
}
