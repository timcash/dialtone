package selfcheck

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
	tsnetv1 "dialtone/dev/plugins/tsnet/src_v1/go"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name: "normalize-hostname",
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			got := tsnetv1.NormalizeHostname("Robot_1 Dev")
			if got != "robot-1-dev" {
				return testv1.StepRunResult{}, fmt.Errorf("expected robot-1-dev, got %s", got)
			}
			if err := ctx.WaitForStepMessageAfterAction("normalize-hostname-ok", 3*time.Second, func() error {
				ctx.Infof("normalize-hostname-ok")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "hostname normalization pass"}, nil
		},
	})

	r.Add(testv1.Step{
		Name: "resolve-config-defaults",
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			cfg, err := tsnetv1.ResolveConfig("", "")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if cfg.Hostname == "" || cfg.StateDir == "" {
				return testv1.StepRunResult{}, fmt.Errorf("empty config fields")
			}
			if err := ctx.WaitForStepMessageAfterAction("resolve-config-defaults-ok", 3*time.Second, func() error {
				ctx.Infof("resolve-config-defaults-ok")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "config defaults pass"}, nil
		},
	})

	r.Add(testv1.Step{
		Name: "build-create-key-request",
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			req := tsnetv1.BuildCreateKeyRequest(tsnetv1.ProvisionOptions{
				Description:   "demo-key",
				Tags:          []string{"robot", "tag:ops"},
				Reusable:      true,
				Ephemeral:     false,
				Preauthorized: true,
				ExpiryHours:   12,
			})
			gotDesc, _ := req["description"].(string)
			if gotDesc != "demo-key" {
				return testv1.StepRunResult{}, fmt.Errorf("description mismatch: %s", gotDesc)
			}
			gotExp, _ := req["expirySeconds"].(int)
			if gotExp != 43200 {
				return testv1.StepRunResult{}, fmt.Errorf("expirySeconds mismatch: %d", gotExp)
			}
			create := req["capabilities"].(map[string]any)["devices"].(map[string]any)["create"].(map[string]any)
			tagsAny, _ := create["tags"].([]string)
			wantTags := []string{"tag:robot", "tag:ops"}
			if !reflect.DeepEqual(tagsAny, wantTags) {
				return testv1.StepRunResult{}, fmt.Errorf("tags mismatch: got=%v want=%v", tagsAny, wantTags)
			}
			if err := ctx.WaitForStepMessageAfterAction("build-create-key-request-ok", 3*time.Second, func() error {
				ctx.Infof("build-create-key-request-ok")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "create key request pass"}, nil
		},
	})

	r.Add(testv1.Step{
		Name: "upsert-env-var",
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			repoRoot, err := findRepoRoot()
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			tmp := filepath.Join(repoRoot, "src", "plugins", "tsnet", "src_v1", "test", "tmp.env")
			defer os.Remove(tmp)
			if err := os.WriteFile(tmp, []byte("A=1\nTS_AUTHKEY=old\n"), 0o644); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := tsnetv1.UpsertEnvVar(tmp, "TS_AUTHKEY", "new-value"); err != nil {
				return testv1.StepRunResult{}, err
			}
			raw, err := os.ReadFile(tmp)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			text := string(raw)
			if !strings.Contains(text, "TS_AUTHKEY=new-value") {
				return testv1.StepRunResult{}, fmt.Errorf("upsert failed")
			}
			if err := ctx.WaitForStepMessageAfterAction("upsert-env-var-ok", 3*time.Second, func() error {
				ctx.Infof("upsert-env-var-ok")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "env upsert pass"}, nil
		},
	})
}

func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(cwd, "dialtone.sh")); err == nil {
			return cwd, nil
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			return "", fmt.Errorf("repo root not found")
		}
		cwd = parent
	}
}
