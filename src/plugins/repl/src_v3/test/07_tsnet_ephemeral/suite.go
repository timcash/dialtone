package tsnetephemeral

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"dialtone/dev/plugins/repl/src_v3/test/support"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
	"github.com/nats-io/nats.go"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name:    "injected-tsnet-ephemeral-up",
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
			if err := rt.StartJoin("llm-codex"); err != nil {
				return testv1.StepRunResult{}, err
			}

			// Send a probe to the leader so it announces its tsnet status.
			nc, err := nats.Connect(rt.NATSURL, nats.Timeout(1200*time.Millisecond))
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer nc.Close()
			probe := map[string]string{
				"type":    "probe",
				"from":    "repl-src-v3-test",
				"room":    rt.Room,
				"message": "probe",
			}
			rawProbe, _ := json.Marshal(probe)
			_ = nc.Publish("repl.cmd", rawProbe)
			_ = nc.Flush()

			matched, err := rt.WaitForAnyPattern(120*time.Second, []string{
				`DIALTONE tsnet NATS endpoint: nats://`,
				`tsnet NATS endpoint: nats://`,
				`Native tailscale already connected`,
			})
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			requireEmbedded := strings.TrimSpace(os.Getenv("DIALTONE_REPL_V3_TEST_REQUIRE_EMBEDDED_TSNET")) == "1"
			if strings.Contains(matched, "native tailscale already connected") || strings.Contains(matched, "Native tailscale already connected") {
				if requireEmbedded {
					return testv1.StepRunResult{}, fmt.Errorf("native tailscale detected, but embedded tsnet was required")
				}
				ctx.TestPassf("detected native tailscale; embedded tsnet fallback correctly skipped for llm-codex session")
				return testv1.StepRunResult{
					Report: "Detected native tailscale and verified REPL leader published explicit skip signal for embedded tsnet fallback.",
				}, nil
			}

			ctx.TestPassf("embedded tsnet endpoint announced by REPL leader for llm-codex session")
			return testv1.StepRunResult{
				Report: "Verified REPL leader published embedded tsnet NATS endpoint when native tailscale was absent, or published the explicit native-tailscale skip signal otherwise.",
			}, nil
		},
	})
}
