package manifestcontract

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(reg *testv1.Registry) {
	reg.Add(testv1.Step{
		Name:    "03-manifest-has-required-sync-artifacts",
		Timeout: 10 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			if err := ctx.WaitForStepMessageAfterAction("manifest contains required artifact keys", 5*time.Second, func() error {
				repo := ctx.RepoRoot()
				manifestPath := filepath.Join(repo, "src", "plugins", "robot", "src_v2", "config", "composition.manifest.json")
				data, err := os.ReadFile(manifestPath)
				if err != nil {
					ctx.Errorf("manifest read failed: %v", err)
					return err
				}
				body := string(data)
				required := []string{
					"\"autoswap\"",
					"\"robot\"",
					"\"repl\"",
					"\"camera\"",
					"\"mavlink\"",
					"\"wlan\"",
					"\"ui_dist\"",
					"dialtone_robot_v2",
				}
				for _, token := range required {
					if !strings.Contains(body, token) {
						ctx.Errorf("manifest missing token %s", token)
						return fmt.Errorf("manifest missing token %s", token)
					}
				}
				ctx.Infof("manifest contains required artifact keys")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "manifest sync artifact contract verified"}, nil
		},
	})
}
