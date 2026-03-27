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
			if err := rt.StartJoin(""); err != nil {
				return testv1.StepRunResult{}, err
			}

			fixture := support.ResolveSSHFixture()
			hostName := fixture.Alias
			hostAddr := fixture.Host
			hostUser := fixture.User

			cmd := fmt.Sprintf("/repl src_v3 add-host --name %s --host %s --user %s", hostName, hostAddr, hostUser)
			if err := rt.RunTranscript([]support.TranscriptStep{
				{
					Send: cmd,
					ExpectRoomAny: support.CombinePatternGroups(
						[]string{fmt.Sprintf(`"message":"%s"`, cmd)},
						support.StandardCommandRoomPatternGroups("repl src_v3", "", "")...,
					),
					ExpectOutputAny: support.CombinePatternGroups(
						[]string{support.PromptLine(cmd)},
						support.StandardCommandOutputPatternGroups("repl src_v3", "", "")...,
					),
					Timeout: 40 * time.Second,
				},
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			cfgPath := filepath.Join(rt.RepoRoot, "env", "dialtone.json")
			if err := verifyMeshHostPersisted(cfgPath, hostName, hostAddr, hostUser); err != nil {
				return testv1.StepRunResult{}, err
			}

			ctx.TestPassf("interactive add-host wrote %s mesh node to env/dialtone.json", hostName)
			return testv1.StepRunResult{
				Report: "Joined REPL with the default hostname prompt, ran the add-host prompt flow through the live REPL path, verified the routed command lifecycle at the top level, and confirmed the mesh host entry was persisted into env/dialtone.json with the expected name, host, and user.",
			}, nil
		},
	})
}

func verifyMeshHostPersisted(cfgPath string, expectedName string, expectedHost string, expectedUser string) error {
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		return fmt.Errorf("expected config file %s after add-host: %w", cfgPath, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("parse %s after add-host: %w", cfgPath, err)
	}
	nodes, _ := doc["mesh_nodes"].([]any)
	for _, rawNode := range nodes {
		node, _ := rawNode.(map[string]any)
		if !strings.EqualFold(strings.TrimSpace(asString(node["name"])), strings.TrimSpace(expectedName)) {
			continue
		}
		if strings.TrimSpace(asString(node["host"])) != strings.TrimSpace(expectedHost) {
			return fmt.Errorf("mesh host %s persisted with unexpected host %q", expectedName, asString(node["host"]))
		}
		if strings.TrimSpace(asString(node["user"])) != strings.TrimSpace(expectedUser) {
			return fmt.Errorf("mesh host %s persisted with unexpected user %q", expectedName, asString(node["user"]))
		}
		return nil
	}
	return fmt.Errorf("mesh host %s not present in %s after add-host", expectedName, cfgPath)
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
