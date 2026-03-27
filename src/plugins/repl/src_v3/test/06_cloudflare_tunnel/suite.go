package cloudflaretunnel

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
				tunnelName = fmt.Sprintf("repl-src-v3-test-%d", time.Now().Unix())
			}
			tunnelURL := strings.TrimSpace(os.Getenv("DIALTONE_REPL_V3_TEST_TUNNEL_URL"))
			if tunnelURL == "" {
				tunnelURL = "http://127.0.0.1:8080"
			}
			domain := strings.TrimSpace(os.Getenv("DIALTONE_DOMAIN"))
			if domain == "" {
				return testv1.StepRunResult{Report: "skipped cloudflare tunnel test (DIALTONE_DOMAIN not configured on this host)"}, nil
			}
			if !hasCloudflareProvisioningConfig() {
				return testv1.StepRunResult{Report: "skipped cloudflare tunnel test (Cloudflare provisioning credentials not configured on this host)"}, nil
			}
			installPath := filepath.Join(strings.TrimSpace(os.Getenv("DIALTONE_ENV")), "cloudflare", "cloudflared")

			defer rt.Stop()
			if err := rt.StartLeader(); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := rt.StartJoin("llm-codex"); err != nil {
				return testv1.StepRunResult{}, err
			}

			installCmd := "/cloudflare src_v1 install"
			if err := rt.RunTranscript([]support.TranscriptStep{
				{
					Send: installCmd,
					ExpectRoomAny: support.CombinePatternGroups(
						[]string{
							fmt.Sprintf(`"message":"%s"`, installCmd),
							`installed cloudflared at `,
						},
						support.StandardCommandRoomPatternGroups("cloudflare src_v1", "", "")...,
					),
					ExpectOutputAny: support.CombinePatternGroups(
						[]string{fmt.Sprintf("llm-codex> %s", installCmd)},
						support.StandardCommandOutputPatternGroups("cloudflare src_v1", "", "")...,
					),
					Timeout: 90 * time.Second,
				},
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			if _, err := os.Stat(installPath); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("expected cloudflared install at %s: %w", installPath, err)
			}

			provisionCmd := fmt.Sprintf("/cloudflare src_v1 provision %s --domain %s", tunnelName, domain)
			if err := rt.RunTranscript([]support.TranscriptStep{
				{
					Send: provisionCmd,
					ExpectRoomAny: support.CombinePatternGroups(
						[]string{
							fmt.Sprintf(`"message":"%s"`, provisionCmd),
							`hostname`,
							`tunnel_id`,
						},
						support.StandardCommandRoomPatternGroups("cloudflare src_v1", "", "")...,
					),
					ExpectOutputAny: support.CombinePatternGroups(
						[]string{fmt.Sprintf("llm-codex> %s", provisionCmd)},
						support.StandardCommandOutputPatternGroups("cloudflare src_v1", "", "")...,
					),
					Timeout: 40 * time.Second,
				},
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			tokenKey := fmt.Sprintf("CF_TUNNEL_TOKEN_%s", strings.ToUpper(strings.ReplaceAll(tunnelName, "-", "_")))
			if value := strings.TrimSpace(readConfigString(filepath.Join(rt.RepoRoot, "env", "dialtone.json"), tokenKey)); value == "" {
				return testv1.StepRunResult{}, fmt.Errorf("expected provisioned token %s in env/dialtone.json", tokenKey)
			}

			replCmd := fmt.Sprintf("/cloudflare src_v1 tunnel start %s --url %s", tunnelName, tunnelURL)
			if err := rt.RunTranscript([]support.TranscriptStep{
				{
					Send: replCmd,
					ExpectRoomAny: support.CombinePatternGroups(
						[]string{
							fmt.Sprintf(`"message":"%s"`, replCmd),
							`cloudflared started pid=`,
							`cloudflared confirmed tunnel connection in background pid=`,
						},
						support.StandardCommandRoomPatternGroups("cloudflare src_v1", "", "")...,
					),
					ExpectOutputAny: support.CombinePatternGroups(
						[]string{fmt.Sprintf("llm-codex> %s", replCmd)},
						support.StandardCommandOutputPatternGroups("cloudflare src_v1", "", "")...,
					),
					Timeout: 40 * time.Second,
				},
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			stopCmd := "/cloudflare src_v1 tunnel stop"
			if err := rt.RunTranscript([]support.TranscriptStep{
				{
					Send: stopCmd,
					ExpectRoomAny: support.CombinePatternGroups(
						[]string{fmt.Sprintf(`"message":"%s"`, stopCmd)},
						support.StandardCommandRoomPatternGroups("cloudflare src_v1", "", "")...,
					),
					ExpectOutputAny: support.CombinePatternGroups(
						[]string{fmt.Sprintf("llm-codex> %s", stopCmd)},
						support.StandardCommandOutputPatternGroups("cloudflare src_v1", "", "")...,
					),
					Timeout: 40 * time.Second,
				},
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			cleanupCmd := fmt.Sprintf("/cloudflare src_v1 tunnel cleanup --name %s --domain %s", tunnelName, domain)
			if err := rt.RunTranscript([]support.TranscriptStep{
				{
					Send: cleanupCmd,
					ExpectRoomAny: support.CombinePatternGroups(
						[]string{
							fmt.Sprintf(`"message":"%s"`, cleanupCmd),
							`cloudflare cleanup verified dns hostname=`,
							`cloudflare cleanup verified connections tunnel_id=`,
							`cloudflare cleanup verified tunnel tunnel_id=`,
							`cloudflare cleanup verified token env=`,
							`token_removed`,
						},
						support.StandardCommandRoomPatternGroups("cloudflare src_v1", "", "")...,
					),
					ExpectOutputAny: support.CombinePatternGroups(
						[]string{fmt.Sprintf("llm-codex> %s", cleanupCmd)},
						support.StandardCommandOutputPatternGroups("cloudflare src_v1", "", "")...,
					),
					Timeout: 40 * time.Second,
				},
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			if value := strings.TrimSpace(readConfigString(filepath.Join(rt.RepoRoot, "env", "dialtone.json"), tokenKey)); value != "" {
				return testv1.StepRunResult{}, fmt.Errorf("expected cleanup to remove token %s from env/dialtone.json", tokenKey)
			}

			ctx.TestPassf("cloudflare tunnel start executed through llm-codex REPL prompt path")
			return testv1.StepRunResult{
				Report: "Joined REPL as llm-codex, ran /cloudflare src_v1 install to provision the managed cloudflared binary, used /cloudflare src_v1 provision to create a real tunnel and persist its token, started and stopped the live tunnel through REPL, then cleaned up the Cloudflare resources and removed the stored token.",
			}, nil
		},
	})
}

func hasCloudflareProvisioningConfig() bool {
	apiToken := strings.TrimSpace(os.Getenv("CLOUDFLARE_API_TOKEN"))
	accountID := strings.TrimSpace(os.Getenv("CLOUDFLARE_ACCOUNT_ID"))
	return apiToken != "" && accountID != ""
}

func readConfigString(path string, key string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ""
	}
	if v, ok := doc[key]; ok {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}
