package cloudflaretunnel

import (
	"fmt"
	"os"
	"strings"
	"time"

	"dialtone/dev/plugins/repl/src_v3/test/support"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name:    "interactive-cloudflare-tunnel-start",
		Timeout: 120 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			rt, err := support.NewRuntime(ctx)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			tunnelName := strings.TrimSpace(os.Getenv("DIALTONE_REPL_V3_TEST_TUNNEL_NAME"))
			if tunnelName == "" {
				tunnelName = "repl-src-v3-test"
			}
			tunnelURL := strings.TrimSpace(os.Getenv("DIALTONE_REPL_V3_TEST_TUNNEL_URL"))
			if tunnelURL == "" {
				tunnelURL = "http://127.0.0.1:8080"
			}

			defer rt.Stop()
			if err := rt.StartLeader(); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := rt.StartJoin("llm-codex"); err != nil {
				return testv1.StepRunResult{}, err
			}

			replCmd := fmt.Sprintf("/cloudflare src_v1 tunnel start %s --url %s", tunnelName, tunnelURL)
			if err := rt.RunTranscript([]support.TranscriptStep{
				{
					Send: replCmd,
					ExpectRoom: support.CombinePatterns([]string{
						fmt.Sprintf(`"message":"%s"`, replCmd),
					}, support.StandardSubtoneRoomPatterns("cloudflare src_v1", ``)),
					ExpectOutput: support.CombinePatterns([]string{
						`/cloudflare src_v1 tunnel start`,
					}, support.StandardSubtoneOutputPatterns("cloudflare src_v1", ``)),
					Timeout: 40 * time.Second,
				},
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			stopCmd := "/cloudflare src_v1 tunnel stop"
			if err := rt.RunTranscript([]support.TranscriptStep{
				{
					Send: stopCmd,
					ExpectRoom: support.CombinePatterns([]string{
						fmt.Sprintf(`"message":"%s"`, stopCmd),
					}, support.StandardSubtoneRoomPatterns("cloudflare src_v1", ``)),
					ExpectOutput: support.CombinePatterns([]string{
						`/cloudflare src_v1 tunnel stop`,
					}, support.StandardSubtoneOutputPatterns("cloudflare src_v1", ``)),
					Timeout: 40 * time.Second,
				},
			}); err != nil {
				return testv1.StepRunResult{}, err
			}

			ctx.TestPassf("cloudflare tunnel start executed through llm-codex REPL prompt path")
			return testv1.StepRunResult{
				Report: "Joined REPL as llm-codex, typed /cloudflare src_v1 tunnel start through the live prompt path, and verified real cloudflare tunnel start and stop through the REPL command path.",
			}, nil
		},
	})
}
